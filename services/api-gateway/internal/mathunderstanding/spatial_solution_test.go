package mathunderstanding

import "testing"

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
