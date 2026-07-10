CREATE TABLE IF NOT EXISTS exam (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  name TEXT NOT NULL,
  subject TEXT NOT NULL,
  exam_type TEXT NOT NULL,
  total_score NUMERIC(8,2) NOT NULL,
  status TEXT NOT NULL,
  grading_mode TEXT NOT NULL,
  appeal_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  publish_policy TEXT NOT NULL,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (total_score >= 0),
  CHECK (status IN ('draft', 'configured', 'collecting', 'grading', 'reviewing', 'finalized', 'published', 'archived')),
  CHECK (grading_mode IN ('auto_objective_only', 'ai_assisted', 'human_review_required', 'double_mark', 'blind_double_mark'))
);

CREATE TABLE IF NOT EXISTS exam_class (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  class_id UUID NOT NULL REFERENCES school_class(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, class_id)
);

CREATE INDEX IF NOT EXISTS idx_exam_tenant_status ON exam (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_exam_school_status ON exam (tenant_id, school_id, status);
CREATE INDEX IF NOT EXISTS idx_exam_class_exam ON exam_class (tenant_id, exam_id);
CREATE INDEX IF NOT EXISTS idx_exam_class_class ON exam_class (tenant_id, class_id);
