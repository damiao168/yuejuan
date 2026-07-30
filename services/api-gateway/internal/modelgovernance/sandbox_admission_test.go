package modelgovernance

import (
	"slices"
	"testing"
	"time"
)

func TestAssessSandboxAdmissionAllowsBoundedSyntheticShadowText(t *testing.T) {
	provider, deployment, policy, secret, evidence, now := validSandboxAdmission()
	decision := AssessSandboxAdmission(provider, deployment, policy, secret, evidence, "text", now)
	if !decision.Allowed || len(decision.Blockers) != 0 {
		t.Fatalf("valid sandbox admission was blocked: %+v", decision)
	}
}

func TestAssessSandboxAdmissionRequiresSeparateImageApproval(t *testing.T) {
	provider, deployment, policy, secret, evidence, now := validSandboxAdmission()
	deployment.Modalities = []string{"text", "image"}
	policy.ImageExportEnabled = true

	decision := AssessSandboxAdmission(provider, deployment, policy, secret, evidence, "image", now)
	assertSandboxBlocker(t, decision, "image_export_not_approved")

	evidence.ImageExportReviewed = true
	decision = AssessSandboxAdmission(provider, deployment, policy, secret, evidence, "image", now)
	if !decision.Allowed {
		t.Fatalf("reviewed image sandbox was blocked: %+v", decision)
	}
}

func TestAssessSandboxAdmissionFailsClosedForMissingComplianceAndCredentials(t *testing.T) {
	provider, deployment, policy, _, evidence, now := validSandboxAdmission()
	secret := SecretProbe{}
	evidence.SandboxAccount = false
	evidence.ContractReviewed = false
	evidence.RetentionReviewed = false
	evidence.DataResidencyReviewed = false
	evidence.PricingReviewed = false
	evidence.SyntheticDataOnly = false

	decision := AssessSandboxAdmission(provider, deployment, policy, secret, evidence, "text", now)
	for _, blocker := range []string{
		"contract_not_reviewed",
		"credential_not_ready",
		"data_residency_not_reviewed",
		"pricing_not_reviewed",
		"retention_not_reviewed",
		"sandbox_account_missing",
		"synthetic_data_only_required",
	} {
		assertSandboxBlocker(t, decision, blocker)
	}
}

func TestAssessSandboxAdmissionRejectsNonShadowAndUnboundedApproval(t *testing.T) {
	provider, deployment, policy, secret, evidence, now := validSandboxAdmission()
	policy.Mode = ModeCloudSuggestion
	evidence.ExpiresAt = now.Add(maxSandboxApprovalLifetime + time.Second)

	decision := AssessSandboxAdmission(provider, deployment, policy, secret, evidence, "text", now)
	assertSandboxBlocker(t, decision, "shadow_policy_not_approved")
	assertSandboxBlocker(t, decision, "approval_expired_or_unbounded")
}

func TestAssessSandboxAdmissionRejectsCompatibleOrMismatchedNativeInventory(t *testing.T) {
	provider, deployment, policy, secret, evidence, now := validSandboxAdmission()
	provider.AdapterType = "openai_compatible"
	evidence.Protocol = "openai_compatible"
	evidence.ApprovedRegion = "cn-shanghai"

	decision := AssessSandboxAdmission(provider, deployment, policy, secret, evidence, "text", now)
	assertSandboxBlocker(t, decision, "provider_not_approved")
	assertSandboxBlocker(t, decision, "native_protocol_not_approved")
	assertSandboxBlocker(t, decision, "region_not_approved")
}

func TestAssessSandboxAdmissionBlockersAreStableAndContainNoSecretFacts(t *testing.T) {
	provider, deployment, policy, _, evidence, now := validSandboxAdmission()
	decision := AssessSandboxAdmission(
		provider,
		deployment,
		policy,
		SecretProbe{Scheme: "env"},
		evidence,
		"text",
		now,
	)
	if !slices.IsSorted(decision.Blockers) {
		t.Fatalf("blockers must be deterministic: %v", decision.Blockers)
	}
	for _, blocker := range decision.Blockers {
		if blocker == provider.CredentialRef || blocker == evidence.ApprovalReference {
			t.Fatalf("decision leaked sensitive or audit facts: %v", decision.Blockers)
		}
	}
}

func validSandboxAdmission() (
	Provider,
	Deployment,
	TenantPolicy,
	SecretProbe,
	SandboxAdmissionEvidence,
	time.Time,
) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	provider := Provider{
		TenantID:      "tenant-1",
		Key:           "dashscope-sandbox",
		Kind:          ProviderExternal,
		AdapterType:   SandboxProtocolDashScopeNative,
		CredentialRef: "env://EDUGRADE_DASHSCOPE_SANDBOX_KEY",
		Region:        "cn-beijing",
		DataPolicy: DataPolicy{
			TrainingAllowed: false,
			RetentionMode:   "no_store",
		},
		Status: "active",
	}
	deployment := Deployment{
		TenantID:          "tenant-1",
		Key:               "qwen-synthetic-shadow",
		ProviderKey:       provider.Key,
		ModelVersion:      "pinned-synthetic-version",
		Region:            provider.Region,
		CapabilityProfile: "synthetic-text-v1",
		Modalities:        []string{"text"},
		PricingPolicy:     map[string]any{"meter": "tokens"},
		Status:            "shadow_only",
		HealthState:       "available",
	}
	policy := TenantPolicy{
		Mode:                     ModeShadowCompare,
		ExternalEnabled:          true,
		TextExportEnabled:        true,
		AllowedDeployments:       []string{deployment.Key},
		MaxCostMicrosPerQuestion: 10_000,
		MaxCostMicrosPerExam:     1_000_000,
		FallbackMode:             "manual_only",
	}
	secret := SecretProbe{
		Scheme:               "env",
		ResolverSupported:    true,
		Configured:           true,
		MeetsMinimumStrength: true,
	}
	evidence := SandboxAdmissionEvidence{
		Protocol:              SandboxProtocolDashScopeNative,
		ApprovalReference:     "approval-061b1a",
		ApprovedRegion:        provider.Region,
		SandboxAccount:        true,
		ContractReviewed:      true,
		RetentionReviewed:     true,
		DataResidencyReviewed: true,
		PricingReviewed:       true,
		SyntheticDataOnly:     true,
		ExpiresAt:             now.Add(30 * 24 * time.Hour),
	}
	return provider, deployment, policy, secret, evidence, now
}

func assertSandboxBlocker(t *testing.T, decision SandboxAdmissionDecision, blocker string) {
	t.Helper()
	if decision.Allowed || !slices.Contains(decision.Blockers, blocker) {
		t.Fatalf("expected blocker %q, got %+v", blocker, decision)
	}
}
