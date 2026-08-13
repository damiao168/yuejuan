CREATE TABLE grading_gold_paper (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  active_version INT,
  status TEXT NOT NULL DEFAULT 'pending_approval',
  nominated_by UUID NOT NULL,
  retired_by UUID,
  retired_at TIMESTAMPTZ,
  retirement_reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, exam_id, question_id, submission_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, nominated_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, retired_by) REFERENCES app_user(tenant_id, id),
  CHECK (active_version IS NULL OR active_version > 0),
  CHECK (status IN ('pending_approval', 'active', 'retired')),
  CHECK ((status = 'active' AND active_version IS NOT NULL AND retired_at IS NULL) OR status <> 'active'),
  CHECK ((status = 'retired' AND retired_by IS NOT NULL AND retired_at IS NOT NULL) OR status <> 'retired')
);

CREATE INDEX idx_grading_gold_paper_question
ON grading_gold_paper (tenant_id, exam_id, question_id, status, created_at, id);

CREATE TABLE grading_gold_paper_version (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  gold_paper_id UUID NOT NULL,
  version INT NOT NULL,
  exam_question_snapshot_id UUID NOT NULL,
  reference_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  rubric_snapshot_json JSONB NOT NULL,
  explanation TEXT NOT NULL,
  trait_scores_json JSONB NOT NULL DEFAULT '{}',
  error_tags_json JSONB NOT NULL DEFAULT '[]',
  source_grade_ids_json JSONB NOT NULL,
  nominated_by UUID NOT NULL,
  approved_by UUID,
  approved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, gold_paper_id, version),
  FOREIGN KEY (tenant_id, gold_paper_id) REFERENCES grading_gold_paper(tenant_id, id),
  FOREIGN KEY (tenant_id, exam_question_snapshot_id) REFERENCES exam_question_snapshot(tenant_id, id),
  FOREIGN KEY (tenant_id, nominated_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, approved_by) REFERENCES app_user(tenant_id, id),
  CHECK (version > 0),
  CHECK (reference_score >= 0 AND max_score >= 0 AND reference_score <= max_score),
  CHECK (length(btrim(explanation)) BETWEEN 1 AND 8000),
  CHECK (jsonb_typeof(rubric_snapshot_json) = 'object'),
  CHECK (jsonb_typeof(trait_scores_json) = 'object'),
  CHECK (jsonb_typeof(error_tags_json) = 'array'),
  CHECK (jsonb_typeof(source_grade_ids_json) = 'array'),
  CHECK (jsonb_array_length(source_grade_ids_json) > 0),
  CHECK ((approved_by IS NULL AND approved_at IS NULL) OR (approved_by IS NOT NULL AND approved_at IS NOT NULL))
);

CREATE UNIQUE INDEX uq_grading_gold_paper_pending_version
ON grading_gold_paper_version (tenant_id, gold_paper_id)
WHERE approved_at IS NULL;

CREATE INDEX idx_grading_gold_paper_version_active_lookup
ON grading_gold_paper_version (tenant_id, gold_paper_id, version DESC, approved_at)
INCLUDE (reference_score, max_score, exam_question_snapshot_id);

CREATE OR REPLACE FUNCTION protect_approved_gold_paper_version()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.approved_at IS NOT NULL THEN
    RAISE EXCEPTION 'approved Gold Paper versions are immutable'
      USING ERRCODE = '55000';
  END IF;
  RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END
$$;

CREATE TRIGGER trg_protect_approved_gold_paper_version
BEFORE UPDATE OR DELETE ON grading_gold_paper_version
FOR EACH ROW EXECUTE FUNCTION protect_approved_gold_paper_version();

CREATE OR REPLACE FUNCTION validate_active_gold_paper_version()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.status = 'active' AND NOT EXISTS (
    SELECT 1
    FROM grading_gold_paper_version version
    WHERE version.tenant_id = NEW.tenant_id
      AND version.gold_paper_id = NEW.id
      AND version.version = NEW.active_version
      AND version.approved_at IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'active Gold Paper must reference an approved version'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$$;

CREATE CONSTRAINT TRIGGER trg_validate_active_gold_paper_version
AFTER INSERT OR UPDATE OF active_version, status ON grading_gold_paper
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_active_gold_paper_version();
