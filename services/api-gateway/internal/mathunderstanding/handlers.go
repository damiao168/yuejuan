package mathunderstanding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/review"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

type Handler struct {
	artifacts   Store
	corrections CorrectionStore
	pilotGates  PilotGateStore
	reviews     review.Store
	audit       auth.Store
	runtime     workerruntime.Store
}

func NewHandler(artifacts Store, corrections CorrectionStore, pilotGates PilotGateStore, reviews review.Store, audit auth.Store) *Handler {
	return &Handler{artifacts: artifacts, corrections: corrections, pilotGates: pilotGates, reviews: reviews, audit: audit}
}

func (h *Handler) WithRuntime(runtime workerruntime.Store) *Handler {
	h.runtime = runtime
	return h
}

func RegisterRoutes(mux *http.ServeMux, handler *Handler, requireWork, requireManage func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /api/v1/math-answer-segments/{segmentId}/understanding", requireWork(handler.GetLatest))
	mux.Handle("GET /api/v1/math-understanding/{artifactId}/corrections", requireWork(handler.ListCorrections))
	mux.Handle("POST /api/v1/math-understanding/{artifactId}/corrections", requireWork(handler.CreateCorrection))
	mux.Handle("GET /api/v1/math-understanding/training-export", requireManage(handler.ExportTraining))
	mux.Handle("GET /api/v1/math-pilot-gates", requireManage(handler.ListPilotGates))
	mux.Handle("POST /api/v1/math-pilot-gates/evaluate", requireManage(handler.EvaluatePilotGate))
}

func RegisterRuntimeRoutes(mux *http.ServeMux, handler *Handler, requireWorker func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /api/v1/internal/math-understanding/tasks/{taskId}/input", requireWorker(handler.GetRuntimeInput))
	mux.Handle("POST /api/v1/internal/math-understanding/tasks/{taskId}/complete", requireWorker(handler.CompleteRuntimeTask))
}

type completeRuntimeRequest struct {
	LeaseToken string              `json:"lease_token"`
	DurationMS int                 `json:"duration_ms"`
	Artifact   CreateArtifactInput `json:"artifact"`
}

const runtimeSolutionBuilderVersion = "math-runtime-spatial-baseline-v1"

func (h *Handler) GetRuntimeInput(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok || h.runtime == nil {
		return
	}
	task, err := h.runtime.Get(r.Context(), user.TenantID, r.PathValue("taskId"))
	if err != nil || task.QueueName != "math-understanding" || task.SourceType != "answer_segment" || task.SourceID == "" {
		httpx.Error(w, r, http.StatusNotFound, "math_runtime_task_not_found", "math understanding task was not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"task": map[string]any{
			"id": task.ID, "answer_segment_id": task.SourceID,
			"exam_question_snapshot_id": task.Payload["exam_question_snapshot_id"],
			"subject_code":              task.Payload["subject_code"], "region_kind": task.Payload["region_kind"],
			"input_hash": task.Payload["input_hash"],
		},
		"image_url": "/api/v1/internal/answer-segments/" + task.SourceID + "/image",
	})
}

func (h *Handler) CompleteRuntimeTask(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok || h.runtime == nil {
		return
	}
	var input completeRuntimeRequest
	if !decodeMathJSON(w, r, &input) {
		return
	}
	task, err := h.runtime.Get(r.Context(), user.TenantID, r.PathValue("taskId"))
	if err != nil || task.QueueName != "math-understanding" || task.SourceType != "answer_segment" {
		httpx.Error(w, r, http.StatusNotFound, "math_runtime_task_not_found", "math understanding task was not found")
		return
	}
	if strings.TrimSpace(input.LeaseToken) == "" || input.DurationMS < 0 || input.Artifact.AnswerSegmentID != task.SourceID ||
		input.Artifact.ExamQuestionSnapshotID != stringPayload(task.Payload, "exam_question_snapshot_id") ||
		input.Artifact.SubjectCode != stringPayload(task.Payload, "subject_code") ||
		input.Artifact.InputHash != stringPayload(task.Payload, "input_hash") {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_math_runtime_result", "math runtime result does not match its immutable task input")
		return
	}
	preparedArtifact, err := prepareRuntimeArtifact(input.Artifact)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	artifact, err := h.artifacts.CreateArtifact(r.Context(), user.TenantID, preparedArtifact)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	_, err = h.runtime.Complete(r.Context(), user.TenantID, task.ID, workerruntime.CompleteInput{
		LeaseToken: input.LeaseToken, ResultSchemaVersion: "math-understanding-result-v1",
		Result:     map[string]any{"artifact_id": artifact.ID, "artifact_version": artifact.Version, "input_hash": artifact.InputHash},
		DurationMS: input.DurationMS,
	})
	if err != nil {
		httpx.Error(w, r, http.StatusConflict, "math_runtime_completion_failed", "math runtime lease is no longer valid")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"artifact": artifact})
}

