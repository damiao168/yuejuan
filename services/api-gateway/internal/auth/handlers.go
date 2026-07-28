package auth

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store        Store
	sessionTTL   time.Duration
	loginLimiter *LoginFailureLimiter
	cookieName   string
	cookieSecure bool
}

type HandlerOptions struct {
	LoginFailureLimit  int
	LoginFailureWindow time.Duration
	CookieName         string
	CookieSecure       bool
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
		if strings.TrimSpace(options[0].CookieName) != "" {
			cfg.CookieName = strings.TrimSpace(options[0].CookieName)
		}
		cfg.CookieSecure = options[0].CookieSecure
	}
	if cfg.CookieName == "" {
		cfg.CookieName = DefaultSessionCookieName
	}
	return &Handler{
		store:        store,
		sessionTTL:   sessionTTL,
		loginLimiter: NewLoginFailureLimiter(cfg.LoginFailureLimit, cfg.LoginFailureWindow),
		cookieName:   cfg.CookieName,
		cookieSecure: cfg.CookieSecure,
	}
}

type loginRequest struct {
	TenantCode string `json:"tenant_code"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
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
	if req.TenantCode == "" || req.Username == "" || req.Password == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "tenant_code, username and password are required")
		return
	}
	if !loginFieldsWithinLimits(req.TenantCode, req.Username, req.Password) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "login fields exceed supported size limits")
		return
	}
	clientIP := remoteIP(r)
	limiterKey := LoginFailureKey(req.TenantCode, req.Username, clientIP)
	if retryAfter, blocked := h.loginLimiter.IsBlocked(limiterKey, time.Now().UTC()); blocked {
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
		_, retryAfter, blocked := h.loginLimiter.RegisterFailure(limiterKey, time.Now().UTC())
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
	h.loginLimiter.Clear(limiterKey)

	token, tokenHash, err := NewToken()
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "token_generation_failed", "failed to create session")
		return
	}
	expiresAt := time.Now().UTC().Add(h.sessionTTL)
	if err := h.store.CreateSession(r.Context(), user.TenantID, user.ID, tokenHash, expiresAt); err != nil {
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

	http.SetCookie(w, h.sessionCookie(token, expiresAt))
	httpx.JSON(w, http.StatusOK, map[string]any{
		"token_type":   "Bearer",
		"access_token": token,
		"expires_at":   expiresAt.Format(time.RFC3339),
		"user":         user.User,
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
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "limit must be a number")
			return
		}
		limit = parsed
	}
	filter, ok := h.auditFilterFromRequest(w, r, limit)
	if !ok {
		return
	}
	records, err := h.store.ListAudits(r.Context(), user.TenantID, filter)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "audit_lookup_failed", "failed to list audit logs")
		return
	}
	records = RedactAuditRecords(records)
	httpx.JSON(w, http.StatusOK, map[string]any{"audit_logs": records})
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
		_ = writer.Write([]string{
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
		})
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
		IPAddress:  remoteIP(r),
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
	if err := h.store.DeleteSession(r.Context(), HashToken(token)); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "logout_failed", "failed to logout")
		return
	}
	RecordAudit(r.Context(), h.store, AuditEvent{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     "auth.logout",
		TargetType: "user",
		TargetID:   user.ID,
		IPAddress:  remoteIP(r),
		UserAgent:  r.UserAgent(),
		RequestID:  logger.RequestID(r.Context()),
	})
	http.SetCookie(w, h.clearSessionCookie())
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
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
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}

func (h *Handler) sessionCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) clearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
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

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
