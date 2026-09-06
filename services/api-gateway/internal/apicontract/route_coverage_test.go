package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type routeCoverageDocument struct {
	Counts struct {
		Registered     int `json:"registered"`
		OpenAPI        int `json:"openapi"`
		RegisteredGaps int `json:"registered_gaps"`
	} `json:"counts"`
	Routes []struct {
		Method      string `json:"method"`
		Path        string `json:"path"`
		Source      string `json:"source"`
		Coverage    string `json:"coverage"`
		OperationID string `json:"operation_id"`
		Reason      string `json:"reason"`
	} `json:"routes"`
}

func TestEveryRegisteredRouteHasContractDisposition(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi", "route-coverage.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document routeCoverageDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("route coverage must be valid JSON: %v", err)
	}
	if len(document.Routes) != document.Counts.Registered || document.Counts.OpenAPI+document.Counts.RegisteredGaps != document.Counts.Registered {
		t.Fatalf("route coverage counts are inconsistent: %#v", document.Counts)
	}
	seen := make(map[string]bool, len(document.Routes))
	for _, route := range document.Routes {
		key := route.Method + " " + route.Path
		if route.Method == "" || route.Path == "" || route.Source == "" || seen[key] {
			t.Fatalf("invalid or duplicate route coverage entry: %#v", route)
		}
		seen[key] = true
		switch route.Coverage {
		case "openapi":
			if route.OperationID == "" {
				t.Fatalf("OpenAPI route %s has no operation_id", key)
			}
		case "registered-gap":
			if route.Reason == "" {
				t.Fatalf("uncontracted route %s has no explicit reason", key)
			}
		default:
			t.Fatalf("route %s has unknown coverage disposition %q", key, route.Coverage)
		}
	}
}
