package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsCredentialsForAllowedOrigin(t *testing.T) {
	handler := CORS(
		[]string{"http://127.0.0.1:5173"},
		[]string{"GET", "POST", "OPTIONS"},
		[]string{"Content-Type"},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("allowed CORS origin must allow credentials, headers=%v", rec.Header())
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, "+CSRFHeaderName {
		t.Fatalf("CORS must always advertise the browser CSRF header, got %q", got)
	}
}

func TestBrowserCSRFRejectsCookieMutationWithoutHeader(t *testing.T) {
	handler := BrowserCSRF("edugrade_session")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/exams", nil)
	req.AddCookie(&http.Cookie{Name: "edugrade_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cookie mutation without CSRF header expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBrowserCSRFDoesNotExemptCookieFallbackForInvalidAuthorization(t *testing.T) {
	for _, authorization := range []string{"Basic unrelated", "Bearer ", "not-bearer", " Bearer token"} {
		handler := BrowserCSRF("edugrade_session")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/totp/enroll", nil)
		req.AddCookie(&http.Cookie{Name: "edugrade_session", Value: "session"})
		req.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusForbidden {
			t.Fatal("invalid Authorization bypassed cookie CSRF protection")
		}
	}
}

func TestBrowserCSRFAcceptsProtectedBrowserMutation(t *testing.T) {
	handler := BrowserCSRF("edugrade_session")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/other", nil)
	req.AddCookie(&http.Cookie{Name: "edugrade_session", Value: "session-token"})
	req.Header.Set(CSRFHeaderName, "1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("protected browser mutation expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBrowserCSRFRejectsCrossOriginLoginForm(t *testing.T) {
	handler := BrowserCSRF("edugrade_session")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://attacker.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin login form expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBrowserCSRFLeavesBearerAndReadRequestsCompatible(t *testing.T) {
	called := 0
	handler := BrowserCSRF("edugrade_session")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}))

	bearerReq := httptest.NewRequest(http.MethodPost, "/api/v1/internal/worker/tasks/claim", nil)
	bearerReq.Header.Set("Authorization", "Bearer worker-token")
	bearerRec := httptest.NewRecorder()
	handler.ServeHTTP(bearerRec, bearerReq)

	readReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	readReq.AddCookie(&http.Cookie{Name: "edugrade_session", Value: "session-token"})
	readRec := httptest.NewRecorder()
	handler.ServeHTTP(readRec, readReq)

	if bearerRec.Code != http.StatusNoContent || readRec.Code != http.StatusNoContent || called != 2 {
		t.Fatalf("bearer and read requests must remain compatible: bearer=%d read=%d called=%d", bearerRec.Code, readRec.Code, called)
	}
}
