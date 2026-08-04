package exam

import (
	"encoding/json"
	"errors"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/pagination"
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
	scope, ok := mustAccessScope(w, r)
	if !ok {
		return
	}
	var input CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateCreate(input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_exam", err.Error())
		return
	}
	out, err := h.store.CreateExam(r.Context(), scope, user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "exam.created", "exam", out.ID, "create exam")
	httpx.JSON(w, http.StatusCreated, map[string]any{"exam": out})
}

func (h *Handler) ListExams(w http.ResponseWriter, r *http.Request) {
	scope, ok := mustAccessScope(w, r)
	if !ok {
		return
	}
	limit, err := pagination.Limit(r.URL.Query().Get("limit"), 50, 200)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_pagination", "pagination limit is invalid")
		return
	}
	cursor, err := pagination.Decode(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_pagination", "pagination cursor is invalid")
		return
	}
	out, err := h.store.ListExams(r.Context(), scope, ListFilter{
		Status:   r.URL.Query().Get("status"),
		SchoolID: r.URL.Query().Get("school_id"),
		Limit:    limit + 1,
		CursorAt: cursor.CreatedAt,
		CursorID: cursor.ID,
	})
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "exam_list_failed", "failed to list exams")
		return
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	nextCursor := ""
	if hasMore && len(out) > 0 {
		last := out[len(out)-1]
		nextCursor = pagination.Encode(last.CreatedAt, last.ID)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exams": out, "next_cursor": nextCursor, "has_more": hasMore})
}

func (h *Handler) GetExam(w http.ResponseWriter, r *http.Request) {
	scope, ok := mustAccessScope(w, r)
	if !ok {
		return
	}
	out, err := h.store.GetExam(r.Context(), scope, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "exam_not_found", "exam not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exam": out})
}

func (h *Handler) UpdateExam(w http.ResponseWriter, r *http.Request) {
	scope, ok := mustAccessScope(w, r)
	if !ok {
		return
	}
	var input UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateUpdate(input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_exam", err.Error())
		return
	}
	out, err := h.store.UpdateExam(r.Context(), scope, r.PathValue("id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "exam.updated", "exam", out.ID, "update exam")
	httpx.JSON(w, http.StatusOK, map[string]any{"exam": out})
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	scope, ok := mustAccessScope(w, r)
	if !ok {
		return
	}
	var input struct {
		Status           string `json:"status"`
		ExpectedRevision int64  `json:"expected_revision"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !IsValidStatus(input.Status) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_status", "unknown exam status")
		return
	}
	if input.ExpectedRevision <= 0 {
		httpx.Error(w, r, http.StatusBadRequest, "expected_revision_required", "expected_revision is required")
		return
	}
	out, err := h.store.UpdateStatus(r.Context(), scope, r.PathValue("id"), input.Status, input.ExpectedRevision)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "exam.status_changed", "exam", out.ID, "change exam status")
	httpx.JSON(w, http.StatusOK, map[string]any{"exam": out})
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	scope, ok := mustAccessScope(w, r)
	if !ok {
		return
	}
	var input struct {
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.ExpectedRevision <= 0 {
		httpx.Error(w, r, http.StatusBadRequest, "expected_revision_required", "expected_revision is required")
		return
	}
	out, err := h.store.UpdateStatus(r.Context(), scope, r.PathValue("id"), "archived", input.ExpectedRevision)
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
	if input.ExpectedRevision <= 0 {
		return errors.New("expected_revision is required")
	}
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
	case errors.Is(err, ErrRevisionConflict):
		httpx.Error(w, r, http.StatusConflict, "resource_version_conflict", "exam was changed by another user; refresh and retry")
	case errors.Is(err, ErrScopeForbidden):
		httpx.Error(w, r, http.StatusForbidden, "access_scope_forbidden", "exam is outside the assigned data scope")
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

func mustAccessScope(w http.ResponseWriter, r *http.Request) (auth.AccessScope, bool) {
	scope, ok := auth.AccessScopeFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusForbidden, "access_scope_missing", "no valid data access scope is assigned")
	}
	return scope, ok
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
