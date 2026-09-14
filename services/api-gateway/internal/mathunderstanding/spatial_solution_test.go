package mathunderstanding

import (
	"math"
	"testing"
)

func block(id string, x, y float64) MathAnswerBlock {
	return MathAnswerBlock{ID: id, Kind: "formula", Status: "active", BoundingBox: BoundingBox{X: x, Y: y, Width: .25, Height: .08}, Normalized: "x", RecognitionEngine: "fixture", RecognitionVersion: "v1", RecognitionConfidence: .9, StructureConfidence: .8, SourceImageHash: "sha256:x"}
}

func TestSpatialGraphSupportsVerticalAndParallelStreams(t *testing.T) {
	blocks := []MathAnswerBlock{block("a1", .05, .05), block("a2", .05, .20), block("b1", .55, .05), block("b2", .55, .20), block("merge", .30, .40)}
	relations := BuildSpatialRelations(blocks, SpatialGraphOptions{})
	want := map[string]bool{"a1:a2": false, "b1:b2": false, "a2:merge": false, "b2:merge": false}
	for _, relation := range relations {
		key := relation.FromID + ":" + relation.ToID
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("missing spatial candidate %s: %#v", key, relations)
		}
	}
}

func TestSolutionBuilderProducesDAGAndExcludesCrossedOut(t *testing.T) {
	blocks := []MathAnswerBlock{block("a", .1, .1), block("b", .1, .3), block("old", .6, .2)}
	blocks[2].Status = "crossed_out"
	relations := BuildSpatialRelations(blocks, SpatialGraphOptions{})
	graph, err := BuildSolutionGraph(SolutionBuildInput{AnswerSegmentID: "segment-1", Blocks: blocks, Relations: relations, BuilderVersion: "graph-v1", FormulaModelVersion: "formula-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Steps) != 2 {
		t.Fatalf("crossed out block included: %#v", graph.Steps)
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected one transition: %#v", graph.Edges)
	}
}

func TestStepSegmenterGroupsSameLineMixedEvidence(t *testing.T) {
	blocks := []MathAnswerBlock{
		{ID: "text-prefix", Kind: "text", Status: "active", BoundingBox: BoundingBox{X: .05, Y: .10, Width: .18, Height: .08}, Text: "由题意", Normalized: "由题意", RecognitionEngine: "ocr", RecognitionVersion: "v1", RecognitionConfidence: .91, StructureConfidence: .9, SourceImageHash: "sha256:x"},
		{ID: "formula-main", Kind: "formula", Status: "active", BoundingBox: BoundingBox{X: .25, Y: .10, Width: .40, Height: .08}, Normalized: "x^2-5x+6=0", RecognitionEngine: "formula", RecognitionVersion: "v1", RecognitionConfidence: .95, StructureConfidence: .92, SourceImageHash: "sha256:x"},
		{ID: "text-suffix", Kind: "text", Status: "active", BoundingBox: BoundingBox{X: .67, Y: .10, Width: .05, Height: .08}, Text: "得", Normalized: "得", RecognitionEngine: "ocr", RecognitionVersion: "v1", RecognitionConfidence: .88, StructureConfidence: .86, SourceImageHash: "sha256:x"},
		{ID: "formula-next", Kind: "formula", Status: "active", BoundingBox: BoundingBox{X: .25, Y: .28, Width: .35, Height: .08}, Normalized: "(x-2)(x-3)=0", RecognitionEngine: "formula", RecognitionVersion: "v1", RecognitionConfidence: .93, StructureConfidence: .89, SourceImageHash: "sha256:x"},
	}
	formulaByBlock := map[string][]string{"formula-main": {"formula-1"}, "formula-next": {"formula-2"}}
	steps, stepByBlock := SegmentSolutionSteps(blocks, formulaByBlock)

	if len(steps) != 2 || len(steps[0].BlockIDs) != 3 {
		t.Fatalf("expected one mixed line and one following line, got %#v", steps)
	}
	if stepByBlock["text-prefix"] != stepByBlock["formula-main"] || stepByBlock["formula-main"] != stepByBlock["text-suffix"] {
		t.Fatalf("same-line blocks were not assigned to one step: %#v", stepByBlock)
	}
	if steps[0].Kind != "setup" || steps[0].NormalizedText != "由题意 x^2-5x+6=0 得" {
		t.Fatalf("unexpected mixed step semantics: %#v", steps[0])
	}
	if steps[0].BoundingBox == nil || steps[0].BoundingBox.X != .05 || steps[0].BoundingBox.Y != .10 || math.Abs(steps[0].BoundingBox.Width-.67) > .000001 || math.Abs(steps[0].BoundingBox.Height-.08) > .000001 {
		t.Fatalf("unexpected mixed step bbox: %#v", steps[0].BoundingBox)
	}
	if steps[0].RecognitionConfidence != .88 || steps[0].StructureConfidence > .86 || steps[0].Confidence != steps[0].StructureConfidence {
		t.Fatalf("critical confidence must be conservative: %#v", steps[0])
	}
}

func TestStepSegmenterDoesNotMergeParallelColumns(t *testing.T) {
	steps, _ := SegmentSolutionSteps(
		[]MathAnswerBlock{block("left", .05, .10), block("right", .55, .10)},
		map[string][]string{},
	)
	if len(steps) != 2 {
		t.Fatalf("parallel solution streams must remain separate: %#v", steps)
	}
}
