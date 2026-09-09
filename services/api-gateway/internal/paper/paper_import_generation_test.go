package paper

import (
	"context"
	"errors"
	"testing"
)

func TestPaperImportCancelThenSameSourceRerunCreatesGeneration(t *testing.T) {
	store := NewMemoryStore()
	job, err := store.CreatePaperImport(context.Background(), "tenant", "exam", "user", CreatePaperImportInput{Subject: "math", Sources: []CreatePaperImportSourceInput{{FileAssetID: "asset", DocumentIndex: 0, RoleHint: "question"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CancelPaperImportGeneration(context.Background(), "tenant", job.ID, job.Generation); err != nil {
		t.Fatal(err)
	}
	restarted, err := store.ReplacePaperImportSources(context.Background(), "tenant", job.ID, "user", ReplacePaperImportSourcesInput{ExpectedGeneration: job.Generation, Sources: []ReplacePaperImportSourceInput{{ID: job.Sources[0].ID, DocumentIndex: 0, RoleHint: "question"}}})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Generation != job.Generation+1 || restarted.RunID == job.RunID || restarted.Status != "processing" {
		t.Fatalf("rerun did not create a distinct generation: %#v", restarted)
	}
}

func TestPaperImportStaleCancelCannotCancelNewGeneration(t *testing.T) {
	store := NewMemoryStore()
	job, _ := store.CreatePaperImport(context.Background(), "tenant", "exam", "user", CreatePaperImportInput{Subject: "math", Sources: []CreatePaperImportSourceInput{{FileAssetID: "asset", DocumentIndex: 0, RoleHint: "question"}}})
	if _, err := store.CancelPaperImportGeneration(context.Background(), "tenant", job.ID, job.Generation); err != nil {
		t.Fatal(err)
	}
	restarted, err := store.ReplacePaperImportSources(context.Background(), "tenant", job.ID, "user", ReplacePaperImportSourcesInput{ExpectedGeneration: job.Generation, Sources: []ReplacePaperImportSourceInput{{ID: job.Sources[0].ID, DocumentIndex: 0, RoleHint: "question"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CancelPaperImportGeneration(context.Background(), "tenant", job.ID, job.Generation); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale cancellation error = %v", err)
	}
	current, _ := store.GetPaperImport(context.Background(), "tenant", job.ID)
	if current.Generation != restarted.Generation || current.Status != "processing" {
		t.Fatalf("stale cancel changed current run: %#v", current)
	}
}
