package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/server"
)

func newTestStore(t *testing.T) *auth.MemoryStore {
	t.Helper()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := auth.NewMemoryStore()
	store.AddUser(auth.UserWithPassword{
		User: auth.User{
			ID:          "u-1",
			TenantID:    "t-1",
			TenantCode:  "demo",
			Username:    "teacher",
			DisplayName: "Teacher",
			Status:      "active",
			Roles:       []string{"teacher"},
			Permissions: []string{"system:read", "grading:review", "exam:manage"},
			DataScope:   map[string]any{"scope": "school"},
		},
		PasswordHash: hash,
	})
	return store
}

func newTestRouter(store *auth.MemoryStore) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{
			Name:             "test",
			Environment:      "test",
			ReadinessTimeout: time.Millisecond,
		},
		Auth: config.AuthConfig{SessionTTL: time.Hour},
	}
	return server.NewRouter(cfg, logger.New(io.Discard, "error"), nil, store, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore())
}

func newTestRouterWithConfig(store *auth.MemoryStore, cfg config.Config) http.Handler {
	return server.NewRouter(cfg, logger.New(io.Discard, "error"), nil, store, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore())
}

func TestLoginMeLogout(t *testing.T) {
	store := newTestStore(t)
	router := newTestRouter(store)

	token := login(t, router, "demo", "teacher", "ChangeMe123!")

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d: %s", logoutRec.Code, logoutRec.Body.String())
	}

	afterLogout := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	afterLogout.Header.Set("Authorization", "Bearer "+token)
	afterLogoutRec := httptest.NewRecorder()
	router.ServeHTTP(afterLogoutRec, afterLogout)
	if afterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout expected 401, got %d", afterLogoutRec.Code)
	}
}

func TestLoginSetsHttpOnlySessionCookieAndMeAcceptsCookie(t *testing.T) {
	store := newTestStore(t)
	router := newTestRouter(store)

	loginRec := loginResponse(t, router, "demo", "teacher", "ChangeMe123!")
	cookie := sessionCookie(t, loginRec)
	if !cookie.HttpOnly {
		t.Fatalf("session cookie must be HttpOnly: %#v", cookie)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie must use SameSite=Lax, got %v", cookie.SameSite)
	}
	if cookie.Value == "" {
		t.Fatal("session cookie must contain token value")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(cookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me with cookie expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
}

func TestLogoutClearsSessionCookieAndRevokesCookieSession(t *testing.T) {
	store := newTestStore(t)
	router := newTestRouter(store)

	loginRec := loginResponse(t, router, "demo", "teacher", "ChangeMe123!")
	cookie := sessionCookie(t, loginRec)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d: %s", logoutRec.Code, logoutRec.Body.String())
	}
	cleared := sessionCookie(t, logoutRec)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("logout must clear session cookie, got %#v", cleared)
	}

	afterLogout := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	afterLogout.AddCookie(cookie)
	afterLogoutRec := httptest.NewRecorder()
	router.ServeHTTP(afterLogoutRec, afterLogout)
	if afterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("me after cookie logout expected 401, got %d", afterLogoutRec.Code)
	}
}

