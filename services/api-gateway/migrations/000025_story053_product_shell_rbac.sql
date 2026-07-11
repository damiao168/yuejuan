INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'school_admin'
  AND p.code = 'exam:manage'
  AND r.deleted_at IS NULL
  AND p.deleted_at IS NULL
ON CONFLICT (tenant_id, role_id, permission_id) DO UPDATE
SET deleted_at = NULL,
    updated_at = now();
