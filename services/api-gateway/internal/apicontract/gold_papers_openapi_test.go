package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoryA08GoldPapersOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi", "edugrade-api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("OpenAPI contract must be valid JSON: %v", err)
	}
	paths := object(t, document, "paths")
	operations := []struct{ path, method, id string }{
		{"/api/v1/exams/{examId}/questions/{questionId}/gold-papers", "post", "nominateGoldPaper"},
		{"/api/v1/exams/{examId}/questions/{questionId}/gold-coverage", "get", "getGoldCoverage"},
		{"/api/v1/gold-papers", "get", "listGoldPapers"},
		{"/api/v1/gold-papers/{goldPaperId}", "get", "getGoldPaper"},
		{"/api/v1/gold-papers/{goldPaperId}/versions", "post", "createGoldPaperVersion"},
		{"/api/v1/gold-papers/{goldPaperId}/versions/{version}/approve", "post", "approveGoldPaperVersion"},
		{"/api/v1/gold-papers/{goldPaperId}/retire", "post", "retireGoldPaper"},
	}
	for _, expected := range operations {
		operation := object(t, object(t, paths, expected.path), expected.method)
		if operation["operationId"] != expected.id {
			t.Fatalf("%s %s operationId = %#v, want %q", expected.method, expected.path, operation["operationId"], expected.id)
		}
	}

	schemas := object(t, object(t, document, "components"), "schemas")
	assertRequired(t, object(t, schemas, "GoldPaperVersion"), []string{"reference_score", "rubric_snapshot", "explanation", "trait_scores", "source_grade_ids"})
	assertRequired(t, object(t, schemas, "CreateGoldPaperVersionRequest"), []string{"reference_score", "explanation", "source_grade_ids"})
	assertRequired(t, object(t, schemas, "GoldCoverage"), []string{"score_bands", "trait_patterns", "error_tags", "gaps", "ready"})
}
