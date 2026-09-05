package paper

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPaperParserResolvesSchoolModelOnlyForInternalRequest(t *testing.T) {
	var received documentParseRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 32) {
			t.Error("missing internal service authentication")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"documents":[],"question_candidates":[],"answer_candidates":[],"solution_candidates":[],"rubric_candidates":[],"issues":[]}`))
	}))
	defer server.Close()
	service := NewDocumentImportService(nil, nil, nil, server.URL, strings.Repeat("t", 32), time.Second).
		WithModelResolver(func(_ context.Context, tenantID string) (*DocumentModelConfig, error) {
			if tenantID != "school-a" {
				t.Fatalf("wrong tenant: %s", tenantID)
			}
			return &DocumentModelConfig{AdapterType: "openai_compatible", BaseURL: "https://provider.test/v1", APIKey: "synthetic-secret", ModelName: "school-a-model", ModelVersion: "v1"}, nil
		})
	result, err := service.parseForTenant(context.Background(), "school-a", "request", "math", nil)
	if err != nil {
		t.Fatal(err)
	}
	if received.ManagedModel == nil || received.ManagedModel.ModelName != "school-a-model" {
		t.Fatal("school configuration was not passed to the parser")
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "synthetic-secret") {
		t.Fatal("model credential leaked into paper result")
	}
	service.WithModelResolver(func(context.Context, string) (*DocumentModelConfig, error) {
		return nil, errors.New("secret backend down")
	})
	if _, err = service.parseForTenant(context.Background(), "school-a", "request", "math", nil); err == nil {
		t.Fatal("invalid school configuration must not fall back to global model")
	}
}
