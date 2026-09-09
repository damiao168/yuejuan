package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryExportsBoundedRouteMetrics(t *testing.T) {
	registry := NewRegistry()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := registry.Middleware()(mux)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/secret-student-id", nil))

	recorder := httptest.NewRecorder()
	registry.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `route="GET /items/{id}"`) || strings.Contains(body, "secret-student-id") {
		t.Fatalf("metrics must use the matched route pattern without resource identifiers: %s", body)
	}
	if !strings.Contains(body, "edugrade_http_request_duration_seconds_bucket") {
		t.Fatalf("duration histogram missing: %s", body)
	}
}

func TestRegistryExportsErrorCodesAndRecoveryOutcomes(t *testing.T) {
	registry := NewRegistry()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /callbacks/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-EduGrade-Error-Code", "worker_task_lease_mismatch")
		w.WriteHeader(http.StatusConflict)
	})
	mux.HandleFunc("GET /commands/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-EduGrade-Operation-Outcome", "succeeded")
		w.WriteHeader(http.StatusOK)
	})
	handler := registry.Middleware()(mux)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/callbacks/private-task", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/commands/private-command", nil))

	recorder := httptest.NewRecorder()
	registry.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `edugrade_http_errors_total{method="POST",route="POST /callbacks/{id}",error_code="worker_task_lease_mismatch"} 1`) {
		t.Fatalf("lease rejection counter missing: %s", body)
	}
	if !strings.Contains(body, `edugrade_operation_outcomes_total{method="GET",route="GET /commands/{id}",outcome="succeeded"} 1`) {
		t.Fatalf("command recovery outcome counter missing: %s", body)
	}
	if strings.Contains(body, "private-task") || strings.Contains(body, "private-command") {
		t.Fatalf("metrics must not contain resource identifiers: %s", body)
	}
}
