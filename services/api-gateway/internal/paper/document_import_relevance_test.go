package paper

import "testing"

func TestAppendNoExamContentIssueForUnrelatedDocument(t *testing.T) {
	parsed := documentParseResponse{}
	documents := []normalizedImportDocument{{SourceID: "source-1", FileAssetID: "file-1", DocumentIndex: 0, Content: "旅行照片"}}

	issues := appendNoExamContentIssue(parsed, documents)

	if len(issues) != 1 || issues[0].Code != "NO_EXAM_CONTENT_DETECTED" || issues[0].Severity != "error" {
		t.Fatalf("expected explicit unrelated-content issue, got %#v", issues)
	}
	if len(issues[0].SourceRefs) != 1 || issues[0].SourceRefs[0].FileAssetID != "file-1" {
		t.Fatalf("expected issue to point back to the uploaded file, got %#v", issues[0].SourceRefs)
	}
}

func TestAppendNoExamContentIssueDoesNotFlagRecognizedExamMaterial(t *testing.T) {
	parsed := documentParseResponse{AnswerCandidates: []AnswerCandidate{{CandidateID: "answer-1"}}}

	if issues := appendNoExamContentIssue(parsed, nil); len(issues) != 0 {
		t.Fatalf("recognized exam material must not be flagged as unrelated, got %#v", issues)
	}
}
