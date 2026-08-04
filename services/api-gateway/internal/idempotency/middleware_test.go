package idempotency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestMiddlewareReplaysAndRejectsChangedRequest(t *testing.T) {
	store := NewMemoryStore()
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/exams", Middleware(store, Options{Enforce: true})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"exam-1"}`))
	})))
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/exams", strings.NewReader(body))
		req.Header.Set(Header, "request-1")
		req = req.WithContext(auth.WithUser(context.Background(), auth.User{ID: "actor-1", TenantID: "tenant-1"}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	first := request(`{"name":"A"}`)
	second := request(`{"name":"A"}`)
	conflict := request(`{"name":"B"}`)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated || second.Header().Get("Idempotency-Replayed") != "true" || calls.Load() != 1 {
		t.Fatalf("unexpected replay: first=%d second=%d calls=%d", first.Code, second.Code, calls.Load())
	}
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "idempotency_key_reused_with_different_request") {
		t.Fatalf("changed request must conflict: %d %s", conflict.Code, conflict.Body.String())
	}
}
