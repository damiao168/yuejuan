-- STORY-A22: students appeal an exact, already-published Score Release fact.
-- The workflow deliberately has no adjusted_score column: an upheld appeal
-- must hand off to A19 Regrade and then name a later immutable release.

CREATE TABLE IF NOT EXISTS question_appeal (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  student_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  source_release_id UUID NOT NULL,
  source_release_version INT NOT NULL,
  question_id UUID NOT NULL,
  question_no TEXT NOT NULL,
  source_final_grade_id UUID NOT NULL,
  source_score NUMERIC(8,2) NOT NULL,
  source_max_score NUMERIC(8,2) NOT NULL,
  reason_code TEXT NOT NULL,
  reason TEXT NOT NULL,
  selected_region JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'submitted',
  assigned_to UUID,
  decision TEXT,
  public_response TEXT NOT NULL DEFAULT '',
  private_note TEXT NOT NULL DEFAULT '',
  regrade_job_id UUID,
  new_release_id UUID,
  created_by UUID NOT NULL,
  decided_by UUID,
  decided_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, student_id) REFERENCES student(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, source_release_id) REFERENCES score_release(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, source_final_grade_id) REFERENCES final_grade(tenant_id, id),
  FOREIGN KEY (tenant_id, assigned_to) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, regrade_job_id) REFERENCES regrade_job(tenant_id, id),
  FOREIGN KEY (tenant_id, new_release_id) REFERENCES score_release(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, decided_by) REFERENCES app_user(tenant_id, id),
  CHECK (source_release_version > 0),
  CHECK (source_score >= 0 AND source_score <= source_max_score),
  CHECK (source_max_score >= 0),
  CHECK (reason_code IN ('recognition_error','missing_step_credit','rubric_disagreement','calculation_error','annotation_issue','other')),
  CHECK (length(btrim(reason)) BETWEEN 1 AND 2000),
  CHECK (jsonb_typeof(selected_region) = 'object'),
  CHECK (status IN ('submitted','under_review','rejected','upheld_pending_regrade','resolved')),
  CHECK (decision IS NULL OR decision IN ('reject','refer_regrade')),
  CHECK (length(public_response) <= 2000 AND length(private_note) <= 4000),
  CHECK (revision > 0),
  CHECK (
    (decision IS NULL AND regrade_job_id IS NULL)
    OR (decision = 'reject' AND regrade_job_id IS NULL)
    OR (decision = 'refer_regrade' AND regrade_job_id IS NOT NULL)
  ),
  CHECK ((new_release_id IS NULL AND status <> 'resolved') OR (new_release_id IS NOT NULL AND status = 'resolved')),
  CHECK ((decided_at IS NULL AND decided_by IS NULL) OR (decided_at IS NOT NULL AND decided_by IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS question_appeal_event (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  appeal_id UUID NOT NULL,
  event_type TEXT NOT NULL,
  actor_id UUID NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, appeal_id) REFERENCES question_appeal(tenant_id, id),
  FOREIGN KEY (tenant_id, actor_id) REFERENCES app_user(tenant_id, id),
  CHECK (event_type IN ('submitted','review_started','decided','resolved')),
  CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_question_appeal_student_status
ON question_appeal (tenant_id, student_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_question_appeal_exam_status
ON question_appeal (tenant_id, exam_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_question_appeal_event_appeal
ON question_appeal_event (tenant_id, appeal_id, created_at, id);

-- Source facts are forensic context. They must remain exactly what the student
-- saw in the published release, even after a later release becomes current.
CREATE OR REPLACE FUNCTION enforce_question_appeal_source_immutable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  valid_resolution BOOLEAN;
  valid_regrade BOOLEAN;
BEGIN
  IF NEW.tenant_id <> OLD.tenant_id OR NEW.exam_id <> OLD.exam_id
    OR NEW.student_id <> OLD.student_id OR NEW.submission_id <> OLD.submission_id
    OR NEW.source_release_id <> OLD.source_release_id OR NEW.source_release_version <> OLD.source_release_version
    OR NEW.question_id <> OLD.question_id OR NEW.question_no <> OLD.question_no
    OR NEW.source_final_grade_id <> OLD.source_final_grade_id
    OR NEW.source_score <> OLD.source_score OR NEW.source_max_score <> OLD.source_max_score
    OR NEW.reason_code <> OLD.reason_code OR NEW.reason <> OLD.reason
    OR NEW.selected_region <> OLD.selected_region OR NEW.created_by <> OLD.created_by
    OR NEW.created_at <> OLD.created_at THEN
    RAISE EXCEPTION 'question appeal source facts are immutable';
  END IF;
  IF NEW.new_release_id IS NOT NULL THEN
    SELECT EXISTS (
      SELECT 1 FROM score_release successor
      JOIN score_release_item successor_item
        ON successor_item.tenant_id = successor.tenant_id AND successor_item.release_id = successor.id
      JOIN score_release_question successor_question
        ON successor_question.tenant_id = successor.tenant_id
       AND successor_question.release_id = successor.id
       AND successor_question.submission_id = successor_item.submission_id
      WHERE successor.tenant_id = NEW.tenant_id AND successor.id = NEW.new_release_id
        AND successor.exam_id = NEW.exam_id AND successor.status = 'published'
        AND successor.version > NEW.source_release_version
        AND successor_item.student_id = NEW.student_id
        AND successor_item.submission_id = NEW.submission_id
        AND successor_question.question_id = NEW.question_id
    ) INTO valid_resolution;
    IF NOT valid_resolution THEN
      RAISE EXCEPTION 'question appeal resolution must reference a published successor release';
    END IF;
  END IF;
  IF NEW.regrade_job_id IS NOT NULL THEN
    SELECT EXISTS (
      SELECT 1 FROM regrade_job
      WHERE tenant_id = NEW.tenant_id AND id = NEW.regrade_job_id
        AND exam_id = NEW.exam_id AND question_id = NEW.question_id
        AND source_release_id = NEW.source_release_id
    ) INTO valid_regrade;
    IF NOT valid_regrade THEN
      RAISE EXCEPTION 'question appeal must reference a matching regrade job';
    END IF;
  END IF;
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_question_appeal_source_immutable ON question_appeal;
CREATE TRIGGER trg_question_appeal_source_immutable
BEFORE UPDATE ON question_appeal
FOR EACH ROW EXECUTE FUNCTION enforce_question_appeal_source_immutable();

-- A malformed direct SQL write must not create an appeal against a draft,
-- unrelated, or mutated current-score record. The source is validated against
-- immutable release rows at insertion time; normal service writes repeat the
-- same check in their transaction for a useful domain error.
CREATE OR REPLACE FUNCTION require_published_question_appeal_source()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  valid_source BOOLEAN;
BEGIN
  SELECT EXISTS (
    SELECT 1
    FROM score_release release
    JOIN score_release_item item
      ON item.tenant_id = release.tenant_id
     AND item.release_id = release.id
     AND item.student_id = NEW.student_id
     AND item.submission_id = NEW.submission_id
    JOIN score_release_question question_fact
      ON question_fact.tenant_id = release.tenant_id
     AND question_fact.release_id = release.id
     AND question_fact.submission_id = item.submission_id
     AND question_fact.question_id = NEW.question_id
     AND question_fact.final_grade_id = NEW.source_final_grade_id
     AND question_fact.score = NEW.source_score
     AND question_fact.max_score = NEW.source_max_score
     AND question_fact.question_no = NEW.question_no
    WHERE release.tenant_id = NEW.tenant_id
      AND release.id = NEW.source_release_id
      AND release.exam_id = NEW.exam_id
      AND release.version = NEW.source_release_version
      AND release.status = 'published'
  ) INTO valid_source;
  IF NOT valid_source THEN
    RAISE EXCEPTION 'question appeal must reference a published release fact';
  END IF;
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_question_appeal_published_source ON question_appeal;
CREATE TRIGGER trg_question_appeal_published_source
BEFORE INSERT ON question_appeal
FOR EACH ROW EXECUTE FUNCTION require_published_question_appeal_source();

DROP TRIGGER IF EXISTS trg_question_appeal_outbox ON question_appeal;
CREATE TRIGGER trg_question_appeal_outbox
AFTER INSERT OR UPDATE ON question_appeal
FOR EACH ROW EXECUTE FUNCTION enqueue_business_mutation_outbox();

COMMENT ON TABLE question_appeal IS
'Student question-level appeal frozen to a published score release. Upholding creates a governed regrade and successor release; it never updates a score in place.';
