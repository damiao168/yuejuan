INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT tenant.id, permission.code, permission.name, permission.resource, permission.action, permission.description
FROM tenant
CROSS JOIN (VALUES
  ('model:read', 'Read model governance', 'model', 'read', '查看模型供应商、部署和租户策略'),
  ('model:provider:manage', 'Manage model providers', 'model_provider', 'manage', '管理供应商、部署和 Secret 引用'),
  ('model:policy:manage', 'Manage tenant model policy', 'model_policy', 'manage', '管理租户模型路由和数据外发策略')
) AS permission(code, name, resource, action, description)
WHERE tenant.deleted_at IS NULL
ON CONFLICT (tenant_id, code) DO UPDATE
SET name = EXCLUDED.name,
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    description = EXCLUDED.description,
    deleted_at = NULL,
    updated_at = now();

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT role.tenant_id, role.id, permission.id
FROM role
JOIN permission
  ON permission.tenant_id = role.tenant_id
WHERE role.code = 'platform_admin'
  AND permission.code IN ('model:read', 'model:provider:manage', 'model:policy:manage')
  AND role.deleted_at IS NULL
  AND permission.deleted_at IS NULL
ON CONFLICT (tenant_id, role_id, permission_id) DO UPDATE
SET deleted_at = NULL,
    updated_at = now();

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT role.tenant_id, role.id, permission.id
FROM role
JOIN permission
  ON permission.tenant_id = role.tenant_id
WHERE role.code = 'tenant_admin'
  AND permission.code IN ('model:read', 'model:policy:manage')
  AND role.deleted_at IS NULL
  AND permission.deleted_at IS NULL
ON CONFLICT (tenant_id, role_id, permission_id) DO UPDATE
SET deleted_at = NULL,
    updated_at = now();
