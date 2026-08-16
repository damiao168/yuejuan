package mathunderstanding

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type MemoryStore struct {
	mu        sync.RWMutex
	next      int64
	current   map[string]Artifact
	artifacts map[string]Artifact
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{next: 1, current: map[string]Artifact{}, artifacts: map[string]Artifact{}}
}

func (s *MemoryStore) CreateArtifact(_ context.Context, tenantID string, input CreateArtifactInput) (Artifact, error) {
	if tenantID == "" || ValidateCreateArtifact(input) != nil {
		return Artifact{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tenantID + ":" + input.AnswerSegmentID
	version := int64(1)
	if previous, ok := s.current[key]; ok {
		version = previous.Version + 1
		previous.IsCurrent = false
		s.artifacts[tenantID+":"+previous.ID] = previous
	}
	item := Artifact{ID: fmt.Sprintf("math-artifact-%d", s.next), TenantID: tenantID, AnswerSegmentID: input.AnswerSegmentID, ExamQuestionSnapshotID: input.ExamQuestionSnapshotID, Version: version, InputHash: input.InputHash, EngineVersion: input.EngineVersion, IsCurrent: true, CreateArtifactInput: cloneInput(input), CreatedAt: time.Now().UTC()}
	s.next++
	s.current[key] = item
	s.artifacts[tenantID+":"+item.ID] = item
	return cloneArtifact(item), nil
}

func (s *MemoryStore) GetArtifact(_ context.Context, tenantID string, artifactID string) (Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.artifacts[tenantID+":"+artifactID]
	if !ok {
		return Artifact{}, ErrNotFound
	}
	return cloneArtifact(item), nil
}

func (s *MemoryStore) GetLatestArtifact(_ context.Context, tenantID string, answerSegmentID string) (Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.current[tenantID+":"+answerSegmentID]
	if !ok {
		return Artifact{}, ErrNotFound
	}
	return cloneArtifact(item), nil
}

func cloneInput(input CreateArtifactInput) CreateArtifactInput {
	raw, _ := json.Marshal(input)
	var out CreateArtifactInput
	_ = json.Unmarshal(raw, &out)
	return out
}
func cloneArtifact(item Artifact) Artifact {
	raw, _ := json.Marshal(item)
	var out Artifact
	_ = json.Unmarshal(raw, &out)
	return out
}
