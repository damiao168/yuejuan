package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/csvsafe"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/pagination"
)

type Handler struct {
	store          Store
	sessionTTL     time.Duration
	rememberedTTL  time.Duration
	loginLimiter   LoginLimiter
	cookieName     string
	cookieSecure   bool
	trustedProxies []*net.IPNet
}

type HandlerOptions struct {
	LoginFailureLimit    int
	LoginFailureWindow   time.Duration
	RememberedSessionTTL time.Duration
	CookieName           string
	CookieSecure         bool
	LoginLimiter         LoginLimiter
	TrustedProxyCIDRs    []string
}

const DefaultSessionCookieName = "edugrade_session"

func NewHandler(store Store, sessionTTL time.Duration, options ...HandlerOptions) *Handler {
	cfg := HandlerOptions{LoginFailureLimit: 5, LoginFailureWindow: 15 * time.Minute}
	if len(options) > 0 {
		if options[0].LoginFailureLimit > 0 {
			cfg.LoginFailureLimit = options[0].LoginFailureLimit
		}
		if options[0].LoginFailureWindow > 0 {
			cfg.LoginFailureWindow = options[0].LoginFailureWindow
		}
		if options[0].RememberedSessionTTL > 0 {
			cfg.RememberedSessionTTL = options[0].RememberedSessionTTL
		}
		if strings.TrimSpace(options[0].CookieName) != "" {
			cfg.CookieName = strings.TrimSpace(options[0].CookieName)
		}
		cfg.CookieSecure = options[0].CookieSecure
		cfg.LoginLimiter = options[0].LoginLimiter
		cfg.TrustedProxyCIDRs = options[0].TrustedProxyCIDRs
	}
	if cfg.CookieName == "" {
		cfg.CookieName = DefaultSessionCookieName
	}
	if cfg.RememberedSessionTTL <= 0 {
		cfg.RememberedSessionTTL = 30 * 24 * time.Hour
	}
	if cfg.LoginLimiter == nil {
		cfg.LoginLimiter = NewLoginFailureLimiter(cfg.LoginFailureLimit, cfg.LoginFailureWindow)
	}
	return &Handler{
		store:          store,
		sessionTTL:     sessionTTL,
		rememberedTTL:  cfg.RememberedSessionTTL,
		loginLimiter:   cfg.LoginLimiter,
		cookieName:     cfg.CookieName,
		cookieSecure:   cfg.CookieSecure,
		trustedProxies: parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs),
	}
}

type loginRequest struct {
	TenantCode     string `json:"tenant_code"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	RememberDevice bool   `json:"remember_device,omitempty"`
	DeviceName     string `json:"device_name,omitempty"`
	ClientType     string `json:"client_type,omitempty"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	h.login(w, r, false)
}

