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

func TestExpiredProcessingReservationRetainsOriginalFingerprint(t *testing.T) {
	store := NewMemoryStore()
	input := BeginInput{
		TenantID: "tenant-1", ActorID: "actor-1", Method: http.MethodPost, Route: "/api/v1/exams/{examId}/publish",
		Key: "stable-command", RequestHash: "original-hash", ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}
	if _, execute, err := store.Begin(context.Background(), input); err != nil || !execute {
		t.Fatalf("initial begin: execute=%v err=%v", execute, err)
	}
	changed := input
	changed.RequestHash = "changed-hash"
	changed.ExpiresAt = time.Now().UTC().Add(time.Hour)
	changed.AllowTakeover = true
	changed.StaleBefore = time.Now().UTC().Add(time.Minute)
	if _, _, err := store.Begin(context.Background(), changed); !errors.Is(err, ErrKeyConflict) {
		t.Fatalf("expired processing fingerprint was discarded: %v", err)
	}
	input.ExpiresAt = changed.ExpiresAt
	input.AllowTakeover = true
	input.StaleBefore = changed.StaleBefore
	if _, execute, err := store.Begin(context.Background(), input); err != nil || !execute {
		t.Fatalf("original frozen request could not take over: execute=%v err=%v", execute, err)
	}
}

func TestExpiredNonRecoverableReservationKeepsLegacyTTLBehavior(t *testing.T) {
	store := NewMemoryStore()
	input := BeginInput{
		TenantID: "tenant-1", ActorID: "actor-1", Method: http.MethodPost, Route: "/api/v1/files",
		Key: "legacy-command", RequestHash: "original", ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}
	if _, execute, err := store.Begin(context.Background(), input); err != nil || !execute {
		t.Fatalf("initial legacy begin: execute=%v err=%v", execute, err)
	}
	input.RequestHash = "replacement"
	input.ExpiresAt = time.Now().UTC().Add(time.Hour)
	if _, execute, err := store.Begin(context.Background(), input); err != nil || !execute {
		t.Fatalf("expired non-recoverable reservation did not retain TTL behavior: execute=%v err=%v", execute, err)
	}
}

func TestRecoverableRoutesAreExactClosedBusinessCommands(t *testing.T) {
	want := []string{
		"POST /api/v1/exam-sessions",
		"POST /api/v1/exams/{examId}/capture-batches",
		"POST /api/v1/exams/{examId}/scoring-runs",
		"POST /api/v1/subjective-grading-batches",
		"POST /api/v1/subjective-grading-batches/{batchId}/enqueue",
		"POST /api/v1/review-tasks/{id}/submit",
		"POST /api/v1/arbitration-tasks/{id}/submit",
		"POST /api/v1/exams/{examId}/confirm-grades",
		"POST /api/v1/exams/{examId}/publish",
		"POST /api/v1/exams/{examId}/reports/export",
	}
	if len(recoverableRoutes) != len(want) {
		t.Fatalf("recoverable route allowlist changed without an explicit closed-command review: got=%d want=%d", len(recoverableRoutes), len(want))
	}
	for _, route := range want {
		if !recoverableRoutes[route] {
			t.Fatalf("closed business command missing from stale takeover allowlist: %s", route)
		}
	}
}
