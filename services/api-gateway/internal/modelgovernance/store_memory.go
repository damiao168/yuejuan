package modelgovernance

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MemoryStore struct {
	mu          sync.RWMutex
	providers   map[string]Provider
	deployments map[string]Deployment
	policies    map[string]TenantPolicy
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		providers:   map[string]Provider{},
		deployments: map[string]Deployment{},
		policies:    map[string]TenantPolicy{},
	}
}

func (s *MemoryStore) EnsureLocalBaseline(_ context.Context, tenantID string, baseline LocalBaseline) error {
	if strings.TrimSpace(tenantID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	providerID := ""
	for id, item := range s.providers {
		if item.TenantID == tenantID && item.Key == baseline.ProviderKey {
			providerID = id
			item.DisplayName = baseline.ProviderName
			item.AdapterType = baseline.AdapterType
			item.Region = baseline.Region
			item.UpdatedAt = now
			s.providers[id] = item
			break
		}
	}
	if providerID == "" {
		providerID = uuid.NewString()
		s.providers[providerID] = Provider{
			ID:          providerID,
			TenantID:    tenantID,
			Key:         baseline.ProviderKey,
			DisplayName: baseline.ProviderName,
			Kind:        ProviderLocal,
			AdapterType: baseline.AdapterType,
			Region:      baseline.Region,
			DataPolicy:  DataPolicy{RetentionMode: "no_store"},
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}
	for id, item := range s.deployments {
		if item.TenantID == tenantID && item.Key == baseline.DeploymentKey {
			item.ProviderID = providerID
			item.ProviderKey = baseline.ProviderKey
			item.ModelName = baseline.ModelName
			item.ModelVersion = baseline.ModelVersion
			item.Region = baseline.Region
			item.CapabilityProfile = baseline.CapabilityProfile
			item.UpdatedAt = now
			s.deployments[id] = item
			return nil
		}
	}
	id := uuid.NewString()
	s.deployments[id] = Deployment{
		ID:                id,
		TenantID:          tenantID,
		ProviderID:        providerID,
		ProviderKey:       baseline.ProviderKey,
		Key:               baseline.DeploymentKey,
		ModelName:         baseline.ModelName,
		ModelVersion:      baseline.ModelVersion,
		Region:            baseline.Region,
		CapabilityProfile: baseline.CapabilityProfile,
		Modalities:        []string{"text"},
		CapabilityPolicy:  map[string]any{},
		PricingPolicy:     map[string]any{"meter": "local_compute"},
		Status:            "shadow_only",
		HealthState:       "unverified",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	return nil
}

func (s *MemoryStore) ListProviders(_ context.Context, tenantID string) ([]Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Provider{}
	for _, item := range s.providers {
		if item.TenantID == tenantID {
			out = append(out, sanitizeProvider(item))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *MemoryStore) CreateProvider(_ context.Context, tenantID string, _ string, input ProviderInput) (Provider, error) {
	provider := ProviderFromInput(input)
	provider.TenantID = tenantID
	provider.DisplayName = strings.TrimSpace(provider.DisplayName)
	if provider.Status == "" {
		provider.Status = "unverified"
	}
	if provider.DisplayName == "" || ValidateProvider(provider) != nil {
		return Provider{}, ErrInvalidProvider
	}
	if provider.Kind == ProviderExternal && provider.Status != "unverified" && provider.Status != "disabled" {
		return Provider{}, ErrConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.providers {
		if item.TenantID == tenantID && item.Key == provider.Key {
			return Provider{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	provider.ID = uuid.NewString()
	provider.CreatedAt = now
	provider.UpdatedAt = now
	s.providers[provider.ID] = provider
	return sanitizeProvider(provider), nil
}

func (s *MemoryStore) UpdateProviderStatus(_ context.Context, tenantID string, id string, input ProviderStatusInput) (Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.providers[id]
	if !ok || item.TenantID != tenantID {
		return Provider{}, ErrNotFound
	}
	item.Status = input.Status
	if ValidateProvider(item) != nil {
		return Provider{}, ErrInvalidProvider
	}
	if item.Kind == ProviderExternal && item.Status != "unverified" && item.Status != "disabled" {
		return Provider{}, ErrConflict
	}
	item.UpdatedAt = time.Now().UTC()
	s.providers[id] = item
	return sanitizeProvider(item), nil
}

func (s *MemoryStore) ListDeployments(_ context.Context, tenantID string) ([]Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Deployment{}
	for _, item := range s.deployments {
		if item.TenantID == tenantID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *MemoryStore) CreateDeployment(_ context.Context, tenantID string, _ string, input DeploymentInput) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	provider, ok := s.providers[input.ProviderID]
	if !ok || provider.TenantID != tenantID {
		return Deployment{}, ErrNotFound
	}
	deployment := DeploymentFromInput(input, provider.Key)
	deployment.TenantID = tenantID
	if deployment.Status == "" {
		deployment.Status = "unverified"
	}
	if deployment.HealthState == "" {
		deployment.HealthState = "unverified"
	}
	if strings.TrimSpace(deployment.ModelName) == "" || ValidateDeployment(deployment, provider) != nil {
		return Deployment{}, ErrInvalidDeployment
	}
	if provider.Kind == ProviderExternal &&
		(deployment.Status != "unverified" || deployment.HealthState != "unverified") {
		return Deployment{}, ErrConflict
	}
	for _, item := range s.deployments {
		if item.TenantID == tenantID && item.Key == deployment.Key {
			return Deployment{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	deployment.ID = uuid.NewString()
	deployment.CreatedAt = now
	deployment.UpdatedAt = now
	s.deployments[deployment.ID] = deployment
	return deployment, nil
}

func (s *MemoryStore) UpdateDeploymentState(_ context.Context, tenantID string, id string, input DeploymentStateInput) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.deployments[id]
	if !ok || item.TenantID != tenantID {
		return Deployment{}, ErrNotFound
	}
	provider, ok := s.providers[item.ProviderID]
	if !ok {
		return Deployment{}, ErrNotFound
	}
	item.Status = input.Status
	item.HealthState = input.HealthState
	if ValidateDeployment(item, provider) != nil {
		return Deployment{}, ErrInvalidDeployment
	}
	externalStateAllowed := item.Status == "unverified" && item.HealthState == "unverified" ||
		item.Status == "disabled" &&
			(item.HealthState == "unverified" || item.HealthState == "unavailable" || item.HealthState == "disabled")
	if provider.Kind == ProviderExternal && !externalStateAllowed {
		return Deployment{}, ErrConflict
	}
	item.UpdatedAt = time.Now().UTC()
	s.deployments[id] = item
	return item, nil
}

func (s *MemoryStore) GetPolicy(_ context.Context, tenantID string) (TenantPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item, ok := s.policies[tenantID]; ok {
		return item, nil
	}
	item := DefaultTenantPolicy()
	item.ID = uuid.NewString()
	item.TenantID = tenantID
	item.PolicyKey = "default"
	item.DisplayName = "Default local-only policy"
	item.Status = "active"
	item.Version = 1
	item.UpdatedAt = time.Now().UTC()
	s.policies[tenantID] = item
	return item, nil
}

func (s *MemoryStore) UpdatePolicy(_ context.Context, tenantID string, _ string, input PolicyUpdateInput) (TenantPolicy, error) {
	policy := PolicyFromUpdate(input)
	if ValidateTenantPolicy(policy) != nil || input.ExpectedVersion < 1 || strings.TrimSpace(input.Reason) == "" {
		return TenantPolicy{}, ErrInvalidPolicy
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.policies[tenantID]
	if !ok {
		current = DefaultTenantPolicy()
		current.ID = uuid.NewString()
		current.TenantID = tenantID
		current.PolicyKey = "default"
		current.Status = "active"
		current.Version = 1
	}
	if current.Version != input.ExpectedVersion {
		return TenantPolicy{}, ErrConflict
	}
	policy.ID = current.ID
	policy.TenantID = tenantID
	policy.PolicyKey = "default"
	policy.DisplayName = strings.TrimSpace(input.DisplayName)
	if policy.DisplayName == "" {
		policy.DisplayName = current.DisplayName
	}
	policy.Status = "active"
	policy.Version = current.Version + 1
	policy.UpdatedAt = time.Now().UTC()
	s.policies[tenantID] = policy
	return policy, nil
}

func sanitizeProvider(provider Provider) Provider {
	provider.CredentialConfigured = provider.CredentialRef != ""
	if parsed, err := url.Parse(provider.CredentialRef); err == nil {
		provider.CredentialScheme = parsed.Scheme
	}
	provider.CredentialRef = ""
	return provider
}
