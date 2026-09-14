-- Optional authenticator-management pilot. No business MFA or login enforcement.
CREATE TABLE auth_totp (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL,
  secret_ciphertext BYTEA NOT NULL CHECK (octet_length(secret_ciphertext) >= 16),
  secret_nonce BYTEA NOT NULL CHECK (octet_length(secret_nonce) = 12),
  enrollment_session_hash TEXT NOT NULL CHECK (enrollment_session_hash ~ '^[0-9a-f]{64}$'),
  security_epoch BIGINT NOT NULL,
  pending_expires_at TIMESTAMPTZ NOT NULL,
  enabled_at TIMESTAMPTZ,
  last_used_step BIGINT NOT NULL DEFAULT -1,
  failed_attempts SMALLINT NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, user_id),
  UNIQUE (tenant_id, user_id, id),
  FOREIGN KEY (tenant_id, user_id) REFERENCES app_user(tenant_id, id)
);

CREATE TABLE auth_mfa_recovery_code (
  tenant_id UUID NOT NULL,
  user_id UUID NOT NULL,
  credential_id UUID NOT NULL,
  code_hash TEXT NOT NULL CHECK (code_hash ~ '^[0-9a-f]{64}$'),
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, user_id, code_hash),
  FOREIGN KEY (tenant_id, user_id, credential_id) REFERENCES auth_totp(tenant_id, user_id, id) ON DELETE CASCADE
);

CREATE TABLE auth_mfa_challenge (
  token_hash TEXT PRIMARY KEY CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  tenant_id UUID NOT NULL,
  user_id UUID NOT NULL,
  credential_id UUID NOT NULL,
  session_hash TEXT NOT NULL CHECK (session_hash ~ '^[0-9a-f]{64}$'),
  security_epoch BIGINT NOT NULL,
  operation TEXT NOT NULL CHECK (operation IN ('mfa.disable', 'mfa.recovery.rotate')),
  attempts SMALLINT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  verified_method TEXT CHECK (verified_method IN ('totp', 'recovery_code')),
  verified_at TIMESTAMPTZ,
  consumed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, user_id, credential_id) REFERENCES auth_totp(tenant_id, user_id, id) ON DELETE CASCADE,
  CHECK ((verified_at IS NULL) = (verified_method IS NULL)),
  CHECK (consumed_at IS NULL OR verified_at IS NOT NULL)
);
CREATE INDEX auth_mfa_challenge_owner_idx ON auth_mfa_challenge (tenant_id, user_id, session_hash, operation);
CREATE INDEX auth_mfa_challenge_expiry_idx ON auth_mfa_challenge (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON auth_totp, auth_mfa_recovery_code, auth_mfa_challenge TO edugrade_tenant_runtime;
DO $$ DECLARE protected_table TEXT; BEGIN
  FOREACH protected_table IN ARRAY ARRAY['auth_totp', 'auth_mfa_recovery_code', 'auth_mfa_challenge'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', protected_table);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', protected_table);
    EXECUTE format('CREATE POLICY edugrade_tenant_isolation ON %I FOR ALL USING (edugrade_tenant_matches(tenant_id)) WITH CHECK (edugrade_tenant_matches(tenant_id))', protected_table);
  END LOOP;
END $$;
