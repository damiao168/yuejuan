package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

func (h *Handler) ListManagedUsers(w http.ResponseWriter, r *http.Request) {
	actor, _ := UserFromContext(r.Context())
	users, err := h.store.ListManagedUsers(r.Context(), actor.TenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "user_list_failed", "failed to list users")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"users": users})
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
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
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
		Reason:     "create organization user", IPAddress: remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
	})
	httpx.JSON(w, http.StatusCreated, map[string]any{"user": created})
}
