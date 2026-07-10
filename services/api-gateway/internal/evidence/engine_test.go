package evidence

import (
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/paper"
)

func TestVerifyPassesValidEvidence(t *testing.T) {
	result := NewEngine().Verify(validEvidenceContext())

	if !result.Passed {
		t.Fatalf("valid evidence should pass, got %#v", result.Failed)
	}
	if result.NeedsHumanReview {
		t.Fatalf("valid evidence should not require review: %#v", result)
	}
}

func TestVerifyFailsWhenSuggestedScoreExceedsMax(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.Grade.SuggestedScore = 3
	ctx.Grade.MatchedPoints[0].Score = 3

	result := NewEngine().Verify(ctx)

	assertFailedIssue(t, result, "score_exceeds_max")
}

func TestVerifyFailsWhenMatchedScoreDiffersFromSuggestedScore(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.Grade.SuggestedScore = 1

	result := NewEngine().Verify(ctx)

	assertFailedIssue(t, result, "matched_score_mismatch")
}

func TestVerifyFailsWhenRubricPointIsUnknown(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.Grade.MatchedPoints[0].Code = "unknown-point"

	result := NewEngine().Verify(ctx)

	assertFailedIssue(t, result, "rubric_point_not_found")
}

func TestVerifyFailsWhenRubricIsMissingForPointReferences(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.Rubric = paper.Rubric{}

	result := NewEngine().Verify(ctx)

	assertFailedIssue(t, result, "rubric_missing")
}

func TestVerifyFailsWhenEvidenceIsEmpty(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.Grade.Evidence = nil

	result := NewEngine().Verify(ctx)

	assertFailedIssue(t, result, "empty_evidence")
}

func TestVerifyFlagsLowOCRConfidenceForReview(t *testing.T) {
	ctx := validEvidenceContext()
	lowConfidence := 0.5
	ctx.OCRConfidence = &lowConfidence

	result := NewEngine().Verify(ctx)

	if !result.Passed {
		t.Fatalf("low OCR confidence should warn but not fail, got %#v", result.Failed)
	}
	if !result.NeedsHumanReview {
		t.Fatalf("low OCR confidence should require human review: %#v", result)
	}
	assertWarningIssue(t, result, "low_ocr_confidence")
	assertCorrectedFlag(t, result, "low_ocr_confidence")
}

func TestVerifyFailsWhenEvidenceTextIsNotInAnswer(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.Grade.Evidence[0].AnswerText = "not in the student answer"

	result := NewEngine().Verify(ctx)

	assertFailedIssue(t, result, "evidence_text_not_found")
}

func TestVerifyFailsWhenEvidenceBBoxLeavesSegment(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.Grade.Evidence[0].BBox = []float64{90, 90, 20, 20}

	result := NewEngine().Verify(ctx)

	assertFailedIssue(t, result, "evidence_bbox_out_of_segment")
}

func validEvidenceContext() Context {
	ocrConfidence := 0.95
	return Context{
		Grade: Grade{
			ID:              "grade-1",
			TenantID:        "tenant-1",
			AnswerSegmentID: "segment-1",
			QuestionID:      "question-1",
			QuestionNo:      "Q1",
			QuestionType:    "short_answer",
			SuggestedScore:  2,
			MaxScore:        2,
			MatchedPoints: []grading.PointResult{
				{Code: "p1", Label: "correct concept", Score: 2},
			},
			Evidence: []grading.Evidence{
				{
					Type:          "text",
					AnswerSegment: "segment-1",
					AnswerText:    "photosynthesis uses sunlight",
					BBox:          []float64{10, 10, 50, 20},
				},
			},
			Status: "succeeded",
		},
		AnswerText:        "Photosynthesis uses sunlight.",
		OCRConfidence:     &ocrConfidence,
		AnswerSegmentBBox: []float64{0, 0, 100, 100},
		Rubric: paper.Rubric{
			ID:         "rubric-1",
			QuestionID: "question-1",
			Version:    "v1",
			Status:     "approved",
			MaxScore:   2,
			Points: []paper.RubricPoint{
				{ID: "p1", Description: "correct concept", Score: 2, Required: true},
			},
		},
	}
}

func assertFailedIssue(t *testing.T, result VerificationResult, code string) {
	t.Helper()
	if result.Passed {
		t.Fatalf("expected verification to fail with %s, got passed result %#v", code, result)
	}
	for _, issue := range result.Failed {
		if issue.Code == code {
			if !result.NeedsHumanReview {
				t.Fatalf("failed result should require human review: %#v", result)
			}
			assertCorrectedFlag(t, result, "evidence_verification_failed")
			return
		}
	}
	t.Fatalf("missing failed issue %s in %#v", code, result.Failed)
}

func assertWarningIssue(t *testing.T, result VerificationResult, code string) {
	t.Helper()
	for _, issue := range result.Warnings {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("missing warning issue %s in %#v", code, result.Warnings)
}

func assertCorrectedFlag(t *testing.T, result VerificationResult, flag string) {
	t.Helper()
	for _, correctedFlag := range result.CorrectedFlags {
		if correctedFlag == flag {
			return
		}
	}
	t.Fatalf("missing corrected flag %s in %#v", flag, result.CorrectedFlags)
}
