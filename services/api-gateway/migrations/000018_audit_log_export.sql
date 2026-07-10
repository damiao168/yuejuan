INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('audit:export', 'Export audit logs', 'audit', 'export', '导出审计日志')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'auditor') AND p.code = 'audit:export'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_audit_log_time ON audit_log (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_ip ON audit_log (tenant_id, ip_address);
