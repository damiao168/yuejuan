CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenant (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  code TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  settings JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (tenant_id = id)
);

CREATE TABLE IF NOT EXISTS app_user (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID,
  username TEXT NOT NULL,
  display_name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  email TEXT,
  phone TEXT,
  status TEXT NOT NULL,
  last_login_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, username)
);

CREATE TABLE IF NOT EXISTS role (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  scope_type TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS permission (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  resource TEXT NOT NULL,
  action TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS user_role (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL REFERENCES app_user(id),
  role_id UUID NOT NULL REFERENCES role(id),
  data_scope JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permission (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  role_id UUID NOT NULL REFERENCES role(id),
  permission_id UUID NOT NULL REFERENCES permission(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS auth_session (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL REFERENCES app_user(id),
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  actor_id UUID,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id UUID,
  before_value JSONB,
  after_value JSONB,
  reason TEXT,
  ip_address TEXT,
  user_agent TEXT,
  request_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tenant (id, tenant_id, name, code, status)
VALUES
  ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Platform', 'platform', 'active'),
  ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'Demo School Tenant', 'demo', 'active')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role (tenant_id, code, name, scope_type, description)
SELECT t.id, r.code, r.name, r.scope_type, r.description
FROM tenant t
CROSS JOIN (VALUES
  ('platform_admin', 'Platform Admin', 'platform', '平台超级管理员'),
  ('tenant_admin', 'Tenant Admin', 'tenant', '租户管理员'),
  ('school_admin', 'School Admin', 'school', '学校管理员'),
  ('teacher', 'Teacher', 'school', '教师'),
  ('grader', 'Grader', 'exam_task', '阅卷员'),
  ('auditor', 'Auditor', 'tenant', '审计员')
) AS r(code, name, scope_type, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('system:read', 'Read system info', 'system', 'read', '查看系统信息'),
  ('audit:read', 'Read audit logs', 'audit', 'read', '查看审计日志'),
  ('exam:manage', 'Manage exams', 'exam', 'manage', '管理考试'),
  ('grading:review', 'Review grading tasks', 'grading', 'review', '人工阅卷'),
  ('grading:arbitrate', 'Arbitrate grading tasks', 'grading', 'arbitrate', '仲裁阅卷'),
  ('score:publish', 'Publish scores', 'score', 'publish', '发布成绩')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'auditor' AND p.code = 'audit:read'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'teacher' AND p.code IN ('system:read', 'exam:manage', 'grading:review')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'grader' AND p.code IN ('system:read', 'grading:review')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO app_user (tenant_id, username, display_name, password_hash, status)
SELECT t.id, u.username, u.display_name, crypt(gen_random_uuid()::text, gen_salt('bf')), 'disabled'
FROM tenant t
JOIN (VALUES
  ('platform', 'platform_admin', '平台超级管理员'),
  ('demo', 'tenant_admin', '租户管理员'),
  ('demo', 'school_admin', '学校管理员'),
  ('demo', 'teacher', '教师'),
  ('demo', 'grader', '阅卷员'),
  ('demo', 'auditor', '审计员')
) AS u(tenant_code, username, display_name) ON u.tenant_code = t.code
ON CONFLICT (tenant_id, username) DO NOTHING;

INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT u.tenant_id, u.id, r.id, jsonb_build_object('scope', r.scope_type)
FROM app_user u
JOIN role r ON r.tenant_id = u.tenant_id AND r.code = u.username
ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_app_user_tenant_username ON app_user (tenant_id, username);
CREATE INDEX IF NOT EXISTS idx_auth_session_token ON auth_session (token_hash);
CREATE INDEX IF NOT EXISTS idx_auth_session_user ON auth_session (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON audit_log (tenant_id, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor_time ON audit_log (tenant_id, actor_id, created_at DESC);
