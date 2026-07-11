package capture

import (
	"context"
	"errors"
	"testing"
)

func TestCaptureBatchDecodeFlow(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	batch, err := store.CreateBatch(ctx, "tenant-1", "exam-1", "user-1", CreateBatchInput{Name: "第一扫描批次", SourceType: "web_upload", IdempotencyKey: "batch-1"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.CreateBatch(ctx, "tenant-1", "exam-1", "user-1", CreateBatchInput{Name: "ignored", SourceType: "web_upload", IdempotencyKey: "batch-1"})
	if err != nil || again.ID != batch.ID {
		t.Fatalf("idempotent create returned %#v, %v", again, err)
	}
	file, err := store.RegisterFile(ctx, "tenant-1", batch.ID, "user-1", RegisterFileInput{FileAssetID: "asset-1", IdempotencyKey: "file-1"}, FileAssetSnapshot{ID: "asset-1", ExamID: "exam-1", OriginalName: "answers.pdf", ContentType: "application/pdf", SizeBytes: 2048, SHA256: "sha256:a"})
	if err != nil {
		t.Fatal(err)
	}
	queued, err := store.QueueBatch(ctx, "tenant-1", batch.ID, "user-1")
	if err != nil || queued.Status != "processing" {
		t.Fatalf("queue returned %#v, %v", queued, err)
	}
	result, err := store.ApplyFileResult(ctx, "tenant-1", file.ID, []DecodedPageInput{
		{SourceIndex: 1, FileAssetID: "page-1", SHA256: "sha256:p1", Width: 2480, Height: 3508},
		{SourceIndex: 2, FileAssetID: "page-2", SHA256: "sha256:p2", Width: 2480, Height: 3508},
	})
	if err != nil || result.PageCount != 2 || result.Status != "completed" {
		t.Fatalf("result returned %#v, %v", result, err)
	}
	pages, err := store.ListPages(ctx, "tenant-1", batch.ID)
	if err != nil || len(pages) != 2 || pages[0].SubmissionID == "" || pages[0].SubmissionID != pages[1].SubmissionID {
		t.Fatalf("pages returned %#v, %v", pages, err)
	}
	if pages[0].Status != "quality_checking" {
		t.Fatalf("decoded page should wait for quality processing, got %q", pages[0].Status)
	}
	if err := store.ApplyQualityOutcome(ctx, "tenant-1", pages[0].SubmissionPageID, "passed"); err != nil {
		t.Fatal(err)
	}
	pages, _ = store.ListPages(ctx, "tenant-1", batch.ID)
	if pages[0].Status != "normalized" {
		t.Fatalf("passed page should expose normalized state, got %q", pages[0].Status)
	}
}

func TestCaptureRejectsStalePageRevisionAndCrossTenant(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	batch, _ := store.CreateBatch(ctx, "tenant-1", "exam-1", "user-1", CreateBatchInput{Name: "批次", SourceType: "web_upload"})
	file, _ := store.RegisterFile(ctx, "tenant-1", batch.ID, "user-1", RegisterFileInput{FileAssetID: "asset", IdempotencyKey: "file"}, FileAssetSnapshot{ID: "asset", SizeBytes: 10, SHA256: "sha256:a"})
	_, _ = store.QueueBatch(ctx, "tenant-1", batch.ID, "user-1")
	_, _ = store.ApplyFileResult(ctx, "tenant-1", file.ID, []DecodedPageInput{{SourceIndex: 1, FileAssetID: "page", SHA256: "sha256:p", Width: 100, Height: 200}})
	pages, _ := store.ListPages(ctx, "tenant-1", batch.ID)
	rotation := 90
	updated, err := store.UpdatePage(ctx, "tenant-1", pages[0].ID, "user-1", UpdatePageInput{Revision: 1, RotationDegrees: &rotation})
	if err != nil || updated.RotationDegrees != 90 {
		t.Fatalf("update returned %#v, %v", updated, err)
	}
	if _, err := store.UpdatePage(ctx, "tenant-1", pages[0].ID, "user-1", UpdatePageInput{Revision: 1, RotationDegrees: &rotation}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if _, err := store.GetBatch(ctx, "tenant-2", batch.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected tenant isolation, got %v", err)
	}
}
