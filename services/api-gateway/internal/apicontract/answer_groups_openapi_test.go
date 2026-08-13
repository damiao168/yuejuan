package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoryA07AnswerGroupsOpenAPIContract(t *testing.T) {
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
		{"/api/v1/exams/{examId}/questions/{questionId}/answer-groups/build", "post", "buildAnswerGroups"},
		{"/api/v1/exams/{examId}/questions/{questionId}/answer-groups", "get", "listAnswerGroups"},
		{"/api/v1/exams/{examId}/questions/{questionId}/answer-group-metrics", "get", "getAnswerGroupMetrics"},
		{"/api/v1/answer-groups/{groupId}", "get", "getAnswerGroup"},
		{"/api/v1/answer-groups/{groupId}/samples/{segmentId}", "put", "reviewAnswerGroupSample"},
		{"/api/v1/answer-groups/{groupId}/decision", "put", "putAnswerGroupDecision"},
		{"/api/v1/answer-groups/{groupId}/confirm", "post", "confirmAnswerGroup"},
		{"/api/v1/answer-groups/{groupId}/rollback", "post", "rollbackAnswerGroup"},
	}
	for _, expected := range operations {
		operation := object(t, object(t, paths, expected.path), expected.method)
		if operation["operationId"] != expected.id {
			t.Fatalf("%s %s operationId = %#v, want %q", expected.method, expected.path, operation["operationId"], expected.id)
		}
	}

	schemas := object(t, object(t, document, "components"), "schemas")
	assertRequired(t, object(t, schemas, "AnswerGroup"), []string{"members", "minimum_sample", "reviewed_sample_count", "can_confirm"})
	assertRequired(t, object(t, schemas, "AnswerGroupListResponse"), []string{"answer_groups", "teacher_reference_cases"})
	assertRequired(t, object(t, schemas, "ConfirmAnswerGroupResponse"), []string{"answer_group", "automation_candidates"})
}
