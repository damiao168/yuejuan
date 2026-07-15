package paper

import (
	"context"
	"testing"
)

func TestMemoryTemplateDifferenceBindsExamPaperReference(t *testing.T) {
	store := NewMemoryStore()
	paper, err := store.CreatePaper(context.Background(), "tenant-1", "exam-1", "user-1", CreatePaperInput{File: FileAssetInput{
		OriginalName: "blank-sheet.pdf", ContentType: "application/pdf", SizeBytes: 10, HashSHA256: "paper-sha", StorageBucket: "bucket", StorageKey: "blank-sheet.pdf",
	}})
	if err != nil {
		t.Fatalf("create paper: %v", err)
	}
	layout := TemplateLayout{
		OMRProfile: TemplateOMRProfile{Mode: OMRProfileModeTemplateDifference, Version: OMRProfileVersionTemplateDifferenceBubbleV1},
		Pages:      []TemplatePage{{PageNo: 1, Width: 100, Height: 100}},
	}
	template, err := store.CreateTemplate(context.Background(), "tenant-1", "exam-1", "user-1", CreateTemplateInput{ExamPaperID: paper.ID, Name: "difference", PageCount: 1, Layout: layout})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if template.Layout.OMRProfile.Reference == nil {
		t.Fatal("difference template must bind an immutable paper reference")
	}
	if got := template.Layout.OMRProfile.Reference; got.FileAssetID != paper.FileAssetID || got.HashSHA256 != paper.File.HashSHA256 || got.ContentType != paper.File.ContentType {
		t.Fatalf("unexpected reference snapshot: %#v", got)
	}

	forged := layout
	forgedReference := TemplateOMRReference{Source: OMRReferenceSourceExamPaper, FileAssetID: "other", HashSHA256: "other", ContentType: "image/png"}
	forged.OMRProfile.Reference = &forgedReference
	updated, err := store.UpdateTemplate(context.Background(), "tenant-1", template.ID, UpdateTemplateInput{Name: template.Name, PageCount: 1, Layout: forged, ExpectedRevision: template.Revision})
	if err != nil {
		t.Fatalf("update template: %v", err)
	}
	if got := updated.Layout.OMRProfile.Reference; got == nil || got.FileAssetID != paper.FileAssetID || got.HashSHA256 != paper.File.HashSHA256 {
		t.Fatalf("server must overwrite client-supplied reference metadata: %#v", got)
	}
}
