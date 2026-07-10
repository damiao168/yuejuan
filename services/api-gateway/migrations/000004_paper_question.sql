CREATE TABLE IF NOT EXISTS file_asset (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID,
  exam_id UUID REFERENCES exam(id),
  submission_id UUID,
  owner_type TEXT NOT NULL,
  owner_id UUID,
  original_name TEXT NOT NULL,
  content_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  hash_sha256 TEXT NOT NULL,
  storage_bucket TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  visibility TEXT NOT NULL,
  uploaded_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, hash_sha256, owner_type, owner_id)
);

CREATE TABLE IF NOT EXISTS exam_paper (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  file_asset_id UUID NOT NULL REFERENCES file_asset(id),
  version_no INT NOT NULL,
  status TEXT NOT NULL,
  uploaded_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, version_no)
);

CREATE TABLE IF NOT EXISTS question (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  exam_paper_id UUID REFERENCES exam_paper(id),
  question_no TEXT NOT NULL,
  question_type TEXT NOT NULL,
  score NUMERIC(8,2) NOT NULL,
  stem TEXT,
  knowledge_points JSONB NOT NULL DEFAULT '[]',
  answer_area JSONB,
  sort_order INT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, question_no),
  CHECK (score >= 0),
  CHECK (question_type IN ('single_choice', 'multiple_choice', 'true_false', 'fill_blank', 'numeric', 'formula', 'short_answer', 'calculation', 'essay', 'discussion', 'coding'))
);

CREATE TABLE IF NOT EXISTS question_answer_key (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  question_id UUID NOT NULL REFERENCES question(id),
  answer_version TEXT NOT NULL,
  standard_answer JSONB NOT NULL,
  equivalent_answers JSONB NOT NULL DEFAULT '[]',
  tolerance JSONB NOT NULL DEFAULT '{}',
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS rubric_version (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  question_id UUID NOT NULL REFERENCES question(id),
  version TEXT NOT NULL,
  status TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  created_by UUID NOT NULL REFERENCES app_user(id),
  approved_by UUID REFERENCES app_user(id),
  approved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, question_id, version),
  CHECK (status IN ('draft', 'pending_review', 'approved', 'locked'))
);

CREATE TABLE IF NOT EXISTS question_rubric (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  question_id UUID NOT NULL REFERENCES question(id),
  rubric_version_id UUID NOT NULL REFERENCES rubric_version(id),
  status TEXT NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  points JSONB NOT NULL DEFAULT '[]',
  deductions JSONB NOT NULL DEFAULT '[]',
  examples JSONB NOT NULL DEFAULT '[]',
  created_by UUID NOT NULL REFERENCES app_user(id),
  approved_by UUID REFERENCES app_user(id),
  approved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (max_score >= 0),
  CHECK (status IN ('draft', 'pending_review', 'approved', 'locked'))
);

CREATE INDEX IF NOT EXISTS idx_exam_paper_exam ON exam_paper (tenant_id, exam_id);
CREATE INDEX IF NOT EXISTS idx_question_exam ON question (tenant_id, exam_id);
CREATE INDEX IF NOT EXISTS idx_answer_key_question ON question_answer_key (tenant_id, question_id);
CREATE INDEX IF NOT EXISTS idx_rubric_question ON question_rubric (tenant_id, question_id);
CREATE INDEX IF NOT EXISTS idx_file_asset_owner ON file_asset (tenant_id, owner_type, owner_id);