func prepareRuntimeArtifact(input CreateArtifactInput) (CreateArtifactInput, error) {
	relations := input.Relations
	if len(relations) == 0 {
		relations = BuildSpatialRelations(input.Blocks, SpatialGraphOptions{})
	}

	formulaModelVersion := "text-only"
	if len(input.Formulas) > 0 {
		formulaModelVersion = strings.TrimSpace(input.SolutionGraph.FormulaModelVersion)
		for index := 0; formulaModelVersion == "" && index < len(input.Formulas); index++ {
			formulaModelVersion = strings.TrimSpace(input.Formulas[index].RecognitionVersion)
		}
	}

	graph, err := BuildSolutionGraph(SolutionBuildInput{
		AnswerSegmentID:     input.AnswerSegmentID,
		Blocks:              input.Blocks,
		Formulas:            input.Formulas,
		Relations:           relations,
		Verifications:       input.Verifications,
		BuilderVersion:      runtimeSolutionBuilderVersion,
		FormulaModelVersion: formulaModelVersion,
	})
	if err != nil {
		return CreateArtifactInput{}, err
	}
	graph.RequiresHumanReview = graph.RequiresHumanReview || input.SolutionGraph.RequiresHumanReview

	input.Relations = relations
	input.SolutionGraph = graph
	return input, nil
}

