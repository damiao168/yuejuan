INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('evidence:manage', 'Manage evidence checks', 'evidence', 'manage', '证据校验 Agent')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher', 'auditor')
  AND p.code = 'evidence:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS agent_job (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  job_type TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id UUID NOT NULL,
  status TEXT NOT NULL,
  result JSONB NOT NULL DEFAULT '{}',
  needs_human_review BOOLEAN NOT NULL DEFAULT FALSE,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (job_type IN ('evidence_check')),
  CHECK (target_type IN ('ai_grade')),
  CHECK (status IN ('passed', 'failed', 'warning'))
);

CREATE INDEX IF NOT EXISTS idx_agent_job_target ON agent_job (tenant_id, target_type, target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_job_status ON agent_job (tenant_id, job_type, status, created_at DESC);
