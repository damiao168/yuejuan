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

func TestOverridePageQualityPreservesMachineEvidenceAndPassesAggregate(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	item, err := store.Create(ctx, "tenant-1", "exam-1", "actor-1", CreateSubmissionInput{SourceType: "scanner_upload", ExpectedPageCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.AddPage(ctx, "tenant-1", item.ID, "actor-1", AddPageInput{FileAssetID: "source-1", PageNo: 1})
	if err != nil {
		t.Fatal(err)
	}
	page, err = store.ApplyPageQualityResult(ctx, "tenant-1", ApplyPageQualityInput{
		SubmissionID:          item.ID,
		PageID:                page.ID,
		LatestQualityRunID:    "quality-run-1",
		NormalizedFileAssetID: "normalized-1",
		QualityStatus:         "failed",
		QualityIssues:         []QualityIssue{{Code: "blur", Message: "machine threshold exceeded"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	overridden, err := store.OverridePageQuality(ctx, "tenant-1", page.ID, "reviewer-1", OverridePageQualityInput{Reason: "人工核对页面清晰可阅卷"})
	if err != nil {
		t.Fatal(err)
	}
	if overridden.QualityStatus != "passed" || overridden.QualityOverride["decision"] != "accepted" ||
		overridden.QualityOverride["original_quality_status"] != "failed" ||
		overridden.QualityOverride["actor_id"] != "reviewer-1" {
		t.Fatalf("override evidence is incomplete: %#v", overridden)
	}
	if len(overridden.QualityIssues) != 1 || overridden.QualityIssues[0].Code != "blur" {
		t.Fatalf("machine issues must remain queryable: %#v", overridden.QualityIssues)
	}
	aggregate, err := store.Get(ctx, "tenant-1", item.ID)
	if err != nil || aggregate.QualityStatus != "passed" || aggregate.Status != "quality_checked" {
		t.Fatalf("override should update aggregate: %#v %v", aggregate, err)
	}
	replayed, err := store.OverridePageQuality(ctx, "tenant-1", page.ID, "reviewer-1", OverridePageQualityInput{Reason: "重复请求应恢复原决定"})
	if err != nil || replayed.QualityOverride["reason"] != "人工核对页面清晰可阅卷" {
		t.Fatalf("idempotent override replay must preserve the original decision: %#v %v", replayed, err)
	}
}
