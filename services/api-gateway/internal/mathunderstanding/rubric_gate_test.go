package mathunderstanding

import (
	"context"
	"errors"
	"testing"
)

func TestRubricMatcherSupportsAlternativePathsWithoutScoring(t *testing.T) {
	input := EvidenceMatchInput{Graph: SolutionGraph{OverallConfidence: .9}, Verifications: []MathVerification{{ID: "v1", Kind: "equivalence", Status: "verified"}}, Concepts: []string{"factorization"}}
	input.Requirements = []EvidenceRequirement{{Type: "any_of", Criterion: "method", Children: []EvidenceRequirement{{Type: "concept", Target: "factorization"}, {Type: "concept", Target: "quadratic_formula"}}}, {Type: "valid_transformation", Criterion: "solve"}}
	evidence, err := MatchRubricEvidence(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range evidence {
		if item.Status != "supported" {
			t.Fatalf("valid alternative rejected: %#v", evidence)
		}
	}
}

func TestPilotGateIsLayeredAndNeverAuthorizesFinalGrade(t *testing.T) {
	policy := PilotPolicy{MinimumSamples: 500, MinimumFormulaExact: .9, MinimumASTExact: .85, MinimumSpatialF1: .8, MinimumGraphF1: .8, MinimumEquivalencePrecision: .99, MinimumRubricPrecision: .95, MaximumUnsafeSuggestionRate: .001, MinimumRiskyCaseRecall: .98}
	metrics := PilotMetrics{SampleCount: 500, FormulaExactRate: .95, ASTExactRate: .9, SpatialRelationF1: .85, SolutionGraphEdgeF1: .85, EquivalencePrecision: .995, RubricEvidencePrecision: .96, UnsafeSuggestionRate: 0, RiskyCaseRecall: .99}
	decision := EvaluatePilotGate("mathematics", metrics, policy)
	if !decision.Passed || decision.Scope != "teacher_suggestion_only" {
		t.Fatalf("unexpected gate: %#v", decision)
	}
	decision = EvaluatePilotGate("history", metrics, policy)
	if decision.Passed {
		t.Fatal("humanities entered math pilot")
	}
	metrics.EquivalencePrecision = .9
	decision = EvaluatePilotGate("physics", metrics, policy)
	if decision.Passed {
		t.Fatal("aggregate score hid equivalence blocker")
	}
}

func TestPilotGateStoreKeepsSubjectBoundaryAndSuggestionOnlyScope(t *testing.T) {
	store := NewMemoryPilotGateStore()
	metrics := PilotMetrics{FormulaExactRate: 1, ASTExactRate: 1, SpatialRelationF1: 1, SolutionGraphEdgeF1: 1, EquivalencePrecision: 1, RubricEvidencePrecision: 1, RiskyCaseRecall: 1, SampleCount: 100}
	policy := PilotPolicy{MinimumSamples: 50, MinimumFormulaExact: .9, MinimumASTExact: .9, MinimumSpatialF1: .8, MinimumGraphF1: .8, MinimumEquivalencePrecision: .95, MinimumRubricPrecision: .9, MaximumUnsafeSuggestionRate: .01, MinimumRiskyCaseRecall: .9}
	item, err := store.CreatePilotGate(context.Background(), "tenant-1", "mathematics", "mathbench://fixture/v1", metrics, policy, "user-1")
	if err != nil || !item.Decision.Passed || item.Decision.Scope != "teacher_suggestion_only" {
		t.Fatalf("unexpected gate result: %#v, %v", item, err)
	}
	if _, err = store.CreatePilotGate(context.Background(), "tenant-1", "history", "mathbench://fixture/v1", metrics, policy, "user-1"); !errors.Is(err, ErrInvalidPilotGate) {
		t.Fatalf("humanities must not enter formula pilot gate, got %v", err)
	}
}
