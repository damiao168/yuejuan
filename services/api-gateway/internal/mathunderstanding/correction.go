package mathunderstanding

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var ErrRevisionConflict = errors.New("math correction revision conflict")

type CorrectionOperation struct {
	Type     string         `json:"type"`
	TargetID string         `json:"target_id"`
	Payload  map[string]any `json:"payload"`
}
type CreateCorrectionInput struct {
	ExpectedArtifactVersion    int64                 `json:"expected_artifact_version"`
	ExpectedCorrectionRevision *int64                `json:"expected_correction_revision,omitempty"`
	Operations                 []CorrectionOperation `json:"operations"`
	CorrectedContract          CreateArtifactInput   `json:"corrected_contract"`
	Reason                     string                `json:"reason"`
}
type Correction struct {
	ID                string                `json:"id"`
	TenantID          string                `json:"tenant_id"`
	ArtifactID        string                `json:"artifact_id"`
	AnswerSegmentID   string                `json:"answer_segment_id"`
	Revision          int64                 `json:"revision"`
	Operations        []CorrectionOperation `json:"operations"`
	CorrectedContract CreateArtifactInput   `json:"corrected_contract"`
	Reason            string                `json:"reason"`
	CreatedBy         string                `json:"created_by"`
	CreatedAt         time.Time             `json:"created_at"`
}

type CorrectionStore interface {
	CreateCorrection(context.Context, string, string, string, CreateCorrectionInput) (Correction, error)
	GetCorrection(context.Context, string, string, int64) (Correction, error)
	GetLatestCorrection(context.Context, string, string) (Correction, error)
	ListCorrections(context.Context, string, string) ([]Correction, error)
	ExportCorrections(context.Context, string, string, int) ([]Correction, error)
}

var correctionOperations = set("move_step", "connect_edge", "delete_edge", "restore_block", "correct_formula", "merge_blocks", "split_step")

func validateCorrection(input CreateCorrectionInput) error {
	if input.ExpectedArtifactVersion <= 0 || (input.ExpectedCorrectionRevision != nil && *input.ExpectedCorrectionRevision < 0) || len(input.Operations) == 0 || len(input.Operations) > 200 || len(input.Reason) > 1000 || ValidateCreateArtifact(input.CorrectedContract) != nil {
		return ErrInvalidInput
	}
	for _, operation := range input.Operations {
		if !correctionOperations[operation.Type] || operation.TargetID == "" {
			return ErrInvalidInput
		}
	}
	return nil
}

type MemoryCorrectionStore struct {
	mu        sync.RWMutex
	next      int
	artifacts Store
	items     map[string][]Correction
}

