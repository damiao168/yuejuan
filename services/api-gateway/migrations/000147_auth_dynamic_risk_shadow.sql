ALTER TABLE auth_session
  ADD COLUMN IF NOT EXISTS risk_action TEXT NOT NULL DEFAULT 'allow',
  ADD COLUMN IF NOT EXISTS risk_score SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS risk_evaluated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS risk_policy_version TEXT NOT NULL DEFAULT 'legacy',
  ADD COLUMN IF NOT EXISTS risk_evidence_quality TEXT NOT NULL DEFAULT 'limited';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'auth_session_risk_action_check'
      AND conrelid = 'auth_session'::regclass
  ) THEN
    ALTER TABLE auth_session ADD CONSTRAINT auth_session_risk_action_check
      CHECK (risk_action IN ('allow', 'allow_restricted', 'step_up', 'deny_temporarily', 'recovery_required'));
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'auth_session_risk_score_check'
      AND conrelid = 'auth_session'::regclass
  ) THEN
    ALTER TABLE auth_session ADD CONSTRAINT auth_session_risk_score_check
      CHECK (risk_score BETWEEN 0 AND 100);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'auth_session_risk_evidence_quality_check'
      AND conrelid = 'auth_session'::regclass
  ) THEN
    ALTER TABLE auth_session ADD CONSTRAINT auth_session_risk_evidence_quality_check
      CHECK (risk_evidence_quality IN ('cold_start', 'limited', 'sufficient'));
  END IF;
END $$;

-- A level-1 row is only a recognized browser binding. It must never be
-- treated as MFA. Future Passkey/TOTP/IdP completion may promote it to level 2
-- or 3 and set trusted_at without changing the opaque browser token.
CREATE TABLE IF NOT EXISTS auth_trusted_device (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL,
  token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  assurance_level SMALLINT NOT NULL DEFAULT 1 CHECK (assurance_level BETWEEN 1 AND 3),
  trust_basis TEXT NOT NULL DEFAULT 'password_observed'
    CHECK (trust_basis IN ('password_observed', 'mfa', 'passkey', 'enterprise_idp')),
  user_agent_hash TEXT NOT NULL DEFAULT '',
  last_ip_prefix TEXT NOT NULL DEFAULT '',
  trusted_at TIMESTAMPTZ,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, user_id, id),
  CONSTRAINT fk_auth_trusted_device_user_tenant
    FOREIGN KEY (tenant_id, user_id) REFERENCES app_user(tenant_id, id),
  CONSTRAINT auth_trusted_device_expiry_check CHECK (expires_at > created_at),
  CONSTRAINT auth_trusted_device_trust_check CHECK (
    (assurance_level = 1 AND trusted_at IS NULL)
    OR (assurance_level >= 2 AND trusted_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_auth_trusted_device_user_active
  ON auth_trusted_device (tenant_id, user_id, last_seen_at DESC)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_auth_trusted_device_expiry
  ON auth_trusted_device (expires_at)
  WHERE revoked_at IS NULL;

-- Events are immutable during their retention window. The application keeps
-- at most the latest 20 login profiles per user and deletes data older than
-- 180 days when it records the next event.
CREATE TABLE IF NOT EXISTS auth_risk_event (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL,
  session_id UUID REFERENCES auth_session(id) ON DELETE SET NULL,
  purpose TEXT NOT NULL
    CHECK (purpose IN ('login', 'session', 'recovery', 'sensitive_action')),
  risk_level TEXT NOT NULL CHECK (risk_level IN ('low', 'medium', 'high')),
  decision_action TEXT NOT NULL
    CHECK (decision_action IN ('allow', 'allow_restricted', 'step_up', 'deny_temporarily', 'recovery_required')),
  risk_score SMALLINT NOT NULL CHECK (risk_score BETWEEN 0 AND 100),
  reason_codes JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(reason_codes) = 'array'),
  family_scores JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(family_scores) = 'object'),
  evidence_quality TEXT NOT NULL CHECK (evidence_quality IN ('cold_start', 'limited', 'sufficient')),
  policy_version TEXT NOT NULL CHECK (length(policy_version) BETWEEN 1 AND 64),
  user_agent_hash TEXT NOT NULL DEFAULT '',
  ip_prefix TEXT NOT NULL DEFAULT '',
  device_recognized BOOLEAN NOT NULL DEFAULT false,
  device_trusted BOOLEAN NOT NULL DEFAULT false,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT fk_auth_risk_event_user_tenant
    FOREIGN KEY (tenant_id, user_id) REFERENCES app_user(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_auth_risk_event_user_time
  ON auth_risk_event (tenant_id, user_id, purpose, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_auth_risk_event_retention
  ON auth_risk_event (occurred_at);

ALTER TABLE auth_trusted_device ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_trusted_device FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS edugrade_tenant_isolation ON auth_trusted_device;
CREATE POLICY edugrade_tenant_isolation ON auth_trusted_device
  FOR ALL
  USING (edugrade_tenant_matches(tenant_id))
  WITH CHECK (edugrade_tenant_matches(tenant_id));

ALTER TABLE auth_risk_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_risk_event FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS edugrade_tenant_isolation ON auth_risk_event;
CREATE POLICY edugrade_tenant_isolation ON auth_risk_event
  FOR ALL
  USING (edugrade_tenant_matches(tenant_id))
  WITH CHECK (edugrade_tenant_matches(tenant_id));
