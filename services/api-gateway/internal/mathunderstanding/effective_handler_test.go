package mathunderstanding

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestMathUnderstandingAPIReturnsEffectiveContractAndKeepsBase(t *testing.T) {
	ctx := context.Background()
	artifacts := NewMemoryStore()
	base, err := artifacts.CreateArtifact(ctx, "tenant-a", validInput())
	if err != nil {
		t.Fatal(err)
	}
	corrections := NewMemoryCorrectionStore(artifacts)
	lookup := &assignmentLookupStub{allowed: true}
	handler := NewHandler(artifacts, corrections, nil, lookup, nil)
	get := func(tenantID string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.SetPathValue("segmentId", base.AnswerSegmentID)
		r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: "teacher-1", TenantID: tenantID}))
		w := httptest.NewRecorder()
		handler.GetLatest(w, r)
		return w
	}
	assertProjection := func(w *httptest.ResponseRecorder, revision int64, latex string) {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("get understanding: %d %s", w.Code, w.Body.String())
		}
		var response struct {
			Artifact           Artifact            `json:"artifact"`
			EffectiveArtifact  CreateArtifactInput `json:"effective_artifact"`
			CorrectionRevision int64               `json:"correction_revision"`
			Corrected          bool                `json:"corrected"`
			Corrections        []Correction        `json:"corrections"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Artifact.ID != base.ID || response.Artifact.Formulas[0].CanonicalLatex != "x=2" {
			t.Fatal("raw artifact no longer represents immutable recognition")
		}
		if response.EffectiveArtifact.Formulas[0].CanonicalLatex != latex || response.CorrectionRevision != revision || response.Corrected != (revision > 0) {
			t.Fatalf("wrong effective projection: %s", w.Body.String())
		}
		if err := ValidateCreateArtifact(response.EffectiveArtifact); err != nil {
			t.Fatalf("effective contract invalid: %v", err)
		}
	}
	assertProjection(get("tenant-a"), 0, "x=2")
	revision := int64(0)
	corrected := validInput()
	corrected.Formulas[0].CanonicalLatex = "x=3"
	input := CreateCorrectionInput{
		ExpectedArtifactVersion: base.Version, ExpectedCorrectionRevision: &revision,
		Operations:        []CorrectionOperation{{Type: "correct_formula", TargetID: "formula-1", Payload: map[string]any{"after": "x=3"}}},
		CorrectedContract: corrected,
	}
	post := func() *httptest.ResponseRecorder {
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
		r.SetPathValue("artifactId", base.ID)
		r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: "teacher-1", TenantID: "tenant-a"}))
		w := httptest.NewRecorder()
		handler.CreateCorrection(w, r)
		return w
	}
	if w := post(); w.Code != http.StatusCreated {
		t.Fatalf("save correction: %d %s", w.Code, w.Body.String())
	}
	assertProjection(get("tenant-a"), 1, "x=3")
	if w := post(); w.Code != http.StatusConflict {
		t.Fatalf("stale effective version accepted: %d %s", w.Code, w.Body.String())
	}
	lookup.allowed = false
	if w := get("tenant-a"); w.Code != http.StatusForbidden {
		t.Fatalf("unassigned reviewer saw evidence: %d", w.Code)
	}
	if w := get("tenant-b"); w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant evidence leaked: %d", w.Code)
	}
}
