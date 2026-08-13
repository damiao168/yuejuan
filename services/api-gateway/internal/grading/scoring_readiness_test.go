package grading

import (
	"strings"
	"testing"
)

func TestBuildScoringReadinessBlocksIncompleteSegmentsAndActiveRun(t *testing.T) {
	run := &ScoringRun{ID: "run-1", Status: "needs_review"}
	result := buildScoringReadiness("collecting", run, scoringReadinessCounts{
		questions: 3, snapshots: 3, segments: 12, processable: 11, automatic: 8, missingAutomation: 2,
	})
	if result.Ready {
		t.Fatal("incomplete segments and an active run must block scoring")
	}
	if result.ReadySegments != 11 || result.AutomaticCandidates != 8 || result.ManualReviewCandidates != 3 {
		t.Fatalf("unexpected readiness counts: %#v", result)
	}
	checks := map[string]ScoringReadinessCheck{}
	for _, check := range result.Checks {
		checks[check.Code] = check
	}
	if checks["answer_segments_processed"].Passed || checks["answer_segments_processed"].Count != 1 {
		t.Fatalf("incomplete segment check should identify one blocker: %#v", checks["answer_segments_processed"])
	}
	if checks["active_run_clear"].Passed || !strings.Contains(checks["active_run_clear"].Message, "人工复核") {
		t.Fatalf("active run check should explain the required action: %#v", checks["active_run_clear"])
	}
	if checks["automation_coverage"].Passed || checks["automation_coverage"].Severity != "warning" {
		t.Fatalf("missing automation is a warning because the answer can enter human review: %#v", checks["automation_coverage"])
	}
}

func TestBuildScoringReadinessAllowsManualReviewCandidates(t *testing.T) {
	result := buildScoringReadiness("grading", nil, scoringReadinessCounts{
		questions: 2, snapshots: 2, segments: 10, processable: 10, automatic: 4,
	})
	if !result.Ready {
		t.Fatalf("manual review candidates must not block a complete scoring batch: %#v", result)
	}
	if result.ManualReviewCandidates != 6 {
		t.Fatalf("expected six manual review candidates, got %#v", result)
	}
}

func TestBuildScoringReadinessBlocksMissingTemplateMetadata(t *testing.T) {
	result := buildScoringReadiness("processing", nil, scoringReadinessCounts{
		questions: 1, snapshots: 1, segments: 5, processable: 5, missingMetadata: 2, automatic: 3,
	})
	if result.Ready || result.ReadySegments != 3 {
		t.Fatalf("missing template metadata must block partial scoring: %#v", result)
	}
	for _, check := range result.Checks {
		if check.Code == "segment_metadata_complete" {
			if check.Passed || check.Count != 2 {
				t.Fatalf("metadata check should expose two blockers: %#v", check)
			}
			return
		}
	}
	t.Fatal("segment metadata check missing")
}
