package mathunderstanding

import (
	"context"
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
	ExpectedArtifactVersion int64                 `json:"expected_artifact_version"`
	Operations              []CorrectionOperation `json:"operations"`
	CorrectedContract       CreateArtifactInput   `json:"corrected_contract"`
	Reason                  string                `json:"reason"`
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
	ListCorrections(context.Context, string, string) ([]Correction, error)
	ExportCorrections(context.Context, string, string, int) ([]Correction, error)
}

var correctionOperations = set("move_step", "connect_edge", "delete_edge", "restore_block", "correct_formula", "merge_blocks", "split_step")

func validateCorrection(input CreateCorrectionInput) error {
	if input.ExpectedArtifactVersion <= 0 || len(input.Operations) == 0 || len(input.Operations) > 200 || len(input.Reason) > 1000 || ValidateCreateArtifact(input.CorrectedContract) != nil {
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
	artifact, err := s.artifacts.GetArtifact(ctx, tenantID, artifactID)
	if err != nil {
		return Correction{}, err
	}
	if artifact.Version != input.ExpectedArtifactVersion || input.CorrectedContract.AnswerSegmentID != artifact.AnswerSegmentID || input.CorrectedContract.ExamQuestionSnapshotID != artifact.ExamQuestionSnapshotID {
		return Correction{}, ErrRevisionConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tenantID + ":" + artifactID
	item := Correction{ID: "math-correction-" + time.Now().UTC().Format("20060102150405.000000000"), TenantID: tenantID, ArtifactID: artifactID, AnswerSegmentID: artifact.AnswerSegmentID, Revision: int64(len(s.items[key]) + 1), Operations: input.Operations, CorrectedContract: cloneInput(input.CorrectedContract), Reason: input.Reason, CreatedBy: actorID, CreatedAt: time.Now().UTC()}
	s.items[key] = append(s.items[key], item)
	return item, nil
}
func (s *MemoryCorrectionStore) ListCorrections(_ context.Context, tenantID, artifactID string) ([]Correction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Correction(nil), s.items[tenantID+":"+artifactID]...), nil
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
				out = append(out, item)
				if limit > 0 && len(out) >= limit {
					return out, nil
				}
			}
		}
	}
	return out, nil
}
