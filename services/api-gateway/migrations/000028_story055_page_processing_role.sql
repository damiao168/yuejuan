INSERT INTO role (tenant_id, code, name, scope_type, description)
SELECT t.id, 'page_processing_worker', 'Page Processing Worker', 'tenant', '页面拆分、配准和切题后台工作账号'
FROM tenant t
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'page_processing_worker'
  AND p.code IN ('file:manage', 'ocr:manage')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;