func (h *Handler) TokenLogin(w http.ResponseWriter, r *http.Request) {
	h.login(w, r, true)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request, tokenResponse bool) {
	var req loginRequest
	if err := decodeAuthJSON(w, r, &req, false); err != nil {
		if authRequestBodyTooLarge(err) {
			httpx.Error(w, r, http.StatusRequestEntityTooLarge, "request_body_too_large", "request body is too large")
			return
		}
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	req.TenantCode = strings.TrimSpace(req.TenantCode)
	req.Username = strings.TrimSpace(req.Username)
	req.DeviceName = strings.TrimSpace(req.DeviceName)
	req.ClientType = strings.ToLower(strings.TrimSpace(req.ClientType))
	if req.TenantCode == "" || req.Username == "" || req.Password == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "tenant_code, username and password are required")
		return
	}
	if !loginFieldsWithinLimits(req.TenantCode, req.Username, req.Password) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "login fields exceed supported size limits")
		return
	}
	clientIP := h.remoteIP(r)
	limiterKey := LoginFailureKey(req.TenantCode, req.Username, clientIP)
	if retryAfter, blocked := h.loginLimiter.IsBlocked(r.Context(), limiterKey, time.Now().UTC()); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		RecordAudit(r.Context(), h.store, AuditEvent{
			TenantID:   PlatformTenantID,
			Action:     "auth.login_rate_limited",
			TargetType: "user",
			Reason:     "too many failed login attempts",
			IPAddress:  clientIP,
			UserAgent:  r.UserAgent(),
			RequestID:  logger.RequestID(r.Context()),
		})
		httpx.Error(w, r, http.StatusTooManyRequests, "login_rate_limited", "too many failed login attempts; retry later")
		return
	}

	user, err := h.store.FindUserByLogin(r.Context(), req.TenantCode, req.Username)
	// Only an explicit credential miss is an authentication failure.  A
	// database/network error must not poison the login limiter or be reported
	// as a bad password, otherwise a dependency outage turns into a 429 lockout.
	if err != nil && !errors.Is(err, ErrInvalidCredentials) {
		httpx.Error(w, r, http.StatusServiceUnavailable, "auth_service_unavailable", "authentication service temporarily unavailable")
		return
	}
	if err != nil || !CheckPassword(user.PasswordHash, req.Password) {
		_, retryAfter, blocked := h.loginLimiter.RegisterFailure(r.Context(), limiterKey, time.Now().UTC())
		tenantID := user.TenantID
		if tenantID == "" {
			tenantID = PlatformTenantID
		}
		RecordAudit(r.Context(), h.store, AuditEvent{
			TenantID:   tenantID,
			ActorID:    user.ID,
			Action:     "auth.login_failed",
			TargetType: "user",
			TargetID:   user.ID,
			Reason:     "invalid credentials",
			IPAddress:  clientIP,
			UserAgent:  r.UserAgent(),
			RequestID:  logger.RequestID(r.Context()),
		})
		if blocked {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			RecordAudit(r.Context(), h.store, AuditEvent{
				TenantID:   tenantID,
				ActorID:    user.ID,
				Action:     "auth.login_rate_limited",
				TargetType: "user",
				TargetID:   user.ID,
				Reason:     "too many failed login attempts",
				IPAddress:  clientIP,
				UserAgent:  r.UserAgent(),
				RequestID:  logger.RequestID(r.Context()),
			})
			httpx.Error(w, r, http.StatusTooManyRequests, "login_rate_limited", "too many failed login attempts; retry later")
			return
		}
		httpx.Error(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid tenant, username or password")
		return
	}
	h.loginLimiter.Clear(r.Context(), limiterKey)
	resolvedScope, err := h.store.ResolveAccessScope(r.Context(), user.User)
	if err == nil {
		organizationScope := resolvedScope.OrganizationScope()
		user.OrganizationScope = &organizationScope
	}

	sessionType := SessionTypeStandard
	sessionTTL := h.sessionTTL
	persistentCookie := req.RememberDevice
	if tokenResponse {
		if req.ClientType == "" {
			// Compatibility for non-browser API clients migrating from the
			// former shared login endpoint. Browser code never calls /auth/token.
			req.ClientType = "desktop"
		}
		switch req.ClientType {
		case "desktop":
			if IsServiceUser(user.User) {
				httpx.Error(w, r, http.StatusForbidden, "client_type_forbidden", "client type is not allowed for this account")
				return
			}
			sessionType = SessionTypeDesktopDevice
		case "service":
			if !IsServiceUser(user.User) {
				httpx.Error(w, r, http.StatusForbidden, "client_type_forbidden", "client type is not allowed for this account")
				return
			}
			sessionType = SessionTypeService
		default:
			httpx.Error(w, r, http.StatusBadRequest, "client_type_required", "client_type must be desktop or service")
			return
		}
	} else {
		if IsServiceUser(user.User) {
			httpx.Error(w, r, http.StatusForbidden, "client_type_forbidden", "service accounts cannot create browser sessions")
			return
		}
		if req.RememberDevice {
			sessionType = SessionTypeRememberedDevice
			sessionTTL = h.rememberedTTL
		}
	}

	token, tokenHash, err := NewToken()
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "token_generation_failed", "failed to create session")
		return
	}
	_, deviceID, err := NewToken()
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "token_generation_failed", "failed to create session")
		return
	}
	expiresAt := time.Now().UTC().Add(sessionTTL)
	if _, err := h.store.CreateSession(r.Context(), CreateSessionInput{
		TenantID: user.TenantID, UserID: user.ID, TokenHash: tokenHash,
		SessionType: sessionType, DeviceID: deviceID,
		DeviceName:    normalizedDeviceName(req.DeviceName, sessionType),
		UserAgentHash: hashUserAgent(r.UserAgent()), IPPrefix: networkPrefix(clientIP),
		ExpiresAt: expiresAt,
	}); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "session_create_failed", "failed to create session")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     "auth.login_succeeded",
		TargetType: "user",
		TargetID:   user.ID,
		IPAddress:  clientIP,
		UserAgent:  r.UserAgent(),
		RequestID:  logger.RequestID(r.Context()),
	})

	if tokenResponse {
		httpx.JSON(w, http.StatusOK, map[string]any{
			"token_type": "Bearer", "access_token": token,
			"expires_at": expiresAt.Format(time.RFC3339), "user": user.User,
		})
		return
	}
	http.SetCookie(w, h.sessionCookie(token, expiresAt, persistentCookie))
	httpx.JSON(w, http.StatusOK, map[string]any{
		"expires_at": expiresAt.Format(time.RFC3339),
		"user":       user.User,
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *Handler) ListAudits(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	limit, err := pagination.Limit(r.URL.Query().Get("limit"), 50, 200)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_pagination", "limit must be between 1 and 200")
		return
	}
	filter, ok := h.auditFilterFromRequest(w, r, limit)
	if !ok {
		return
	}
	cursor, err := pagination.Decode(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_pagination", "cursor is invalid")
		return
	}
	filter.Limit = limit + 1
	filter.CursorCreatedAt = cursor.CreatedAt
	filter.CursorID = cursor.ID
	if scope, ok := AccessScopeFromContext(r.Context()); ok {
		filter.ScopeMode = scope.QueryMode()
	}
	if scope, ok := AccessScopeFromContext(r.Context()); ok {
		filter.ScopeMode = scope.QueryMode()
	}
	records, err := h.store.ListAudits(r.Context(), user.TenantID, filter)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "audit_lookup_failed", "failed to list audit logs")
		return
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	nextCursor := ""
	if hasMore && len(records) > 0 {
		last := records[len(records)-1]
		nextCursor = pagination.Encode(last.CreatedAt, last.ID)
	}
	records = RedactAuditRecords(records)
	httpx.JSON(w, http.StatusOK, map[string]any{"audit_logs": records, "next_cursor": nextCursor, "has_more": hasMore})
}

