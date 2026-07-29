package server

import (
	"context"
	"os"
	"strings"
	"testing"

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
