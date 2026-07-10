package exam

import (
	"encoding/json"
	"errors"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store Store
	audit auth.Store
}

func NewHandler(store Store, audit auth.Store) *Handler {
	return &Handler{store: store, audit: audit}
}

func (h *Handler) CreateExam(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateCreate(input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_exam", err.Error())
		return
	}
	out, err := h.store.CreateExam(r.Context(), user.TenantID, user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "exam.created", "exam", out.ID, "create exam")
	httpx.JSON(w, http.StatusCreated, map[string]any{"exam": out})
}

func (h *Handler) ListExams(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListExams(r.Context(), user.TenantID, ListFilter{
		Status:   r.URL.Query().Get("status"),
		SchoolID: r.URL.Query().Get("school_id"),
	})
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "exam_list_failed", "failed to list exams")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exams": out})
}

func (h *Handler) GetExam(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.GetExam(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "exam_not_found", "exam not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exam": out})
}

func (h *Handler) UpdateExam(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateUpdate(input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_exam", err.Error())
		return
	}
	out, err := h.store.UpdateExam(r.Context(), user.TenantID, r.PathValue("id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "exam.updated", "exam", out.ID, "update exam")
	httpx.JSON(w, http.StatusOK, map[string]any{"exam": out})
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !IsValidStatus(input.Status) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_status", "unknown exam status")
		return
	}
	out, err := h.store.UpdateStatus(r.Context(), user.TenantID, r.PathValue("id"), input.Status)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "exam.status_changed", "exam", out.ID, "change exam status")
	httpx.JSON(w, http.StatusOK, map[string]any{"exam": out})
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.UpdateStatus(r.Context(), user.TenantID, r.PathValue("id"), "archived")
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "exam.archived", "exam", out.ID, "archive exam")
	httpx.JSON(w, http.StatusOK, map[string]any{"exam": out})
}

func validateCreate(input CreateInput) error {
	if input.SchoolID == "" || input.Name == "" || input.Subject == "" || input.ExamType == "" {
		return ErrInvalidInput
	}
	if input.TotalScore <= 0 {
		return errors.New("total_score must be greater than 0")
	}
	if !IsValidGradingMode(input.GradingMode) {
		return errors.New("invalid grading_mode")
	}
	if input.PublishPolicy == "" {
		return errors.New("publish_policy is required")
	}
	return nil
}

func validateUpdate(input UpdateInput) error {
	if input.TotalScore != nil && *input.TotalScore <= 0 {
		return errors.New("total_score must be greater than 0")
	}
	if input.GradingMode != nil && !IsValidGradingMode(*input.GradingMode) {
		return errors.New("invalid grading_mode")
	}
	return nil
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "exam_not_found", "exam not found")
	case errors.Is(err, ErrInvalidTransition):
		httpx.Error(w, r, http.StatusConflict, "invalid_status_transition", "invalid exam status transition")
	case errors.Is(err, ErrLocked):
		httpx.Error(w, r, http.StatusConflict, "exam_locked", "published or archived exam cannot be modified")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_exam", "exam input is invalid")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "exam_operation_failed", "exam operation failed")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
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
