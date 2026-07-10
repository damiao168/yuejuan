package ocr

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	submissionpkg "edugrade-enterprise/services/api-gateway/internal/submission"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

type Handler struct {
	store       Store
	queue       Queue
	submissions submissionpkg.Store
	audit       auth.Store
	runtime     workerruntime.Store
}

func NewHandler(store Store, queue Queue, submissions submissionpkg.Store, audit auth.Store, runtimes ...workerruntime.Store) *Handler {
	handler := &Handler{store: store, queue: queue, submissions: submissions, audit: audit}
	if len(runtimes) > 0 {
		handler.runtime = runtimes[0]
	}
	return handler
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	submissionID := r.PathValue("id")
	sub, err := h.submissions.Get(r.Context(), user.TenantID, submissionID)
	if err != nil {
		writeStoreError(w, r, ErrSubmissionNotReady)
		return
	}
	if sub.Status != "ready_for_ocr" {
		writeStoreError(w, r, ErrSubmissionNotReady)
		return
	}
	var input CreateTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := h.store.CreateTask(r.Context(), user.TenantID, submissionID, user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if err := h.queue.Enqueue(r.Context(), task); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "ocr_enqueue_failed", "failed to enqueue ocr task")
		return
	}
	if h.runtime != nil {
		_, err = h.runtime.CreateTask(r.Context(), user.TenantID, user.ID, workerruntime.CreateTaskInput{
			TaskType: "ocr", QueueName: "ocr", SourceType: "ocr_task", SourceID: task.ID,
			IdempotencyKey: "ocr-task:" + task.ID, PayloadSchemaVersion: "ocr-task.v1",
			Payload:     map[string]any{"ocr_task_id": task.ID, "submission_id": task.SubmissionID, "engine": task.Engine, "engine_version": task.EngineVersion},
			MaxAttempts: 3, RetryBackoffSeconds: 30,
		})
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
	}
	h.auditAction(r, "ocr.task_created", "ocr_task", task.ID, "create ocr task")
	httpx.JSON(w, http.StatusCreated, map[string]any{"task": task})
}

