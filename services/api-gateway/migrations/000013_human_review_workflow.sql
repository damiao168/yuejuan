INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('review:manage', 'Manage human review tasks', 'review', 'manage', '人工复核与阅卷任务管理'),
  ('review:work', 'Work assigned human review tasks', 'review', 'work', '处理分配给自己的人工阅卷任务')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'review:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'grader'
  AND p.code = 'review:work'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS review_task (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  question_id UUID NOT NULL REFERENCES question(id),
  question_no TEXT NOT NULL,
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  anonymous_code TEXT NOT NULL,
  source TEXT NOT NULL,
  status TEXT NOT NULL,
  priority INT NOT NULL DEFAULT 0,
  assigned_to UUID REFERENCES app_user(id),
  return_reason TEXT NOT NULL DEFAULT '',
  due_at TIMESTAMPTZ,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (source IN (
    'ai_low_confidence',
    'ocr_low_confidence',
    'subjective_default_review',
    'evidence_verification_failed',
    'double_mark_required',
    'score_anomaly',
    'manual_sample'
  )),
  CHECK (status IN ('pending', 'assigned', 'in_progress', 'submitted', 'returned', 'completed')),
  CHECK (priority >= 0)
);

CREATE TABLE IF NOT EXISTS human_grade (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  review_task_id UUID NOT NULL REFERENCES review_task(id),
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  reviewer_id UUID NOT NULL REFERENCES app_user(id),
  score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  rubric_selections JSONB NOT NULL DEFAULT '[]',
  comments TEXT NOT NULL DEFAULT '',
  private_note TEXT NOT NULL DEFAULT '',
  student_feedback TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  grade_round TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (score >= 0 AND score <= max_score),
  CHECK (max_score >= 0),
  CHECK (grade_round IN ('single', 'first_mark', 'second_mark', 'appeal_review'))
);

CREATE INDEX IF NOT EXISTS idx_review_task_status ON review_task (tenant_id, status, priority DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_review_task_assignee ON review_task (tenant_id, assigned_to, status, priority DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_review_task_segment ON review_task (tenant_id, answer_segment_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_human_grade_task ON human_grade (tenant_id, review_task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_human_grade_segment ON human_grade (tenant_id, answer_segment_id, created_at DESC);
