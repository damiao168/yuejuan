CREATE TABLE IF NOT EXISTS exam_session (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  grade_id UUID NOT NULL REFERENCES grade(id),
  name TEXT NOT NULL,
  exam_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  grading_mode TEXT NOT NULL,
  appeal_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  publish_policy TEXT NOT NULL,
  created_by UUID NOT NULL REFERENCES app_user(id),
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (status IN ('draft', 'configured', 'collecting', 'grading', 'reviewing', 'finalized', 'published', 'archived')),
  CHECK (grading_mode IN ('auto_objective_only', 'ai_assisted', 'human_review_required', 'double_mark', 'blind_double_mark'))
);

ALTER TABLE exam ADD COLUMN IF NOT EXISTS exam_session_id UUID REFERENCES exam_session(id);
ALTER TABLE exam ADD COLUMN IF NOT EXISTS duration_minutes INT;
ALTER TABLE exam ADD COLUMN IF NOT EXISTS candidate_rule TEXT NOT NULL DEFAULT 'all_selected_classes';

CREATE TABLE IF NOT EXISTS exam_blueprint_section (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  title TEXT NOT NULL,
  question_type TEXT NOT NULL,
  question_count INT NOT NULL,
  score_per_question NUMERIC(8,2) NOT NULL,
  sort_order INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, sort_order),
  CHECK (question_count > 0),
  CHECK (score_per_question > 0)
);

CREATE INDEX IF NOT EXISTS idx_exam_session_school ON exam_session (tenant_id, school_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_exam_session_grade ON exam_session (tenant_id, grade_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_exam_parent_session ON exam (tenant_id, exam_session_id);
CREATE INDEX IF NOT EXISTS idx_exam_blueprint_section_exam ON exam_blueprint_section (tenant_id, exam_id, sort_order);
