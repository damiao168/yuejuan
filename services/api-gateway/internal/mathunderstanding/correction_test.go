package mathunderstanding

import (
	"context"
	"errors"
	"testing"
)

func TestCorrectionsAreVersionBoundAndTenantIsolated(t *testing.T) {
	artifacts := NewMemoryStore()
	artifact, err := artifacts.CreateArtifact(context.Background(), "tenant-a", validInput())
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryCorrectionStore(artifacts)
	input := CreateCorrectionInput{ExpectedArtifactVersion: artifact.Version, Reason: "formula recognition correction", Operations: []CorrectionOperation{{Type: "correct_formula", TargetID: "formula-1", Payload: map[string]any{"canonical_latex": "x=3"}}}, CorrectedContract: validInput()}
	item, err := store.CreateCorrection(context.Background(), "tenant-a", artifact.ID, "teacher-1", input)
	if err != nil || item.Revision != 1 {
		t.Fatalf("create correction: %#v %v", item, err)
	}
	if items, _ := store.ListCorrections(context.Background(), "tenant-b", artifact.ID); len(items) != 0 {
		t.Fatal("cross tenant correction leaked")
	}
	input.ExpectedArtifactVersion++
	if _, err = store.CreateCorrection(context.Background(), "tenant-a", artifact.ID, "teacher-1", input); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale correction accepted: %v", err)
	}
}
