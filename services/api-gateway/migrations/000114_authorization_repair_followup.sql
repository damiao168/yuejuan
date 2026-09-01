-- Follow-up repair for 000113. The first local repair used a target-table
-- update join that PostgreSQL interpreted too broadly and removed all active
-- role assignments for three acceptance identities. Restore the canonical
-- assignments explicitly and complete the school scope projection.

WITH target AS (
  SELECT u.tenant_id, u.id AS user_id
  FROM app_user u JOIN tenant t ON t.id = u.tenant_id
  WHERE t.code = 'demo' AND u.username = 'demo_admin' AND u.deleted_at IS NULL
), role_ref AS (
  SELECT r.tenant_id, r.id AS role_id
  FROM role r JOIN target x ON x.tenant_id = r.tenant_id
  WHERE r.code = 'tenant_admin' AND r.deleted_at IS NULL
)
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT x.tenant_id, x.user_id, r.role_id, jsonb_build_object('scope','tenant')
FROM target x JOIN role_ref r ON r.tenant_id = x.tenant_id
ON CONFLICT (tenant_id, user_id, role_id) DO UPDATE
SET data_scope = EXCLUDED.data_scope, deleted_at = NULL, updated_at = now();

WITH target AS (
  SELECT u.tenant_id, u.id AS user_id
  FROM app_user u JOIN tenant t ON t.id = u.tenant_id
  WHERE t.code = 'demo' AND u.username = 'story_acceptance_admin' AND u.deleted_at IS NULL
), role_ref AS (
  SELECT r.tenant_id, r.id AS role_id
  FROM role r JOIN target x ON x.tenant_id = r.tenant_id
  WHERE r.code = 'tenant_admin' AND r.deleted_at IS NULL
)
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT x.tenant_id, x.user_id, r.role_id, jsonb_build_object('scope','tenant')
FROM target x JOIN role_ref r ON r.tenant_id = x.tenant_id
ON CONFLICT (tenant_id, user_id, role_id) DO UPDATE
SET data_scope = EXCLUDED.data_scope, deleted_at = NULL, updated_at = now();

-- Restore the intentionally dual-role platform acceptance identity.
WITH target AS (
  SELECT u.tenant_id, u.id AS user_id
  FROM app_user u JOIN tenant t ON t.id = u.tenant_id
  WHERE t.code = 'platform' AND u.username = 'story_acceptance_admin' AND u.deleted_at IS NULL
)
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT x.tenant_id, x.user_id, r.id,
       CASE r.code WHEN 'platform_admin' THEN jsonb_build_object('scope','platform')
                   ELSE jsonb_build_object('scope','tenant') END
FROM target x JOIN role r ON r.tenant_id = x.tenant_id
WHERE r.code IN ('platform_admin','tenant_admin') AND r.deleted_at IS NULL
ON CONFLICT (tenant_id, user_id, role_id) DO UPDATE
SET data_scope = EXCLUDED.data_scope, deleted_at = NULL, updated_at = now();

-- Complete the school scope after 000113 populated app_user.school_id.
UPDATE user_role ur
SET data_scope = jsonb_build_object('scope','school','school_id',u.school_id::text),
    updated_at = now()
FROM app_user u, tenant t, role r
WHERE ur.tenant_id = u.tenant_id AND ur.user_id = u.id
  AND r.tenant_id = ur.tenant_id AND r.id = ur.role_id
  AND t.id = u.tenant_id AND t.code = 'demo'
  AND u.school_id IS NOT NULL AND r.code = 'school_admin'
  AND ur.deleted_at IS NULL AND u.deleted_at IS NULL;
