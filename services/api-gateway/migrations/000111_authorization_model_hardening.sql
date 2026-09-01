-- Canonical four-layer authorization contract:
-- role -> identity, permission -> operation, data_scope -> data boundary,
-- resource resolver -> concrete object ownership.

INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, 'dashboard:read', 'Read role dashboard', 'dashboard', 'read', '查看当前身份和数据范围内的工作台'
FROM tenant t
WHERE t.deleted_at IS NULL
ON CONFLICT (tenant_id, code) DO UPDATE
SET name=EXCLUDED.name,resource=EXCLUDED.resource,action=EXCLUDED.action,
    description=EXCLUDED.description,deleted_at=NULL,updated_at=now();

UPDATE role
SET scope_type=matrix.scope_type,updated_at=now()
FROM (VALUES
  ('platform_admin','platform'),('tenant_admin','tenant'),('school_admin','school'),
  ('teacher','class'),('grader','exam_task'),('arbitrator','exam_task'),
  ('student','self'),('auditor','tenant'),('page_processing_worker','service'),
  ('subjective_grading_worker','service')
) AS matrix(code,scope_type)
WHERE role.code=matrix.code AND role.deleted_at IS NULL;

INSERT INTO role_permission (tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id
FROM role r JOIN permission p ON p.tenant_id=r.tenant_id
WHERE r.code IN ('platform_admin','tenant_admin') AND r.deleted_at IS NULL
  AND p.code='dashboard:read' AND p.deleted_at IS NULL
ON CONFLICT (tenant_id,role_id,permission_id) DO UPDATE
SET deleted_at=NULL,updated_at=now();

-- School administrators need the complete management surface, but every
-- object operation is still constrained by school-scoped resource resolution.
INSERT INTO role_permission (tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id
FROM role r JOIN permission p ON p.tenant_id=r.tenant_id
WHERE r.code='school_admin' AND r.deleted_at IS NULL AND p.deleted_at IS NULL
  AND p.code IN (
    'dashboard:read','org:manage','student:import','exam:manage','file:manage',
    'submission:manage','capture:manage','ocr:manage','segment:manage',
    'grading:manage','evidence:manage','review:manage','review:work',
    'arbitration:manage','score:manage','report:read','report:export',
    'appeal:read','appeal:manage','audit:read','audit:export','session:revoke'
  )
ON CONFLICT (tenant_id,role_id,permission_id) DO UPDATE
SET deleted_at=NULL,updated_at=now();

-- Management permissions cannot be used as identity or data-scope shortcuts.
UPDATE role_permission rp
SET deleted_at=now(),updated_at=now()
FROM role r,permission p
WHERE rp.tenant_id=r.tenant_id AND rp.role_id=r.id
  AND rp.tenant_id=p.tenant_id AND rp.permission_id=p.id
  AND rp.deleted_at IS NULL AND r.deleted_at IS NULL AND p.deleted_at IS NULL
  AND r.code='teacher'
  AND (p.action='manage' OR p.code IN (
    'tenant:manage','org:manage','exam:manage','system:read','audit:export',
    'model:read','model:provider:manage','model:policy:manage','model:evaluation:manage',
    'capture:manage','ocr:manage','segment:manage','submission:manage','file:manage',
    'grading:manage','evidence:manage','orchestrator:manage','review:manage',
    'arbitration:manage','score:manage','appeal:manage'
  ));

INSERT INTO role_permission (tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id
FROM role r JOIN permission p ON p.tenant_id=r.tenant_id
WHERE r.code='teacher' AND r.deleted_at IS NULL AND p.deleted_at IS NULL
  AND p.code IN ('dashboard:read','review:work','report:read','appeal:read')
ON CONFLICT (tenant_id,role_id,permission_id) DO UPDATE
SET deleted_at=NULL,updated_at=now();

-- Assigned-task identities receive only their task-work permissions.
UPDATE role_permission rp
SET deleted_at=now(),updated_at=now()
FROM role r
WHERE rp.tenant_id=r.tenant_id AND rp.role_id=r.id AND rp.deleted_at IS NULL
  AND r.deleted_at IS NULL AND r.code IN ('grader','arbitrator');

INSERT INTO role_permission (tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id
FROM role r JOIN permission p ON p.tenant_id=r.tenant_id
WHERE r.deleted_at IS NULL AND p.deleted_at IS NULL
  AND ((r.code='grader' AND p.code IN ('dashboard:read','review:work'))
    OR (r.code='arbitrator' AND p.code IN ('dashboard:read','arbitration:work')))
ON CONFLICT (tenant_id,role_id,permission_id) DO UPDATE
SET deleted_at=NULL,updated_at=now();

-- Auditors are tenant-wide read-only identities.
UPDATE role_permission rp
SET deleted_at=now(),updated_at=now()
FROM role r,permission p
WHERE rp.tenant_id=r.tenant_id AND rp.role_id=r.id
  AND rp.tenant_id=p.tenant_id AND rp.permission_id=p.id
  AND rp.deleted_at IS NULL AND r.code='auditor'
  AND p.action IN ('manage','create','work','publish','write','update','delete');

-- Repair the historical managed-user scope bug. Missing trustworthy bindings
-- fail closed to scope=none; no single-school guessing is performed.
-- First recover app_user.school_id only when the existing role scope contains
-- exactly one valid school binding for the same tenant.
WITH declared_school_candidates AS (
  SELECT ur.tenant_id, ur.user_id, school.id AS school_id
  FROM user_role ur
  JOIN role r
    ON r.tenant_id=ur.tenant_id
   AND r.id=ur.role_id
   AND r.deleted_at IS NULL
  CROSS JOIN LATERAL (VALUES (
    COALESCE(
      NULLIF(ur.data_scope->>'school_id',''),
      NULLIF(ur.data_scope->r.code->>'school_id','')
    )
  )) AS binding(school_id)
  JOIN school
    ON school.tenant_id=ur.tenant_id
   AND school.id::text=binding.school_id
   AND school.deleted_at IS NULL
  WHERE ur.deleted_at IS NULL
    AND r.code IN ('school_admin','teacher','grader','arbitrator')
), declared_school_binding AS (
  SELECT tenant_id,user_id,MIN(school_id::text)::uuid AS school_id
  FROM declared_school_candidates
  GROUP BY tenant_id,user_id
  HAVING COUNT(DISTINCT school_id)=1
)
UPDATE app_user u
SET school_id=binding.school_id,updated_at=now()
FROM declared_school_binding binding
WHERE u.tenant_id=binding.tenant_id
  AND u.id=binding.user_id
  AND u.school_id IS NULL
  AND u.deleted_at IS NULL;

UPDATE user_role ur
SET data_scope=CASE
  WHEN r.code='platform_admin' AND ur.tenant_id='00000000-0000-0000-0000-000000000001'::uuid
    THEN jsonb_build_object('scope','platform')
  WHEN r.code='platform_admin' THEN jsonb_build_object('scope','none')
  WHEN r.code='tenant_admin' THEN jsonb_build_object('scope','tenant')
  WHEN r.code='school_admin' AND u.school_id IS NOT NULL
    THEN jsonb_build_object('scope','school','school_id',u.school_id::text)
  WHEN r.code='school_admin' THEN jsonb_build_object('scope','none')
  WHEN r.code='teacher' THEN jsonb_build_object('scope','class')
  WHEN r.code IN ('grader','arbitrator') THEN jsonb_build_object('scope','exam_task')
  WHEN r.code='auditor' THEN jsonb_build_object('scope','tenant')
  WHEN r.code='student' AND NULLIF(ur.data_scope->>'student_id','') IS NOT NULL
    AND EXISTS (
      SELECT 1 FROM student st
      WHERE st.tenant_id=ur.tenant_id
        AND st.id::text=ur.data_scope->>'student_id'
        AND st.deleted_at IS NULL
    )
    THEN jsonb_build_object('scope','self','student_id',ur.data_scope->>'student_id')
  WHEN r.code='student' THEN jsonb_build_object('scope','none')
  WHEN r.code IN ('page_processing_worker','subjective_grading_worker') OR r.code LIKE '%\_worker' ESCAPE '\'
    THEN jsonb_build_object('scope','service')
  ELSE ur.data_scope
END,
updated_at=now()
FROM role r JOIN app_user u ON u.tenant_id=r.tenant_id
WHERE ur.tenant_id=r.tenant_id AND ur.role_id=r.id
  AND u.tenant_id=ur.tenant_id AND u.id=ur.user_id
  AND ur.deleted_at IS NULL AND r.deleted_at IS NULL AND u.deleted_at IS NULL;

-- Remove class bindings from identities that do not actually hold teacher.
UPDATE teacher_class tc
SET deleted_at=now(),updated_at=now()
WHERE tc.deleted_at IS NULL AND NOT EXISTS (
  SELECT 1 FROM user_role ur
  JOIN role r ON r.tenant_id=ur.tenant_id AND r.id=ur.role_id
  WHERE ur.tenant_id=tc.tenant_id AND ur.user_id=tc.teacher_id
    AND ur.deleted_at IS NULL AND r.deleted_at IS NULL AND r.code='teacher'
);
