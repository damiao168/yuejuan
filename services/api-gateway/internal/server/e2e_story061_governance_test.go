package server

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/modelgovernance"

	"github.com/google/uuid"
)

func TestStory061GovernanceFoundationE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping STORY-061 PostgreSQL E2E")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	ctx := context.Background()

	var tenantID string
	if err := db.QueryRowContext(ctx, `
SELECT id::text
FROM tenant
WHERE code = 'demo' AND deleted_at IS NULL
`).Scan(&tenantID); err != nil {
		t.Fatalf("lookup demo tenant: %v", err)
	}

	var mode, fallback string
	var externalEnabled, textEnabled, imageEnabled bool
	var allowed string
	if err := db.QueryRowContext(ctx, `
SELECT mode, external_enabled, text_export_enabled, image_export_enabled,
       allowed_deployments::text, fallback_mode
FROM tenant_model_policy
WHERE tenant_id = $1 AND policy_key = 'default' AND deleted_at IS NULL
`, tenantID).Scan(&mode, &externalEnabled, &textEnabled, &imageEnabled, &allowed, &fallback); err != nil {
		t.Fatalf("read default model policy: %v", err)
	}
	if mode != "local_only" || externalEnabled || textEnabled || imageEnabled || allowed != "[]" || fallback != "manual_only" {
		t.Fatalf("external model access must default closed: mode=%s external=%v text=%v image=%v allowed=%s fallback=%s",
			mode, externalEnabled, textEnabled, imageEnabled, allowed, fallback)
	}

	if _, err := db.ExecContext(ctx, `
INSERT INTO model_provider (
  tenant_id, provider_key, display_name, provider_kind, adapter_type,
  credential_ref, region, data_policy
)
VALUES ($1, 'plaintext-provider', 'Plaintext Provider', 'external', 'vendor_native',
        'actual-secret-value', 'cn-east', '{"training_allowed":false,"retention_mode":"no_store"}')
`, tenantID); err == nil {
		t.Fatal("database accepted a plaintext provider credential")
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO model_provider (
  tenant_id, provider_key, display_name, provider_kind, adapter_type,
  credential_ref, region, data_policy
)
VALUES ($1, 'compat-provider', 'Compatibility Provider', 'external', 'openai_compatible',
        'vault://edugrade/vendor-a', 'cn-east', '{"training_allowed":false,"retention_mode":"no_store"}')
`, tenantID); err == nil {
		t.Fatal("database accepted an OpenAI-compatible external adapter")
	}

	var providerID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO model_provider (
  tenant_id, provider_key, display_name, provider_kind, adapter_type,
  credential_ref, region, data_policy, status
)
VALUES ($1, 'vendor-a', 'Vendor A', 'external', 'vendor_a_native',
        'vault://edugrade/vendor-a', 'cn-east',
        '{"training_allowed":false,"retention_mode":"no_store"}', 'unverified')
RETURNING id::text
`, tenantID).Scan(&providerID); err != nil {
		t.Fatalf("insert reference-only external provider: %v", err)
	}

	var deploymentID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO model_deployment (
  tenant_id, provider_id, deployment_key, model_name, model_version,
  region, capability_profile, modalities, capability_policy, pricing_policy
)
VALUES (
  $1, $2, 'vendor-a-text-v1', 'Vendor A Text', 'vendor-a-model-v1',
  'cn-east', 'subjective-shadow-v1', '["text"]',
  '{"subjects":["chinese"],"question_types":["short_answer"]}',
  '{"meter":"token","input_micros":1,"output_micros":2}'
)
RETURNING id::text
`, tenantID, providerID).Scan(&deploymentID); err != nil {
		t.Fatalf("insert governed deployment: %v", err)
	}
	if deploymentID == "" {
		t.Fatal("deployment id was not returned")
	}

	if _, err := db.ExecContext(ctx, `
UPDATE tenant_model_policy
SET mode = 'shadow_compare',
    external_enabled = TRUE,
    text_export_enabled = TRUE,
    allowed_deployments = '["vendor-a-text-v1"]',
    updated_at = now(),
    version = version + 1