func (h *Handler) ListBySubmission(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListBySubmission(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tasks": out})
}

func (h *Handler) ListPending(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 10
	}
	out, err := h.store.ListPending(r.Context(), user.TenantID, limit)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tasks": out})
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	task, err := h.store.GetTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func (h *Handler) GetTaskInput(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	task, err := h.store.GetTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	pages, err := h.submissions.ListPages(r.Context(), user.TenantID, task.SubmissionID)
	if err != nil {
		writeStoreError(w, r, ErrInvalidInput)
		return
	}
	type pageInput struct {
		ID          string `json:"id"`
		FileAssetID string `json:"file_asset_id"`
		PageNo      int    `json:"page_no"`
		Status      string `json:"status"`
		DownloadURL string `json:"download_url"`
	}
	pageInputs := make([]pageInput, 0, len(pages))
	for _, page := range pages {
		pageInputs = append(pageInputs, pageInput{
			ID:          page.ID,
			FileAssetID: page.FileAssetID,
			PageNo:      page.PageNo,
			Status:      page.Status,
			DownloadURL: "/api/v1/files/" + page.FileAssetID + "/download",
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"task": map[string]any{
			"id":             task.ID,
			"tenant_id":      task.TenantID,
			"submission_id":  task.SubmissionID,
			"engine":         task.Engine,
			"engine_version": task.EngineVersion,
			"min_confidence": task.MinConfidence,
		},
		"pages": pageInputs,
	})
}

func (h *Handler) StartTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	task, err := h.store.StartTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "ocr.task_started", "ocr_task", task.ID, "start ocr task")
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func (h *Handler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input CompleteTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := h.store.GetTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	pages, err := h.submissions.ListPages(r.Context(), user.TenantID, task.SubmissionID)
	if err != nil {
		writeStoreError(w, r, ErrInvalidInput)
		return
	}
	allowedPages := map[string]bool{}
	for _, page := range pages {
		allowedPages[page.ID] = true
	}
	for _, result := range input.Results {
		if !allowedPages[strings.TrimSpace(result.SubmissionPageID)] {
			writeStoreError(w, r, ErrInvalidInput)
			return
		}
	}
	var out Task
	if task.Status == "completed" {
		if !sameCompletion(task, input) {
			writeStoreError(w, r, ErrResultConflict)
			return
		}
		out = task
	} else {
		out, err = h.store.CompleteTask(r.Context(), user.TenantID, task.ID, input)
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
	}
	if h.runtime != nil && input.RuntimeTaskID != "" {
		runtimeTask, runtimeErr := h.runtime.Get(r.Context(), user.TenantID, input.RuntimeTaskID)
		if runtimeErr != nil || runtimeTask.SourceType != "ocr_task" || runtimeTask.SourceID != out.ID {
			writeStoreError(w, r, workerruntime.ErrInvalidInput)
			return
		}
		_, runtimeErr = h.runtime.Complete(r.Context(), user.TenantID, runtimeTask.ID, workerruntime.CompleteInput{
			LeaseToken: input.RuntimeLeaseToken, ResultSchemaVersion: "ocr-result.v1", DurationMS: input.DurationMS,
			Result: map[string]any{"ocr_task_id": out.ID, "result_count": out.ResultCount, "model_version": out.ModelVersion, "config_hash": out.ConfigHash, "input_hash": out.InputHash},
		})
		if runtimeErr != nil {
			writeStoreError(w, r, runtimeErr)
			return
		}
	}
	h.auditAction(r, "ocr.task_completed", "ocr_task", out.ID, "complete ocr task")
	httpx.JSON(w, http.StatusOK, map[string]any{"task": out})
}

func sameCompletion(task Task, input CompleteTaskInput) bool {
	if task.ModelVersion != input.ModelVersion || task.ConfigHash != input.ConfigHash || task.InputHash != input.InputHash ||
		task.DurationMS != input.DurationMS || task.WorkerID != input.WorkerID || len(task.Results) != len(input.Results) {
		return false
	}
	for index, current := range task.Results {
		candidate := input.Results[index]
		if current.SubmissionPageID != candidate.SubmissionPageID || current.Text != candidate.Text ||
			current.Confidence != candidate.Confidence || current.SourceImageFileID != candidate.SourceImageFileID ||
			current.PreprocessProfile != input.PreprocessProfile || len(current.BBox) != len(candidate.BBox) {
			return false
		}
		for bboxIndex := range current.BBox {
			if current.BBox[bboxIndex] != candidate.BBox[bboxIndex] {
				return false
			}
		}
	}
	return true
}

func (h *Handler) FailTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input FailTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ErrorMessage = strings.TrimSpace(input.ErrorMessage)
	if input.ErrorMessage == "" {
		writeStoreError(w, r, ErrInvalidInput)
		return
	}
	task, err := h.store.FailTask(r.Context(), user.TenantID, r.PathValue("id"), input.ErrorMessage)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if h.runtime != nil && input.RuntimeTaskID != "" {
		runtimeTask, runtimeErr := h.runtime.Get(r.Context(), user.TenantID, input.RuntimeTaskID)
		if runtimeErr != nil || runtimeTask.SourceType != "ocr_task" || runtimeTask.SourceID != task.ID {
			writeStoreError(w, r, workerruntime.ErrInvalidInput)
			return
		}
		_, runtimeErr = h.runtime.Fail(r.Context(), user.TenantID, runtimeTask.ID, workerruntime.FailInput{
			LeaseToken: input.RuntimeLeaseToken, Retryable: input.Retryable, ErrorCode: input.ErrorMessage,
			ErrorDetail: map[string]any{"source_type": "ocr_task"},
		})
		if runtimeErr != nil {
			writeStoreError(w, r, runtimeErr)
			return
		}
	}
	h.auditAction(r, "ocr.task_failed", "ocr_task", task.ID, "fail ocr task")
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "ocr_task_not_found", "ocr task not found")
	case errors.Is(err, ErrSubmissionNotReady):
		httpx.Error(w, r, http.StatusConflict, "submission_not_ready_for_ocr", "submission must be ready_for_ocr before creating an ocr task")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_ocr_input", "ocr input is invalid")
	case errors.Is(err, ErrInvalidTransition):
		httpx.Error(w, r, http.StatusConflict, "invalid_ocr_status_transition", "ocr task status transition is not allowed")
	case errors.Is(err, ErrResultConflict):
		httpx.Error(w, r, http.StatusConflict, "ocr_result_conflict", "ocr result conflicts with the completed task")
	case errors.Is(err, workerruntime.ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_worker_task_input", "worker task input is invalid")
	case errors.Is(err, workerruntime.ErrConflict), errors.Is(err, workerruntime.ErrInvalidTransition):
		httpx.Error(w, r, http.StatusConflict, "worker_task_conflict", "worker task operation conflicts with current state")
	case errors.Is(err, workerruntime.ErrLeaseExpired):
		httpx.Error(w, r, http.StatusConflict, "worker_task_lease_expired", "worker task lease expired")
	case errors.Is(err, workerruntime.ErrLeaseMismatch):
		httpx.Error(w, r, http.StatusConflict, "worker_task_lease_mismatch", "worker task lease mismatch")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "ocr_operation_failed", "ocr operation failed")
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func (h *Handler) auditAction(r *http.Request, action string, targetType string, targetID string, reason string) {
	user := mustUser(r)
	_ = h.audit.Audit(r.Context(), auth.AuditEvent{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		RequestID:  logger.RequestID(r.Context()),
	})
}