func (h *Handler) ExportAudits(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	filter, ok := h.auditFilterFromRequest(w, r, 200)
	if !ok {
		return
	}
	if scope, ok := AccessScopeFromContext(r.Context()); ok {
		filter.ScopeMode = scope.QueryMode()
	}
	if scope, ok := AccessScopeFromContext(r.Context()); ok {
		filter.ScopeMode = scope.QueryMode()
	}
	records, err := h.store.ListAudits(r.Context(), user.TenantID, filter)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "audit_export_failed", "failed to export audit logs")
		return
	}
	records = RedactAuditRecords(records)
	exportedAt := time.Now().UTC().Format(time.RFC3339)
	watermark := fmt.Sprintf("EduGrade audit export tenant=%s actor=%s at=%s", user.TenantID, user.ID, exportedAt)
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.Write([]string{"id", "tenant_id", "actor_id", "action", "target_type", "target_id", "before_value", "after_value", "reason", "ip_address", "user_agent", "request_id", "created_at", "watermark"})
	for _, record := range records {
		_ = writer.Write(csvsafe.Row([]string{
			record.ID,
			record.TenantID,
			record.ActorID,
			record.Action,
			record.TargetType,
			record.TargetID,
			jsonString(record.BeforeValue),
			jsonString(record.AfterValue),
			record.Reason,
			record.IPAddress,
			record.UserAgent,
			record.RequestID,
			record.CreatedAt.Format(time.RFC3339),
			watermark,
		}))
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "audit_export_failed", "failed to build audit export")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     "audit.exported",
		TargetType: "audit_log",
		Reason:     "export audit logs",
		IPAddress:  h.remoteIP(r),
		UserAgent:  r.UserAgent(),
		RequestID:  logger.RequestID(r.Context()),
	})
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
	w.Header().Set("X-EduGrade-Watermark", watermark)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

