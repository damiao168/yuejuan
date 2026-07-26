package score

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

func (h *Handler) FinalizeExam(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	result, err := h.store.FinalizeExam(r.Context(), user.TenantID, r.PathValue("examId"), user.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "score.finalized", "exam", r.PathValue("examId"), "finalize exam grades")
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) ListExamGrades(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	grades, err := h.store.ListExamGrades(r.Context(), user.TenantID, r.PathValue("examId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"grades": grades})
}

func (h *Handler) CheckQuality(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	stage := r.URL.Query().Get("stage")
	requirePendingPublish := stage == "publish" || r.URL.Query().Get("require_publish") == "true"
	quality, err := h.store.CheckQuality(r.Context(), user.TenantID, r.PathValue("examId"), requirePendingPublish)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"quality":     quality,
		"can_publish": quality.Passed && requirePendingPublish,
		"stage":       map[bool]string{true: "publish", false: "confirmation"}[requirePendingPublish],
	})
}

func (h *Handler) ConfirmGrades(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input ConfirmInput
	if !decodeJSON(w, r, &input) {
		return
	}
	grades, err := h.store.ConfirmGrades(r.Context(), user.TenantID, r.PathValue("examId"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "score.confirmed", "exam", r.PathValue("examId"), "confirm exam grades")
	httpx.JSON(w, http.StatusOK, map[string]any{"grades": grades})
}

func (h *Handler) PublishGrades(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input PublishInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.store.PublishGrades(r.Context(), user.TenantID, r.PathValue("examId"), user.ID, input)
	if err != nil {
		if errors.Is(err, ErrQualityGateFailed) {
			httpx.JSON(w, http.StatusConflict, result)
			return
		}
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "score.published", "exam", r.PathValue("examId"), "publish exam grades")
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) GetStudentGrade(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	studentID := r.PathValue("studentId")
	if !auth.HasPermission(user, "score:manage") {
		scoped, ok := scopedStudentID(user)
		if !auth.HasPermission(user, "student:grade:read") || !ok || scoped != studentID {
			httpx.Error(w, r, http.StatusForbidden, "student_grade_scope_violation", "student can only view own grade")
			return
		}
	}
	grade, err := h.store.GetStudentGrade(r.Context(), user.TenantID, studentID, r.PathValue("examId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"grade": grade})
}

func (h *Handler) ExportGrades(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	result, err := h.store.ExportGradesCSV(r.Context(), user.TenantID, r.PathValue("examId"), user.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "score.exported", "exam", r.PathValue("examId"), "export exam grades")
	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+result.Filename+`"`)
	w.Header().Set("X-EduGrade-Watermark", result.Watermark)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Content)
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
		httpx.Error(w, r, http.StatusNotFound, "score_resource_not_found", "score resource not found")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_score_input", "score input is invalid")
	case errors.Is(err, ErrInvalidTransition):
		httpx.Error(w, r, http.StatusConflict, "invalid_score_transition", "score transition is invalid")
	case errors.Is(err, ErrQualityGateFailed):
		httpx.Error(w, r, http.StatusConflict, "score_quality_gate_failed", "score quality gate failed")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "score_operation_failed", "score operation failed")
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func scopedStudentID(user auth.User) (string, bool) {
	return auth.ScopedStudentID(user)
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
