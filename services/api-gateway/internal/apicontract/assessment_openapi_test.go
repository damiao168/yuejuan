package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoryA01AssessmentOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi", "edugrade-api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("OpenAPI contract must be valid JSON: %v", err)
	}

	paths := object(t, document, "paths")
	operations := []struct {
		path   string
		method string
		id     string
	}{
		{"/api/v1/assessment/subject-profiles", "get", "listAssessmentSubjectProfiles"},
		{"/api/v1/assessment/question-archetypes", "get", "listAssessmentQuestionArchetypes"},
		{"/api/v1/exams/{examId}/questions/{questionId}/assessment-profile", "get", "getExamQuestionAssessmentProfile"},
		{"/api/v1/exams/{examId}/questions/{questionId}/assessment-profile", "put", "putExamQuestionAssessmentProfile"},
		{"/api/v1/exams/{examId}/questions/{questionId}/assessment-snapshot", "get", "getExamQuestionAssessmentSnapshot"},
	}
	for _, expected := range operations {
		operation := object(t, object(t, paths, expected.path), expected.method)
		if operation["operationId"] != expected.id {
			t.Fatalf("%s %s operationId = %#v, want %q", expected.method, expected.path, operation["operationId"], expected.id)
		}
		responses := object(t, operation, "responses")
		for _, status := range []string{"200", "401", "403"} {
			if _, ok := responses[status]; !ok {
				t.Fatalf("%s %s is missing response %s", expected.method, expected.path, status)
			}
		}
	}

	schemas := object(t, object(t, document, "components"), "schemas")
	assertStringEnum(t, schemas, "EducationStage", []string{"junior", "senior"})
	assertStringEnum(t, schemas, "SubjectCode", []string{"chinese", "mathematics", "english", "physics", "chemistry", "biology", "history", "geography", "ethics_politics"})
	assertStringEnum(t, schemas, "SubjectProfileStatus", []string{"active", "retired"})
	assertStringEnum(t, schemas, "QuestionArchetypeCode", []string{"selected_response", "exact_text", "numeric_expression", "structured_steps", "short_constructed", "extended_response", "diagram_graph", "table_experiment"})
	assertStringEnum(t, schemas, "EvidenceType", []string{"selected_option", "exact_text", "text_span", "numeric_value", "math_expression", "math_step", "unit_value", "chemical_equation", "concept", "relation", "diagram_feature", "table_cell"})
	assertStringEnum(t, schemas, "ScoringMode", []string{"RULE_AUTO", "AI_ASSIST", "AI_FAST_CONFIRM", "HUMAN_PRIMARY", "DUAL_HUMAN", "MANUAL_ONLY"})
	assertStringEnum(t, schemas, "ExamRiskTier", []string{"R1", "R2", "R3"})

	putRequest := object(t, schemas, "PutQuestionAssessmentProfileRequest")
	assertRequired(t, putRequest, []string{"subject_profile_id", "archetype_code", "allowed_evidence_types", "risk_tier", "scoring_policy", "expected_revision"})
	allOf, ok := putRequest["allOf"].([]any)
	if !ok || len(allOf) == 0 {
		t.Fatal("assessment profile request must contract the R3 extended-response scoring restriction")
	}
	firstRule, ok := allOf[0].(map[string]any)
	if !ok {
		t.Fatal("assessment profile policy rule must be an object")
	}
	forbidden := object(t, firstRule, "not")
	properties := object(t, forbidden, "properties")
	forbiddenScoringPolicy := object(t, properties, "scoring_policy")
	forbiddenScoringProperties := object(t, forbiddenScoringPolicy, "properties")
	if object(t, properties, "risk_tier")["const"] != "R3" ||
		object(t, properties, "archetype_code")["const"] != "extended_response" ||
		object(t, forbiddenScoringProperties, "mode")["const"] != "AI_FAST_CONFIRM" {
		t.Fatal("assessment profile request must reject R3 + extended_response + AI_FAST_CONFIRM")
	}
	scoringPolicy := object(t, schemas, "QuestionScoringPolicy")
	assertRequired(t, scoringPolicy, []string{"mode", "require_evidence"})
	if object(t, object(t, scoringPolicy, "properties"), "human_review_below_confidence")["type"] != "boolean" {
		t.Fatal("human_review_below_confidence must match the backend boolean policy switch")
	}

	configuration := object(t, schemas, "QuestionAssessmentProfile")
	assertRequired(t, configuration, []string{
		"id", "tenant_id", "exam_id", "question_id", "subject_profile_id", "subject_profile_code",
		"subject_profile_version", "education_stage", "subject_code", "archetype_code",
		"allowed_evidence_types", "risk_tier", "scoring_policy", "revision", "created_at", "updated_at",
	})

	snapshot := object(t, schemas, "ExamQuestionAssessmentSnapshot")
	assertRequired(t, snapshot, []string{
		"id", "tenant_id", "exam_id", "question_id", "snapshot_version", "subject_profile_id",
		"subject_profile_code", "subject_profile_version", "education_stage", "subject_code",
		"archetype_code", "allowed_evidence_types", "risk_tier", "profile_snapshot", "archetype_snapshot",
		"rubric_snapshot", "scoring_policy_snapshot", "content_hash", "created_at",
	})
}

func assertStringEnum(t *testing.T, schemas map[string]any, name string, expected []string) {
	t.Helper()
	schema := object(t, schemas, name)
	values, ok := schema["enum"].([]any)
	if !ok {
		t.Fatalf("schema %s must define an enum", name)
	}
	actual := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("schema %s enum contains a non-string value: %#v", name, value)
		}
		actual = append(actual, text)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("schema %s enum = %#v, want %#v", name, actual, expected)
	}
}

func assertRequired(t *testing.T, schema map[string]any, expected []string) {
	t.Helper()
	values, ok := schema["required"].([]any)
	if !ok {
		t.Fatal("schema must define required properties")
	}
	present := make(map[string]bool, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			present[text] = true
		}
	}
	for _, name := range expected {
		if !present[name] {
			t.Fatalf("schema is missing required property %q", name)
		}
	}
}
