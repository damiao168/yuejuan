package files

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MemoryStore struct {
	mu     sync.RWMutex
	next   int
	assets map[string]FileAsset
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{next: 1, assets: map[string]FileAsset{}}
}

func (s *MemoryStore) Create(_ context.Context, input CreateAssetInput) (FileAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.findDuplicateLocked(input.TenantID, input.OwnerType, input.OwnerID, input.HashSHA256); ok {
		return existing, ErrDuplicateFile
	}
	asset := FileAsset{
		ID:            fmt.Sprintf("file-%d", s.next),
		TenantID:      input.TenantID,
		SchoolID:      input.SchoolID,
		ExamID:        input.ExamID,
		SubmissionID:  input.SubmissionID,
		OwnerType:     input.OwnerType,
		OwnerID:       input.OwnerID,
		OriginalName:  input.OriginalName,
		ContentType:   input.ContentType,
		SizeBytes:     input.SizeBytes,
		HashSHA256:    input.HashSHA256,
		StorageBucket: input.StorageBucket,
		StorageKey:    input.StorageKey,
		Visibility:    input.Visibility,
		UploadedBy:    input.UploadedBy,
		CreatedAt:     time.Now().UTC(),
	}
	s.next++
	s.assets[asset.ID] = asset
	return asset, nil
}

func (s *MemoryStore) FindDuplicate(_ context.Context, tenantID string, ownerType string, ownerID string, hashSHA256 string) (FileAsset, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	asset, ok := s.findDuplicateLocked(tenantID, ownerType, ownerID, hashSHA256)
	return asset, ok, nil
}

func (s *MemoryStore) Get(_ context.Context, tenantID string, id string) (FileAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	asset, ok := s.assets[id]
	if !ok || asset.TenantID != tenantID || asset.DeletedAt != nil {
		return FileAsset{}, ErrNotFound
	}
	return asset, nil
}

func (s *MemoryStore) Delete(_ context.Context, tenantID string, id string) (FileAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	asset, ok := s.assets[id]
	if !ok || asset.TenantID != tenantID || asset.DeletedAt != nil {
		return FileAsset{}, ErrNotFound
	}
	now := time.Now().UTC()
	asset.DeletedAt = &now
	s.assets[id] = asset
	return asset, nil
}

func (s *MemoryStore) findDuplicateLocked(tenantID string, ownerType string, ownerID string, hashSHA256 string) (FileAsset, bool) {
	for _, asset := range s.assets {
		if asset.TenantID == tenantID && asset.OwnerType == ownerType && asset.OwnerID == ownerID && asset.HashSHA256 == hashSHA256 && asset.DeletedAt == nil {
			return asset, true
		}
	}
	return FileAsset{}, false
}
