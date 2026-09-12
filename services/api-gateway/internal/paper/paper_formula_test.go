package paper

import (
	"strings"
	"testing"
)

func acceptedFormulaRegion() PaperImportFormulaRegion {
	latex := `x^{2}+1`
	return PaperImportFormulaRegion{SourceID: "source", DocumentIndex: 0, PageNo: 1, RegionID: "f1", BBox: []float64{10, 20, 80, 20}, DetectorModel: "PP-DocLayout_plus-L", DetectorConfidence: .9, CropSHA256: strings.Repeat("a", 64), EdgeInkRatio: .01, CropComplete: true, ValidationVersion: "latex-structure-render-v1", Candidates: []PaperImportFormulaCandidate{{ModelVersion: "PP-FormulaNet_plus-M", RawLatex: latex, CanonicalLatex: latex, Confidence: .91, Valid: true, SyntaxValid: true, StructureValid: true, ValidationAction: "accept"}}, SelectedLatex: latex, SelectedModel: "PP-FormulaNet_plus-M", Status: "accepted"}
}

func TestMergePaperFormulaResultPreservesMixedTextAsSegments(t *testing.T) {
	input := PaperImportParseRequest{Documents: []PaperImportParseDocument{{
		SourceID: "source", DocumentIndex: 0,
		Blocks: []PaperImportOCRBlock{{
			SourceID: "source", DocumentIndex: 0, PageNo: 1, BlockID: "mixed",
			Text: "已知函数f(x)=x²+1，则求其最小值", BBox: []float64{0, 20, 220, 22},
		}},
	}}}
	region := acceptedFormulaRegion()
	region.BBox = []float64{55, 20, 85, 22}
	region.SelectedLatex = `f(x)=x^{2}+1`
	region.Candidates[0].RawLatex = region.SelectedLatex
	region.Candidates[0].CanonicalLatex = region.SelectedLatex
	merged := mergePaperFormulaResult(input, []PaperImportFormulaRegion{region})
	blocks := merged.Documents[0].Blocks
	if len(blocks) != 1 || blocks[0].Kind != "mixed" || len(blocks[0].Segments) != 3 {
		t.Fatalf("mixed line was not retained as text/formula/text segments: %#v", blocks)
	}
	if blocks[0].Segments[0].Text != "已知函数" || blocks[0].Segments[1].Latex != region.SelectedLatex || blocks[0].Segments[2].Text != "，则求其最小值" {
		t.Fatalf("unexpected mixed segments: %#v", blocks[0].Segments)
	}
	if blocks[0].RawText == "" || !strings.Contains(merged.Documents[0].Content, `\(f(x)=x^{2}+1\)`) {
		t.Fatalf("source evidence or canonical content missing: %#v %q", blocks[0], merged.Documents[0].Content)
	}
}

func TestValidPaperFormulaResultAllowsUncertainDetectorWithoutModelCall(t *testing.T) {
	region := acceptedFormulaRegion()
	region.DetectorConfidence = .53
	region.CropComplete = true
	region.Candidates = nil
	region.SelectedLatex = ""
	region.SelectedModel = ""
	region.Status = "review_required"
	if !validPaperFormulaResult([]PaperImportParseDocument{{SourceID: "source", DocumentIndex: 0}}, []PaperImportFormulaRegion{region}) {
		t.Fatal("review-only detector result should not require a FormulaNet candidate")
	}
}

func TestValidPaperFormulaResultRejectsUnmatchedSelection(t *testing.T) {
	documents := []PaperImportParseDocument{{SourceID: "source", DocumentIndex: 0}}
	region := acceptedFormulaRegion()
	if !validPaperFormulaResult(documents, []PaperImportFormulaRegion{region}) {
		t.Fatal("valid region was rejected")
	}
	region.SelectedLatex = `x^{3}`
	if validPaperFormulaResult(documents, []PaperImportFormulaRegion{region}) {
		t.Fatal("unmatched selected formula was accepted")
	}
}

func TestMergePaperFormulaResultReplacesOverlappingOCRInPlace(t *testing.T) {
	input := PaperImportParseRequest{Documents: []PaperImportParseDocument{
		{
			SourceID: "source", DocumentIndex: 0,
			Blocks: []PaperImportOCRBlock{
				{SourceID: "source", DocumentIndex: 0, PageNo: 1, BlockID: "before", Text: "题目", BBox: []float64{1, 1, 20, 10}},
				{SourceID: "source", DocumentIndex: 0, PageNo: 1, BlockID: "bad-formula", Text: "x2+1", BBox: []float64{12, 20, 70, 18}},
				{SourceID: "source", DocumentIndex: 0, PageNo: 1, BlockID: "after", Text: "答案", BBox: []float64{1, 50, 20, 10}},
			},
		},
	}}
	merged := mergePaperFormulaResult(input, []PaperImportFormulaRegion{acceptedFormulaRegion()})
	blocks := merged.Documents[0].Blocks
	if len(blocks) != 3 || blocks[1].Kind != "formula" || blocks[1].Text != `\(x^{2}+1\)` || strings.Contains(merged.Documents[0].Content, "x2+1") {
		t.Fatalf("unexpected merged blocks/content: %#v %q", blocks, merged.Documents[0].Content)
	}
}