WHERE tenant_id = $1 AND policy_key = 'default'
`, tenantID); err != nil {
		t.Fatalf("explicitly authorize shadow text deployment: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
UPDATE tenant_model_policy
SET external_enabled = FALSE
WHERE tenant_id = $1 AND policy_key = 'default'
`, tenantID); err == nil {
		t.Fatal("database allowed external opt-out while retaining an external route")
	}

	requestID := "story061-" + uuid.NewString()
	if _, err := db.ExecContext(ctx, `
INSERT INTO model_call_fact (
  tenant_id, request_id, provider_key, deployment_key, adapter_type,
  model_version, prompt_version, rubric_version, capability_profile,
  deployment_region, route_mode, route_reason, status, attempts, latency_ms
)
VALUES (
  $1, $2, 'vendor-a', 'vendor-a-text-v1', 'vendor_a_native',
  'vendor-a-model-v1', 'prompt-v1', 'rubric-v1', 'subjective-shadow-v1',
  'cn-east', 'shadow_compare', 'explicit test authorization', 'succeeded', 1, 120
)
`, tenantID, requestID); err != nil {
		t.Fatalf("insert provider-neutral call fact: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO model_call_fact (
  tenant_id, request_id, provider_key, deployment_key, adapter_type,
  model_version, prompt_version, rubric_version, capability_profile,
  deployment_region, route_mode, status
)
VALUES (
  $1, $2, 'vendor-a', 'vendor-a-text-v1', 'vendor_a_native',
  'vendor-a-model-v1', 'prompt-v1', 'rubric-v1', 'subjective-shadow-v1',
  'cn-east', 'shadow_compare', 'replayed'
)
	`, tenantID, requestID); err == nil {
		t.Fatal("idempotent replay created a duplicate billable call fact")
	}

	var platformProviderManage, demoPolicyManage bool
	if err := db.QueryRowContext(ctx, `
SELECT
  EXISTS (
    SELECT 1
    FROM role
    JOIN role_permission ON role_permission.tenant_id = role.tenant_id AND role_permission.role_id = role.id
    JOIN permission ON permission.tenant_id = role_permission.tenant_id AND permission.id = role_permission.permission_id
    JOIN tenant ON tenant.id = role.tenant_id
    WHERE tenant.code = 'platform' AND role.code = 'platform_admin'
      AND permission.code = 'model:provider:manage'
      AND role.deleted_at IS NULL AND role_permission.deleted_at IS NULL AND permission.deleted_at IS NULL
  ),
  EXISTS (
    SELECT 1
    FROM role
    JOIN role_permission ON role_permission.tenant_id = role.tenant_id AND role_permission.role_id = role.id
    JOIN permission ON permission.tenant_id = role_permission.tenant_id AND permission.id = role_permission.permission_id
    JOIN tenant ON tenant.id = role.tenant_id
    WHERE tenant.code = 'demo' AND role.code = 'tenant_admin'
      AND permission.code = 'model:policy:manage'
      AND role.deleted_at IS NULL AND role_permission.deleted_at IS NULL AND permission.deleted_at IS NULL
  )
`).Scan(&platformProviderManage, &demoPolicyManage); err != nil {
		t.Fatalf("read STORY-061 governance RBAC: %v", err)
	}
	if !platformProviderManage || !demoPolicyManage {
		t.Fatalf("governance RBAC was not assigned: platform_provider=%v demo_policy=%v", platformProviderManage, demoPolicyManage)
	}

	governanceStore := modelgovernance.NewPostgresStore(db)
	baseline := modelgovernance.LocalBaseline{
		ProviderKey:       "local",
		ProviderName:      "Local grading runtime",
		DeploymentKey:     "local-qwen3-4b-q4-k-m",
		ModelName:         "Qwen/Qwen3-4B-GGUF",
		ModelVersion:      "Qwen/Qwen3-4B-GGUF:Q4_K_M",
		AdapterType:       "local_llama_cpp",
		Region:            "on_premise",
		CapabilityProfile: "local-pilot-v1",
	}
	if err := governanceStore.EnsureLocalBaseline(ctx, tenantID, baseline); err != nil {
		t.Fatalf("register local baseline: %v", err)
	}
	providers, err := governanceStore.ListProviders(ctx, tenantID)
	if err != nil {
		t.Fatalf("list governed providers: %v", err)
	}
	var localRegistered bool
	for _, provider := range providers {
		if provider.Key == baseline.ProviderKey {
			localRegistered = true
		}
		if provider.CredentialRef != "" {
			t.Fatalf("provider read model exposed credential_ref: %#v", provider)
		}
	}
	if !localRegistered {
		t.Fatal("local baseline provider was not registered")
	}
	createdProvider, err := governanceStore.CreateProvider(ctx, tenantID, "", modelgovernance.ProviderInput{
		Key:           "vendor-b",
		DisplayName:   "Vendor B",
		Kind:          modelgovernance.ProviderExternal,
		AdapterType:   "vendor_b_native",
		CredentialRef: "vault://edugrade/vendor-b",
		Region:        "cn-east",
		DataPolicy:    modelgovernance.DataPolicy{RetentionMode: "no_store"},
		Status:        "unverified",
	})
	if err != nil {
		t.Fatalf("create provider through governance store: %v", err)
	}
	if createdProvider.CredentialRef != "" || !createdProvider.CredentialConfigured || createdProvider.CredentialScheme != "vault" {
		t.Fatalf("provider store did not redact credential reference: %#v", createdProvider)
	}
	createdDeployment, err := governanceStore.CreateDeployment(ctx, tenantID, "", modelgovernance.DeploymentInput{
		ProviderID:        createdProvider.ID,
		Key:               "vendor-b-text-v1",
		ModelName:         "Vendor B Text",
		ModelVersion:      "vendor-b-model-v1",
		Region:            "cn-east",
		CapabilityProfile: "subjective-shadow-v1",
		Modalities:        []string{"text"},
		CapabilityPolicy:  map[string]any{"question_types": []string{"short_answer"}},
		PricingPolicy:     map[string]any{"meter": "token", "input_micros": 1, "output_micros": 2},
		Status:            "unverified",
		HealthState:       "unverified",
	})
	if err != nil {
		t.Fatalf("create deployment through governance store: %v", err)
	}
	if createdDeployment.ProviderKey != createdProvider.Key || createdDeployment.Key != "vendor-b-text-v1" {
		t.Fatalf("unexpected governed deployment: %#v", createdDeployment)
	}
	policy, err := governanceStore.GetPolicy(ctx, tenantID)
	if err != nil {
		t.Fatalf("get tenant model policy through store: %v", err)
	}
	updatedPolicy, err := governanceStore.UpdatePolicy(ctx, tenantID, "", modelgovernance.PolicyUpdateInput{
		DisplayName:        policy.DisplayName,
		Mode:               modelgovernance.ModeLocalOnly,
		AllowedDeployments: []string{},
		FallbackMode:       "manual_only",
		ExpectedVersion:    policy.Version,
		Reason:             "close external route after governance E2E",
	})
	if err != nil {
		t.Fatalf("update tenant model policy through store: %v", err)
	}
	if updatedPolicy.ExternalEnabled || updatedPolicy.Mode != modelgovernance.ModeLocalOnly {
		t.Fatalf("store policy update did not close external routing: %#v", updatedPolicy)
	}
	if _, err := governanceStore.UpdatePolicy(ctx, tenantID, "", modelgovernance.PolicyUpdateInput{
		Mode:               modelgovernance.ModeLocalOnly,
		AllowedDeployments: []string{},
		FallbackMode:       "manual_only",
		ExpectedVersion:    policy.Version,
		Reason:             "stale update must fail",
	}); !errors.Is(err, modelgovernance.ErrConflict) {
		t.Fatalf("stale policy version was not rejected: %v", err)
	}

	dashscopeProvider, err := governanceStore.CreateProvider(ctx, tenantID, "", modelgovernance.ProviderInput{
		Key:           "dashscope-sandbox-" + uuid.NewString(),
		DisplayName:   "DashScope synthetic sandbox",
		Kind:          modelgovernance.ProviderExternal,
		AdapterType:   modelgovernance.SandboxProtocolDashScopeNative,
		CredentialRef: "vault://edugrade/dashscope-sandbox",
		Region:        "cn-beijing",
		DataPolicy:    modelgovernance.DataPolicy{RetentionMode: "no_store"},
		Status:        "unverified",
	})
	if err != nil {
		t.Fatalf("create DashScope sandbox provider: %v", err)
	}
	dashscopeDeployment, err := governanceStore.CreateDeployment(ctx, tenantID, "", modelgovernance.DeploymentInput{
		ProviderID:        dashscopeProvider.ID,
		Key:               "qwen-synthetic-" + uuid.NewString(),
		ModelName:         "Qwen synthetic sandbox",
		ModelVersion:      "pinned-synthetic-version",
		Region:            dashscopeProvider.Region,
		CapabilityProfile: "synthetic-text-v1",
		Modalities:        []string{"text", "image"},
		PricingPolicy:     map[string]any{"meter": "token"},
		Status:            "unverified",
		HealthState:       "unverified",
	})
	if err != nil {
		t.Fatalf("create DashScope sandbox deployment: %v", err)
	}
	approvalNow := time.Now().UTC()
	approvalInput := modelgovernance.SandboxApprovalInput{
		ProviderID:            dashscopeProvider.ID,
		DeploymentID:          dashscopeDeployment.ID,
		Protocol:              modelgovernance.SandboxProtocolDashScopeNative,
		ApprovalReference:     "approval-" + uuid.NewString(),
		ApprovedRegion:        dashscopeProvider.Region,
		SandboxAccount:        true,
		ContractReviewed:      true,
		RetentionReviewed:     true,
		DataResidencyReviewed: true,
		PricingReviewed:       true,
		SyntheticDataOnly:     true,
		ImageExportReviewed:   false,
		ExpiresAt:             approvalNow.Add(30 * 24 * time.Hour),
		Reason:                "approve bounded synthetic sandbox preparation",
	}
	approval, err := governanceStore.CreateSandboxApproval(ctx, tenantID, "", approvalInput)
	if err != nil {
		t.Fatalf("create persisted sandbox approval: %v", err)
	}
	if approval.ProviderKey != dashscopeProvider.Key ||
		approval.DeploymentKey != dashscopeDeployment.Key ||
		!approval.IsActive(approvalNow) {
		t.Fatalf("sandbox approval was not bound to governed inventory: %#v", approval)
	}
	approvals, err := governanceStore.ListSandboxApprovals(ctx, tenantID)
	if err != nil || len(approvals) != 1 || approvals[0].ID != approval.ID {
		t.Fatalf("unexpected sandbox approval history: %#v %v", approvals, err)
	}
	if _, err := governanceStore.CreateSandboxApproval(ctx, tenantID, "", approvalInput); !errors.Is(err, modelgovernance.ErrConflict) {
		t.Fatalf("duplicate active sandbox approval was not rejected: %v", err)
	}
	revokedApproval, err := governanceStore.RevokeSandboxApproval(
		ctx, tenantID, "", approval.ID, "synthetic sandbox approval withdrawn",
	)
	if err != nil || revokedApproval.RevokedAt == nil {
		t.Fatalf("revoke sandbox approval: %#v %v", revokedApproval, err)
	}
	if _, err := governanceStore.RevokeSandboxApproval(
		ctx, tenantID, "", approval.ID, "duplicate revoke",
	); !errors.Is(err, modelgovernance.ErrConflict) {
		t.Fatalf("duplicate sandbox approval revocation was not rejected: %v", err)
	}
	approvalInput.ApprovalReference = "approval-" + uuid.NewString()
	replacementApproval, err := governanceStore.CreateSandboxApproval(ctx, tenantID, "", approvalInput)
	if err != nil {
		t.Fatalf("replacement approval after explicit revocation failed: %v", err)
	}
	if _, err := governanceStore.RevokeSandboxApproval(
		ctx, tenantID, "", replacementApproval.ID, "release deployment for constraint verification",
	); err != nil {
		t.Fatalf("revoke replacement sandbox approval: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO model_sandbox_approval (
  tenant_id, provider_id, deployment_id, protocol,
  approval_reference, approved_region, sandbox_account,
  contract_reviewed, retention_reviewed, data_residency_reviewed,
  pricing_reviewed, synthetic_data_only, expires_at
)
VALUES (
  $1, $2, $3, 'dashscope_native',
  $4, 'cn-beijing', TRUE,
  FALSE, TRUE, TRUE,
  TRUE, TRUE, now() + INTERVAL '1 day'
)
`, tenantID, dashscopeProvider.ID, dashscopeDeployment.ID, "invalid-"+uuid.NewString()); err == nil {
		t.Fatal("database accepted incomplete sandbox approval facts")
	}

	newTenantID := uuid.NewString()
	if _, err := db.ExecContext(ctx, `
INSERT INTO tenant (id, tenant_id, name, code, status, settings)
VALUES ($1, $1, 'STORY-061 Trigger Tenant', $2, 'active', '{}')
`, newTenantID, "story061-"+uuid.NewString()); err != nil {
		t.Fatalf("insert tenant for default-policy trigger: %v", err)
	}
	var triggeredMode string
	var triggeredExternal bool
	if err := db.QueryRowContext(ctx, `
SELECT mode, external_enabled
FROM tenant_model_policy
WHERE tenant_id = $1 AND policy_key = 'default'
`, newTenantID).Scan(&triggeredMode, &triggeredExternal); err != nil {
		t.Fatalf("default policy trigger did not create a policy: %v", err)
	}
	if triggeredMode != "local_only" || triggeredExternal {
		t.Fatalf("new tenant did not fail closed: mode=%s external=%v", triggeredMode, triggeredExternal)
	}
	if _, err := governanceStore.CreateSandboxApproval(ctx, newTenantID, "", approvalInput); !errors.Is(err, modelgovernance.ErrNotFound) {
		t.Fatalf("cross-tenant sandbox approval did not fail closed: %v", err)
	}

	var demoUserID string
	if err := db.QueryRowContext(ctx, `
SELECT id::text
FROM app_user
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY created_at
LIMIT 1
`, tenantID).Scan(&demoUserID); err != nil {
		t.Fatalf("lookup demo user for tenant-isolation check: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO model_provider (
  tenant_id, provider_key, display_name, provider_kind, adapter_type,
  region, data_policy, created_by
)
VALUES (
  $1, 'cross-tenant-provider', 'Cross Tenant Provider', 'local', 'local_llama_cpp',
  'on_premise', '{"training_allowed":false,"retention_mode":"no_store"}', $2
)
`, newTenantID, demoUserID); err == nil {
		t.Fatal("database accepted a provider creator from another tenant")
	}
}
