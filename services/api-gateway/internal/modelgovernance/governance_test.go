package modelgovernance

import (
	"errors"
	"testing"
)

func TestDefaultPolicyCannotRouteToExternalDeployment(t *testing.T) {
	external := Provider{
		Key:           "vendor-a",
		Kind:          ProviderExternal,
		AdapterType:   "vendor_a_native",
		CredentialRef: "vault://edugrade/vendor-a",
		Region:        "cn-east",
		DataPolicy:    DataPolicy{RetentionMode: "no_store"},
		Status:        "active",
	}
	deployment := Deployment{
		Key:               "vendor-a-text-v1",
		ProviderKey:       external.Key,
		ModelVersion:      "vendor-a-model-v1",
		Region:            "cn-east",
		CapabilityProfile: "subjective-shadow-v1",
		Modalities:        []string{"text"},
		Status:            "shadow_only",
		HealthState:       "available",
	}
	_, err := SelectDeployment(DefaultTenantPolicy(), RouteRequest{Modality: "text"}, []Provider{external}, []Deployment{deployment})
	if !errors.Is(err, ErrNoDeployment) {
		t.Fatalf("default policy must fail closed instead of exporting data, got %v", err)
	}
}

func TestExternalDeploymentRequiresEveryAuthorizationLayer(t *testing.T) {
	external := Provider{
		Key:           "vendor-a",
		Kind:          ProviderExternal,
		AdapterType:   "vendor_a_native",
		CredentialRef: "vault://edugrade/vendor-a",
		Region:        "cn-east",
		DataPolicy:    DataPolicy{RetentionMode: "no_store"},
		Status:        "active",
	}
	deployment := Deployment{
		Key:               "vendor-a-text-v1",
		ProviderKey:       external.Key,
		ModelVersion:      "vendor-a-model-v1",
		Region:            "cn-east",
		CapabilityProfile: "subjective-shadow-v1",
		Modalities:        []string{"text"},
		Status:            "shadow_only",
		HealthState:       "available",
	}
	policy := TenantPolicy{
		Mode:               ModeShadowCompare,
		ExternalEnabled:    true,
		TextExportEnabled:  true,
		AllowedDeployments: []string{deployment.Key},
		FallbackMode:       "manual_only",
	}
	decision, err := SelectDeployment(policy, RouteRequest{Modality: "text"}, []Provider{external}, []Deployment{deployment})
	if err != nil {
		t.Fatalf("explicitly authorized shadow deployment should route: %v", err)
	}
	if decision.ProviderKey != external.Key || decision.DeploymentKey != deployment.Key {
		t.Fatalf("unexpected route decision: %#v", decision)
	}
	policy.TextExportEnabled = false
	if _, err := SelectDeployment(policy, RouteRequest{Modality: "text"}, []Provider{external}, []Deployment{deployment}); !errors.Is(err, ErrNoDeployment) {
		t.Fatalf("text export opt-out must not be bypassed, got %v", err)
	}
}

func TestProviderRejectsPlaintextCredentialAndCompatibilityAdapter(t *testing.T) {
	provider := Provider{
		Key:           "vendor-a",
		Kind:          ProviderExternal,
		AdapterType:   "vendor_a_native",
		CredentialRef: "secret-value",
		Region:        "cn-east",
		DataPolicy:    DataPolicy{RetentionMode: "no_store"},
		Status:        "unverified",
	}
	if !errors.Is(ValidateProvider(provider), ErrInvalidProvider) {
		t.Fatal("plaintext credential must be rejected")
	}
	provider.CredentialRef = "vault://edugrade/vendor-a"
	provider.AdapterType = "openai_compatible"
	if !errors.Is(ValidateProvider(provider), ErrInvalidProvider) {
		t.Fatal("OpenAI-compatible external adapter must be rejected")
	}
}

func TestLocalDeploymentRemainsAvailableWithoutExternalAuthorization(t *testing.T) {
	local := Provider{
		Key:         "local",
		Kind:        ProviderLocal,
		AdapterType: "local_llama_cpp",
		Region:      "on_premise",
		DataPolicy:  DataPolicy{RetentionMode: "no_store"},
		Status:      "active",
	}
	deployment := Deployment{
		Key:               "local-qwen3-4b-q4-k-m",
		ProviderKey:       local.Key,
		ModelVersion:      "Qwen/Qwen3-4B-GGUF:Q4_K_M",
		Region:            "on_premise",
		CapabilityProfile: "local-pilot-v1",
		Modalities:        []string{"text"},
		Status:            "shadow_only",
		HealthState:       "available",
	}
	decision, err := SelectDeployment(DefaultTenantPolicy(), RouteRequest{Modality: "text"}, []Provider{local}, []Deployment{deployment})
	if err != nil {
		t.Fatalf("local-only default should keep the governed local deployment available: %v", err)
	}
	if decision.ProviderKey != "local" || decision.DeploymentKey != deployment.Key {
		t.Fatalf("unexpected local decision: %#v", decision)
	}
}
