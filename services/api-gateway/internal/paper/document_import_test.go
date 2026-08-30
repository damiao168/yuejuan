package paper

import "testing"

func TestSortPaperImportOCRBlocksPreservesReadingOrderInsidePage(t *testing.T) {
	blocks := []PaperImportOCRBlock{
		{SourceID: "second", DocumentIndex: 1, PageNo: 1, BlockID: "b1"},
		{SourceID: "first", DocumentIndex: 0, PageNo: 2, BlockID: "b3"},
		{SourceID: "first", DocumentIndex: 0, PageNo: 1, BlockID: "b10"},
		{SourceID: "first", DocumentIndex: 0, PageNo: 1, BlockID: "b2"},
	}
	sortPaperImportOCRBlocks(blocks)
	got := []string{blocks[0].BlockID, blocks[1].BlockID, blocks[2].BlockID, blocks[3].BlockID}
	want := []string{"b10", "b2", "b3", "b1"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("block order = %#v, want %#v", got, want)
		}
	}
}

func TestPossiblePageMissingIssueIsSuspectedAndGrounded(t *testing.T) {
	sources := []PaperImportSource{{ID: "source", FileAssetID: "file", DocumentIndex: 2}}
	issues := possiblePageMissingIssues(sources, []PaperImportOCRBlock{{SourceID: "source", PageNo: 1}, {SourceID: "source", PageNo: 3}})
	if len(issues) != 1 || issues[0].Code != "POSSIBLE_PAGE_MISSING" || issues[0].Certainty != "suspected" || len(issues[0].SourceRefs) != 1 || issues[0].SourceRefs[0].PageNo != 2 {
		t.Fatalf("unexpected possible page issue: %#v", issues)
	}
}
