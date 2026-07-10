INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('grading:manage', 'Manage rule grading', 'grading', 'manage', '客观题和填空题规则判分')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'grading:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS answer_segment_answer (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  answer_text TEXT NOT NULL,
  answer_payload JSONB NOT NULL DEFAULT '{}',
  source TEXT NOT NULL,
  confidence NUMERIC(5,4),
  recorded_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (source IN ('manual_entry', 'ocr_text', 'imported_answer')),
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1))
);

CREATE TABLE IF NOT EXISTS ai_grade (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  question_id UUID NOT NULL REFERENCES question(id),
  question_no TEXT NOT NULL,
  question_type TEXT NOT NULL,
  answer_version TEXT NOT NULL,
  grader_type TEXT NOT NULL,
  rule_version TEXT NOT NULL,
  model_version_id UUID,
  prompt_version_id UUID,
  rubric_version_id UUID REFERENCES rubric_version(id),
  suggested_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  confidence NUMERIC(5,4) NOT NULL,
  matched_points JSONB NOT NULL DEFAULT '[]',
  missing_points JSONB NOT NULL DEFAULT '[]',
  evidence JSONB NOT NULL DEFAULT '[]',
  risk_flags JSONB NOT NULL DEFAULT '[]',
  needs_human_review BOOLEAN NOT NULL,
  auto_pass BOOLEAN NOT NULL DEFAULT FALSE,
  mock BOOLEAN NOT NULL DEFAULT FALSE,
  raw_output JSONB NOT NULL DEFAULT '{}',
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (question_type IN ('single_choice', 'multiple_choice', 'true_false', 'fill_blank', 'numeric')),
  CHECK (grader_type IN ('rule_based_objective')),
  CHECK (suggested_score >= 0 AND suggested_score <= max_score),
  CHECK (max_score >= 0),
  CHECK (confidence >= 0 AND confidence <= 1)
);

CREATE INDEX IF NOT EXISTS idx_answer_segment_answer_latest ON answer_segment_answer (tenant_id, answer_segment_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_grade_segment ON ai_grade (tenant_id, answer_segment_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_grade_question ON ai_grade (tenant_id, question_id, created_at DESC);
