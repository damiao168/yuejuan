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
	for _, expected := range []string{`"exam_id":"exam-1"`, `"anonymous_code":"ANON-1"`, `"ocr_text":"answer"`, `"question":`, `"rubric":`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %s in %s", expected, text)
		}
	}
	if strings.Contains(text, "AnonymousCode") {
		t.Fatalf("Go field names leaked into API: %s", text)
	}
}
