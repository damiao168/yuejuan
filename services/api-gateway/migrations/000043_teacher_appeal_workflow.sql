INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, 'appeal:work', 'Work assigned appeals', 'appeal', 'work', '处理分配给当前教师的申诉并提交复核建议'
FROM tenant t
ON CONFLICT (tenant_id, code) DO UPDATE
SET name = EXCLUDED.name,
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    description = EXCLUDED.description,
    updated_at = now();

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('teacher', 'grader', 'arbitrator')
  AND p.code IN ('appeal:read', 'appeal:work')
ON CONFLICT (tenant_id, role_id, permission_id) DO UPDATE
SET deleted_at = NULL,
    updated_at = now();

DELETE FROM role_permission rp
USING role r, permission p
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND rp.tenant_id = r.tenant_id
  AND p.tenant_id = r.tenant_id
  AND r.code IN ('teacher', 'grader', 'arbitrator')
  AND p.code = 'appeal:manage';

ALTER TABLE appeal
  ADD COLUMN IF NOT EXISTS teacher_recommendation TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS teacher_recommendation_reason TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS teacher_recommended_score NUMERIC(8,2),
  ADD COLUMN IF NOT EXISTS teacher_recommendation_by UUID REFERENCES app_user(id),
  ADD COLUMN IF NOT EXISTS teacher_recommendation_at TIMESTAMPTZ;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'fk_appeal_teacher_recommendation_by_tenant'
  ) THEN
    ALTER TABLE appeal
      ADD CONSTRAINT fk_appeal_teacher_recommendation_by_tenant
      FOREIGN KEY (tenant_id, teacher_recommendation_by)
      REFERENCES app_user(tenant_id, id);
  END IF;
END $$;

ALTER TABLE appeal
  DROP CONSTRAINT IF EXISTS ck_appeal_teacher_recommendation;

ALTER TABLE appeal
  ADD CONSTRAINT ck_appeal_teacher_recommendation
  CHECK (teacher_recommendation IN ('', 'accept', 'reject', 'adjust_score', 'need_more_info'));

ALTER TABLE appeal
  DROP CONSTRAINT IF EXISTS ck_appeal_teacher_recommended_score;

ALTER TABLE appeal
  ADD CONSTRAINT ck_appeal_teacher_recommended_score
  CHECK (
    teacher_recommended_score IS NULL
    OR (teacher_recommended_score >= 0 AND teacher_recommendation = 'adjust_score')
  );

CREATE INDEX IF NOT EXISTS idx_appeal_assigned_queue
  ON appeal (tenant_id, assigned_to, status, created_at DESC)
  WHERE deleted_at IS NULL;
