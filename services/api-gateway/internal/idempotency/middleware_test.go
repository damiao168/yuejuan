package idempotency

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestMemoryStoreAllowsOnlyExplicitStaleTakeover(t *testing.T) {
	store := NewMemoryStore()
	input := BeginInput{
		TenantID: "tenant-1", ActorID: "actor-1", Method: http.MethodPost, Route: "/api/v1/exam-sessions",
		Key: "stable-command-1", RequestHash: "hash-1", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if _, execute, err := store.Begin(context.Background(), input); err != nil || !execute {
		t.Fatalf("initial begin: execute=%v err=%v", execute, err)
	}
	if _, _, err := store.Begin(context.Background(), input); !errors.Is(err, ErrInProgress) {
		t.Fatalf("active reservation must not be stolen: %v", err)
	}
	input.AllowTakeover = true
	input.StaleBefore = time.Now().UTC().Add(time.Minute)
	if _, execute, err := store.Begin(context.Background(), input); err != nil || !execute {
		t.Fatalf("explicit stale reservation should be recoverable: execute=%v err=%v", execute, err)
	}
}
