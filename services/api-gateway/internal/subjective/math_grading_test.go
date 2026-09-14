package subjective

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/assessment"
	"edugrade-enterprise/services/api-gateway/internal/mathunderstanding"
	"edugrade-enterprise/services/api-gateway/internal/paper"
)

func TestMathEvidenceSourceSanitizesVerifiedArtifactAndDerivesCriticalQuality(t *testing.T) {
	value, source, _, _ := mathGradingFixture(t)
	evidence, err := source.Prepare(context.Background(), "tenant-a", value)
	if err != nil {
		t.Fatal(err)
	}
	if evidence == nil || evidence.ArtifactVersion != 2 || evidence.CorrectionRevision != 1 || evidence.ScoringVersion != MathScoringVersionV1 || evidence.Quality.Critical != .9 {
		t.Fatalf("verified evidence was not prepared: %#v", evidence)
	}
	if evidence.Quality.Critical != minMathEvidenceQuality(evidence.Quality) {
		t.Fatalf("critical quality is not the minimum dimension: %#v", evidence.Quality)
	}
	raw, _ := json.Marshal(evidence)
	for _, forbidden := range []string{`"ast"`, `"details"`, `"raw_latex"`, `"symbols"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("private mathematical representation leaked to the model: %s", raw)
		}
	}
}

func TestSettleMathOutputUsesFrozenServerRubricNotModelScore(t *testing.T) {
	value, source, _, _ := mathGradingFixture(t)
	prepared, err := source.Prepare(context.Background(), "tenant-a", value)
	if err != nil {
		t.Fatal(err)
	}
	value.MathEvidence = prepared
	handler := &Handler{mathEvidence: source}
	output := AdapterOutput{
		SchemaVersion:  gradingAgentV2BuilderSchemaVersion,
		DeliveryMode:   "teacher_suggestion",
		MathCandidates: []MathCriterionCandidate{{RubricPointID: "p1", Status: "uncertain", EvidenceIDs: []string{"step-1"}, Confidence: .1, ReasonCode: "model_unsure"}},
	}
	if err := handler.settleMathOutput(context.Background(), "tenant-a", value, ModelPolicy{MinConfidence: .8}, &output); err != nil {
		t.Fatal(err)
	}
	if output.SuggestedScore != 2 || output.MathScore == nil || output.MathScore.SuggestedScore == nil || *output.MathScore.SuggestedScore != 2 || len(output.MatchedPoints) != 1 {
		t.Fatalf("server rubric did not remain score authority: %#v", output)
	}
	if !output.NeedsHumanReview || !containsString(output.RiskFlags, "human_review_required") {
		t.Fatalf("math suggestion escaped teacher confirmation: %#v", output)
	}
}

func TestSettleMathOutputPreservesUncertaintyWithoutFakeTotal(t *testing.T) {
	value, source, _, _ := mathGradingFixture(t)
	value.Rubric.Points[0].EvidenceRequirements = []paper.EvidenceRequirement{{Type: "concept", Target: "factorization"}}
	prepared, err := source.Prepare(context.Background(), "tenant-a", value)
	if err != nil {
		t.Fatal(err)
	}
	value.MathEvidence = prepared
	output := AdapterOutput{SchemaVersion: gradingAgentV2BuilderSchemaVersion, DeliveryMode: "teacher_suggestion", MathCandidates: []MathCriterionCandidate{{RubricPointID: "p1", Status: "supported", EvidenceIDs: []string{"step-1"}, Confidence: .99, ReasonCode: "semantic_alignment"}}}
	err = (&Handler{mathEvidence: source}).settleMathOutput(context.Background(), "tenant-a", value, ModelPolicy{MinConfidence: .8}, &output)
	if !errors.Is(err, ErrMathHumanReviewRequired) || output.MathScore == nil || output.MathScore.SuggestedScore != nil || output.MathScore.ScoreRange.Min != 0 || output.MathScore.ScoreRange.Max != 2 || output.SuggestedScore != 0 {
		t.Fatalf("uncertainty became a fake total: output=%#v err=%v", output, err)
	}
}

func TestSettleMathOutputRejectsArtifactVersionDrift(t *testing.T) {
	value, source, artifacts, input := mathGradingFixture(t)
	prepared, err := source.Prepare(context.Background(), "tenant-a", value)
	if err != nil {
		t.Fatal(err)
	}
	value.MathEvidence = prepared
	input.InputHash = "sha256:replacement"
	if _, err = artifacts.CreateArtifact(context.Background(), "tenant-a", input); err != nil {
		t.Fatal(err)
	}
	output := AdapterOutput{SchemaVersion: gradingAgentV2BuilderSchemaVersion, DeliveryMode: "teacher_suggestion"}
	err = (&Handler{mathEvidence: source}).settleMathOutput(context.Background(), "tenant-a", value, ModelPolicy{MinConfidence: .8}, &output)
	if !isMathRevisionConflict(err) {
		t.Fatalf("artifact drift was not rejected: %v", err)
	}
}

func TestSettleMathOutputRejectsCorrectionArrivingDuringInference(t *testing.T) {
	value, source, _, input := mathGradingFixture(t)
	prepared, err := source.Prepare(context.Background(), "tenant-a", value)
	if err != nil {
		t.Fatal(err)
	}
	value.MathEvidence = prepared
	zero := int64(0)
	if _, err = source.Corrections.CreateCorrection(context.Background(), "tenant-a", prepared.ArtifactID, "teacher-1", mathunderstanding.CreateCorrectionInput{
		ExpectedArtifactVersion: prepared.ArtifactVersion, ExpectedCorrectionRevision: &zero,
		Operations:        []mathunderstanding.CorrectionOperation{{Type: "correct_formula", TargetID: "formula-1", Payload: map[string]any{"canonical_latex": "x=2"}}},
		CorrectedContract: input, Reason: "teacher correction arrived while grading",
	}); err != nil {
		t.Fatal(err)
	}
	output := AdapterOutput{SchemaVersion: gradingAgentV2BuilderSchemaVersion, DeliveryMode: "teacher_suggestion"}
	err = (&Handler{mathEvidence: source}).settleMathOutput(context.Background(), "tenant-a", value, ModelPolicy{MinConfidence: .8}, &output)
	if !isMathRevisionConflict(err) {
		t.Fatalf("correction drift was not rejected: %v", err)
	}
}

func TestMathBindingChangesStableRequestIdentity(t *testing.T) {
	value, source, _, _ := mathGradingFixture(t)
	prepared, err := source.Prepare(context.Background(), "tenant-a", value)
	if err != nil {
		t.Fatal(err)
	}
	value.MathEvidence = prepared
	policy := ModelPolicy{ModelVersion: "model-v1", PromptVersion: "prompt-v2", MinConfidence: .8}
	first, err := stableRequestID("tenant-a", value, policy, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	changed := value
	copyEvidence := *prepared
	copyEvidence.CorrectionRevision++
	changed.MathEvidence = &copyEvidence
	second, err := stableRequestID("tenant-a", changed, policy, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("math correction revision was omitted from idempotency identity")
	}
}

func mathGradingFixture(t *testing.T) (Context, MathEvidenceSource, *mathunderstanding.MemoryStore, mathunderstanding.CreateArtifactInput) {
	t.Helper()
	input := mathunderstanding.CreateArtifactInput{
		SubjectCode: "mathematics", AnswerSegmentID: "segment-1", ExamQuestionSnapshotID: "snapshot-1", InputHash: validResolvedActiveCrop(t).SHA256, EngineVersion: "math-engine-v1",
		Blocks:        []mathunderstanding.MathAnswerBlock{{ID: "block-1", Kind: "formula", Status: "active", BoundingBox: mathunderstanding.BoundingBox{X: .1, Y: .1, Width: .4, Height: .2}, RecognitionEngine: "fixture", RecognitionVersion: "v1", RecognitionConfidence: .95, StructureConfidence: .9, SourceImageHash: "sha256:image"}},
		Formulas:      []mathunderstanding.FormulaArtifact{{ID: "formula-1", BlockID: "block-1", BoundingBox: mathunderstanding.BoundingBox{X: .1, Y: .1, Width: .4, Height: .2}, RawLatex: "x=2", CanonicalLatex: "x=2", RecognitionEngine: "fixture", RecognitionVersion: "v1", ParserVersion: "v1", ParseStatus: "parsed", Confidence: .94, AST: &mathunderstanding.FormulaAST{Kind: "equation"}}},
		SolutionGraph: mathunderstanding.SolutionGraph{ID: "graph-1", AnswerSegmentID: "segment-1", BuilderVersion: "v2", FormulaModelVersion: "v1", OverallConfidence: .93, Steps: []mathunderstanding.SolutionStep{{ID: "step-1", OrderHint: 1, BlockIDs: []string{"block-1"}, FormulaIDs: []string{"formula-1"}, NormalizedText: "x=2", Kind: "conclusion", RecognitionConfidence: .95, StructureConfidence: .91, Confidence: .92}}, Edges: []mathunderstanding.SolutionEdge{}},
		Verifications: []mathunderstanding.MathVerification{{ID: "syntax-1", StepID: "step-1", FormulaID: "formula-1", Kind: "syntax", Status: "verified", ReasonCode: "syntax_valid", Domain: "real", Engine: "sympy", EngineVersion: "1.14", RulesetVersion: "v1", Details: map[string]any{"private": "must-not-leak"}, Confidence: .98}},
		Relations:     []mathunderstanding.SpatialRelation{}, RubricEvidence: []mathunderstanding.RubricEvidence{},
	}
	artifacts := mathunderstanding.NewMemoryStore()
	base, err := artifacts.CreateArtifact(context.Background(), "tenant-a", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = artifacts.CreateDerivedArtifact(context.Background(), "tenant-a", base.ID, 1, input, map[string]any{"critical_confidence": .9}); err != nil {
		t.Fatal(err)
	}
	corrections := mathunderstanding.NewMemoryCorrectionStore(artifacts)
	value := Context{
		SegmentID: "segment-1", Subject: "mathematics", AssessmentSnapshot: assessment.ExamQuestionSnapshot{ID: "snapshot-1", SubjectCode: assessment.SubjectMathematics},
		Question: paper.Question{ID: "question-1", QuestionType: "calculation", Score: 2},
		Rubric:   paper.Rubric{ID: "rubric-1", QuestionID: "question-1", Version: "locked-v1", Status: "locked", MaxScore: 2, Points: []paper.RubricPoint{{ID: "p1", Description: "final result", Score: 2, Required: true, EvidenceRequirements: []paper.EvidenceRequirement{{Type: "final_result", Target: "x=2"}}}}},
	}
	return value, MathEvidenceSource{Artifacts: artifacts, Corrections: corrections}, artifacts, input
}

func minMathEvidenceQuality(value MathEvidenceQuality) float64 {
	minimum := value.Recognition
	for _, item := range []float64{value.Formula, value.Structure, value.Verification, value.RubricMapping} {
		if item < minimum {
			minimum = item
		}
	}
	return minimum
}
