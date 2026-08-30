package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPaperImportOpenAPIContract(t *testing.T) {
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
		{"/api/v1/exams/{examId}/paper-imports", "get", "listPaperImports"},
		{"/api/v1/exams/{examId}/paper-imports", "post", "createPaperImport"},
		{"/api/v1/paper-imports/{paperImportId}", "get", "getPaperImport"},
		{"/api/v1/paper-imports/{paperImportId}/sources", "post", "addPaperImportSources"},
		{"/api/v1/paper-imports/{paperImportId}/sources", "put", "replacePaperImportSources"},
		{"/api/v1/paper-imports/{paperImportId}/review", "put", "savePaperImportReview"},
		{"/api/v1/paper-imports/{paperImportId}/apply", "post", "applyPaperImport"},
	}
	for _, expected := range operations {
		operation := object(t, object(t, paths, expected.path), expected.method)
		if operation["operationId"] != expected.id {
			t.Fatalf("%s %s operationId = %#v, want %q", expected.method, expected.path, operation["operationId"], expected.id)
		}
	}

	schemas := object(t, object(t, document, "components"), "schemas")
	createRequest := object(t, schemas, "CreatePaperImportRequest")
	assertRequired(t, createRequest, []string{"subject"})
	createProperties := object(t, createRequest, "properties")
	for _, legacyOrCurrent := range []string{"sources", "paper_file_asset_id", "answer_file_asset_id"} {
		if _, ok := createProperties[legacyOrCurrent]; !ok {
			t.Fatalf("CreatePaperImportRequest is missing compatibility field %q", legacyOrCurrent)
		}
	}
	if alternatives, ok := createRequest["anyOf"].([]any); !ok || len(alternatives) != 3 {
		t.Fatalf("CreatePaperImportRequest must accept sources or either legacy file field: %#v", createRequest["anyOf"])
	}
	assertRequired(t, object(t, schemas, "PaperImportSourceRef"), []string{"source_id", "file_asset_id", "document_index"})
	assertRequired(t, object(t, schemas, "PaperImportQuestionCandidate"), []string{"candidate_id", "source_refs", "confidence"})
	assertRequired(t, object(t, schemas, "PaperImportAnswerCandidate"), []string{"candidate_id", "source_refs", "confidence"})
	assertRequired(t, object(t, schemas, "PaperImportSolutionCandidate"), []string{"candidate_id", "raw_text", "steps", "source_refs", "confidence"})
	assertRequired(t, object(t, schemas, "PaperImportJob"), []string{"sources", "question_candidates", "answer_candidates", "solution_candidates", "structured_issues", "questions"})
}
