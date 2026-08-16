package mathunderstanding

import (
	"context"
	"errors"
	"testing"
)

func validInput() CreateArtifactInput {
	return CreateArtifactInput{
		SubjectCode: "mathematics", AnswerSegmentID: "segment-1", ExamQuestionSnapshotID: "snapshot-1", InputHash: "sha256:crop-1", EngineVersion: "contract-fixture-v1",
		Blocks:         []MathAnswerBlock{{ID: "block-1", Kind: "formula", Status: "active", BoundingBox: BoundingBox{X: .1, Y: .2, Width: .5, Height: .2}, RecognitionEngine: "fixture", RecognitionVersion: "v1", RecognitionConfidence: .9, StructureConfidence: .8, SourceImageHash: "sha256:crop-1"}},
		Formulas:       []FormulaArtifact{{ID: "formula-1", BlockID: "block-1", BoundingBox: BoundingBox{X: .1, Y: .2, Width: .5, Height: .2}, RawLatex: "x=2", CanonicalLatex: "x=2", RecognitionEngine: "fixture", RecognitionVersion: "v1", ParserVersion: "restricted-v1", ParseStatus: "parsed", Confidence: .8, AST: &FormulaAST{Kind: "equation", Children: []FormulaAST{{Kind: "symbol", Value: "x"}, {Kind: "number", Value: "2"}}}}},
		SolutionGraph:  SolutionGraph{ID: "graph-1", AnswerSegmentID: "segment-1", BuilderVersion: "fixture-v1", FormulaModelVersion: "fixture-v1", OverallConfidence: .8, Steps: []SolutionStep{{ID: "step-1", OrderHint: 1, BlockIDs: []string{"block-1"}, FormulaIDs: []string{"formula-1"}, Confidence: .8}}},
		Verifications:  []MathVerification{{ID: "check-1", StepID: "step-1", FormulaID: "formula-1", Kind: "syntax", Status: "verified", ReasonCode: "ast_well_formed", Domain: "real", Engine: "fixture", EngineVersion: "v1", RulesetVersion: "v1", Confidence: 1}},
		RubricEvidence: []RubricEvidence{{ID: "evidence-1", RubricCriterionKey: "solve_equation", EvidenceType: "math_step", SourceArtifactIDs: []string{"step-1", "formula-1"}, Status: "supported", Confidence: .8}},
	}
}

func TestValidateCreateArtifactRejectsHumanities(t *testing.T) {
	input := validInput()
	input.SubjectCode = "history"
	if !errors.Is(ValidateCreateArtifact(input), ErrInvalidInput) {
		t.Fatal("humanities artifact entered formula pipeline")
	}
}

func TestValidateCreateArtifactAcceptsRestrictedEvidenceGraph(t *testing.T) {
	if err := ValidateCreateArtifact(validInput()); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
}

func TestValidateCreateArtifactRejectsCyclesAndExecutableAST(t *testing.T) {
	input := validInput()
	input.SolutionGraph.Steps = append(input.SolutionGraph.Steps, SolutionStep{ID: "step-2", BlockIDs: []string{"block-1"}, Confidence: 1})
	input.SolutionGraph.Edges = []SolutionEdge{{FromStepID: "step-1", ToStepID: "step-2", Kind: "next"}, {FromStepID: "step-2", ToStepID: "step-1", Kind: "next"}}
	if !errors.Is(ValidateCreateArtifact(input), ErrInvalidInput) {
		t.Fatal("cyclic solution graph accepted")
	}
	input = validInput()
	input.Formulas[0].AST.Kind = "execute"
	if !errors.Is(ValidateCreateArtifact(input), ErrInvalidInput) {
		t.Fatal("unrestricted AST node accepted")
	}
}

func TestMemoryStoreVersionsAndIsolatesArtifacts(t *testing.T) {
	store := NewMemoryStore()
	first, err := store.CreateArtifact(context.Background(), "tenant-a", validInput())
	if err != nil || first.Version != 1 {
		t.Fatalf("create first: %#v %v", first, err)
	}
	input := validInput()
	input.InputHash = "sha256:crop-2"
	second, err := store.CreateArtifact(context.Background(), "tenant-a", input)
	if err != nil || second.Version != 2 {
		t.Fatalf("create second: %#v %v", second, err)
	}
	latest, err := store.GetLatestArtifact(context.Background(), "tenant-a", input.AnswerSegmentID)
	if err != nil || latest.InputHash != input.InputHash {
		t.Fatalf("latest artifact: %#v %v", latest, err)
	}
	if _, err = store.GetLatestArtifact(context.Background(), "tenant-b", input.AnswerSegmentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant artifact leaked: %v", err)
	}
}