func TestLoginFailureAudited(t *testing.T) {
	store := newTestStore(t)
	router := newTestRouter(store)

	body := bytes.NewBufferString(`{"tenant_code":"demo","username":"teacher","password":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	audits := store.Audits()
	if len(audits) != 1 || audits[0].Action != "auth.login_failed" {
		t.Fatalf("expected login failure audit, got %#v", audits)
	}
	if audits[0].IPAddress == "203.0.113.99" {
		t.Fatalf("login audit must not trust unverified X-Forwarded-For: %#v", audits[0])
	}
}

func TestLoginFailureRateLimited(t *testing.T) {
	store := newTestStore(t)
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth: config.AuthConfig{
			SessionTTL:         time.Hour,
			LoginFailureLimit:  2,
			LoginFailureWindow: time.Hour,
		},
	}
	router := newTestRouterWithConfig(store, cfg)

	for i, expected := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusTooManyRequests} {
		body := bytes.NewBufferString(`{"tenant_code":"demo","username":"teacher","password":"bad"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != expected {
			t.Fatalf("attempt %d expected %d, got %d %s", i+1, expected, rec.Code, rec.Body.String())
		}
		if expected == http.StatusTooManyRequests && rec.Header().Get("Retry-After") == "" {
			t.Fatalf("rate limited response must include Retry-After")
		}
	}
	audits := store.Audits()
	found := false
	for _, audit := range audits {
		if audit.Action == "auth.login_rate_limited" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected auth.login_rate_limited audit, got %#v", audits)
	}
}

func TestListAuditsFiltersByTarget(t *testing.T) {
	store := newTestStore(t)
	addAuditUser(t, store)
	router := newTestRouter(store)
	_ = store.Audit(context.Background(), auth.AuditEvent{
		TenantID:   "t-1",
		ActorID:    "u-1",
		Action:     "arbitration.submitted",
		TargetType: "arbitration_task",
		TargetID:   "task-1",
		Reason:     "submit arbitration final score",
	})
	_ = store.Audit(context.Background(), auth.AuditEvent{
		TenantID:   "t-1",
		ActorID:    "u-1",
		Action:     "final_grade.created",
		TargetType: "final_grade",
		TargetID:   "grade-1",
		Reason:     "create final grade from arbitration",
	})

	token := login(t, router, "demo", "auditor", "ChangeMe123!")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?target_type=arbitration_task&target_id=task-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("audit list expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		AuditLogs []auth.AuditRecord `json:"audit_logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	if len(response.AuditLogs) != 1 {
		t.Fatalf("expected one filtered audit log, got %#v", response.AuditLogs)
	}
	if response.AuditLogs[0].Action != "arbitration.submitted" || response.AuditLogs[0].TargetID != "task-1" {
		t.Fatalf("unexpected audit log: %#v", response.AuditLogs[0])
	}
}

func TestListAuditsFiltersByIPAndIncludesValues(t *testing.T) {
	store := newTestStore(t)
	addAuditUser(t, store)
	router := newTestRouter(store)
	_ = store.Audit(context.Background(), auth.AuditEvent{
		TenantID:    "t-1",
		ActorID:     "u-1",
		Action:      "exam.updated",
		TargetType:  "exam",
		TargetID:    "exam-1",
		BeforeValue: map[string]any{"status": "draft"},
		AfterValue:  map[string]any{"status": "configured"},
		Reason:      "update exam status",
		IPAddress:   "10.0.0.8",
	})
	_ = store.Audit(context.Background(), auth.AuditEvent{
		TenantID:   "t-1",
		ActorID:    "u-1",
		Action:     "exam.updated",
		TargetType: "exam",
		TargetID:   "exam-2",
		Reason:     "update another exam",
		IPAddress:  "10.0.0.9",
	})

	token := login(t, router, "demo", "auditor", "ChangeMe123!")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?action=exam.updated&exam_id=exam-1&ip_address=10.0.0.8", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("audit list expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		AuditLogs []auth.AuditRecord `json:"audit_logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	if len(response.AuditLogs) != 1 {
		t.Fatalf("expected one filtered audit log, got %#v", response.AuditLogs)
	}
	if response.AuditLogs[0].BeforeValue["status"] != "draft" || response.AuditLogs[0].AfterValue["status"] != "configured" {
		t.Fatalf("expected before/after values, got %#v", response.AuditLogs[0])
	}
}

func TestAuditListAndExportRedactSensitiveFields(t *testing.T) {
	store := newTestStore(t)
	addAuditUser(t, store)
	router := newTestRouter(store)
	_ = store.Audit(context.Background(), auth.AuditEvent{
		TenantID:   "t-1",
		ActorID:    "u-1",
		Action:     "user.updated",
		TargetType: "user",
		TargetID:   "u-1",
		BeforeValue: map[string]any{
			"password": "plain-text-should-not-leak",
			"profile":  map[string]any{"api_token": "token-should-not-leak", "display_name": "Teacher"},
		},
		AfterValue: map[string]any{"secret": "secret-should-not-leak", "status": "active"},
		Reason:     "update user",
	})

	token := login(t, router, "demo", "auditor", "ChangeMe123!")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?action=user.updated", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit list expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"plain-text-should-not-leak", "token-should-not-leak", "secret-should-not-leak"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("audit list leaked %s: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "[REDACTED]") || !strings.Contains(body, "Teacher") {
		t.Fatalf("audit list should redact sensitive fields and preserve non-sensitive context: %s", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/audit-logs/export?action=user.updated", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit export expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	for _, forbidden := range []string{"plain-text-should-not-leak", "token-should-not-leak", "secret-should-not-leak"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("audit export leaked %s: %s", forbidden, body)
		}
	}
}

func TestExportAuditsWritesAuditAndWatermark(t *testing.T) {
	store := newTestStore(t)
	addAuditUser(t, store)
	router := newTestRouter(store)
	_ = store.Audit(context.Background(), auth.AuditEvent{
		TenantID:   "t-1",
		ActorID:    "u-1",
		Action:     "score.published",
		TargetType: "exam",
		TargetID:   "exam-1",
		Reason:     "publish scores",
		IPAddress:  "10.0.0.8",
	})

	token := login(t, router, "demo", "auditor", "ChangeMe123!")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit-logs/export?action=score.published&limit=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("audit export expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-EduGrade-Watermark") == "" {
		t.Fatalf("expected watermark header")
	}
	if !strings.Contains(rec.Body.String(), "score.published") || !strings.Contains(rec.Body.String(), "EduGrade audit export") {
		t.Fatalf("expected CSV body with audit row and watermark, got %s", rec.Body.String())
	}
	audits := store.Audits()
	if len(audits) == 0 || audits[len(audits)-1].Action != "audit.exported" {
		t.Fatalf("expected audit.exported event, got %#v", audits)
	}
}

func TestAuditLogsCannotBeDeletedThroughOrdinaryAPI(t *testing.T) {
	store := newTestStore(t)
	addAuditUser(t, store)
	router := newTestRouter(store)
	_ = store.Audit(context.Background(), auth.AuditEvent{
		TenantID:   "t-1",
		ActorID:    "u-1",
		Action:     "score.exported",
		TargetType: "exam",
		TargetID:   "exam-1",
		Reason:     "export grades",
	})

	token := login(t, router, "demo", "auditor", "ChangeMe123!")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/audit-logs/memory-audit-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("audit delete route must not exist, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestListAuditsRequiresPermission(t *testing.T) {
	store := newTestStore(t)
	router := newTestRouter(store)
	token := login(t, router, "demo", "teacher", "ChangeMe123!")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMeRequiresAuthentication(t *testing.T) {
	router := newTestRouter(newTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestPermissionMiddlewareRejectsMissingPermission(t *testing.T) {
	store := newTestStore(t)
	token, tokenHash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if err := store.CreateSession(context.Background(), "t-1", "u-1", tokenHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	handler := auth.AuthMiddleware(store)(auth.RequirePermission("audit:read")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func login(t *testing.T, router http.Handler, tenant string, username string, password string) string {
	t.Helper()
	rec := loginResponse(t, router, tenant, username, password)
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		t.Fatal("expected access token")
	}
	return response.AccessToken
}

func loginResponse(t *testing.T, router http.Handler, tenant string, username string, password string) *httptest.ResponseRecorder {
	t.Helper()
	payload := map[string]string{"tenant_code": tenant, "username": username, "password": password}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == auth.DefaultSessionCookieName {
			return cookie
		}
	}
	t.Fatalf("expected %s cookie, got %#v", auth.DefaultSessionCookieName, rec.Result().Cookies())
	return nil
}

func addAuditUser(t *testing.T, store *auth.MemoryStore) {
	t.Helper()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store.AddUser(auth.UserWithPassword{
		User: auth.User{
			ID:          "u-2",
			TenantID:    "t-1",
			TenantCode:  "demo",
			Username:    "auditor",
			DisplayName: "Auditor",
			Status:      "active",
			Roles:       []string{"auditor"},
			Permissions: []string{"audit:read", "audit:export"},
			DataScope:   map[string]any{"scope": "tenant"},
		},
		PasswordHash: hash,
	})
}
