package subjective

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

type Handler struct {
	store   Store
	adapter LLMGradingAdapter
	audit   auth.Store
	runtime workerruntime.Store
}

func NewHandler(store Store, adapter LLMGradingAdapter, audit auth.Store) *Handler {
	return &Handler{store: store, adapter: adapter, audit: audit}
}

func (h *Handler) WithWorkerRuntimeStore(runtime workerruntime.Store) *Handler {
	h.runtime = runtime
	return h
}

func (h *Handler) Grade(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.requireAvailable(w, r) {
		return
	}
	var input GradeRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		writeStoreError(w, r, err)
		return
	}
	policy := NormalizePolicy(input.ModelPolicy)
	if governed, ok := h.adapter.(GovernedPolicyProvider); ok {
		policy = governed.Policy()
	}
	if err := ValidatePolicy(policy); err != nil {
		writeStoreError(w, r, err)
		return
	}
	ctx, err := h.store.LoadContext(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if !IsSupportedQuestionType(ctx.Question.QuestionType) {
		writeStoreError(w, r, ErrUnsupportedQuestionType)
		return
	}
	requestID, err := stableRequestID(user.TenantID, ctx, policy, input.IdempotencyKey)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if input.IdempotencyKey != "" {
		if existing, lookupErr := h.store.GetGradeByAdapterRequestID(r.Context(), user.TenantID, requestID); lookupErr == nil {
			httpx.JSON(w, http.StatusOK, map[string]any{"grade": existing, "idempotent_replay": true})
			return
		} else if !errors.Is(lookupErr, ErrNotFound) {
			writeStoreError(w, r, lookupErr)
			return
		}
	}
	runID := ""
	if run, runErr := h.store.GetOrCreateRun(r.Context(), user.TenantID, user.ID, CreateRunInput{
		AnswerSegmentID: ctx.SegmentID,
		AnswerVersion:   ctx.AnswerVersion,
		QuestionID:      ctx.Question.ID,
		RubricVersion:   ctx.Rubric.Version,
		ModelVersion:    policy.ModelVersion,
		PromptVersion:   policy.PromptVersion,
		MinConfidence:   policy.MinConfidence,
		RequestID:       requestID,
	}); runErr != nil {
		writeStoreError(w, r, runErr)
		return
	} else {
		runID = run.ID
	}
	finishRun := func(status string, grade Grade, errorCode string) error {
		_, updateErr := h.store.UpdateRun(r.Context(), user.TenantID, runID, UpdateRunInput{Status: status, GradeID: grade.ID, ErrorCode: errorCode})
		return updateErr
	}
	promptGuard := InspectPromptInjection(ctx.AnswerText)
	adapterInput := AdapterInput{
		RequestID:      requestID,
		SegmentID:      ctx.SegmentID,
		Subject:        ctx.Subject,
		GradeLevel:     ctx.GradeLevel,
		Question:       ctx.Question,
		Rubric:         ctx.Rubric,
		AnswerText:     ctx.AnswerText,
		AnswerImageRef: ctx.AnswerImageRef,
		OCRConfidence:  ctx.OCRConfidence,
		ModelPolicy:    policy,
		PromptGuard:    promptGuard,
	}
	output, err := h.adapter.Grade(r.Context(), adapterInput)
	output.RequestID = requestID
	if err != nil {
		var agentErr *GradingAgentError
		if errors.As(err, &agentErr) {
			code := "ai_service_unavailable"
			if agentErr.Code == "adapter_not_configured" {
				code = "ai_service_not_configured"
			}
			if _, updateErr := h.store.UpdateRun(r.Context(), user.TenantID, runID, UpdateRunInput{Status: RunFailed, ErrorCode: code}); updateErr != nil {
				writeStoreError(w, r, updateErr)
				return
			}
			h.auditAction(r, "subjective.ai_service_unavailable", "subjective_grading_run", runID, code)
			httpx.Error(w, r, http.StatusServiceUnavailable, code, "AI grading service is unavailable; use manual review")
			return
		}
		ApplyPromptGuard(&output, promptGuard)
		grade, createErr := h.store.CreateGrade(r.Context(), user.TenantID, user.ID, failedGrade(ctx, policy, runID, err.Error(), output))
		if createErr != nil {
			writeStoreError(w, r, createErr)
			return
		}
		if updateErr := finishRun(RunFailed, grade, "adapter_failed"); updateErr != nil {
			writeStoreError(w, r, updateErr)
			return
		}
		h.auditAction(r, "subjective.ai_grade_failed", "ai_grade", grade.ID, "subjective adapter failed")
		httpx.JSON(w, http.StatusCreated, map[string]any{"grade": grade})
		return
	}
	if err := ValidateOutput(output, ctx); err != nil {
		ApplyPromptGuard(&output, promptGuard)
		grade, createErr := h.store.CreateGrade(r.Context(), user.TenantID, user.ID, failedGrade(ctx, policy, runID, err.Error(), output))
		if createErr != nil {
			writeStoreError(w, r, createErr)
			return
		}
		if updateErr := finishRun(RunFailed, grade, "invalid_model_output"); updateErr != nil {
			writeStoreError(w, r, updateErr)
			return
		}
		h.auditAction(r, "subjective.ai_grade_failed", "ai_grade", grade.ID, "subjective adapter output failed schema validation")
		httpx.JSON(w, http.StatusCreated, map[string]any{"grade": grade})
		return
	}
	ApplyPromptGuard(&output, promptGuard)
	ApplyReviewPolicy(&output, ctx, policy)
	grade, err := h.store.CreateGrade(r.Context(), user.TenantID, user.ID, successfulGrade(ctx, policy, runID, output))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if updateErr := finishRun(RunSucceeded, grade, ""); updateErr != nil {
		writeStoreError(w, r, updateErr)
		return
	}
	h.auditAction(r, "subjective.ai_grade_created", "ai_grade", grade.ID, "create subjective ai grade")
	httpx.JSON(w, http.StatusCreated, map[string]any{"grade": grade})
}

