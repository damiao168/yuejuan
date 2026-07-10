package grading

import (
	"encoding/json"
	"errors"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store  Store
	engine *Engine
	audit  auth.Store
}

func NewHandler(store Store, engine *Engine, audit auth.Store) *Handler {
	return &Handler{store: store, engine: engine, audit: audit}
}

func (h *Handler) RecordAnswer(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input RecordAnswerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	answer, err := h.store.RecordAnswer(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "grading.answer_recorded", "answer_segment", answer.AnswerSegmentID, "record answer for rule grading")
	httpx.JSON(w, http.StatusOK, map[string]any{"answer": answer})
}

func (h *Handler) RuleGrade(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	ctx, err := h.store.LoadContext(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	evaluation, err := h.engine.Grade(ctx)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	grade, err := h.store.CreateGrade(r.Context(), user.TenantID, user.ID, evaluation)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "grading.rule_grade_created", "ai_grade", grade.ID, "create rule-based objective grade")
	httpx.JSON(w, http.StatusCreated, map[string]any{"grade": grade})
}

func (h *Handler) ListGrades(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	grades, err := h.store.ListGrades(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"grades": grades})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "grading_resource_not_found", "grading resource not found")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_grading_input", "grading input is invalid")
	case errors.Is(err, ErrAnswerMissing):
		httpx.Error(w, r, http.StatusConflict, "answer_segment_answer_missing", "answer segment has no recorded answer")
	case errors.Is(err, ErrAnswerKeyMissing):
		httpx.Error(w, r, http.StatusConflict, "question_answer_key_missing", "question has no answer key")
	case errors.Is(err, ErrUnsupportedQuestionType):
		httpx.Error(w, r, http.StatusBadRequest, "unsupported_rule_grading_question_type", "question type is not supported by rule grading")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "grading_operation_failed", "grading operation failed")
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
