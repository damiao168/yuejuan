UPDATE role_permission AS rp
SET deleted_at = now(),
    updated_at = now()
FROM role AS r, permission AS p
WHERE rp.tenant_id = r.tenant_id
  AND rp.role_id = r.id
  AND rp.tenant_id = p.tenant_id
  AND rp.permission_id = p.id
  AND rp.deleted_at IS NULL
  AND r.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND r.code = 'teacher'
  AND p.code IN ('review:manage', 'arbitration:manage');
