ALTER TABLE exam DROP CONSTRAINT IF EXISTS exam_status_check;
ALTER TABLE exam ADD CONSTRAINT exam_status_check CHECK (
  status IN ('draft', 'configured', 'ready', 'collecting', 'grading', 'reviewing', 'finalized', 'published', 'archived')
);

CREATE TABLE IF NOT EXISTS answer_sheet_template (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  exam_paper_id UUID NOT NULL,
  version_no INT NOT NULL,
  revision INT NOT NULL DEFAULT 1,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  page_count INT NOT NULL,
  layout JSONB NOT NULL,
  content_hash TEXT NOT NULL,
  created_by UUID NOT NULL,
  locked_by UUID,
  locked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT fk_answer_sheet_template_exam_tenant FOREIGN KEY (tenant_id, exam_id)
    REFERENCES exam (tenant_id, id),
  CONSTRAINT fk_answer_sheet_template_paper_tenant FOREIGN KEY (tenant_id, exam_paper_id)
    REFERENCES exam_paper (tenant_id, id),
  CONSTRAINT fk_answer_sheet_template_created_by_tenant FOREIGN KEY (tenant_id, created_by)
    REFERENCES app_user (tenant_id, id),
  CONSTRAINT fk_answer_sheet_template_locked_by_tenant FOREIGN KEY (tenant_id, locked_by)
    REFERENCES app_user (tenant_id, id),
  UNIQUE (tenant_id, exam_id, version_no),
  UNIQUE (tenant_id, id),
  CHECK (status IN ('draft', 'locked', 'retired')),
  CHECK (page_count BETWEEN 1 AND 100),
  CHECK (revision > 0),
  CHECK (octet_length(layout::text) <= 1048576)
);

CREATE INDEX IF NOT EXISTS idx_answer_sheet_template_exam
  ON answer_sheet_template (tenant_id, exam_id, version_no DESC)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS exam_readiness_snapshot (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  configuration_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  checks JSONB NOT NULL,
  confirmed_by UUID NOT NULL,
  confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  invalidated_at TIMESTAMPTZ,
  invalidation_reason TEXT,
  CONSTRAINT fk_exam_readiness_exam_tenant FOREIGN KEY (tenant_id, exam_id)
    REFERENCES exam (tenant_id, id),
  CONSTRAINT fk_exam_readiness_confirmed_by_tenant FOREIGN KEY (tenant_id, confirmed_by)
    REFERENCES app_user (tenant_id, id),
  CHECK (status IN ('passed', 'invalidated'))
);

CREATE INDEX IF NOT EXISTS idx_exam_readiness_current
  ON exam_readiness_snapshot (tenant_id, exam_id, confirmed_at DESC);

CREATE OR REPLACE FUNCTION invalidate_exam_readiness_for(target_tenant UUID, target_exam UUID, reason TEXT)
RETURNS VOID AS $$
BEGIN
  UPDATE exam_readiness_snapshot
  SET status = 'invalidated', invalidated_at = now(), invalidation_reason = reason
  WHERE tenant_id = target_tenant AND exam_id = target_exam AND status = 'passed';

  UPDATE exam
  SET status = 'configured', updated_at = now()
  WHERE tenant_id = target_tenant AND id = target_exam AND status = 'ready';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION invalidate_exam_readiness_from_config()
RETURNS TRIGGER AS $$
DECLARE
  target_tenant UUID;
  target_exam UUID;
BEGIN
  target_tenant := COALESCE(NEW.tenant_id, OLD.tenant_id);

  IF TG_TABLE_NAME = 'exam' THEN
    target_exam := COALESCE(NEW.id, OLD.id);
  ELSIF TG_TABLE_NAME IN ('exam_paper', 'question', 'answer_sheet_template', 'exam_class') THEN
    target_exam := COALESCE(NEW.exam_id, OLD.exam_id);
  ELSIF TG_TABLE_NAME IN ('question_answer_key', 'rubric_version', 'question_rubric') THEN
    SELECT q.exam_id INTO target_exam
    FROM question q
    WHERE q.tenant_id = target_tenant
      AND q.id = COALESCE(NEW.question_id, OLD.question_id);
  END IF;

  IF target_exam IS NOT NULL THEN
    PERFORM invalidate_exam_readiness_for(target_tenant, target_exam, TG_TABLE_NAME || ' changed');
  END IF;
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION invalidate_exam_readiness_from_student()
RETURNS TRIGGER AS $$
DECLARE
  target_tenant UUID;
  target_class UUID;
  linked_exam UUID;
BEGIN
  target_tenant := COALESCE(NEW.tenant_id, OLD.tenant_id);
  target_class := COALESCE(NEW.class_id, OLD.class_id);
  FOR linked_exam IN
    SELECT ec.exam_id
    FROM exam_class ec
    WHERE ec.tenant_id = target_tenant AND ec.class_id = target_class AND ec.deleted_at IS NULL
  LOOP
    PERFORM invalidate_exam_readiness_for(target_tenant, linked_exam, 'student scope changed');
  END LOOP;
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_exam_invalidate_readiness ON exam;
CREATE TRIGGER trg_exam_invalidate_readiness
AFTER UPDATE OF school_id, total_score, grading_mode ON exam
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();

DROP TRIGGER IF EXISTS trg_exam_class_invalidate_readiness ON exam_class;
CREATE TRIGGER trg_exam_class_invalidate_readiness
AFTER INSERT OR UPDATE OR DELETE ON exam_class
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();

DROP TRIGGER IF EXISTS trg_student_invalidate_readiness ON student;
CREATE TRIGGER trg_student_invalidate_readiness
AFTER INSERT OR DELETE OR UPDATE OF class_id, status, deleted_at ON student
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_student();

DROP TRIGGER IF EXISTS trg_exam_paper_invalidate_readiness ON exam_paper;
CREATE TRIGGER trg_exam_paper_invalidate_readiness
AFTER INSERT OR UPDATE OR DELETE ON exam_paper
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();

DROP TRIGGER IF EXISTS trg_question_invalidate_readiness ON question;
CREATE TRIGGER trg_question_invalidate_readiness
AFTER INSERT OR UPDATE OR DELETE ON question
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();

DROP TRIGGER IF EXISTS trg_answer_key_invalidate_readiness ON question_answer_key;
CREATE TRIGGER trg_answer_key_invalidate_readiness
AFTER INSERT OR UPDATE OR DELETE ON question_answer_key
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();

DROP TRIGGER IF EXISTS trg_rubric_version_invalidate_readiness ON rubric_version;
CREATE TRIGGER trg_rubric_version_invalidate_readiness
AFTER INSERT OR UPDATE OR DELETE ON rubric_version
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();

DROP TRIGGER IF EXISTS trg_question_rubric_invalidate_readiness ON question_rubric;
CREATE TRIGGER trg_question_rubric_invalidate_readiness
AFTER INSERT OR UPDATE OR DELETE ON question_rubric
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();

DROP TRIGGER IF EXISTS trg_answer_template_invalidate_readiness ON answer_sheet_template;
CREATE TRIGGER trg_answer_template_invalidate_readiness
AFTER INSERT OR UPDATE OR DELETE ON answer_sheet_template
FOR EACH ROW EXECUTE FUNCTION invalidate_exam_readiness_from_config();
