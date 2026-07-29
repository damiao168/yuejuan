package modelgovernance

import (
	"errors"
	"testing"
)

func TestProductionGateRejectsWeakConfiguredSecret(t *testing.T) {
	t.Setenv("STORY061_WEAK_KEY", "short")
	provider := productionExternalProvider("env://STORY061_WEAK_KEY")
	err := ValidateProductionInventory(
		[]Provider{provider},
		nil,
		NewEnvironmentSecretResolver(t.TempDir()),
	)
	if !errors.Is(err, ErrProductionUnsafe) {
		t.Fatalf("production gate accepted weak secret: %v", err)
	}
}

func TestProductionGateRejectsExternalDeploymentWithoutPricingMeter(t *testing.T) {
	provider := productionExternalProvider("vault://edugrade/vendor-a")
	deployment := productionExternalDeployment()
	deployment.PricingPolicy = map[string]any{"meter": "not_configured"}
	err := ValidateProductionInventory(
		[]Provider{provider},
		[]Deployment{deployment},
		NewEnvironmentSecretResolver(t.TempDir()),
	)
	if !errors.Is(err, ErrProductionUnsafe) {
		t.Fatalf("production gate accepted deployment without pricing meter: %v", err)
	}
}

func TestProductionGateAllowsCompleteUnverifiedNativeProviderMetadata(t *testing.T) {
	provider := productionExternalProvider("vault://edugrade/vendor-a")
	deployment := productionExternalDeployment()
	err := ValidateProductionInventory(
		[]Provider{provider},
		[]Deployment{deployment},
		NewEnvironmentSecretResolver(t.TempDir()),
	)
	if err != nil {
		t.Fatalf("complete unverified provider metadata should be stageable: %v", err)
	}
}

func TestProductionGateRejectsActiveProviderWithoutInstalledResolver(t *testing.T) {
	provider := productionExternalProvider("vault://edugrade/vendor-a")
	provider.Status = "active"
	err := ValidateProductionInventory(
		[]Provider{provider},
		nil,
		NewEnvironmentSecretResolver(t.TempDir()),
	)
	if !errors.Is(err, ErrProductionUnsafe) {
		t.Fatalf("production gate accepted active provider without resolver: %v", err)
	}
}

func productionExternalProvider(reference string) Provider {
	return Provider{
		ID:            "provider-a",
		TenantID:      "tenant-a",
		Key:           "vendor-a",
		DisplayName:   "Vendor A",
		Kind:          ProviderExternal,
		AdapterType:   "vendor_a_native",
		CredentialRef: reference,
		Region:        "cn-east",
		DataPolicy:    DataPolicy{RetentionMode: "no_store"},
		Status:        "unverified",
	}
}

func productionExternalDeployment() Deployment {
	return Deployment{
		ID:                "deployment-a",
		TenantID:          "tenant-a",
		ProviderID:        "provider-a",
		ProviderKey:       "vendor-a",
		Key:               "vendor-a-text-v1",
		ModelName:         "Vendor A Text",
		ModelVersion:      "vendor-a-model-v1",
		Region:            "cn-east",
		CapabilityProfile: "subjective-shadow-v1",
		Modalities:        []string{"text"},
		CapabilityPolicy:  map[string]any{},
		PricingPolicy:     map[string]any{"meter": "token"},
		Status:            "unverified",
		HealthState:       "unverified",
	}
}
