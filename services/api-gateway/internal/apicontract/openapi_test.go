package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCorePilotOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi", "edugrade-api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("OpenAPI contract must be valid JSON: %v", err)
	}
	if document["openapi"] != "3.1.0" || document["x-edugrade-coverage"] != "core-pilot-partial" {
		t.Fatalf("contract must declare OpenAPI 3.1 and honest partial coverage: %#v", document)
	}
	paths := object(t, document, "paths")
	operationIDs := map[string]bool{}
	for _, path := range []string{"/api/v1/exams", "/api/v1/exams/{examId}/submissions", "/api/v1/review-tasks", "/api/v1/appeals"} {
		operation := object(t, object(t, paths, path), "get")
		operationID, ok := operation["operationId"].(string)
		if !ok || operationID == "" || operationIDs[operationID] {
			t.Fatalf("operationId must be present and unique for %s", path)
		}
		operationIDs[operationID] = true
		parameters, ok := operation["parameters"].([]any)
		if !ok || !hasRef(parameters, "#/components/parameters/Limit") || !hasRef(parameters, "#/components/parameters/Cursor") {
			t.Fatalf("cursor list %s must use the common limit and cursor contract", path)
		}
		responses := object(t, operation, "responses")
		for _, status := range []string{"200", "400", "401", "403"} {
			if _, ok := responses[status]; !ok {
				t.Fatalf("%s is missing response %s", path, status)
			}
		}
	}
	if _, ok := paths["/api/v1/subjective-grading-batches/{batchId}/enqueue"]; !ok {
		t.Fatal("subjective batch partial enqueue semantics must remain contracted")
	}
}

func object(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	object, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("%s must be an object", key)
	}
	return object
}

func hasRef(values []any, expected string) bool {
	for _, value := range values {
		if object, ok := value.(map[string]any); ok && object["$ref"] == expected {
			return true
		}
	}
	return false
}