func (h *Handler) auditFilterFromRequest(w http.ResponseWriter, r *http.Request, defaultLimit int) (AuditFilter, bool) {
	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_audit_limit", "audit limit is invalid")
			return AuditFilter{}, false
		}
		limit = parsed
	}
	createdFrom, ok := parseAuditTime(w, r, "created_from")
	if !ok {
		return AuditFilter{}, false
	}
	createdTo, ok := parseAuditTime(w, r, "created_to")
	if !ok {
		return AuditFilter{}, false
	}
	return AuditFilter{
		Action:      r.URL.Query().Get("action"),
		ActorID:     r.URL.Query().Get("actor_id"),
		TargetType:  r.URL.Query().Get("target_type"),
		TargetID:    r.URL.Query().Get("target_id"),
		ExamID:      r.URL.Query().Get("exam_id"),
		IPAddress:   r.URL.Query().Get("ip_address"),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Limit:       limit,
	}, true
}

func parseAuditTime(w http.ResponseWriter, r *http.Request, key string) (time.Time, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" && key == "created_from" {
		raw = r.URL.Query().Get("from")
	}
	if raw == "" && key == "created_to" {
		raw = r.URL.Query().Get("to")
	}
	if raw == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		if dateOnly, dateErr := time.Parse("2006-01-02", raw); dateErr == nil {
			if key == "created_to" {
				return dateOnly.Add(24*time.Hour - time.Nanosecond), true
			}
			return dateOnly, true
		}
		httpx.Error(w, r, http.StatusBadRequest, "invalid_audit_time", "audit time filter is invalid")
		return time.Time{}, false
	}
	return parsed, true
}

