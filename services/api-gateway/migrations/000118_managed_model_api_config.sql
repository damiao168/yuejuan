-- Platform-managed, per-school third-party model API configuration.
-- API keys are encrypted by the API gateway before they reach PostgreSQL;
-- plaintext credentials are never stored in this table.

CREATE TABLE managed_model_api_config (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  provider_key TEXT NOT NULL,
  display_name TEXT NOT NULL,
  adapter_type TEXT NOT NULL,
  base_url TEXT NOT NULL,
  model_name TEXT NOT NULL,
  model_version TEXT NOT NULL,
  region TEXT NOT NULL DEFAULT 'global',
  credential_ciphertext BYTEA NOT NULL,
  credential_nonce BYTEA NOT NULL,
  credential_hint TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  last_test_status TEXT NOT NULL DEFAULT 'untested',
  last_test_message TEXT NOT NULL DEFAULT '',
  last_tested_at TIMESTAMPTZ,
  created_by UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  CONSTRAINT uq_managed_model_api_provider UNIQUE (tenant_id, provider_key),
  CONSTRAINT chk_managed_model_api_provider_key
    CHECK (provider_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CONSTRAINT chk_managed_model_api_adapter
    CHECK (adapter_type IN ('openai_compatible', 'dashscope_native')),
  CONSTRAINT chk_managed_model_api_url
    CHECK (base_url ~ '^https://[^[:space:]]+$'),
  CONSTRAINT chk_managed_model_api_identity
    CHECK (
      btrim(display_name) <> ''
      AND btrim(model_name) <> ''
      AND btrim(model_version) <> ''
      AND btrim(region) <> ''
    ),
  CONSTRAINT chk_managed_model_api_credential
    CHECK (
      octet_length(credential_ciphertext) >= 16
      AND octet_length(credential_nonce) = 12
      AND char_length(credential_hint) <= 16
    ),
  CONSTRAINT chk_managed_model_api_status
    CHECK (status IN ('active', 'disabled')),
  CONSTRAINT chk_managed_model_api_test_status
    CHECK (last_test_status IN ('untested', 'success', 'failed'))
);

CREATE UNIQUE INDEX uq_managed_model_api_default
  ON managed_model_api_config (tenant_id)
  WHERE is_default AND deleted_at IS NULL;

CREATE INDEX idx_managed_model_api_tenant
  ON managed_model_api_config (tenant_id, status, updated_at DESC)
  WHERE deleted_at IS NULL;
