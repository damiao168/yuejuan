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
}
