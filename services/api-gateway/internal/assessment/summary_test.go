package assessment

import (
	"context"
	"testing"
)

func TestMemoryExamAssessmentSummaryUsesHighestEffectiveRisk(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	profiles, err := store.ListSubjectProfiles(ctx, "tenant-a", StageJunior, SubjectMathematics)
	if err != nil || len(profiles) != 1 {
		t.Fatalf("profiles=(%d,%v)", len(profiles), err)
	}
	for _, item := range []struct {
		questionID string
		risk       RiskTier
	}{
		{"question-1", RiskR1},
		{"question-2", RiskR3},
	} {
		_, err := store.ConfigureQuestion(ctx, "tenant-a", "exam-a", item.questionID, ConfigureQuestionInput{
			SubjectProfileID: profiles[0].ID, ArchetypeCode: "extended_response",
			AllowedEvidenceTypes: []EvidenceType{EvidenceTextSpan}, RiskTier: item.risk,
			ScoringPolicy: ScoringPolicy{Mode: ScoringHumanPrimary, RequireEvidence: true},
		})
		if err != nil {
			t.Fatalf("configure %s: %v", item.questionID, err)
		}
	}
	if _, err := store.FreezeQuestionSnapshot(ctx, "tenant-a", "exam-a", "question-1"); err != nil {
		t.Fatal(err)
	}

	summary, err := store.GetExamAssessmentSummary(ctx, "tenant-a", "exam-a")
	if err != nil {
		t.Fatal(err)
	}
	if summary.RiskTier != RiskR3 || summary.SubjectCode != SubjectMathematics ||
		summary.ConfiguredQuestionCount != 2 || summary.FrozenQuestionCount != 1 || summary.Source != "mixed" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}
