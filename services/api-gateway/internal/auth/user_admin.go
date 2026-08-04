package auth

import (
	"errors"
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/pagination"
)

func (h *Handler) ListManagedUsers(w http.ResponseWriter, r *http.Request) {
	actor, _ := UserFromContext(r.Context())
	limit, err := pagination.Limit(r.URL.Query().Get("limit"), 100, 200)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_pagination", "limit must be between 1 and 200")
		return
	}
	cursor, err := pagination.Decode(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_pagination", "cursor is invalid")
		return
	}
	users, err := h.store.ListManagedUsers(r.Context(), actor.TenantID, ManagedUserFilter{
		Query: strings.TrimSpace(r.URL.Query().Get("q")), Role: strings.TrimSpace(r.URL.Query().Get("role")),
		Limit: limit + 1, CursorCreatedAt: cursor.CreatedAt, CursorID: cursor.ID,
	})
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "user_list_failed", "failed to list users")
		return
	}
	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}
	nextCursor := ""
	if hasMore && len(users) > 0 {
		last := users[len(users)-1]
		nextCursor = pagination.Encode(last.CreatedAt, last.ID)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"users": users, "next_cursor": nextCursor, "has_more": hasMore})
}

func (h *Handler) ListAssignableRoles(w http.ResponseWriter, r *http.Request) {
	actor, _ := UserFromContext(r.Context())
	roles, err := h.store.ListAssignableRoles(r.Context(), actor.TenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "role_list_failed", "failed to list roles")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func (h *Handler) CreateManagedUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := UserFromContext(r.Context())
	var input CreateManagedUserInput
	if err := decodeAuthJSON(w, r, &input, true); err != nil {
		if authRequestBodyTooLarge(err) {
			httpx.Error(w, r, http.StatusRequestEntityTooLarge, "request_body_too_large", "request body is too large")
			return
		}
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid user fields")
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.RoleCode = strings.TrimSpace(input.RoleCode)
	if input.Username == "" || input.DisplayName == "" || input.RoleCode == "" || input.Password == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "username, display_name, password and role_code are required")
		return
	}
	if !managedUserFieldsWithinLimits(input) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "user fields exceed supported size limits")
		return
	}
	if !strongBootstrapPassword(input.Password) {
		httpx.Error(w, r, http.StatusBadRequest, "weak_password", "password must contain upper, lower, number and symbol and be at least 12 characters")
		return
	}
	hash, err := HashPassword(input.Password)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "password_hash_failed", "failed to create user")
		return
	}
	created, err := h.store.CreateManagedUser(r.Context(), actor.TenantID, actor.TenantCode, input, hash)
	if err != nil {
		switch {
		case errors.Is(err, ErrUsernameExists):
			httpx.Error(w, r, http.StatusConflict, "username_exists", "username already exists")
		case errors.Is(err, ErrRoleNotFound):
			httpx.Error(w, r, http.StatusBadRequest, "role_not_assignable", "role is not assignable in the current organization")
		default:
			httpx.Error(w, r, http.StatusInternalServerError, "user_create_failed", "failed to create user")
		}
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID: actor.TenantID, ActorID: actor.ID, Action: "auth.user_created",
		TargetType: "user", TargetID: created.ID,
		AfterValue: map[string]any{"username": created.Username, "display_name": created.DisplayName, "role_code": input.RoleCode, "status": created.Status},
		Reason:     "create organization user", IPAddress: h.remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
	})
	httpx.JSON(w, http.StatusCreated, map[string]any{"user": created})
}