func jsonString(value map[string]any) string {
	if len(value) == 0 {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r, h.cookieName)
	if token == "" {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	user, _ := UserFromContext(r.Context())
	if err := h.store.DeleteSession(r.Context(), HashToken(token), "user_logout"); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "logout_failed", "failed to logout")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     "auth.logout",
		TargetType: "user",
		TargetID:   user.ID,
		IPAddress:  h.remoteIP(r),
		UserAgent:  r.UserAgent(),
		RequestID:  logger.RequestID(r.Context()),
	})
	http.SetCookie(w, h.clearSessionCookie())
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok || IsServiceUser(user) {
		httpx.Error(w, r, http.StatusForbidden, "password_change_forbidden", "password change is only available to signed-in users")
		return
	}
	var input changePasswordRequest
	if err := decodeAuthJSON(w, r, &input, false); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if input.CurrentPassword == "" || input.NewPassword == "" || len(input.CurrentPassword) > maxPasswordBytes || len(input.NewPassword) > maxPasswordBytes {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "current_password and new_password are required")
		return
	}
	if !StrongPassword(input.NewPassword) {
		httpx.Error(w, r, http.StatusBadRequest, "weak_password", "new password must contain upper, lower, number and symbol and be at least 12 characters")
		return
	}
	current, err := h.store.FindUserByLogin(r.Context(), user.TenantCode, user.Username)
	if err != nil || !CheckPassword(current.PasswordHash, input.CurrentPassword) {
		httpx.Error(w, r, http.StatusUnauthorized, "current_password_invalid", "current password is incorrect")
		return
	}
	if CheckPassword(current.PasswordHash, input.NewPassword) {
		httpx.Error(w, r, http.StatusBadRequest, "password_unchanged", "new password must differ from the current password")
		return
	}
	newHash, err := HashPassword(input.NewPassword)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "password_hash_failed", "failed to update password")
		return
	}
	updated, revoked, err := h.store.UpdatePasswordAndRevokeSessions(r.Context(), user.TenantID, user.ID, current.PasswordHash, newHash)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "password_change_failed", "failed to update password")
		return
	}
	if !updated {
		httpx.Error(w, r, http.StatusConflict, "password_changed_concurrently", "password changed in another session; sign in again")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID: user.TenantID, ActorID: user.ID, Action: "auth.password_changed",
		TargetType: "user", TargetID: user.ID, Reason: "user changed password and revoked all sessions",
		IPAddress: h.remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
	})
	http.SetCookie(w, h.clearSessionCookie())
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "password_changed", "revoked_count": revoked})
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	token := sessionToken(r, h.cookieName)
	sessions, err := h.store.ListSessions(r.Context(), user.TenantID, user.ID, HashToken(token), time.Now().UTC())
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "session_lookup_failed", "failed to list sessions")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	sessionID := strings.TrimSpace(r.PathValue("id"))
	revoked, err := h.store.RevokeSession(r.Context(), user.TenantID, user.ID, sessionID, "user_revoked_device")
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "session_revoke_failed", "failed to revoke session")
		return
	}
	if !revoked {
		httpx.Error(w, r, http.StatusNotFound, "session_not_found", "session not found")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID: user.TenantID, ActorID: user.ID, Action: "auth.session_revoked",
		TargetType: "auth_session", TargetID: sessionID, Reason: "user revoked device session",
		IPAddress: h.remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
	})
	currentToken := sessionToken(r, h.cookieName)
	currentSessions, _ := h.store.ListSessions(r.Context(), user.TenantID, user.ID, HashToken(currentToken), time.Now().UTC())
	currentStillActive := false
	for _, session := range currentSessions {
		if session.Current {
			currentStillActive = true
			break
		}
	}
	if !currentStillActive {
		http.SetCookie(w, h.clearSessionCookie())
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "revoked"})
}

func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	revoked, err := h.store.RevokeAllSessions(r.Context(), user.TenantID, user.ID, "user_logout_all")
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "session_revoke_failed", "failed to revoke sessions")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID: user.TenantID, ActorID: user.ID, Action: "auth.sessions_revoked_all",
		TargetType: "user", TargetID: user.ID, Reason: "user logged out all devices",
		IPAddress: h.remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
	})
	http.SetCookie(w, h.clearSessionCookie())
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "logged_out_all", "revoked_count": revoked})
}

func (h *Handler) AdminRevokeUserSessions(w http.ResponseWriter, r *http.Request) {
	actor, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	targetUserID := strings.TrimSpace(r.PathValue("id"))
	revoked, err := h.store.RevokeAllSessions(r.Context(), actor.TenantID, targetUserID, "administrator_revoked_all")
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "session_revoke_failed", "failed to revoke sessions")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID: actor.TenantID, ActorID: actor.ID, Action: "auth.user_sessions_revoked",
		TargetType: "user", TargetID: targetUserID, Reason: "administrator revoked user sessions",
		IPAddress: h.remoteIP(r), UserAgent: r.UserAgent(), RequestID: logger.RequestID(r.Context()),
	})
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "revoked", "revoked_count": revoked})
}