func (h *Handler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input CreateBatchInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		writeStoreError(w, r, ErrInvalidInput)
		return
	}
	segments, err := normalizeBatchSegments(input.SegmentIDs)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	for _, segmentID := range segments {
		ctx, loadErr := h.store.LoadContext(r.Context(), user.TenantID, segmentID)
		if loadErr != nil {
			writeStoreError(w, r, loadErr)
			return
		}
		if !IsSupportedQuestionType(ctx.Question.QuestionType) {
			writeStoreError(w, r, ErrUnsupportedQuestionType)
			return
		}
	}
	batch, err := h.store.CreateBatch(r.Context(), user.TenantID, user.ID, CreateBatchInput{IdempotencyKey: input.IdempotencyKey, SegmentIDs: segments})
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "subjective.batch_created", "subjective_grading_batch", batch.ID, "create subjective grading batch")
	httpx.JSON(w, http.StatusCreated, map[string]any{"batch": batch})
}

func (h *Handler) GetBatch(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	batch, err := h.store.GetBatch(r.Context(), user.TenantID, r.PathValue("batchId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if batch.Status != "planned" && batch.Status != "cancelled" {
		batch, err = h.store.RefreshBatch(r.Context(), user.TenantID, batch.ID)
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"batch": batch})
}

func (h *Handler) EnqueueBatch(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.requireAvailable(w, r) {
		return
	}
	if h.runtime == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "subjective_worker_unavailable", "subjective worker runtime is unavailable")
		return
	}
	batch, err := h.store.GetBatch(r.Context(), user.TenantID, r.PathValue("batchId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	policy := NormalizePolicy(ModelPolicy{})
	if governed, ok := h.adapter.(GovernedPolicyProvider); ok {
		policy = governed.Policy()
	}
	if err := ValidatePolicy(policy); err != nil {
		writeStoreError(w, r, err)
		return
	}
	tasks := make([]workerruntime.Task, 0, len(batch.SegmentIDs))
	queued, processing, succeeded, failed := 0, 0, 0, 0
	for _, segmentID := range batch.SegmentIDs {
		ctx, loadErr := h.store.LoadContext(r.Context(), user.TenantID, segmentID)
		if loadErr != nil {
			writeStoreError(w, r, loadErr)
			return
		}
		requestID, idErr := stableRequestID(user.TenantID, ctx, policy, batch.IdempotencyKey+":"+segmentID)
		if idErr != nil {
			writeStoreError(w, r, idErr)
			return
		}
		run, runErr := h.store.GetOrCreateRun(r.Context(), user.TenantID, user.ID, CreateRunInput{AnswerSegmentID: ctx.SegmentID, BatchID: batch.ID, AnswerVersion: ctx.AnswerVersion, QuestionID: ctx.Question.ID, RubricVersion: ctx.Rubric.Version, ModelVersion: policy.ModelVersion, PromptVersion: policy.PromptVersion, MinConfidence: policy.MinConfidence, RequestID: requestID})
		if runErr != nil {
			writeStoreError(w, r, runErr)
			return
		}
		if run.Status == RunSucceeded {
			succeeded++
			continue
		}
		if run.Status == RunFailed {
			failed++
			continue
		}
		task, taskErr := h.runtime.CreateTask(r.Context(), user.TenantID, user.ID, workerruntime.CreateTaskInput{TaskType: "ai_grade", QueueName: "subjective-grading", SourceType: "subjective_grading_run", SourceID: run.ID, Priority: 100, Payload: map[string]any{"run_id": run.ID, "answer_segment_id": ctx.SegmentID, "answer_version": ctx.AnswerVersion, "question_id": ctx.Question.ID, "rubric_version": ctx.Rubric.Version, "model_version": policy.ModelVersion, "prompt_version": policy.PromptVersion}, PayloadSchemaVersion: "subjective-grade-v1", IdempotencyKey: requestID, DedupeKey: "subjective:" + batch.ID + ":" + ctx.SegmentID, MaxAttempts: 3, RetryBackoffSeconds: 30})
		if taskErr != nil {
			writeStoreError(w, r, taskErr)
			return
		}
		tasks = append(tasks, task)
		switch task.Status {
		case workerruntime.StatusQueued:
			queued++
		case workerruntime.StatusLeased, workerruntime.StatusRunning:
			processing++
		case workerruntime.StatusSucceeded:
			succeeded++
		case workerruntime.StatusFailed, workerruntime.StatusDeadLetter:
			failed++
		}
	}
	status := "processing"
	if succeeded+failed == batch.TotalCount {
		if failed > 0 {
			status = "failed"
		} else {
			status = "completed"
		}
	}
	updated, err := h.store.UpdateBatch(r.Context(), user.TenantID, batch.ID, UpdateBatchInput{Status: status, QueuedCount: queued, ProcessingCount: processing, SucceededCount: succeeded, FailedCount: failed})
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "subjective.batch_enqueued", "subjective_grading_batch", batch.ID, "enqueue subjective grading worker tasks")
	httpx.JSON(w, http.StatusOK, map[string]any{"batch": updated, "tasks": tasks})
}

