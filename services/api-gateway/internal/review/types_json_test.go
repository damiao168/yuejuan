package review

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkspaceContextUsesStableSnakeCaseContract(t *testing.T) {
	raw, err := json.Marshal(Context{ExamID: "exam-1", AnonymousCode: "ANON-1", OCRText: "answer"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{`"exam_id":"exam-1"`, `"anonymous_code":"ANON-1"`, `"ocr_text":"answer"`, `"question":`, `"rubric":`, `"ai_suggestion":null`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %s in %s", expected, text)
		}
	}
	if strings.Contains(text, "AnonymousCode") {
		t.Fatalf("Go field names leaked into API: %s", text)
	}
}

func TestWorkspaceContextIncludesTaskScopedAISuggestion(t *testing.T) {
	raw, err := json.Marshal(Context{AISuggestion: map[string]any{
		"id":                 "grade-1",
		"suggested_score":    4.5,
		"confidence":         0.91,
		"needs_human_review": true,
		"status":             "succeeded",
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{`"ai_suggestion":{`, `"id":"grade-1"`, `"suggested_score":4.5`, `"confidence":0.91`, `"needs_human_review":true`, `"status":"succeeded"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %s in %s", expected, text)
		}
	}
}

func TestWorkspaceContextIncludesTaskScopedAutomationResult(t *testing.T) {
	confidence, score, maxScore := 0.88, 2.0, 4.0
	raw, err := json.Marshal(Context{AutomationResult: &AutomationResult{
		Source: "ocr", RecognizedAnswer: "12", Confidence: &confidence, Decision: "ambiguous",
		StandardAnswer: "12", RuleType: "fill_blank", Score: &score, MaxScore: &maxScore,
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{`"automation_result":{`, `"source":"ocr"`, `"recognized_answer":"12"`, `"confidence":0.88`, `"standard_answer":"12"`, `"rule_type":"fill_blank"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %s in %s", expected, text)
		}
	}
}
