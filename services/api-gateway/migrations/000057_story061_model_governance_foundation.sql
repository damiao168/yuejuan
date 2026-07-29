-- STORY-061A1: provider-neutral governance foundation. This migration does
-- not enable any external provider and does not store provider credentials.

CREATE TABLE model_provider (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  provider_key TEXT NOT NULL,
  display_name TEXT NOT NULL,
  provider_kind TEXT NOT NULL,
  adapter_type TEXT NOT NULL,
  credential_ref TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  data_policy JSONB NOT NULL DEFAULT '{"training_allowed":false,"retention_mode":"no_store"}'::jsonb,
  status TEXT NOT NULL DEFAULT 'unverified',
  created_by UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  CONSTRAINT uq_model_provider_key UNIQUE (tenant_id, provider_key),
  CONSTRAINT chk_model_provider_key
    CHECK (provider_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CONSTRAINT chk_model_provider_kind
    CHECK (provider_kind IN ('local', 'external')),
  CONSTRAINT chk_model_provider_adapter
    CHECK (
      adapter_type ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND adapter_type !~* 'openai[-_ ]?compatible'
    ),
  CONSTRAINT chk_model_provider_status
    CHECK (status IN ('unverified', 'active', 'degraded', 'rate_limited', 'disabled')),
  CONSTRAINT chk_model_provider_data_policy
    CHECK (
      jsonb_typeof(data_policy) = 'object'
      AND data_policy ? 'training_allowed'
      AND data_policy->>'training_allowed' = 'false'
      AND data_policy ? 'retention_mode'
      AND data_policy->>'retention_mode' IN ('no_store', 'contractual')
    ),
  CONSTRAINT chk_model_provider_secret_reference
    CHECK (
      (
        provider_kind = 'local'
        AND credential_ref = ''
        AND region <> ''
      )
      OR
      (
        provider_kind = 'external'
        AND region <> ''
        AND credential_ref ~ '^(env|docker_secret|vault|aws_secrets_manager|azure_key_vault|gcp_secret_manager)://[A-Za-z0-9._/-]+$'
      )
    )
);

CREATE TABLE model_deployment (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  provider_id UUID NOT NULL,
  deployment_key TEXT NOT NULL,
  model_name TEXT NOT NULL,
  model_version TEXT NOT NULL,
  region TEXT NOT NULL,
  capability_profile TEXT NOT NULL,
  modalities JSONB NOT NULL DEFAULT '["text"]'::jsonb,
  capability_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
  pricing_policy JSONB NOT NULL DEFAULT '{"meter":"not_configured"}'::jsonb,
  health_state TEXT NOT NULL DEFAULT 'unverified',
  status TEXT NOT NULL DEFAULT 'unverified',
  created_by UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  CONSTRAINT uq_model_deployment_key UNIQUE (tenant_id, deployment_key),
  CONSTRAINT fk_model_deployment_provider
    FOREIGN KEY (tenant_id, provider_id) REFERENCES model_provider(tenant_id, id),
  CONSTRAINT chk_model_deployment_key
    CHECK (deployment_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CONSTRAINT chk_model_deployment_identity
    CHECK (
      btrim(model_name) <> ''
      AND btrim(model_version) <> ''
      AND btrim(region) <> ''
      AND capability_profile ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$'
    ),
  CONSTRAINT chk_model_deployment_modalities
    CHECK (
      jsonb_typeof(modalities) = 'array'
      AND jsonb_array_length(modalities) >= 1
      AND modalities <@ '["text","image"]'::jsonb
    ),
  CONSTRAINT chk_model_deployment_capability_policy
    CHECK (jsonb_typeof(capability_policy) = 'object'),
  CONSTRAINT chk_model_deployment_pricing_policy
    CHECK (jsonb_typeof(pricing_policy) = 'object' AND pricing_policy ? 'meter'),
  CONSTRAINT chk_model_deployment_health
    CHECK (health_state IN ('unverified', 'available', 'degraded', 'rate_limited', 'unavailable', 'disabled')),
  CONSTRAINT chk_model_deployment_status
    CHECK (status IN ('unverified', 'shadow_only', 'disabled'))
);

CREATE TABLE tenant_model_policy (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  policy_key TEXT NOT NULL,
  display_name TEXT NOT NULL,
  mode TEXT NOT NULL DEFAULT 'local_only',
  external_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  text_export_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  image_export_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  allowed_deployments JSONB NOT NULL DEFAULT '[]'::jsonb,
  max_cost_micros_per_question BIGINT NOT NULL DEFAULT 0,
  max_cost_micros_per_exam BIGINT NOT NULL DEFAULT 0,
  fallback_mode TEXT NOT NULL DEFAULT 'manual_only',
  status TEXT NOT NULL DEFAULT 'active',
  version BIGINT NOT NULL DEFAULT 1,
  created_by UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  CONSTRAINT uq_tenant_model_policy_key UNIQUE (tenant_id, policy_key),
  CONSTRAINT chk_tenant_model_policy_key
    CHECK (policy_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CONSTRAINT chk_tenant_model_policy_mode
    CHECK (mode IN ('local_only', 'shadow_compare', 'cloud_suggestion', 'hybrid_escalation', 'dual_provider_review')),
  CONSTRAINT chk_tenant_model_policy_external_closed
    CHECK (
      external_enabled
      OR (
        mode = 'local_only'
        AND NOT text_export_enabled
        AND NOT image_export_enabled
        AND allowed_deployments = '[]'::jsonb
      )
    ),
  CONSTRAINT chk_tenant_model_policy_deployments
    CHECK (jsonb_typeof(allowed_deployments) = 'array'),
  CONSTRAINT chk_tenant_model_policy_budget
    CHECK (max_cost_micros_per_question >= 0 AND max_cost_micros_per_exam >= 0),
  CONSTRAINT chk_tenant_model_policy_fallback
    CHECK (fallback_mode IN ('manual_only', 'approved_deployment_only')),
  CONSTRAINT chk_tenant_model_policy_status
    CHECK (status IN ('draft', 'active', 'disabled')),
  CONSTRAINT chk_tenant_model_policy_version
    CHECK (version >= 1)
);

CREATE TABLE model_call_fact (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  request_id TEXT NOT NULL,
  answer_segment_id UUID REFERENCES answer_segment(id),
  question_id UUID REFERENCES question(id),
  provider_key TEXT NOT NULL,
  deployment_key TEXT NOT NULL,
  adapter_type TEXT NOT NULL,
  model_version TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  rubric_version TEXT NOT NULL,
  capability_profile TEXT NOT NULL,
  deployment_region TEXT NOT NULL,
  route_mode TEXT NOT NULL DEFAULT 'local_only',
  route_reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  native_request_id TEXT NOT NULL DEFAULT '',
  input_hash TEXT NOT NULL DEFAULT '',
  output_hash TEXT NOT NULL DEFAULT '',
  input_units BIGINT NOT NULL DEFAULT 0,
  output_units BIGINT NOT NULL DEFAULT 0,
  estimated_cost_micros BIGINT NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  fallback_chain JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_model_call_fact_request UNIQUE (tenant_id, request_id),
  CONSTRAINT chk_model_call_fact_identity
    CHECK (
      provider_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND deployment_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND adapter_type ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND adapter_type !~* 'openai[-_ ]?compatible'
      AND btrim(model_version) <> ''
      AND btrim(prompt_version) <> ''
      AND btrim(rubric_version) <> ''
      AND btrim(capability_profile) <> ''
      AND btrim(deployment_region) <> ''
    ),
  CONSTRAINT chk_model_call_fact_mode
    CHECK (route_mode IN ('local_only', 'shadow_compare', 'cloud_suggestion', 'hybrid_escalation', 'dual_provider_review')),
  CONSTRAINT chk_model_call_fact_status
    CHECK (status IN ('succeeded', 'failed', 'replayed')),
  CONSTRAINT chk_model_call_fact_measurements
    CHECK (
      attempts >= 0
      AND latency_ms >= 0
      AND input_units >= 0
      AND output_units >= 0
      AND estimated_cost_micros >= 0
      AND jsonb_typeof(fallback_chain) = 'array'
    )
);

ALTER TABLE ai_grade
  ADD COLUMN provider_key TEXT NOT NULL DEFAULT '',
  ADD COLUMN deployment_key TEXT NOT NULL DEFAULT '',
  ADD COLUMN deployment_region TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_model_deployment_routing
  ON model_deployment (tenant_id, status, health_state, deployment_key)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_model_call_fact_operations
  ON model_call_fact (tenant_id, provider_key, deployment_key, status, created_at DESC);

CREATE INDEX idx_model_call_fact_question
  ON model_call_fact (tenant_id, question_id, created_at DESC);

CREATE OR REPLACE FUNCTION ensure_default_tenant_model_policy()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO tenant_model_policy (
    tenant_id, policy_key, display_name, mode, external_enabled,
    text_export_enabled, image_export_enabled, allowed_deployments,
    fallback_mode, status
  )
  VALUES (
    NEW.id, 'default', 'Default local-only policy', 'local_only', FALSE,
    FALSE, FALSE, '[]'::jsonb, 'manual_only', 'active'
  )
  ON CONFLICT (tenant_id, policy_key) DO NOTHING;
  RETURN NEW;
END;
$$;

INSERT INTO tenant_model_policy (
  tenant_id, policy_key, display_name, mode, external_enabled,
  text_export_enabled, image_export_enabled, allowed_deployments,
  fallback_mode, status
)
SELECT
  id, 'default', 'Default local-only policy', 'local_only', FALSE,
  FALSE, FALSE, '[]'::jsonb, 'manual_only', 'active'
FROM tenant
WHERE deleted_at IS NULL
ON CONFLICT (tenant_id, policy_key) DO NOTHING;

DROP TRIGGER IF EXISTS trg_tenant_default_model_policy ON tenant;
CREATE TRIGGER trg_tenant_default_model_policy
AFTER INSERT ON tenant
FOR EACH ROW EXECUTE FUNCTION ensure_default_tenant_model_policy();
