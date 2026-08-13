package answergroup

import (
	"context"
	"errors"
	"testing"
)

func TestAnswerGroupingRequiresSamplingAndNeverProducesFinalGrade(t *testing.T) {
	store := NewMemoryStore(nil, DefaultPolicy())
	store.SetSourceAnswers("tenant", "exam", "question", []SourceAnswer{
		{SubmissionID: "submission-1", SegmentID: "segment-1", SnapshotID: "snapshot", ArchetypeCode: ArchetypeExactText, AnswerText: "ATP", Source: "manual_entry"},
		{SubmissionID: "submission-2", SegmentID: "segment-2", SnapshotID: "snapshot", ArchetypeCode: ArchetypeExactText, AnswerText: "ＡＴＰ", Source: "manual_entry"},
		{SubmissionID: "submission-3", SegmentID: "segment-3", SnapshotID: "snapshot", ArchetypeCode: ArchetypeExactText, AnswerText: "ATP。", Source: "manual_entry"},
		{SubmissionID: "submission-4", SegmentID: "segment-4", SnapshotID: "snapshot", ArchetypeCode: ArchetypeExactText, AnswerText: "ADP", Source: "manual_entry"},
	})
	groups, err := store.Build(context.Background(), "tenant", "exam", "question", "teacher", BuildInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].MemberCount != 3 || groups[1].MemberCount != 1 {
		t.Fatalf("expected conservative 3+1 grouping, got %#v", groups)
	}
	if !groups[1].Members[0].Outlier || groups[1].CanConfirm {
		t.Fatalf("singleton must return to individual review, got %#v", groups[1])
	}

	group, err := store.PutDecision(context.Background(), "tenant", groups[0].ID, "teacher", DecisionInput{
		ScoreCandidate:  map[string]any{"score": 2.0, "max_score": 2.0},
		RubricSelection: map[string]any{"P1": "matched"}, ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Confirm(context.Background(), "tenant", group.ID, "teacher", ConfirmInput{ExpectedRevision: 1}); !errors.Is(err, ErrSamplingIncomplete) {
		t.Fatalf("confirmation must fail before sampling, got %v", err)
	}
	for _, member := range group.Members {
		group, err = store.ReviewSample(context.Background(), "tenant", group.ID, member.SegmentID, "teacher", SampleReviewInput{Outcome: SampleAccepted})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !group.CanConfirm || group.ReviewedSampleCount != group.MinimumSample {
		t.Fatalf("group should become confirmable only after policy is met: %#v", group)
	}
	group, candidates, err := store.Confirm(context.Background(), "tenant", group.ID, "teacher", ConfirmInput{ExpectedRevision: group.Decision.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if group.Status != StatusConfirmed || len(candidates) != 3 || group.Decision.RollbackReference == "" {
		t.Fatalf("expected three traceable proposed facts, got group=%#v candidates=%#v", group, candidates)
	}
	for _, candidate := range candidates {
		if candidate.Kind != CandidateGroupScore || candidate.Status != CandidateActive || candidate.RollbackReference != group.Decision.RollbackReference {
			t.Fatalf("unexpected candidate: %#v", candidate)
		}
		if _, pretendsFinal := candidate.ScoreCandidate["final_grade"]; pretendsFinal {
			t.Fatal("grouping candidate must not represent a final grade")
		}
	}

	group, rolledBack, err := store.Rollback(context.Background(), "tenant", group.ID, "supervisor", RollbackInput{
		RollbackReference: group.Decision.RollbackReference, Reason: "post-audit mismatch",
	})
	if err != nil {
		t.Fatal(err)
	}
	if group.Status != StatusRolledBack || len(rolledBack) != 3 {
		t.Fatalf("rollback did not invalidate all candidates: %#v %#v", group, rolledBack)
	}
}

func TestAnswerGroupingFiltersUnreliableAndUnsupportedText(t *testing.T) {
	store := NewMemoryStore(nil, DefaultPolicy())
	low := 0.4
	high := 0.95
	store.SetSourceAnswers("tenant", "exam", "question", []SourceAnswer{
		{SubmissionID: "s1", SegmentID: "a1", SnapshotID: "snapshot", ArchetypeCode: ArchetypeShortConstructed, AnswerText: "valid answer", Source: "ocr_text", Confidence: &high},
		{SubmissionID: "s2", SegmentID: "a2", SnapshotID: "snapshot", ArchetypeCode: ArchetypeShortConstructed, AnswerText: "low confidence", Source: "ocr_text", Confidence: &low},
		{SubmissionID: "s3", SegmentID: "a3", SnapshotID: "snapshot", ArchetypeCode: "extended_response", AnswerText: "essay", Source: "manual_entry"},
	})
	groups, err := store.Build(context.Background(), "tenant", "exam", "question", "teacher", BuildInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].MemberCount != 1 || groups[0].Members[0].SegmentID != "a1" {
		t.Fatalf("only reliably textified eligible answers should remain: %#v", groups)
	}
}

func TestAnswerGroupingBuildIsIdempotentForSameSourceFacts(t *testing.T) {
	store := NewMemoryStore(nil, DefaultPolicy())
	store.SetSourceAnswers("tenant", "exam", "question", []SourceAnswer{{
		SubmissionID: "s1", SegmentID: "a1", SnapshotID: "snapshot", ArchetypeCode: ArchetypeExactText,
		AnswerText: "A", Source: "manual_entry",
	}})
	first, err := store.Build(context.Background(), "tenant", "exam", "question", "teacher", BuildInput{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Build(context.Background(), "tenant", "exam", "question", "teacher", BuildInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID {
		t.Fatalf("same source facts must reuse the build: first=%#v second=%#v", first, second)
	}
}
