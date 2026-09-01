-- Existing tenants may predate permissions added to the platform RBAC template.
-- Copy the canonical school_admin permission definitions first so the matching
-- role_permission rows can be restored without bypassing backend authorization.
WITH school_admin_template AS (
  SELECT DISTINCT
    permission.code,
    permission.name,
    permission.resource,
    permission.action,
    permission.description
  FROM tenant source_tenant
  JOIN role source_role
    ON source_role.tenant_id = source_tenant.id
   AND source_role.code = 'school_admin'
   AND source_role.deleted_at IS NULL
  JOIN role_permission source_assignment
    ON source_assignment.tenant_id = source_role.tenant_id
   AND source_assignment.role_id = source_role.id
   AND source_assignment.deleted_at IS NULL
  JOIN permission
    ON permission.tenant_id = source_assignment.tenant_id
   AND permission.id = source_assignment.permission_id
   AND permission.deleted_at IS NULL
  WHERE source_tenant.code = 'platform'
    AND source_tenant.deleted_at IS NULL
)
INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT
  target_tenant.id,
  template.code,
  template.name,
  template.resource,
  template.action,
  template.description
FROM tenant target_tenant
CROSS JOIN school_admin_template template
WHERE target_tenant.deleted_at IS NULL
ON CONFLICT (tenant_id, code) DO UPDATE
SET name = EXCLUDED.name,
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    description = EXCLUDED.description,
    deleted_at = NULL,
    updated_at = now();

-- Add only the canonical school_admin grants. Extra tenant-specific grants are
-- deliberately preserved so this production repair remains additive.
WITH school_admin_template_codes AS (
  SELECT DISTINCT permission.code
  FROM tenant source_tenant
  JOIN role source_role
    ON source_role.tenant_id = source_tenant.id
   AND source_role.code = 'school_admin'
   AND source_role.deleted_at IS NULL
  JOIN role_permission source_assignment
    ON source_assignment.tenant_id = source_role.tenant_id
   AND source_assignment.role_id = source_role.id
   AND source_assignment.deleted_at IS NULL
  JOIN permission
    ON permission.tenant_id = source_assignment.tenant_id
   AND permission.id = source_assignment.permission_id
   AND permission.deleted_at IS NULL
  WHERE source_tenant.code = 'platform'
    AND source_tenant.deleted_at IS NULL
)
INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT
  target_role.tenant_id,
  target_role.id,
  target_permission.id
FROM role target_role
JOIN tenant target_tenant
  ON target_tenant.id = target_role.tenant_id
 AND target_tenant.deleted_at IS NULL
JOIN permission target_permission
  ON target_permission.tenant_id = target_role.tenant_id
 AND target_permission.deleted_at IS NULL
JOIN school_admin_template_codes template
  ON template.code = target_permission.code
WHERE target_role.code = 'school_admin'
  AND target_role.deleted_at IS NULL
ON CONFLICT (tenant_id, role_id, permission_id) DO UPDATE
SET deleted_at = NULL,
    updated_at = now();
