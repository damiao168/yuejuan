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
	actorScope, _ := AccessScopeFromContext(r.Context())
	filter := ManagedUserFilter{
		Query: strings.TrimSpace(r.URL.Query().Get("q")), Role: strings.TrimSpace(r.URL.Query().Get("role")),
		Limit: limit + 1, CursorCreatedAt: cursor.CreatedAt, CursorID: cursor.ID,
	}
	if !actorScope.IsPlatform && !actorScope.TenantWide {
		filter.RestrictSchools = true
		filter.SchoolIDs = append([]string(nil), actorScope.SchoolIDs...)
	}
	users, err := h.store.ListManagedUsers(r.Context(), actor.TenantID, filter)
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
	roles, err := h.store.ListAssignableRoles(r.Context(), actor)
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
	input.SchoolID = strings.TrimSpace(input.SchoolID)
	input.ClassIDs = uniqueSortedIDs(input.ClassIDs)
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
	actorScope, ok := AccessScopeFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusForbidden, "access_scope_missing", "no valid data access scope is assigned")
		return
	}
	created, err := h.store.CreateManagedUser(r.Context(), actor, actorScope, input, hash)
	if err != nil {
		switch {
		case errors.Is(err, ErrUsernameExists):
			httpx.Error(w, r, http.StatusConflict, "username_exists", "username already exists")
		case errors.Is(err, ErrRoleNotFound):
			httpx.Error(w, r, http.StatusBadRequest, "role_not_assignable", "role is not assignable in the current organization")
		case errors.Is(err, ErrRoleAssignment):
			httpx.Error(w, r, http.StatusForbidden, "role_assignment_forbidden", "current identity cannot assign this role")
		case errors.Is(err, ErrOrganizationScope):
			httpx.Error(w, r, http.StatusForbidden, "organization_scope_forbidden", "organization binding is outside the current data access scope")
		case errors.Is(err, ErrInvalidRoleBinding):
			httpx.Error(w, r, http.StatusBadRequest, "invalid_role_binding", "role organization binding is invalid")
		default:
			httpx.Error(w, r, http.StatusInternalServerError, "user_create_failed", "failed to create user")
		}
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID: actor.TenantID, ActorID: actor.ID, Action: "auth.user_created",
		TargetType: "user", TargetID: created.ID,
		AfterValue: map[string]any{"username": created.Username, "display_name": created.DisplayName, "role_code": input.RoleCode, "school_id": created.SchoolID, "class_ids": input.ClassIDs, "status": created.Status},
		Reason:     "create organization user", IPAddress: h.remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
	})
	httpx.JSON(w, http.StatusCreated, map[string]any{"user": created})
}

func (h *Handler) UpdateManagedUserStatus(w http.ResponseWriter, r *http.Request) {
	actor, _ := UserFromContext(r.Context())
	var input UpdateManagedUserStatusInput
	if err := decodeAuthJSON(w, r, &input, true); err != nil {
		if authRequestBodyTooLarge(err) {
			httpx.Error(w, r, http.StatusRequestEntityTooLarge, "request_body_too_large", "request body is too large")
			return
		}
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid status field")
		return
	}
	input.Status = strings.TrimSpace(input.Status)
	if input.Status != "active" && input.Status != "disabled" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_user_status", "status must be active or disabled")
		return
	}
	actorScope, ok := AccessScopeFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusForbidden, "access_scope_missing", "no valid data access scope is assigned")
		return
	}
	updated, previousStatus, err := h.store.UpdateManagedUserStatus(r.Context(), actor, actorScope, strings.TrimSpace(r.PathValue("id")), input.Status)
	if err != nil {
		switch {
		case errors.Is(err, ErrManagedUserNotFound):
			httpx.Error(w, r, http.StatusNotFound, "managed_user_not_found", "managed user not found")
		case errors.Is(err, ErrUserStatusForbidden):
			httpx.Error(w, r, http.StatusForbidden, "user_status_forbidden", "current identity cannot update this user")
		case errors.Is(err, ErrOrganizationScope):
			httpx.Error(w, r, http.StatusForbidden, "organization_scope_forbidden", "organization binding is outside the current data access scope")
		case errors.Is(err, ErrLastSchoolAdmin):
			httpx.Error(w, r, http.StatusConflict, "last_school_admin", "the last active school administrator cannot be disabled")
		default:
			httpx.Error(w, r, http.StatusInternalServerError, "user_status_update_failed", "failed to update user status")
		}
		return
	}
	if previousStatus != updated.Status {
		RecordAudit(r.Context(), h.store, AuditEvent{
			TenantID: actor.TenantID, ActorID: actor.ID, Action: "auth.user_status_updated",
			TargetType: "user", TargetID: updated.ID,
			BeforeValue: map[string]any{"status": previousStatus}, AfterValue: map[string]any{"status": updated.Status},
			Reason: "update organization user status", IPAddress: h.remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": updated})
}
