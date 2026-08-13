-- STORY-A18: a public score is an immutable release, never a mutable view of
-- submission_grade/final_grade.  Regrades and appeal corrections create a
-- later release and retain the earlier facts for audit and student context.

CREATE TABLE IF NOT EXISTS score_release (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  version INT NOT NULL,
  source TEXT NOT NULL,
  reason TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  idempotency_key TEXT NOT NULL,
  visibility_policy JSONB NOT NULL DEFAULT '{}',
  appeal_window JSONB NOT NULL DEFAULT '{"enabled":false}',
  gate_snapshot JSONB NOT NULL DEFAULT '{}',
  source_release_id UUID,
  supersedes_release_id UUID,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_by UUID,
  published_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, id, exam_id),
  UNIQUE (tenant_id, exam_id, version),
  UNIQUE (tenant_id, exam_id, idempotency_key),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, published_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, source_release_id) REFERENCES score_release(tenant_id, id),
  FOREIGN KEY (tenant_id, supersedes_release_id) REFERENCES score_release(tenant_id, id),
  CHECK (version > 0),
  CHECK (source IN ('initial', 'regrade', 'appeal', 'rollback', 'migration')),
  CHECK (status IN ('draft', 'published')),
  CHECK (length(btrim(reason)) > 0 AND octet_length(reason) <= 2000),
  CHECK (length(btrim(idempotency_key)) BETWEEN 8 AND 200),
  CHECK (jsonb_typeof(visibility_policy) = 'object'),
  CHECK (jsonb_typeof(appeal_window) = 'object'),
  CHECK (jsonb_typeof(gate_snapshot) = 'object'),
  CHECK ((status = 'draft' AND published_at IS NULL AND published_by IS NULL)
      OR (status = 'published' AND published_at IS NOT NULL AND published_by IS NOT NULL)),
  CHECK (source <> 'rollback' OR source_release_id IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS score_release_current (
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  release_id UUID NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, exam_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, release_id, exam_id) REFERENCES score_release(tenant_id, id, exam_id)
);

CREATE TABLE IF NOT EXISTS score_release_item (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  release_id UUID NOT NULL,
  student_id UUID,
  submission_id UUID NOT NULL,
  total_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  status TEXT NOT NULL,
  snapshot_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, release_id, submission_id),
  FOREIGN KEY (tenant_id, release_id) REFERENCES score_release(tenant_id, id),
  FOREIGN KEY (tenant_id, student_id) REFERENCES student(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  CHECK (total_score >= 0 AND total_score <= max_score),
  CHECK (max_score >= 0),
  CHECK (status IN ('confirmed', 'published', 'locked')),
  CHECK (length(snapshot_hash) = 64)
);

CREATE TABLE IF NOT EXISTS score_release_question (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  release_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  question_id UUID NOT NULL,
  question_no TEXT NOT NULL,
  final_grade_id UUID NOT NULL,
  score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  source_type TEXT NOT NULL,
  source_id UUID,
  student_explanation JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, release_id, submission_id, question_id),
  FOREIGN KEY (tenant_id, release_id) REFERENCES score_release(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, final_grade_id) REFERENCES final_grade(tenant_id, id),
  CHECK (score >= 0 AND score <= max_score),
  CHECK (max_score >= 0),
  CHECK (source_type IN ('double_mark_auto', 'arbitration', 'single_review', 'rule_auto')),
  CHECK (jsonb_typeof(student_explanation) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_score_release_exam_version
ON score_release (tenant_id, exam_id, version DESC);

CREATE INDEX IF NOT EXISTS idx_score_release_current_release
ON score_release_current (tenant_id, release_id);

CREATE INDEX IF NOT EXISTS idx_score_release_item_student
ON score_release_item (tenant_id, student_id, release_id);

CREATE INDEX IF NOT EXISTS idx_score_release_question_submission
ON score_release_question (tenant_id, release_id, submission_id, question_no);

-- A published release is forensic evidence.  It must neither be modified nor
-- deleted; a rollback is represented by a new release that snapshots an older
-- version, and score_release_current moves to that new version.
CREATE OR REPLACE FUNCTION reject_published_score_release_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  release_status TEXT;
  target_release UUID;
BEGIN
  IF TG_TABLE_NAME = 'score_release' THEN
    IF TG_OP IN ('UPDATE', 'DELETE') AND OLD.status = 'published' THEN
      RAISE EXCEPTION 'published score release is immutable';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
  END IF;

  target_release := CASE WHEN TG_OP = 'DELETE' THEN OLD.release_id ELSE NEW.release_id END;
  SELECT status INTO release_status FROM score_release WHERE id = target_release;
  IF release_status = 'published' THEN
    RAISE EXCEPTION 'published score release snapshot is immutable';
  END IF;
  RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END
$$;

DROP TRIGGER IF EXISTS trg_score_release_immutable ON score_release;
CREATE TRIGGER trg_score_release_immutable
BEFORE UPDATE OR DELETE ON score_release
FOR EACH ROW EXECUTE FUNCTION reject_published_score_release_mutation();

DROP TRIGGER IF EXISTS trg_score_release_item_immutable ON score_release_item;
CREATE TRIGGER trg_score_release_item_immutable
BEFORE UPDATE OR DELETE ON score_release_item
FOR EACH ROW EXECUTE FUNCTION reject_published_score_release_mutation();

DROP TRIGGER IF EXISTS trg_score_release_question_immutable ON score_release_question;
CREATE TRIGGER trg_score_release_question_immutable
BEFORE UPDATE OR DELETE ON score_release_question
FOR EACH ROW EXECUTE FUNCTION reject_published_score_release_mutation();

CREATE OR REPLACE FUNCTION require_current_score_release_published()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  release_status TEXT;
BEGIN
  SELECT status INTO release_status
  FROM score_release
  WHERE tenant_id = NEW.tenant_id AND id = NEW.release_id AND exam_id = NEW.exam_id;
  IF release_status IS DISTINCT FROM 'published' THEN
    RAISE EXCEPTION 'current score release must be published';
  END IF;
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_score_release_current_published ON score_release_current;
CREATE TRIGGER trg_score_release_current_published
BEFORE INSERT OR UPDATE ON score_release_current
FOR EACH ROW EXECUTE FUNCTION require_current_score_release_published();

DROP TRIGGER IF EXISTS trg_score_release_outbox ON score_release;
CREATE TRIGGER trg_score_release_outbox
AFTER INSERT OR UPDATE ON score_release
FOR EACH ROW EXECUTE FUNCTION enqueue_business_mutation_outbox();

COMMENT ON TABLE score_release IS
'Immutable public score-release versions. A corrected score always creates another version.';
COMMENT ON COLUMN score_release_question.student_explanation IS
'Student-safe materialised explanation only. It must never contain private notes, reviewer identities, model prompts or hidden quality signals.';