func (h *Handler) CompleteWorker(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if h.runtime == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "subjective_worker_unavailable", "subjective worker runtime is unavailable")
		return
	}
	var input WorkerResultInput
	if !decodeJSON(w, r, &input) {
		return
	}
	run, err := h.store.GetRun(r.Context(), user.TenantID, r.PathValue("runId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	task, err := h.runtime.Get(r.Context(), user.TenantID, input.TaskID)
	if err != nil || task.SourceType != "subjective_grading_run" || task.SourceID != run.ID {
		httpx.Error(w, r, http.StatusConflict, "subjective_task_mismatch", "worker task does not belong to this subjective grading run")
		return
	}
	if input.LeaseToken == "" || task.LeaseToken != input.LeaseToken || (task.Status != workerruntime.StatusLeased && task.Status != workerruntime.StatusRunning) {
		httpx.Error(w, r, http.StatusConflict, "subjective_task_lease_mismatch", "subjective worker lease is missing, expired, or no longer active")
		return
	}
	if _, err := h.store.UpdateRun(r.Context(), user.TenantID, run.ID, UpdateRunInput{Status: RunProcessing, AttemptCount: task.AttemptCount}); err != nil {
		writeStoreError(w, r, err)
		return
	}
	if input.ResultSchemaVersion == "" || input.DurationMS < 0 || input.Output.RequestID != run.RequestID || input.Output.ModelVersion != run.ModelVersion || input.Output.PromptVersion != run.PromptVersion || input.Output.RubricVersion != run.RubricVersion {
		httpx.Error(w, r, http.StatusConflict, "subjective_result_version_conflict", "worker result does not match the requested grading versions")
		return
	}
	ctx, err := h.store.LoadContext(r.Context(), user.TenantID, run.AnswerSegmentID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if err := ValidateOutput(input.Output, ctx); err != nil {
		_, _ = h.runtime.Fail(r.Context(), user.TenantID, input.TaskID, workerruntime.FailInput{LeaseToken: input.LeaseToken, Retryable: false, ErrorCode: "invalid_model_output", ErrorDetail: map[string]any{"reason": err.Error()}, DurationMS: input.DurationMS})
		_, _ = h.store.UpdateRun(r.Context(), user.TenantID, run.ID, UpdateRunInput{Status: RunFailed, ErrorCode: "invalid_model_output", AttemptCount: task.AttemptCount})
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_output", "worker result failed schema validation")
		return
	}
	policy := ModelPolicy{ModelVersion: run.ModelVersion, PromptVersion: run.PromptVersion, MinConfidence: run.MinConfidence}
	promptGuard := InspectPromptInjection(ctx.AnswerText)
	output := input.Output
	ApplyPromptGuard(&output, promptGuard)
	ApplyReviewPolicy(&output, ctx, policy)
	grade, err := h.store.GetGradeByAdapterRequestID(r.Context(), user.TenantID, run.RequestID)
	if errors.Is(err, ErrNotFound) {
		grade, err = h.store.CreateGrade(r.Context(), user.TenantID, user.ID, successfulGrade(ctx, policy, run.ID, output))
	}
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	completed, err := h.runtime.Complete(r.Context(), user.TenantID, input.TaskID, workerruntime.CompleteInput{LeaseToken: input.LeaseToken, ResultSchemaVersion: input.ResultSchemaVersion, Result: map[string]any{"run_id": run.ID, "grade_id": grade.ID}, DurationMS: input.DurationMS})
	if err != nil {
		httpx.Error(w, r, http.StatusConflict, "subjective_task_completion_rejected", "subjective worker lease or result is no longer valid")
		return
	}
	updated, err := h.store.UpdateRun(r.Context(), user.TenantID, run.ID, UpdateRunInput{Status: RunSucceeded, GradeID: grade.ID, AttemptCount: completed.AttemptCount})
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "subjective.worker_completed", "subjective_grading_run", run.ID, "complete subjective worker result")
	httpx.JSON(w, http.StatusOK, map[string]any{"run": updated, "grade": grade, "task": completed})
}

func (h *Handler) ExecuteWorker(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.requireAvailable(w, r) {
		return
	}
	if h.runtime == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "subjective_worker_unavailable", "subjective worker runtime is unavailable")
		return
	}
	var input struct {
		TaskID     string `json:"task_id"`
		LeaseToken string `json:"lease_token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	run, err := h.store.GetRun(r.Context(), user.TenantID, r.PathValue("runId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	task, err := h.runtime.Get(r.Context(), user.TenantID, input.TaskID)
	if err != nil || task.SourceType != "subjective_grading_run" || task.SourceID != run.ID {
		httpx.Error(w, r, http.StatusConflict, "subjective_task_mismatch", "worker task does not belong to this subjective grading run")
		return
	}
	if input.LeaseToken == "" || task.LeaseToken != input.LeaseToken || (task.Status != workerruntime.StatusLeased && task.Status != workerruntime.StatusRunning) {
		httpx.Error(w, r, http.StatusConflict, "subjective_task_lease_mismatch", "subjective worker lease is missing, expired, or no longer active")
		return
	}
	if _, err := h.store.UpdateRun(r.Context(), user.TenantID, run.ID, UpdateRunInput{Status: RunProcessing, AttemptCount: task.AttemptCount}); err != nil {
		writeStoreError(w, r, err)
		return
	}
	ctx, err := h.store.LoadContext(r.Context(), user.TenantID, run.AnswerSegmentID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	policy := ModelPolicy{ModelVersion: run.ModelVersion, PromptVersion: run.PromptVersion, MinConfidence: run.MinConfidence}
	promptGuard := InspectPromptInjection(ctx.AnswerText)
	output, err := h.adapter.Grade(r.Context(), AdapterInput{RequestID: run.RequestID, SegmentID: ctx.SegmentID, Subject: ctx.Subject, GradeLevel: ctx.GradeLevel, Question: ctx.Question, Rubric: ctx.Rubric, AnswerText: ctx.AnswerText, AnswerImageRef: ctx.AnswerImageRef, OCRConfidence: ctx.OCRConfidence, ModelPolicy: policy, PromptGuard: promptGuard})
	output.RequestID = run.RequestID
	if err != nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "ai_service_unavailable", "AI grading service is unavailable; use manual review")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"run_id": run.ID, "task_id": task.ID, "output": output})
}

func (h *Handler) FailWorker(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if h.runtime == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "subjective_worker_unavailable", "subjective worker runtime is unavailable")
		return
	}
	var input WorkerFailureInput
	if !decodeJSON(w, r, &input) {
		return
	}
	run, err := h.store.GetRun(r.Context(), user.TenantID, r.PathValue("runId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	task, err := h.runtime.Get(r.Context(), user.TenantID, input.TaskID)
	if err != nil || task.SourceType != "subjective_grading_run" || task.SourceID != run.ID {
		httpx.Error(w, r, http.StatusConflict, "subjective_task_mismatch", "worker task does not belong to this subjective grading run")
		return
	}
	updatedTask, err := h.runtime.Fail(r.Context(), user.TenantID, input.TaskID, workerruntime.FailInput{LeaseToken: input.LeaseToken, Retryable: input.Retryable, ErrorCode: input.ErrorCode, ErrorDetail: input.ErrorDetail, DurationMS: input.DurationMS})
	if err != nil {
		httpx.Error(w, r, http.StatusConflict, "subjective_task_failure_rejected", "subjective worker lease or failure is no longer valid")
		return
	}
	status := RunQueued
	if updatedTask.Status == workerruntime.StatusFailed || updatedTask.Status == workerruntime.StatusDeadLetter {
		status = RunFailed
	} else if updatedTask.Status == workerruntime.StatusLeased || updatedTask.Status == workerruntime.StatusRunning {
		status = RunProcessing
	}
	updatedRun, err := h.store.UpdateRun(r.Context(), user.TenantID, run.ID, UpdateRunInput{Status: status, ErrorCode: input.ErrorCode, AttemptCount: updatedTask.AttemptCount})
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "subjective.worker_failed", "subjective_grading_run", run.ID, input.ErrorCode)
	httpx.JSON(w, http.StatusOK, map[string]any{"run": updatedRun, "task": updatedTask})
}

func successfulGrade(ctx Context, policy ModelPolicy, runID string, output AdapterOutput) Grade {
	graderType := LLMGraderType
	if output.Mock {
		graderType = MockGraderType
	}
	return Grade{
		AnswerSegmentID:        ctx.SegmentID,
		QuestionID:             ctx.Question.ID,
		QuestionNo:             ctx.Question.QuestionNo,
		QuestionType:           ctx.Question.QuestionType,
		AnswerVersion:          ctx.AnswerVersion,
		GraderType:             graderType,
		ModelVersion:           firstNonEmpty(output.ModelVersion, policy.ModelVersion),
		PromptVersion:          firstNonEmpty(output.PromptVersion, policy.PromptVersion),
		RubricVersion:          firstNonEmpty(output.RubricVersion, ctx.Rubric.Version),
		DeliveryMode:           firstNonEmpty(output.DeliveryMode, "teacher_review"),
		CapabilityProfile:      output.CapabilityProfile,
		AdapterRequestID:       output.RequestID,
		RunID:                  runID,
		AdapterName:            output.Telemetry.Adapter,
		ProviderKey:            output.Telemetry.Provider,
		DeploymentKey:          output.Telemetry.Deployment,
		DeploymentRegion:       output.Telemetry.Region,
		AdapterAttempts:        output.Telemetry.Attempts,
		AdapterLatencyMS:       output.Telemetry.ElapsedMS,
		AdapterRepairAttempted: output.Telemetry.RepairAttempted,
		SuggestedScore:         output.SuggestedScore,
		MaxScore:               ctx.Question.Score,
		Confidence:             output.Confidence,
		MatchedPoints:          output.MatchedPoints,
		MissingPoints:          output.MissingPoints,
		Evidence:               output.Evidence,
		RiskFlags:              output.RiskFlags,
		NeedsHumanReview:       output.NeedsHumanReview,
		StudentFeedback:        output.StudentFeedback,
		TeacherNote:            output.TeacherNote,
		Mock:                   output.Mock,
		Status:                 "succeeded",
		RawOutput:              output.RawOutput,
	}
}

func failedGrade(ctx Context, policy ModelPolicy, runID string, reason string, output AdapterOutput) Grade {
	riskFlags := appendFlag(output.RiskFlags, "invalid_model_output")
	if output.Mock {
		riskFlags = appendFlag(riskFlags, "mock_llm_output")
	}
	graderType := LLMGraderType
	if output.Mock {
		graderType = MockGraderType
	}
	raw := output.RawOutput
	if raw == nil {
		raw = map[string]any{}
	}
	raw["failure_reason"] = reason
	return Grade{
		AnswerSegmentID:        ctx.SegmentID,
		QuestionID:             ctx.Question.ID,
		QuestionNo:             ctx.Question.QuestionNo,
		QuestionType:           ctx.Question.QuestionType,
		AnswerVersion:          ctx.AnswerVersion,
		GraderType:             graderType,
		ModelVersion:           firstNonEmpty(output.ModelVersion, policy.ModelVersion),
		PromptVersion:          firstNonEmpty(output.PromptVersion, policy.PromptVersion),
		RubricVersion:          firstNonEmpty(output.RubricVersion, ctx.Rubric.Version),
		DeliveryMode:           "teacher_review",
		CapabilityProfile:      output.CapabilityProfile,
		AdapterRequestID:       output.RequestID,
		RunID:                  runID,
		AdapterName:            output.Telemetry.Adapter,
		ProviderKey:            output.Telemetry.Provider,
		DeploymentKey:          output.Telemetry.Deployment,
		DeploymentRegion:       output.Telemetry.Region,
		AdapterAttempts:        output.Telemetry.Attempts,
		AdapterLatencyMS:       output.Telemetry.ElapsedMS,
		AdapterRepairAttempted: output.Telemetry.RepairAttempted,
		SuggestedScore:         0,
		MaxScore:               ctx.Question.Score,
		Confidence:             0,
		MatchedPoints:          []grading.PointResult{},
		MissingPoints:          []grading.PointResult{},
		Evidence:               []grading.Evidence{},
		RiskFlags:              riskFlags,
		NeedsHumanReview:       true,
		StudentFeedback:        "",
		TeacherNote:            "Adapter output failed schema validation; no valid subjective score was produced.",
		Mock:                   output.Mock,
		Status:                 "failed",
		FailureReason:          reason,
		RawOutput:              raw,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (h *Handler) Availability(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{"ai_grading": h.runtimeStatus()})
}

func (h *Handler) runtimeStatus() RuntimeStatus {
	if provider, ok := h.adapter.(RuntimeStatusProvider); ok {
		return provider.RuntimeStatus()
	}
	return RuntimeStatus{Enabled: true, Available: true, Mode: "custom"}
}

func (h *Handler) requireAvailable(w http.ResponseWriter, r *http.Request) bool {
	status := h.runtimeStatus()
	if status.Available {
		return true
	}
	code := status.ErrorCode
	if code == "" {
		code = "ai_service_unavailable"
	}
	message := "AI grading service is unavailable; manual review remains available"
	if code == "ai_grading_disabled" {
		message = "AI grading is disabled; use manual review"
	}
	httpx.Error(w, r, http.StatusServiceUnavailable, code, message)
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil {
		return true
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "request body must contain one JSON object")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "subjective_grading_resource_not_found", "subjective grading resource not found")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_subjective_grading_input", "subjective grading input is invalid")
	case errors.Is(err, ErrUnsupportedQuestionType):
		httpx.Error(w, r, http.StatusBadRequest, "unsupported_subjective_question_type", "question type is not supported by subjective ai grading")
	case errors.Is(err, ErrAnswerMissing):
		httpx.Error(w, r, http.StatusConflict, "answer_segment_answer_missing", "answer segment has no recorded answer")
	case errors.Is(err, ErrRubricMissing):
		httpx.Error(w, r, http.StatusConflict, "question_rubric_missing", "question has no rubric for subjective grading")
	case errors.Is(err, ErrInvalidModelOutput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_output", "model output failed schema validation")
	case errors.Is(err, ErrIdempotencyConflict):
		httpx.Error(w, r, http.StatusConflict, "subjective_grade_idempotency_conflict", "same idempotency request produced a different grading fact")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "subjective_grading_operation_failed", "subjective grading operation failed")
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func (h *Handler) auditAction(r *http.Request, action string, targetType string, targetID string, reason string) {
	user := mustUser(r)
	auth.RecordAudit(r.Context(), h.audit, auth.AuditEvent{
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
