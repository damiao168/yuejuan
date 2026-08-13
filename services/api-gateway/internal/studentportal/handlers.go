package studentportal

import (
	"errors"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// RegisterRoutes adds the A21 discovery endpoint. A18 already registers the
// two canonical student-safe result endpoints on this same path family:
//
//	GET /api/v1/student/exams/{examId}/result
//	GET /api/v1/student/exams/{examId}/questions/{questionId}
//
// Keeping a single owner for those DTOs prevents the portal from drifting into
// an admin/review representation or registering duplicate ServeMux patterns.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireStudentRead func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /api/v1/student/exams", requireStudentRead(h.ListPublishedExams))
}

func (h *Handler) ListPublishedExams(w http.ResponseWriter, r *http.Request) {
	user, studentID, ok := studentUser(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListPublishedExams(r.Context(), user.TenantID, studentID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exams": items})
}

func studentUser(w http.ResponseWriter, r *http.Request) (auth.User, string, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return auth.User{}, "", false
	}
	studentID, scoped := auth.ScopedStudentID(user)
	if !auth.HasPermission(user, "student:grade:read") || !scoped {
		httpx.Error(w, r, http.StatusForbidden, "student_score_scope_required", "student scope is required for published scores")
		return auth.User{}, "", false
	}
	return user, studentID, true
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_student_portal_input", "student portal request is invalid")
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "student_result_not_found", "no published result was found")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "student_portal_operation_failed", "student portal operation failed")
	}
}