func AuthMiddleware(store Store, options ...HandlerOptions) func(http.Handler) http.Handler {
	cookieName := DefaultSessionCookieName
	if len(options) > 0 && strings.TrimSpace(options[0].CookieName) != "" {
		cookieName = strings.TrimSpace(options[0].CookieName)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sessionToken(r, cookieName)
			if token == "" {
				httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			user, err := store.FindUserBySession(r.Context(), HashToken(token), time.Now().UTC())
			if err != nil {
				if errors.Is(err, ErrUnauthenticated) {
					httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
					return
				}
				httpx.Error(w, r, http.StatusInternalServerError, "auth_lookup_failed", "failed to authenticate request")
				return
			}
			scope, err := store.ResolveAccessScope(r.Context(), user)
			if err != nil {
				if errors.Is(err, ErrAccessScopeMissing) || errors.Is(err, ErrAccessScopeInvalid) {
					httpx.Error(w, r, http.StatusForbidden, "access_scope_missing", "no valid data access scope is assigned")
					return
				}
				httpx.Error(w, r, http.StatusInternalServerError, "access_scope_lookup_failed", "failed to resolve data access scope")
				return
			}
			// A human identity without any resolved data boundary is not an
			// authenticated-but-empty user.  It is an invalid authorization
			// state and must fail closed.  Service identities use the explicit
			// service scope and are handled by their worker-only routes.
			if !scope.HasDataAccess() && !scope.syntheticUnbounded && !IsServiceUser(user) {
				httpx.Error(w, r, http.StatusForbidden, "access_scope_missing", "no valid data access scope is assigned")
				return
			}
			organizationScope := scope.OrganizationScope()
			user.OrganizationScope = &organizationScope
			ctx := WithAccessScope(WithUser(r.Context(), user), scope)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (h *Handler) sessionCookie(token string, expiresAt time.Time, persistent bool) *http.Cookie {
	cookie := &http.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/api/v1",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if persistent {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(time.Until(expiresAt).Seconds())
	}
	return cookie
}

func (h *Handler) clearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/api/v1",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func normalizedDeviceName(value string, sessionType string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		value = value[:128]
	}
	if value != "" {
		return value
	}
	switch sessionType {
	case SessionTypeService:
		return "后台服务"
	case SessionTypeDesktopDevice:
		return "桌面客户端"
	default:
		return "浏览器"
	}
}

func hashUserAgent(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

func networkPrefix(value string) string {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return ""
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return (&net.IPNet{IP: ipv4.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}).String()
	}
	return (&net.IPNet{IP: ip.Mask(net.CIDRMask(64, 128)), Mask: net.CIDRMask(64, 128)}).String()
}

func IsServiceUser(user User) bool {
	for _, role := range user.Roles {
		if role == "page_processing_worker" || strings.HasSuffix(role, "_worker") {
			return true
		}
	}
	return false
}

func sessionToken(r *http.Request, cookieName string) string {
	if token := bearerToken(r); token != "" {
		return token
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			if HasPermission(user, permission) {
				next.ServeHTTP(w, r)
				return
			}
			httpx.Error(w, r, http.StatusForbidden, "forbidden", "missing permission: "+permission)
		})
	}
}

func RequireAnyPermission(permissions ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			for _, permission := range permissions {
				if HasPermission(user, permission) {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.Error(w, r, http.StatusForbidden, "forbidden", "missing required permission")
		})
	}
}

func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			for _, role := range roles {
				if HasRole(user, role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.Error(w, r, http.StatusForbidden, "forbidden", "missing required role")
		})
	}
}

func HasPermission(user User, permission string) bool {
	for _, current := range user.Permissions {
		if current == permission {
			return true
		}
	}
	return false
}

func HasRole(user User, role string) bool {
	for _, current := range user.Roles {
		if current == role {
			return true
		}
	}
	return false
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (h *Handler) remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer == nil || !trustedIP(peer, h.trustedProxies) {
		return host
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	current := peer
	for index := len(forwarded) - 1; index >= 0 && trustedIP(current, h.trustedProxies); index-- {
		next := net.ParseIP(strings.TrimSpace(forwarded[index]))
		if next == nil {
			return host
		}
		current = next
	}
	return current.String()
}

func parseTrustedProxyCIDRs(values []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(strings.TrimSpace(value))
		if err == nil {
			out = append(out, network)
		}
	}
	return out
}

func trustedIP(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
