package modelgovernance

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *MemoryStore) ListManagedAPIConfigs(_ context.Context, tenantID string) ([]ManagedAPIConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := []ManagedAPIConfig{}
	for _, item := range s.managedConfigs {
		if item.TenantID == tenantID {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		return items[i].ProviderKey < items[j].ProviderKey
	})
	return items, nil
}

func (s *MemoryStore) CreateManagedAPIConfig(_ context.Context, tenantID, _ string, input ManagedAPIConfigInput) (ManagedAPIConfig, error) {
	input.TenantID = tenantID
	normalized, err := normalizeManagedAPIInput(input, true)
	if err != nil {
		return ManagedAPIConfig{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.managedConfigs {
		if item.TenantID == tenantID && item.ProviderKey == normalized.ProviderKey {
			return ManagedAPIConfig{}, ErrConflict
		}
	}
	if normalized.IsDefault {
		s.clearManagedDefault(tenantID, "")
	}
	now := time.Now().UTC()
	id := uuid.NewString()
	item := ManagedAPIConfig{
		ID: id, TenantID: tenantID, ProviderKey: normalized.ProviderKey,
		DisplayName: normalized.DisplayName, AdapterType: normalized.AdapterType,
		BaseURL: normalized.BaseURL, ModelName: normalized.ModelName, ModelVersion: normalized.ModelVersion,
		Region: normalized.Region, CredentialConfigured: true, CredentialHint: credentialHint(normalized.APIKey),
		Status: normalized.Status, IsDefault: normalized.IsDefault, LastTestStatus: "untested",
		CreatedAt: now, UpdatedAt: now,
	}
	s.managedConfigs[id] = item
	s.managedSecrets[id] = normalized.APIKey
	return item, nil
}

func (s *MemoryStore) UpdateManagedAPIConfig(_ context.Context, tenantID, id string, input ManagedAPIConfigUpdateInput) (ManagedAPIConfig, error) {
	normalized, err := normalizeManagedAPIUpdate(input)
	if err != nil {
		return ManagedAPIConfig{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.managedConfigs[id]
	if !ok || item.TenantID != tenantID {
		return ManagedAPIConfig{}, ErrNotFound
	}
	if normalized.IsDefault {
		s.clearManagedDefault(tenantID, id)
	}
	connectionChanged := item.BaseURL != normalized.BaseURL || normalized.APIKey != ""
	item.DisplayName = normalized.DisplayName
	item.AdapterType = normalized.AdapterType
	item.BaseURL = normalized.BaseURL
	item.ModelName = normalized.ModelName
	item.ModelVersion = normalized.ModelVersion
	item.Region = normalized.Region
	item.Status = normalized.Status
	item.IsDefault = normalized.IsDefault
	item.UpdatedAt = time.Now().UTC()
	if normalized.APIKey != "" {
		s.managedSecrets[id] = normalized.APIKey
		item.CredentialHint = credentialHint(normalized.APIKey)
	}
	if connectionChanged {
		item.LastTestStatus = "untested"
		item.LastTestMessage = ""
		item.LastTestedAt = nil
	}
	s.managedConfigs[id] = item
	return item, nil
}

func (s *MemoryStore) GetManagedAPIConnection(_ context.Context, tenantID, id string) (ManagedAPIConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.managedConfigs[id]
	if !ok || item.TenantID != tenantID {
		return ManagedAPIConnection{}, ErrNotFound
	}
	secret := s.managedSecrets[id]
	if strings.TrimSpace(secret) == "" {
		return ManagedAPIConnection{}, ErrManagedConfigUnavailable
	}
	return ManagedAPIConnection{Config: item, APIKey: secret}, nil
}

func (s *MemoryStore) RecordManagedAPIProbe(_ context.Context, tenantID, id string, result ManagedAPIProbeResult) (ManagedAPIConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.managedConfigs[id]
	if !ok || item.TenantID != tenantID {
		return ManagedAPIConfig{}, ErrNotFound
	}
	item.LastTestStatus = "failed"
	if result.OK {
		item.LastTestStatus = "success"
	}
	item.LastTestMessage = strings.TrimSpace(result.Message)
	now := time.Now().UTC()
	item.LastTestedAt = &now
	item.UpdatedAt = now
	s.managedConfigs[id] = item
	return item, nil
}

func (s *MemoryStore) clearManagedDefault(tenantID, exceptID string) {
	for id, item := range s.managedConfigs {
		if item.TenantID == tenantID && id != exceptID && item.IsDefault {
			item.IsDefault = false
			item.UpdatedAt = time.Now().UTC()
			s.managedConfigs[id] = item
		}
	}
}

var _ ManagedAPIConfigStore = (*MemoryStore)(nil)
