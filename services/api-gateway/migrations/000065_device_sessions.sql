ALTER TABLE auth_session
  ADD COLUMN IF NOT EXISTS session_type TEXT NOT NULL DEFAULT 'standard',
  ADD COLUMN IF NOT EXISTS device_id TEXT,
  ADD COLUMN IF NOT EXISTS device_name TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS user_agent_hash TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS ip_prefix TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS revoke_reason TEXT;

UPDATE auth_session
SET device_id = id::text
WHERE device_id IS NULL OR btrim(device_id) = '';

ALTER TABLE auth_session
  ALTER COLUMN device_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'auth_session_type_check'
      AND conrelid = 'auth_session'::regclass
  ) THEN
    ALTER TABLE auth_session
      ADD CONSTRAINT auth_session_type_check
      CHECK (session_type IN ('standard', 'remembered_device', 'desktop_device', 'service'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_auth_session_user_active
  ON auth_session (tenant_id, user_id, expires_at DESC)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_auth_session_last_seen
  ON auth_session (last_seen_at)
  WHERE revoked_at IS NULL;

INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT id, 'session:revoke', 'Revoke user sessions', 'session', 'revoke', '撤销用户的设备会话'
FROM tenant
WHERE deleted_at IS NULL
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p
  ON p.tenant_id = r.tenant_id
 AND p.code = 'session:revoke'
WHERE r.code IN ('tenant_admin', 'school_admin')
  AND r.deleted_at IS NULL
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;
