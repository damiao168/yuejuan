package dashboard

import (
	"errors"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(deps Dependencies) *Handler {
	return &Handler{service: NewService(deps)}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	scope, ok := auth.AccessScopeFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusForbidden, "access_scope_missing", "no valid data access scope is assigned")
		return
	}
	summary, err := h.service.Summary(r.Context(), user, scope)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			httpx.Error(w, r, http.StatusForbidden, "dashboard_scope_forbidden", "this workspace does not have a school dashboard")
			return
		}
		httpx.Error(w, r, http.StatusServiceUnavailable, "dashboard_summary_unavailable", "dashboard summary is temporarily unavailable")
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

func RegisterRoutes(mux *http.ServeMux, handler *Handler, requireDashboardRead func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /api/v1/dashboard/summary", requireDashboardRead(handler.Summary))
}
