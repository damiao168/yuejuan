package submission

import (
	"context"
	"testing"
)

func TestAddPageInitializesImageQualityFields(t *testing.T) {
	store := NewMemoryStore()
	item, err := store.Create(context.Background(), "tenant-1", "exam-1", "actor-1", CreateSubmissionInput{SourceType: "scanner_upload", ExpectedPageCount: 1})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}

	page, err := store.AddPage(context.Background(), "tenant-1", item.ID, "actor-1", AddPageInput{FileAssetID: "file-1", PageNo: 1})
	if err != nil {
		t.Fatalf("add page: %v", err)
	}
	if page.QualityStatus != "unchecked" {
		t.Fatalf("expected page quality unchecked, got %s", page.QualityStatus)
	}
	if page.LatestQualityRunID != "" || page.NormalizedFileAssetID != "" {
		t.Fatalf("new page should not have quality pointers: %#v", page)
	}
	if len(page.QualityOverride) != 0 {
		t.Fatalf("new page should not have quality override: %#v", page.QualityOverride)
	}
}

func TestReplacePageInvalidatesImageQualityPointers(t *testing.T) {
	store := NewMemoryStore()
	item, err := store.Create(context.Background(), "tenant-1", "exam-1", "actor-1", CreateSubmissionInput{SourceType: "scanner_upload", ExpectedPageCount: 1})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	page, err := store.AddPage(context.Background(), "tenant-1", item.ID, "actor-1", AddPageInput{FileAssetID: "file-1", PageNo: 1})
	if err != nil {
		t.Fatalf("add page: %v", err)
	}
	store.MarkPageQualityForTest(page.ID, "quality-run-1", "file-normalized-1", "passed")

	replaced, err := store.ReplacePage(context.Background(), "tenant-1", item.ID, "actor-1", 1, AddPageInput{FileAssetID: "file-2"})
	if err != nil {
		t.Fatalf("replace page: %v", err)
	}
	if replaced.FileAssetID != "file-2" {
		t.Fatalf("expected replacement file, got %s", replaced.FileAssetID)
	}
	if replaced.QualityStatus != "unchecked" {
		t.Fatalf("replace should reset quality status, got %s", replaced.QualityStatus)
	}
	if replaced.LatestQualityRunID != "" || replaced.NormalizedFileAssetID != "" {
		t.Fatalf("replace should clear quality pointers: %#v", replaced)
	}
	if len(replaced.QualityOverride) != 0 {
		t.Fatalf("replace should clear quality override: %#v", replaced.QualityOverride)
	}
}