func NewMemoryCorrectionStore(artifacts Store) *MemoryCorrectionStore {
	return &MemoryCorrectionStore{next: 1, artifacts: artifacts, items: map[string][]Correction{}}
}
func (s *MemoryCorrectionStore) CreateCorrection(ctx context.Context, tenantID, artifactID, actorID string, input CreateCorrectionInput) (Correction, error) {
	if tenantID == "" || actorID == "" || validateCorrection(input) != nil {
		return Correction{}, ErrInvalidInput
	}
	var artifact Artifact
	var err error
	if artifacts, ok := s.artifacts.(*MemoryStore); ok {
		artifacts.mu.RLock()
		defer artifacts.mu.RUnlock()
		stored, found := artifacts.artifacts[tenantID+":"+artifactID]
		if !found {
			return Correction{}, ErrNotFound
		}
		artifact = cloneArtifact(stored)
	} else {
		artifact, err = s.artifacts.GetArtifact(ctx, tenantID, artifactID)
	}
	if err != nil {
		return Correction{}, err
	}
	if !artifact.IsCurrent || artifact.Version != input.ExpectedArtifactVersion || !correctionMatchesArtifact(input.CorrectedContract, artifact) {
		return Correction{}, ErrRevisionConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tenantID + ":" + artifactID
	revision := int64(len(s.items[key]))
	if input.ExpectedCorrectionRevision != nil && *input.ExpectedCorrectionRevision != revision {
		return Correction{}, ErrRevisionConflict
	}
	item := Correction{ID: "math-correction-" + time.Now().UTC().Format("20060102150405.000000000"), TenantID: tenantID, ArtifactID: artifactID, AnswerSegmentID: artifact.AnswerSegmentID, Revision: revision + 1, Operations: input.Operations, CorrectedContract: input.CorrectedContract, Reason: input.Reason, CreatedBy: actorID, CreatedAt: time.Now().UTC()}
	s.items[key] = append(s.items[key], cloneCorrection(item))
	return cloneCorrection(item), nil
}
func (s *MemoryCorrectionStore) ListCorrections(_ context.Context, tenantID, artifactID string) ([]Correction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Correction{}
	for _, item := range s.items[tenantID+":"+artifactID] {
		out = append(out, cloneCorrection(item))
	}
	return out, nil
}
func (s *MemoryCorrectionStore) GetLatestCorrection(_ context.Context, tenantID, artifactID string) (Correction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.items[tenantID+":"+artifactID]
	if len(items) == 0 {
		return Correction{}, ErrNotFound
	}
	return cloneCorrection(items[len(items)-1]), nil
}
func (s *MemoryCorrectionStore) GetCorrection(_ context.Context, tenantID, artifactID string, revision int64) (Correction, error) {
	if revision <= 0 {
		return Correction{}, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.items[tenantID+":"+artifactID]
	if revision > int64(len(items)) || items[revision-1].Revision != revision {
		return Correction{}, ErrNotFound
	}
	return cloneCorrection(items[revision-1]), nil
}
func (s *MemoryCorrectionStore) ExportCorrections(_ context.Context, tenantID, subject string, limit int) ([]Correction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Correction{}
	for key, items := range s.items {
		if len(key) < len(tenantID)+1 || key[:len(tenantID)+1] != tenantID+":" {
			continue
		}
		for _, item := range items {
			if item.CorrectedContract.SubjectCode == subject {
				out = append(out, cloneCorrection(item))
				if limit > 0 && len(out) >= limit {
					return out, nil
				}
			}
		}
	}
	return out, nil
}

func cloneCorrection(item Correction) Correction {
	raw, _ := json.Marshal(item)
	var out Correction
	_ = json.Unmarshal(raw, &out)
	return out
}

func correctionMatchesArtifact(contract CreateArtifactInput, artifact Artifact) bool {
	return contract.AnswerSegmentID == artifact.AnswerSegmentID &&
		contract.ExamQuestionSnapshotID == artifact.ExamQuestionSnapshotID &&
		contract.SubjectCode == artifact.SubjectCode &&
		contract.InputHash == artifact.InputHash &&
		contract.EngineVersion == artifact.EngineVersion
}

// ResolveEffectiveArtifact is the single projection boundary for mathematical
// evidence. Callers must not independently interpret the correction log.
func ResolveEffectiveArtifact(ctx context.Context, artifacts Store, corrections CorrectionStore, tenantID, answerSegmentID string) (EffectiveArtifact, error) {
	base, err := artifacts.GetLatestArtifact(ctx, tenantID, answerSegmentID)
	if err != nil {
		return EffectiveArtifact{}, err
	}
	return resolveEffectiveContract(ctx, corrections, tenantID, base)
}

func resolveEffectiveContract(ctx context.Context, corrections CorrectionStore, tenantID string, base Artifact) (EffectiveArtifact, error) {
	effective := EffectiveArtifact{
		BaseArtifact:      base,
		EffectiveContract: cloneInput(base.CreateArtifactInput),
	}
	latest, err := corrections.GetLatestCorrection(ctx, tenantID, base.ID)
	if errors.Is(err, ErrNotFound) {
		return effective, nil
	}
	if err != nil {
		return EffectiveArtifact{}, err
	}
	if latest.TenantID != tenantID || latest.ArtifactID != base.ID || latest.Revision <= 0 ||
		!correctionMatchesArtifact(latest.CorrectedContract, base) || ValidateCreateArtifact(latest.CorrectedContract) != nil {
		return EffectiveArtifact{}, ErrInvalidInput
	}
	effective.EffectiveContract = cloneInput(latest.CorrectedContract)
	effective.CorrectionRevision = latest.Revision
	effective.Corrected = true
	return effective, nil
}
