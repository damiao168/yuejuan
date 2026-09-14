ALTER TABLE app_user
  ADD COLUMN IF NOT EXISTS employee_no TEXT,
  ADD COLUMN IF NOT EXISTS phone_normalized TEXT,
  ADD COLUMN IF NOT EXISTS activated_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS security_epoch BIGINT NOT NULL DEFAULT 1;

-- Reuse legacy phone data only when it can be normalized without guessing and
-- is unambiguous inside the tenant. Conflicting values stay unbound for an
-- administrator to resolve explicitly instead of silently merging identities.
WITH phone_candidates AS (
  SELECT
    id,
    tenant_id,
    CASE
      WHEN compact_phone ~ '^1[0-9]{10}$' THEN '+86' || compact_phone
      WHEN compact_phone ~ '^86[1-9][0-9]{10}$' THEN '+' || compact_phone
      WHEN compact_phone ~ '^0086[1-9][0-9]{10}$' THEN '+86' || substring(compact_phone FROM 5)
      WHEN compact_phone ~ '^\+[1-9][0-9]{7,14}$' THEN compact_phone
      ELSE NULL
    END AS normalized_phone
  FROM (
    SELECT id, tenant_id, regexp_replace(btrim(phone), '[[:space:]()\-]', '', 'g') AS compact_phone
    FROM app_user
    WHERE phone IS NOT NULL AND btrim(phone) <> '' AND deleted_at IS NULL
  ) source
), unique_phones AS (
  SELECT tenant_id, normalized_phone
  FROM phone_candidates
  WHERE normalized_phone IS NOT NULL
  GROUP BY tenant_id, normalized_phone
  HAVING count(*) = 1
)
UPDATE app_user target
SET phone_normalized = candidate.normalized_phone
FROM phone_candidates candidate
JOIN unique_phones unique_phone
  ON unique_phone.tenant_id = candidate.tenant_id
 AND unique_phone.normalized_phone = candidate.normalized_phone
WHERE target.id = candidate.id
  AND target.tenant_id = candidate.tenant_id
  AND target.phone_normalized IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM app_user existing
    WHERE existing.tenant_id = candidate.tenant_id
      AND existing.phone_normalized = candidate.normalized_phone
      AND existing.id <> candidate.id
      AND existing.deleted_at IS NULL
  );

UPDATE app_user
SET activated_at = COALESCE(activated_at, created_at)
WHERE status IN ('active', 'disabled');

CREATE UNIQUE INDEX IF NOT EXISTS uq_app_user_tenant_phone_active
  ON app_user (tenant_id, phone_normalized)
  WHERE phone_normalized IS NOT NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_app_user_tenant_employee_no_active
  ON app_user (tenant_id, employee_no)
  WHERE employee_no IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE auth_session
  ADD COLUMN IF NOT EXISTS auth_method TEXT NOT NULL DEFAULT 'password',
  ADD COLUMN IF NOT EXISTS auth_level SMALLINT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS reauthenticated_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS security_epoch BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT 'low';

-- Both newly introduced epochs default to 1 for legacy accounts and sessions.
-- Never refresh existing session epochs here: repeating a migration must not
-- turn a stale credential snapshot into a current one.

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'auth_session_auth_level_check'
      AND conrelid = 'auth_session'::regclass
  ) THEN
    ALTER TABLE auth_session
      ADD CONSTRAINT auth_session_auth_level_check CHECK (auth_level BETWEEN 1 AND 3);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'auth_session_risk_level_check'
      AND conrelid = 'auth_session'::regclass
  ) THEN
    ALTER TABLE auth_session
      ADD CONSTRAINT auth_session_risk_level_check CHECK (risk_level IN ('low', 'medium', 'high'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_auth_session_security_epoch
  ON auth_session (tenant_id, user_id, security_epoch)
  WHERE revoked_at IS NULL;

ALTER TABLE auth_session DROP CONSTRAINT IF EXISTS auth_session_type_check;
ALTER TABLE auth_session
  ADD CONSTRAINT auth_session_type_check
  CHECK (session_type IN ('standard', 'remembered_device', 'public_device', 'desktop_device', 'service'));

CREATE TABLE IF NOT EXISTS auth_activation (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL,
  token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, user_id, id),
  CONSTRAINT fk_auth_activation_user_tenant
    FOREIGN KEY (tenant_id, user_id) REFERENCES app_user(tenant_id, id),
  CONSTRAINT auth_activation_expiry_check CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_auth_activation_pending
  ON auth_activation (tenant_id, user_id, expires_at)
  WHERE used_at IS NULL;

CREATE TABLE IF NOT EXISTS auth_recovery (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL,
  channel TEXT NOT NULL DEFAULT 'admin_assisted' CHECK (channel IN ('admin_assisted')),
  security_epoch BIGINT NOT NULL DEFAULT 1,
  token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, user_id, id),
  CONSTRAINT fk_auth_recovery_user_tenant
    FOREIGN KEY (tenant_id, user_id) REFERENCES app_user(tenant_id, id),
  CONSTRAINT fk_auth_recovery_creator_tenant
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT auth_recovery_expiry_check CHECK (expires_at > created_at)
);

ALTER TABLE auth_recovery
  ADD COLUMN IF NOT EXISTS security_epoch BIGINT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_auth_recovery_pending
  ON auth_recovery (tenant_id, user_id, expires_at)
  WHERE used_at IS NULL;
