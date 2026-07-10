INSERT INTO role (tenant_id, code, name, scope_type, description)
SELECT t.id, r.code, r.name, r.scope_type, r.description
FROM tenant t
CROSS JOIN (VALUES
  ('arbitrator', 'Arbitrator', 'exam_task', '仲裁员')
) AS r(code, name, scope_type, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('arbitration:manage', 'Manage arbitration tasks', 'arbitration', 'manage', '双评仲裁任务管理'),
  ('arbitration:work', 'Work assigned arbitration tasks', 'arbitration', 'work', '处理分配给自己的双评仲裁任务')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'arbitration:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'arbitrator'
  AND p.code = 'arbitration:work'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO app_user (tenant_id, username, display_name, password_hash, status)
SELECT t.id, 'arbitrator', '仲裁员', crypt(gen_random_uuid()::text, gen_salt('bf')), 'disabled'
FROM tenant t
WHERE t.code = 'demo'
ON CONFLICT (tenant_id, username) DO NOTHING;

INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT u.tenant_id, u.id, r.id, jsonb_build_object('scope', r.scope_type)
FROM app_user u
JOIN role r ON r.tenant_id = u.tenant_id AND r.code = 'arbitrator'
WHERE u.username = 'arbitrator'
ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING;

ALTER TABLE review_task ADD COLUMN IF NOT EXISTS grade_round TEXT NOT NULL DEFAULT 'single';

ALTER TABLE review_task DROP CONSTRAINT IF EXISTS ck_review_task_grade_round;
ALTER TABLE review_task ADD CONSTRAINT ck_review_task_grade_round CHECK (
  grade_round IN ('single', 'first_mark', 'second_mark', 'appeal_review')
);

CREATE TABLE IF NOT EXISTS double_mark_policy (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  question_id UUID REFERENCES question(id),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  threshold NUMERIC(8,2) NOT NULL,
  resolution_strategy TEXT NOT NULL,
  allow_same_arbitrator BOOLEAN NOT NULL DEFAULT FALSE,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (threshold >= 0),
  CHECK (resolution_strategy IN ('average', 'first', 'second', 'higher', 'lower'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_double_mark_policy_exam
ON double_mark_policy (tenant_id, exam_id)
WHERE question_id IS NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_double_mark_policy_question
ON double_mark_policy (tenant_id, question_id)
WHERE question_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS double_mark_session (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  question_id UUID NOT NULL REFERENCES question(id),
  question_no TEXT NOT NULL,
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  anonymous_code TEXT NOT NULL,
  first_review_task_id UUID NOT NULL REFERENCES review_task(id),
  second_review_task_id UUID NOT NULL REFERENCES review_task(id),
  first_reviewer_id UUID NOT NULL REFERENCES app_user(id),
  second_reviewer_id UUID NOT NULL REFERENCES app_user(id),
  threshold NUMERIC(8,2) NOT NULL,
  resolution_strategy TEXT NOT NULL,
  status TEXT NOT NULL,
  score_difference NUMERIC(8,2),
  final_grade_id UUID,
  arbitration_task_id UUID,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (first_reviewer_id <> second_reviewer_id),
  CHECK (threshold >= 0),
  CHECK (resolution_strategy IN ('average', 'first', 'second', 'higher', 'lower')),
  CHECK (status IN ('pending', 'first_submitted', 'second_submitted', 'auto_finalized', 'needs_arbitration', 'arbitrated'))
);

CREATE TABLE IF NOT EXISTS arbitration_task (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  double_mark_session_id UUID NOT NULL REFERENCES double_mark_session(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  question_id UUID NOT NULL REFERENCES question(id),
  question_no TEXT NOT NULL,
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  anonymous_code TEXT NOT NULL,
  first_reviewer_id UUID NOT NULL REFERENCES app_user(id),
  second_reviewer_id UUID NOT NULL REFERENCES app_user(id),
  first_score NUMERIC(8,2) NOT NULL,
  second_score NUMERIC(8,2) NOT NULL,
  score_difference NUMERIC(8,2) NOT NULL,
  difference_reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  assigned_to UUID REFERENCES app_user(id),
  final_score NUMERIC(8,2),
  reason TEXT NOT NULL DEFAULT '',
  student_feedback TEXT NOT NULL DEFAULT '',
  allow_same_arbitrator BOOLEAN NOT NULL DEFAULT FALSE,
  context JSONB NOT NULL DEFAULT '{}',
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (first_score >= 0),
  CHECK (second_score >= 0),
  CHECK (score_difference >= 0),
  CHECK (final_score IS NULL OR final_score >= 0),
  CHECK (status IN ('pending', 'assigned', 'submitted'))
);

CREATE TABLE IF NOT EXISTS final_grade (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  question_id UUID NOT NULL REFERENCES question(id),
  question_no TEXT NOT NULL,
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  anonymous_code TEXT NOT NULL,
  score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  source TEXT NOT NULL,
  double_mark_session_id UUID REFERENCES double_mark_session(id),
  arbitration_task_id UUID REFERENCES arbitration_task(id),
  resolution_strategy TEXT,
  locked BOOLEAN NOT NULL DEFAULT FALSE,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (score >= 0 AND score <= max_score),
  CHECK (max_score >= 0),
  CHECK (source IN ('double_mark_auto', 'arbitration', 'single_review')),
  CHECK (resolution_strategy IS NULL OR resolution_strategy IN ('average', 'first', 'second', 'higher', 'lower', 'arbitration'))
);

CREATE INDEX IF NOT EXISTS idx_double_mark_policy_exam ON double_mark_policy (tenant_id, exam_id);
CREATE INDEX IF NOT EXISTS idx_double_mark_policy_question ON double_mark_policy (tenant_id, question_id);
CREATE INDEX IF NOT EXISTS idx_double_mark_session_status ON double_mark_session (tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_double_mark_session_segment ON double_mark_session (tenant_id, answer_segment_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_double_mark_session_tasks ON double_mark_session (tenant_id, first_review_task_id, second_review_task_id);
CREATE INDEX IF NOT EXISTS idx_arbitration_task_status ON arbitration_task (tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_arbitration_task_assignee ON arbitration_task (tenant_id, assigned_to, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_final_grade_segment ON final_grade (tenant_id, answer_segment_id, created_at DESC);
