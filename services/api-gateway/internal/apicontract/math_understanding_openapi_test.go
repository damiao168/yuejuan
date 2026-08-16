package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMathUnderstandingContractKeepsEvidenceAndSubjectBoundaries(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi", "edugrade-api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	paths := object(t, document, "paths")
	for _, path := range []string{"/api/v1/math-answer-segments/{segmentId}/understanding", "/api/v1/math-understanding/{artifactId}/corrections", "/api/v1/math-pilot-gates", "/api/v1/math-pilot-gates/evaluate"} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("missing math contract path %s", path)
		}
	}
	schemas := object(t, object(t, document, "components"), "schemas")
	decision := object(t, schemas, "MathPilotGateDecision")
	scope := object(t, object(t, decision, "properties"), "scope")
	if scope["const"] != "teacher_suggestion_only" {
		t.Fatalf("pilot gate must never grant automatic final scoring: %#v", scope)
	}
	request := object(t, schemas, "EvaluateMathPilotGateRequest")
	subject := object(t, object(t, request, "properties"), "subject_code")
	enums, ok := subject["enum"].([]any)
	if !ok || len(enums) != 3 || enums[0] != "mathematics" || enums[1] != "physics" || enums[2] != "chemistry" {
		t.Fatalf("formula subjects must remain explicitly bounded: %#v", subject)
	}
}
