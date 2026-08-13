INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT tenant.id, item.code, item.name, item.resource, item.action, item.description
FROM tenant
CROSS JOIN (VALUES
  ('answer_group:read', 'Read answer groups', 'answer_group', 'read', '查看相似答案分组与抽检状态'),
  ('answer_group:manage', 'Manage answer groups', 'answer_group', 'manage', '构建、抽检、确认和回滚答案分组候选')
) AS item(code, name, resource, action, description)
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT role.tenant_id, role.id, permission.id
FROM role
JOIN permission ON permission.tenant_id = role.tenant_id
WHERE role.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND permission.code = 'answer_group:read'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

ALTER TABLE review_task DROP CONSTRAINT review_task_source_check;
ALTER TABLE review_task ADD CONSTRAINT review_task_source_check CHECK (source IN (
  'ai_low_confidence', 'ocr_low_confidence', 'subjective_default_review',
  'evidence_verification_failed', 'double_mark_required', 'score_anomaly',
  'manual_sample', 'omr_ambiguous', 'rule_review_required', 'grading_failure',
  'answer_group_outlier'
));

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT role.tenant_id, role.id, permission.id
FROM role
JOIN permission ON permission.tenant_id = role.tenant_id
WHERE role.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND permission.code = 'answer_group:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE answer_group (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  exam_question_snapshot_id UUID NOT NULL,
  build_input_hash TEXT NOT NULL,
  algorithm_version TEXT NOT NULL,
  representation_version TEXT NOT NULL,
  member_count INT NOT NULL,
  representative_submission_id UUID NOT NULL,
  homogeneity DOUBLE PRECISION NOT NULL,
  minimum_sample INT NOT NULL,
  status TEXT NOT NULL DEFAULT 'sampling',
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, exam_question_snapshot_id) REFERENCES exam_question_snapshot(tenant_id, id),
  FOREIGN KEY (tenant_id, representative_submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CHECK (length(build_input_hash) = 64),
  CHECK (length(btrim(algorithm_version)) BETWEEN 1 AND 160),
  CHECK (length(btrim(representation_version)) BETWEEN 1 AND 160),
  CHECK (member_count > 0 AND minimum_sample > 0 AND minimum_sample <= member_count),
  CHECK (homogeneity >= 0 AND homogeneity <= 1),
  CHECK (status IN ('sampling', 'ready_for_confirmation', 'confirmed', 'rolled_back'))
);

CREATE INDEX idx_answer_group_question
ON answer_group (tenant_id, exam_id, question_id, created_at DESC);

CREATE INDEX idx_answer_group_build
ON answer_group (tenant_id, exam_id, question_id, build_input_hash, created_at DESC);

CREATE TABLE answer_group_member (
  tenant_id UUID NOT NULL,
  group_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  segment_id UUID NOT NULL,
  representation_hash TEXT NOT NULL,
  similarity DOUBLE PRECISION NOT NULL,
  outlier_score DOUBLE PRECISION NOT NULL,
  is_representative BOOLEAN NOT NULL DEFAULT false,
  is_boundary BOOLEAN NOT NULL DEFAULT false,
  is_outlier BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, group_id, segment_id),
  FOREIGN KEY (tenant_id, group_id) REFERENCES answer_group(tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, segment_id) REFERENCES answer_segment(tenant_id, id),
  CHECK (length(representation_hash) = 64),
  CHECK (similarity >= 0 AND similarity <= 1),
  CHECK (outlier_score >= 0 AND outlier_score <= 1)
);

CREATE INDEX idx_answer_group_member_submission
ON answer_group_member (tenant_id, submission_id, group_id);

CREATE TABLE answer_group_sample_review (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  group_id UUID NOT NULL,
  segment_id UUID NOT NULL,
  outcome TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  reviewed_by UUID NOT NULL,
  reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, group_id, segment_id),
  FOREIGN KEY (tenant_id, group_id, segment_id)
    REFERENCES answer_group_member(tenant_id, group_id, segment_id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, reviewed_by) REFERENCES app_user(tenant_id, id),
  CHECK (outcome IN ('accepted', 'rejected')),
  CHECK (length(notes) <= 4000)
);

CREATE TABLE answer_group_decision (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  group_id UUID NOT NULL,
  score_candidate_json JSONB NOT NULL,
  rubric_selection_json JSONB NOT NULL,
  sample_size INT NOT NULL DEFAULT 0,
  min_sample INT NOT NULL,
  confirmed_by UUID,
  confirmed_at TIMESTAMPTZ,
  revision INT NOT NULL DEFAULT 1,
  rollback_reference UUID,
  rolled_back_by UUID,
  rolled_back_at TIMESTAMPTZ,
  rollback_reason TEXT NOT NULL DEFAULT '',
  created_by UUID NOT NULL,
  updated_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, group_id),
  FOREIGN KEY (tenant_id, group_id) REFERENCES answer_group(tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, confirmed_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, rolled_back_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, updated_by) REFERENCES app_user(tenant_id, id),
  CHECK (jsonb_typeof(score_candidate_json) = 'object' AND score_candidate_json <> '{}'::jsonb),
  CHECK (jsonb_typeof(rubric_selection_json) = 'object' AND rubric_selection_json <> '{}'::jsonb),
  CHECK (sample_size >= 0 AND min_sample > 0),
  CHECK (revision > 0),
  CHECK ((confirmed_by IS NULL) = (confirmed_at IS NULL)),
  CHECK ((rolled_back_by IS NULL) = (rolled_back_at IS NULL)),
  CHECK (length(rollback_reason) <= 4000)
);

-- These are proposed grading facts only. No FK or trigger targets final_grade;
-- a later human-review workflow must explicitly consume an active candidate.
CREATE TABLE answer_group_automation_candidate (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  group_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  segment_id UUID NOT NULL,
  decision_revision INT NOT NULL DEFAULT 0,
  candidate_kind TEXT NOT NULL,
  score_candidate_json JSONB NOT NULL DEFAULT '{}',
  rubric_selection_json JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL,
  algorithm_version TEXT NOT NULL,
  rollback_reference UUID,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  rolled_back_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, group_id, segment_id, decision_revision, candidate_kind),
  FOREIGN KEY (tenant_id, group_id, segment_id)
    REFERENCES answer_group_member(tenant_id, group_id, segment_id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CHECK (decision_revision >= 0),
  CHECK (candidate_kind IN ('group_score', 'individual_review')),
  CHECK (status IN ('active', 'manual_required', 'rolled_back')),
  CHECK (jsonb_typeof(score_candidate_json) = 'object'),
  CHECK (jsonb_typeof(rubric_selection_json) = 'object'),
  CHECK (length(btrim(algorithm_version)) BETWEEN 1 AND 160)
);

CREATE INDEX idx_answer_group_candidate_segment
ON answer_group_automation_candidate (tenant_id, segment_id, created_at DESC)
WHERE status IN ('active', 'manual_required');
