package modelgovernance

import "context"

type Store interface {
	EnsureLocalBaseline(ctx context.Context, tenantID string, baseline LocalBaseline) error
	ListProviders(ctx context.Context, tenantID string) ([]Provider, error)
	CreateProvider(ctx context.Context, tenantID string, actorID string, input ProviderInput) (Provider, error)
	UpdateProviderStatus(ctx context.Context, tenantID string, id string, input ProviderStatusInput) (Provider, error)
	ListDeployments(ctx context.Context, tenantID string) ([]Deployment, error)
	CreateDeployment(ctx context.Context, tenantID string, actorID string, input DeploymentInput) (Deployment, error)
	UpdateDeploymentState(ctx context.Context, tenantID string, id string, input DeploymentStateInput) (Deployment, error)
	GetPolicy(ctx context.Context, tenantID string) (TenantPolicy, error)
	UpdatePolicy(ctx context.Context, tenantID string, actorID string, input PolicyUpdateInput) (TenantPolicy, error)
}

func ProviderFromInput(input ProviderInput) Provider {
	return Provider{
		Key:           input.Key,
		DisplayName:   input.DisplayName,
		Kind:          input.Kind,
		AdapterType:   input.AdapterType,
		CredentialRef: input.CredentialRef,
		Region:        input.Region,
		DataPolicy:    input.DataPolicy,
		Status:        input.Status,
	}
}

func DeploymentFromInput(input DeploymentInput, providerKey string) Deployment {
	return Deployment{
		ProviderID:        input.ProviderID,
		Key:               input.Key,
		ProviderKey:       providerKey,
		ModelName:         input.ModelName,
		ModelVersion:      input.ModelVersion,
		Region:            input.Region,
		CapabilityProfile: input.CapabilityProfile,
		Modalities:        input.Modalities,
		CapabilityPolicy:  input.CapabilityPolicy,
		PricingPolicy:     input.PricingPolicy,
		Status:            input.Status,
		HealthState:       input.HealthState,
	}
}

func PolicyFromUpdate(input PolicyUpdateInput) TenantPolicy {
	return TenantPolicy{
		Mode:                     input.Mode,
		ExternalEnabled:          input.ExternalEnabled,
		TextExportEnabled:        input.TextExportEnabled,
		ImageExportEnabled:       input.ImageExportEnabled,
		AllowedDeployments:       input.AllowedDeployments,
		MaxCostMicrosPerQuestion: input.MaxCostMicrosPerQuestion,
		MaxCostMicrosPerExam:     input.MaxCostMicrosPerExam,
		FallbackMode:             input.FallbackMode,
	}
}
