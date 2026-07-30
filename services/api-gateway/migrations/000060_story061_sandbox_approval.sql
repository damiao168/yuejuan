-- STORY-061B1C1: immutable, tenant-scoped approval evidence for future
-- synthetic native-provider sandbox calls. This table stores no credentials.

CREATE TABLE model_sandbox_approval (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  provider_id UUID NOT NULL,
  deployment_id UUID NOT NULL,
  protocol TEXT NOT NULL,
  approval_reference TEXT NOT NULL,
  approved_region TEXT NOT NULL,
  sandbox_account BOOLEAN NOT NULL,
  contract_reviewed BOOLEAN NOT NULL,
  retention_reviewed BOOLEAN NOT NULL,
  data_residency_reviewed BOOLEAN NOT NULL,
  pricing_reviewed BOOLEAN NOT NULL,
  synthetic_data_only BOOLEAN NOT NULL,
  image_export_reviewed BOOLEAN NOT NULL DEFAULT FALSE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_by UUID,
  revoked_at TIMESTAMPTZ,
  revoke_reason TEXT NOT NULL DEFAULT '',
  UNIQUE (tenant_id, id),
  CONSTRAINT fk_model_sandbox_approval_provider
    FOREIGN KEY (tenant_id, provider_id) REFERENCES model_provider(tenant_id, id),
  CONSTRAINT fk_model_sandbox_approval_deployment
    FOREIGN KEY (tenant_id, deployment_id) REFERENCES model_deployment(tenant_id, id),
  CONSTRAINT fk_model_sandbox_approval_creator
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT fk_model_sandbox_approval_revoker
    FOREIGN KEY (tenant_id, revoked_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT chk_model_sandbox_approval_protocol
    CHECK (protocol = 'dashscope_native'),
  CONSTRAINT chk_model_sandbox_approval_reference
    CHECK (approval_reference ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CONSTRAINT chk_model_sandbox_approval_region
    CHECK (btrim(approved_region) <> ''),
  CONSTRAINT chk_model_sandbox_approval_facts
    CHECK (
      sandbox_account
      AND contract_reviewed
      AND retention_reviewed
      AND data_residency_reviewed
      AND pricing_reviewed
      AND synthetic_data_only
    ),
  CONSTRAINT chk_model_sandbox_approval_lifetime
    CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL '90 days'),
  CONSTRAINT chk_model_sandbox_approval_revocation
    CHECK (
      (revoked_at IS NULL AND revoked_by IS NULL AND revoke_reason = '')
      OR
      (revoked_at IS NOT NULL AND btrim(revoke_reason) <> '')
    )
);

CREATE UNIQUE INDEX uq_model_sandbox_approval_active_deployment
  ON model_sandbox_approval (tenant_id, deployment_id)
  WHERE revoked_at IS NULL;

CREATE INDEX idx_model_sandbox_approval_expiry
  ON model_sandbox_approval (tenant_id, expires_at)
  WHERE revoked_at IS NULL;
