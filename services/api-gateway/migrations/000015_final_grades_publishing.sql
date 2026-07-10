INSERT INTO role (tenant_id, code, name, scope_type, description)
SELECT t.id, r.code, r.name, r.scope_type, r.description
FROM tenant t
CROSS JOIN (VALUES
  ('student', 'Student', 'self', '学生')
) AS r(code, name, scope_type, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('score:manage', 'Manage final grades', 'score', 'manage', '最终成绩确认、发布和导出'),
  ('student:grade:read', 'Read published student grades', 'student_grade', 'read', '学生查看已发布成绩')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher', 'auditor')
  AND p.code = 'score:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'student:grade:read'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'student'
  AND p.code = 'student:grade:read'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO app_user (tenant_id, username, display_name, password_hash, status)
SELECT t.id, 'student', '学生', crypt(gen_random_uuid()::text, gen_salt('bf')), 'disabled'
FROM tenant t
WHERE t.code = 'demo'
ON CONFLICT (tenant_id, username) DO NOTHING;

INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT u.tenant_id, u.id, r.id, jsonb_build_object('scope', r.scope_type)
FROM app_user u
JOIN role r ON r.tenant_id = u.tenant_id AND r.code = 'student'
WHERE u.username = 'student'
ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING;

ALTER TABLE final_grade ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending_confirmation';

ALTER TABLE final_grade DROP CONSTRAINT IF EXISTS final_grade_source_check;
ALTER TABLE final_grade DROP CONSTRAINT IF EXISTS ck_final_grade_source;
ALTER TABLE final_grade ADD CONSTRAINT ck_final_grade_source CHECK (
  source IN ('double_mark_auto', 'arbitration', 'single_review', 'rule_auto')
);

ALTER TABLE final_grade DROP CONSTRAINT IF EXISTS ck_final_grade_status;
ALTER TABLE final_grade ADD CONSTRAINT ck_final_grade_status CHECK (
  status IN ('calculating', 'pending_confirmation', 'confirmed', 'pending_publish', 'published', 'locked')
);

CREATE TABLE IF NOT EXISTS submission_grade (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  student_id UUID REFERENCES student(id),
  anonymous_code TEXT NOT NULL,
  total_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  status TEXT NOT NULL,
  locked BOOLEAN NOT NULL DEFAULT FALSE,
  confirmed_by UUID REFERENCES app_user(id),
  confirmed_at TIMESTAMPTZ,
  published_by UUID REFERENCES app_user(id),
  published_at TIMESTAMPTZ,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, submission_id),
  CHECK (total_score >= 0 AND total_score <= max_score),
  CHECK (max_score >= 0),
  CHECK (status IN ('calculating', 'pending_confirmation', 'confirmed', 'pending_publish', 'published', 'locked'))
);

CREATE INDEX IF NOT EXISTS idx_final_grade_exam_status ON final_grade (tenant_id, exam_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_final_grade_submission ON final_grade (tenant_id, submission_id, question_no);
CREATE INDEX IF NOT EXISTS idx_submission_grade_exam_status ON submission_grade (tenant_id, exam_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_submission_grade_student ON submission_grade (tenant_id, student_id, exam_id);
