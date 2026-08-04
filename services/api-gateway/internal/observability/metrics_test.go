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
