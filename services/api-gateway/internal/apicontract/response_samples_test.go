package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/processing"
)

func TestSharedResponseSamplesMatchGoTransportTypes(t *testing.T) {
	samples := filepath.Join("..", "..", "..", "..", "contracts", "samples")
	t.Run("error envelope", func(t *testing.T) {
		var response httpx.ErrorResponse
		readResponseSample(t, filepath.Join(samples, "error.response.json"), &response)
		if response.RequestID == "" || response.TraceID == "" || response.Error.Code != "revision_conflict" {
			t.Fatalf("error sample lost diagnostic context: %#v", response)
		}
	})
	t.Run("processing summary", func(t *testing.T) {
		var response struct {
			Summary processing.Summary `json:"summary"`
		}
		readResponseSample(t, filepath.Join(samples, "processing-summary.response.json"), &response)
		if response.Summary.TotalPages != 500 || response.Summary.ReadyPages != 480 || response.Summary.GeneratedAt.IsZero() {
			t.Fatalf("processing sample does not match the production response: %#v", response.Summary)
		}
	})
}

func readResponseSample(t *testing.T, path string, target any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
