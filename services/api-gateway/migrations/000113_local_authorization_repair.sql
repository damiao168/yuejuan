-- Local authorization repair for the synthetic demo/acceptance identities.
-- This migration is deliberately limited to the known local tenant/account
-- names. Production tenants must use the account-review workflow instead of
-- guessing organizational bindings.

-- A platform role in the demo tenant is not a valid platform identity. Keep
-- the account usable by reducing it to the tenant administrator role.
WITH target AS (
  SELECT u.tenant_id, u.id AS user_id
  FROM app_user u
  JOIN tenant t ON t.id = u.tenant_id
  WHERE t.code = 'demo' AND u.username = 'demo_admin' AND u.deleted_at IS NULL
), tenant_role AS (
  SELECT r.tenant_id, r.id AS role_id
  FROM role r JOIN target x ON x.tenant_id = r.tenant_id
  WHERE r.code = 'tenant_admin' AND r.deleted_at IS NULL
)
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT x.tenant_id, x.user_id, r.role_id, jsonb_build_object('scope','tenant')
FROM target x JOIN tenant_role r ON r.tenant_id = x.tenant_id
ON CONFLICT (tenant_id, user_id, role_id) DO UPDATE
SET data_scope = EXCLUDED.data_scope, deleted_at = NULL, updated_at = now();

UPDATE user_role ur
SET deleted_at = now(), updated_at = now()
FROM app_user u, tenant t, role r
WHERE ur.tenant_id = u.tenant_id AND ur.user_id = u.id
  AND t.code = 'demo' AND u.username = 'demo_admin'
  AND r.code = 'platform_admin' AND ur.deleted_at IS NULL;

-- The custom acceptance administrator role is not understood by the runtime
-- policy. Normalize it to the standard tenant administrator role.
WITH target AS (
  SELECT u.tenant_id, u.id AS user_id
  FROM app_user u JOIN tenant t ON t.id = u.tenant_id
  WHERE t.code = 'demo' AND u.username = 'story_acceptance_admin' AND u.deleted_at IS NULL
), tenant_role AS (
  SELECT r.tenant_id, r.id AS role_id
  FROM role r JOIN target x ON x.tenant_id = r.tenant_id
  WHERE r.code = 'tenant_admin' AND r.deleted_at IS NULL
)
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT x.tenant_id, x.user_id, r.role_id, jsonb_build_object('scope','tenant')
FROM target x JOIN tenant_role r ON r.tenant_id = x.tenant_id
ON CONFLICT (tenant_id, user_id, role_id) DO UPDATE
SET data_scope = EXCLUDED.data_scope, deleted_at = NULL, updated_at = now();

UPDATE user_role ur
SET deleted_at = now(), updated_at = now()
FROM app_user u, tenant t, role r
WHERE ur.tenant_id = u.tenant_id AND ur.user_id = u.id
  AND t.code = 'demo' AND u.username = 'story_acceptance_admin'
  AND r.code = 'story_acceptance_admin' AND ur.deleted_at IS NULL;

-- Bind school-scoped demo identities to the explicit S053 school and its
-- only teaching class. No cross-tenant inference is performed.
WITH school_ref AS (
  SELECT s.tenant_id, s.id AS school_id
  FROM school s JOIN tenant t ON t.id = s.tenant_id
  WHERE t.code = 'demo' AND s.code = 'S053' AND s.deleted_at IS NULL
), class_ref AS (
  SELECT c.tenant_id, c.id AS class_id
  FROM school_class c JOIN school_ref s ON s.tenant_id = c.tenant_id AND s.school_id = c.school_id
  WHERE c.code = 'G10-01' AND c.deleted_at IS NULL
)
UPDATE app_user u
SET school_id = s.school_id, updated_at = now()
FROM school_ref s
WHERE u.tenant_id = s.tenant_id
  AND u.username IN ('demo_school_admin','story053_school_admin','demo_teacher','story054_acceptance')
  AND u.deleted_at IS NULL;

WITH school_ref AS (
  SELECT s.tenant_id, s.id AS school_id
  FROM school s JOIN tenant t ON t.id = s.tenant_id
  WHERE t.code = 'demo' AND s.code = 'S053' AND s.deleted_at IS NULL
), class_ref AS (
  SELECT c.tenant_id, c.id AS class_id
  FROM school_class c JOIN school_ref s ON s.tenant_id = c.tenant_id AND s.school_id = c.school_id
  WHERE c.code = 'G10-01' AND c.deleted_at IS NULL
)
INSERT INTO teacher_class (tenant_id, teacher_id, class_id)
SELECT u.tenant_id, u.id, c.class_id
FROM app_user u JOIN tenant t ON t.id = u.tenant_id
JOIN class_ref c ON c.tenant_id = u.tenant_id
WHERE t.code = 'demo' AND u.username IN ('demo_teacher','story054_acceptance')
  AND u.deleted_at IS NULL
ON CONFLICT (tenant_id, teacher_id, class_id) DO UPDATE
SET deleted_at = NULL, updated_at = now();

-- Grader/arbitrator identities need a school for organization management.
-- Existing task ownership remains authoritative; idle synthetic identities
-- receive the simulation school only as an explicit local fixture binding.
WITH school_ref AS (
  SELECT s.tenant_id, s.id AS school_id
  FROM school s JOIN tenant t ON t.id = s.tenant_id
  WHERE t.code = 'demo' AND s.code = 'sim-fj2024-math-v1' AND s.deleted_at IS NULL
)
UPDATE app_user u
SET school_id = s.school_id, updated_at = now()
FROM school_ref s
WHERE u.tenant_id = s.tenant_id
  AND u.username IN ('demo_grader','demo_arbitrator','real_review_regression_0718','real_sample_grader','test_grader')
  AND u.deleted_at IS NULL;

WITH school_ref AS (
  SELECT s.tenant_id, s.id AS school_id
  FROM school s JOIN tenant t ON t.id = s.tenant_id
  WHERE t.code = 'demo' AND s.code = 'S053' AND s.deleted_at IS NULL
)
UPDATE app_user u
SET school_id = s.school_id, updated_at = now()
FROM school_ref s
WHERE u.tenant_id = s.tenant_id AND u.username = 'story053_grader' AND u.deleted_at IS NULL;

-- A teacher in the platform tenant has no valid school boundary. Disable the
-- synthetic identity rather than inventing a platform school.
UPDATE app_user u
SET status = 'disabled', updated_at = now()
FROM tenant t
WHERE t.id = u.tenant_id AND t.code = 'platform'
  AND u.username = 'story_acceptance' AND u.deleted_at IS NULL;

-- Revoke sessions for identities whose role/scope changed. Worker sessions
-- are revoked once so the fixed clients establish one clean session each.
UPDATE auth_session s
SET revoked_at = now(), revoke_reason = 'authorization_model_repair', updated_at = now()
WHERE s.revoked_at IS NULL
  AND EXISTS (
    SELECT 1 FROM app_user u JOIN tenant t ON t.id = u.tenant_id
    WHERE u.tenant_id = s.tenant_id AND u.id = s.user_id
      AND ((t.code = 'demo' AND u.username IN ('demo_admin','story_acceptance_admin'))
        OR u.username IN ('page_processing_worker','subjective_grading_worker'))
  );
