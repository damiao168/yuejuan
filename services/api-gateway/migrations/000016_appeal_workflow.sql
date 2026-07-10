INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('appeal:create', 'Create appeals', 'appeal', 'create', '学生提交成绩申诉'),
  ('appeal:read', 'Read appeals', 'appeal', 'read', '查看申诉处理结果'),
  ('appeal:manage', 'Manage appeals', 'appeal', 'manage', '教师、仲裁员和管理员处理申诉')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'student'
  AND p.code IN ('appeal:create', 'appeal:read')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher', 'arbitrator')
  AND p.code IN ('appeal:read', 'appeal:manage')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'auditor'
  AND p.code = 'appeal:read'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS appeal (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  submission_grade_id UUID NOT NULL REFERENCES submission_grade(id),
  student_id UUID NOT NULL REFERENCES student(id),
  target_type TEXT NOT NULL,
  final_grade_id UUID REFERENCES final_grade(id),
  question_id UUID REFERENCES question(id),
  question_no TEXT NOT NULL DEFAULT '',
  deduction_point_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL,
  attachment JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL,
  result_reason TEXT NOT NULL DEFAULT '',
  assigned_to UUID REFERENCES app_user(id),
  reviewed_by UUID REFERENCES app_user(id),
  reviewed_at TIMESTAMPTZ,
  closed_by UUID REFERENCES app_user(id),
  closed_at TIMESTAMPTZ,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (target_type IN ('exam', 'question', 'deduction_point')),
  CHECK (status IN ('submitted', 'under_review', 'need_more_info', 'accepted', 'rejected', 'score_adjusted', 'closed')),
  CHECK (reason <> ''),
  CHECK (target_type = 'exam' OR final_grade_id IS NOT NULL),
  CHECK (target_type <> 'deduction_point' OR deduction_point_id <> '')
);

CREATE TABLE IF NOT EXISTS score_adjustment (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  appeal_id UUID NOT NULL REFERENCES appeal(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  submission_grade_id UUID NOT NULL REFERENCES submission_grade(id),
  final_grade_id UUID NOT NULL REFERENCES final_grade(id),
  question_id UUID NOT NULL REFERENCES question(id),
  question_no TEXT NOT NULL,
  previous_score NUMERIC(8,2) NOT NULL,
  adjusted_score NUMERIC(8,2) NOT NULL,
  delta NUMERIC(8,2) NOT NULL,
  reason TEXT NOT NULL,
  adjusted_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (previous_score >= 0),
  CHECK (adjusted_score >= 0),
  CHECK (reason <> '')
);

CREATE INDEX IF NOT EXISTS idx_appeal_exam_status ON appeal (tenant_id, exam_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_appeal_student ON appeal (tenant_id, student_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_appeal_submission ON appeal (tenant_id, submission_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_score_adjustment_appeal ON score_adjustment (tenant_id, appeal_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_score_adjustment_final_grade ON score_adjustment (tenant_id, final_grade_id, created_at DESC);