func stringPayload(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

type evaluatePilotGateRequest struct {
	SubjectCode  string       `json:"subject_code"`
	BenchmarkRef string       `json:"benchmark_ref"`
	Metrics      PilotMetrics `json:"metrics"`
	Policy       PilotPolicy  `json:"policy"`
}

func (h *Handler) EvaluatePilotGate(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok {
		return
	}
	var input evaluatePilotGateRequest
	if !decodeMathJSON(w, r, &input) {
		return
	}
	item, err := h.pilotGates.CreatePilotGate(r.Context(), user.TenantID, input.SubjectCode, input.BenchmarkRef, input.Metrics, input.Policy, user.ID)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	h.auditEvent(r, user, "math_understanding.pilot_gate_evaluated", item.ID, map[string]any{"subject_code": item.SubjectCode, "passed": item.Decision.Passed, "scope": item.Decision.Scope})
	httpx.JSON(w, http.StatusCreated, map[string]any{"evaluation": item})
}

func (h *Handler) ListPilotGates(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok {
		return
	}
	subject := r.URL.Query().Get("subject")
	if subject != "" && !set("mathematics", "physics", "chemistry")[subject] {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_subject", "math pilot gates require an enabled subject")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.pilotGates.ListPilotGates(r.Context(), user.TenantID, subject, limit)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"evaluations": items})
}

func (h *Handler) GetLatest(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok {
		return
	}
	item, err := h.artifacts.GetLatestArtifact(r.Context(), user.TenantID, r.PathValue("segmentId"))
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	if !h.canAccess(r, user, item.AnswerSegmentID) {
		httpx.Error(w, r, http.StatusForbidden, "math_evidence_forbidden", "math evidence is limited to assigned review work")
		return
	}
	corrections, err := h.corrections.ListCorrections(r.Context(), user.TenantID, item.ID)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"artifact": item, "corrections": corrections})
}
func (h *Handler) ListCorrections(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok {
		return
	}
	artifact, err := h.artifacts.GetArtifact(r.Context(), user.TenantID, r.PathValue("artifactId"))
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	if !h.canAccess(r, user, artifact.AnswerSegmentID) {
		httpx.Error(w, r, http.StatusForbidden, "math_evidence_forbidden", "math evidence is limited to assigned review work")
		return
	}
	items, err := h.corrections.ListCorrections(r.Context(), user.TenantID, artifact.ID)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"corrections": items})
}
func (h *Handler) CreateCorrection(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok {
		return
	}
	artifact, err := h.artifacts.GetArtifact(r.Context(), user.TenantID, r.PathValue("artifactId"))
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	if !h.canAccess(r, user, artifact.AnswerSegmentID) {
		httpx.Error(w, r, http.StatusForbidden, "math_correction_forbidden", "only the assigned reviewer or quality manager may correct math evidence")
		return
	}
	var input CreateCorrectionInput
	if !decodeMathJSON(w, r, &input) {
		return
	}
	item, err := h.corrections.CreateCorrection(r.Context(), user.TenantID, artifact.ID, user.ID, input)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	h.auditEvent(r, user, "math_understanding.corrected", artifact.ID, map[string]any{"revision": item.Revision, "operation_count": len(item.Operations)})
	httpx.JSON(w, http.StatusCreated, map[string]any{"correction": item})
}
func (h *Handler) ExportTraining(w http.ResponseWriter, r *http.Request) {
	user, ok := currentMathUser(w, r)
	if !ok {
		return
	}
	subject := r.URL.Query().Get("subject")
	if !set("mathematics", "physics", "chemistry")[subject] {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_subject", "math training export requires an enabled subject")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.corrections.ExportCorrections(r.Context(), user.TenantID, subject, limit)
	if err != nil {
		writeMathError(w, r, err)
		return
	}
	safe := make([]map[string]any, 0, len(items))
	for _, item := range items {
		contract := cloneInput(item.CorrectedContract)
		contract.AnswerSegmentID = ""
		contract.ExamQuestionSnapshotID = ""
		digest := sha256.Sum256([]byte(user.TenantID + ":" + item.AnswerSegmentID))
		safe = append(safe, map[string]any{"sample_key": hex.EncodeToString(digest[:]), "subject_code": subject, "operations": item.Operations, "corrected_contract": contract, "created_at": item.CreatedAt})
	}
	h.auditEvent(r, user, "math_understanding.training_exported", subject, map[string]any{"count": len(safe)})
	httpx.JSON(w, http.StatusOK, map[string]any{"samples": safe})
}

func (h *Handler) canAccess(r *http.Request, user auth.User, segmentID string) bool {
	for _, permission := range user.Permissions {
		if permission == "review:manage" {
			return true
		}
	}
	tasks, err := h.reviews.ListTasks(r.Context(), user.TenantID, review.ListFilter{AssignedTo: user.ID, Limit: 500})
	if err != nil {
		return false
	}
	for _, task := range tasks {
		if task.AnswerSegmentID == segmentID && task.AssignedTo == user.ID && task.Status != "completed" && task.Status != "cancelled" {
			return true
		}
	}
	return false
}
func (h *Handler) auditEvent(r *http.Request, user auth.User, action, targetID string, after map[string]any) {
	if h.audit == nil {
		return
	}
	auth.RecordAudit(r.Context(), h.audit, auth.AuditEvent{TenantID: user.TenantID, ActorID: user.ID, Action: action, TargetType: "math_understanding", TargetID: targetID, AfterValue: after, Reason: action, IPAddress: r.RemoteAddr, UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context())})
}
func currentMathUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
	}
	return user, ok
}
func decodeMathJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_json", "request body must use the documented math evidence contract")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return false
	}
	return true
}
func writeMathError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "math_understanding_not_found", "math understanding artifact was not found")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_math_understanding", "math understanding input is invalid")
	case errors.Is(err, ErrRevisionConflict):
		httpx.Error(w, r, http.StatusConflict, "math_understanding_revision_conflict", "math understanding changed; reload before correcting")
	case errors.Is(err, ErrInvalidPilotGate):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_math_pilot_gate", "math pilot gate evidence or policy is invalid")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "math_understanding_failed", "math understanding operation failed")
	}
}
