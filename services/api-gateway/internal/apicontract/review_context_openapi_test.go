package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoryA05ReviewTaskContextOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi", "edugrade-api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("OpenAPI contract must be valid JSON: %v", err)
	}

	operation := object(t, object(t, object(t, document, "paths"), "/api/v1/review-tasks/{taskId}/context"), "get")
	if operation["operationId"] != "getReviewTaskContext" {
		t.Fatalf("review context operationId = %#v", operation["operationId"])
	}
	for _, status := range []string{"200", "401", "403", "404"} {
		if _, ok := object(t, operation, "responses")[status]; !ok {
			t.Fatalf("review context is missing response %s", status)
		}
	}

	schemas := object(t, object(t, document, "components"), "schemas")
	contextSchema := object(t, schemas, "ReviewTaskContext")
	assertRequired(t, contextSchema, []string{
		"task", "expected_revision", "question_snapshot", "question", "answer_artifact",
		"frozen_rubric", "ai_candidates", "scoring_evidence", "claim", "draft", "subject_tool_hints",
	})
	secondOpinion := object(t, schemas, "ReviewAISecondOpinion")
	assertRequired(t, secondOpinion, []string{"available", "presentation", "score_prefill_allowed", "metadata"})
	properties := object(t, secondOpinion, "properties")
	if object(t, properties, "presentation")["const"] != "explicit_second_opinion" {
		t.Fatal("AI material must be contracted as an explicit second opinion")
	}
}
