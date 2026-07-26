package subjective

import (
	"encoding/json"
	"errors"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store   Store
	adapter LLMGradingAdapter
	audit   auth.Store
}

func NewHandler(store Store, adapter LLMGradingAdapter, audit auth.Store) *Handler {
	return &Handler{store: store, adapter: adapter, audit: audit}
}

func (h *Handler) Grade(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input GradeRequest
	if !decodeJSON(w, r, &input) {
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
	promptGuard := InspectPromptInjection(ctx.AnswerText)
	adapterInput := AdapterInput{
		RequestID:      newAdapterRequestID(),
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
	if err != nil {
		ApplyPromptGuard(&output, promptGuard)
		grade, createErr := h.store.CreateGrade(r.Context(), user.TenantID, user.ID, failedGrade(ctx, policy, err.Error(), output))
		if createErr != nil {
			writeStoreError(w, r, createErr)
			return
		}
		h.auditAction(r, "subjective.ai_grade_failed", "ai_grade", grade.ID, "subjective adapter failed")
		httpx.JSON(w, http.StatusCreated, map[string]any{"grade": grade})
		return
	}
	if err := ValidateOutput(output, ctx); err != nil {
		ApplyPromptGuard(&output, promptGuard)
		grade, createErr := h.store.CreateGrade(r.Context(), user.TenantID, user.ID, failedGrade(ctx, policy, err.Error(), output))
		if createErr != nil {
			writeStoreError(w, r, createErr)
			return
		}
		h.auditAction(r, "subjective.ai_grade_failed", "ai_grade", grade.ID, "subjective adapter output failed schema validation")
		httpx.JSON(w, http.StatusCreated, map[string]any{"grade": grade})
		return
	}
	ApplyPromptGuard(&output, promptGuard)
	ApplyReviewPolicy(&output, ctx, policy)
	grade, err := h.store.CreateGrade(r.Context(), user.TenantID, user.ID, successfulGrade(ctx, policy, output))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "subjective.ai_grade_created", "ai_grade", grade.ID, "create subjective ai grade")
	httpx.JSON(w, http.StatusCreated, map[string]any{"grade": grade})
}

func successfulGrade(ctx Context, policy ModelPolicy, output AdapterOutput) Grade {
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
		AdapterName:            output.Telemetry.Adapter,
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

func failedGrade(ctx Context, policy ModelPolicy, reason string, output AdapterOutput) Grade {
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
		AdapterName:            output.Telemetry.Adapter,
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
