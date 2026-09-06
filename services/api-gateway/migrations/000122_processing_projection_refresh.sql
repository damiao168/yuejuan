-- Processing summaries are read models. Source-table changes enqueue a
-- durable, per-exam projection version so API reads never need to write.
CREATE TABLE IF NOT EXISTS processing_projection_cursor (
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  requested_version BIGINT NOT NULL DEFAULT 1,
  projected_version BIGINT NOT NULL DEFAULT 0,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  projected_at TIMESTAMPTZ,
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  lease_owner TEXT,
  lease_expires_at TIMESTAMPTZ,
  attempt_count INT NOT NULL DEFAULT 0,
  last_error TEXT,
  PRIMARY KEY (tenant_id, exam_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  CHECK (requested_version > 0),
  CHECK (projected_version >= 0 AND projected_version <= requested_version),
  CHECK (attempt_count >= 0),
  CHECK ((lease_owner IS NULL) = (lease_expires_at IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_processing_projection_claim
  ON processing_projection_cursor (available_at, requested_at, tenant_id, exam_id)
  WHERE requested_version > projected_version;

CREATE OR REPLACE FUNCTION request_processing_projection_refresh(target_tenant UUID, target_exam UUID)
RETURNS VOID
LANGUAGE SQL
SET search_path = pg_catalog, public
AS $$
  INSERT INTO processing_projection_cursor (
    tenant_id, exam_id, requested_version, projected_version, requested_at, available_at
  )
  SELECT target_tenant, target_exam, 1, 0, clock_timestamp(), clock_timestamp()
  WHERE target_tenant IS NOT NULL AND target_exam IS NOT NULL
  ON CONFLICT (tenant_id, exam_id) DO UPDATE SET
    requested_version = processing_projection_cursor.requested_version + 1,
    requested_at = EXCLUDED.requested_at,
    available_at = LEAST(processing_projection_cursor.available_at, EXCLUDED.available_at),
    last_error = NULL;
$$;

CREATE OR REPLACE FUNCTION request_processing_projection_from_submission()
RETURNS TRIGGER
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
DECLARE
  old_exam UUID;
  new_exam UUID;
BEGIN
  IF TG_OP <> 'INSERT' THEN
    SELECT exam_id INTO old_exam
    FROM submission
    WHERE tenant_id = OLD.tenant_id AND id = OLD.submission_id;
    PERFORM request_processing_projection_refresh(OLD.tenant_id, old_exam);
  END IF;
  IF TG_OP <> 'DELETE' AND (
    TG_OP = 'INSERT'
    OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
    OR NEW.submission_id IS DISTINCT FROM OLD.submission_id
  ) THEN
    SELECT exam_id INTO new_exam
    FROM submission
    WHERE tenant_id = NEW.tenant_id AND id = NEW.submission_id;
    PERFORM request_processing_projection_refresh(NEW.tenant_id, new_exam);
  END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION request_processing_projection_from_exam_submission()
RETURNS TRIGGER
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
BEGIN
  IF TG_OP <> 'INSERT' THEN
    PERFORM request_processing_projection_refresh(OLD.tenant_id, OLD.exam_id);
  END IF;
  IF TG_OP <> 'DELETE' AND (
    TG_OP = 'INSERT'
    OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
    OR NEW.exam_id IS DISTINCT FROM OLD.exam_id
  ) THEN
    PERFORM request_processing_projection_refresh(NEW.tenant_id, NEW.exam_id);
  END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION request_processing_projection_from_page()
RETURNS TRIGGER
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
DECLARE
  old_exam UUID;
  new_exam UUID;
BEGIN
  IF TG_OP <> 'INSERT' THEN
    SELECT s.exam_id INTO old_exam
    FROM submission_page p
    JOIN submission s ON s.tenant_id = p.tenant_id AND s.id = p.submission_id
    WHERE p.tenant_id = OLD.tenant_id AND p.id = OLD.submission_page_id;
    PERFORM request_processing_projection_refresh(OLD.tenant_id, old_exam);
  END IF;
  IF TG_OP <> 'DELETE' AND (
    TG_OP = 'INSERT'
    OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
    OR NEW.submission_page_id IS DISTINCT FROM OLD.submission_page_id
  ) THEN
    SELECT s.exam_id INTO new_exam
    FROM submission_page p
    JOIN submission s ON s.tenant_id = p.tenant_id AND s.id = p.submission_id
    WHERE p.tenant_id = NEW.tenant_id AND p.id = NEW.submission_page_id;
    PERFORM request_processing_projection_refresh(NEW.tenant_id, new_exam);
  END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_processing_projection_submission ON submission;
CREATE TRIGGER trg_processing_projection_submission
AFTER INSERT OR UPDATE OR DELETE ON submission
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_exam_submission();

DROP TRIGGER IF EXISTS trg_processing_projection_submission_page ON submission_page;
CREATE TRIGGER trg_processing_projection_submission_page
AFTER INSERT OR UPDATE OR DELETE ON submission_page
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_submission();

DROP TRIGGER IF EXISTS trg_processing_projection_quality_run ON submission_page_quality_run;
CREATE TRIGGER trg_processing_projection_quality_run
AFTER INSERT OR UPDATE OR DELETE ON submission_page_quality_run
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_submission();

DROP TRIGGER IF EXISTS trg_processing_projection_ocr_task ON ocr_task;
CREATE TRIGGER trg_processing_projection_ocr_task
AFTER INSERT OR UPDATE OR DELETE ON ocr_task
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_submission();

DROP TRIGGER IF EXISTS trg_processing_projection_ocr_result ON ocr_result;
CREATE TRIGGER trg_processing_projection_ocr_result
AFTER INSERT OR UPDATE OR DELETE ON ocr_result
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_submission();

DROP TRIGGER IF EXISTS trg_processing_projection_answer_segment ON answer_segment;
CREATE TRIGGER trg_processing_projection_answer_segment
AFTER INSERT OR UPDATE OR DELETE ON answer_segment
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_submission();

DROP TRIGGER IF EXISTS trg_processing_projection_registration_run ON page_registration_run;
CREATE TRIGGER trg_processing_projection_registration_run
AFTER INSERT OR UPDATE OR DELETE ON page_registration_run
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_page();

-- Existing capture pages can be linked by either submission reference. A
-- dedicated function handles the nullable relationship safely.
CREATE OR REPLACE FUNCTION request_processing_projection_from_capture_page()
RETURNS TRIGGER
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
DECLARE
  target_tenant UUID;
  target_submission UUID;
  target_page UUID;
  target_exam UUID;
BEGIN
  IF TG_OP <> 'INSERT' THEN
    target_tenant := OLD.tenant_id;
    target_submission := OLD.submission_id;
    target_page := OLD.submission_page_id;
    IF target_submission IS NOT NULL THEN
      SELECT exam_id INTO target_exam FROM submission
      WHERE tenant_id = target_tenant AND id = target_submission;
    ELSIF target_page IS NOT NULL THEN
      SELECT s.exam_id INTO target_exam
      FROM submission_page p
      JOIN submission s ON s.tenant_id = p.tenant_id AND s.id = p.submission_id
      WHERE p.tenant_id = target_tenant AND p.id = target_page;
    END IF;
    PERFORM request_processing_projection_refresh(target_tenant, target_exam);
  END IF;
  IF TG_OP <> 'DELETE' AND (
    TG_OP = 'INSERT'
    OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
    OR NEW.submission_id IS DISTINCT FROM OLD.submission_id
    OR NEW.submission_page_id IS DISTINCT FROM OLD.submission_page_id
  ) THEN
    target_tenant := NEW.tenant_id;
    target_submission := NEW.submission_id;
    target_page := NEW.submission_page_id;
    target_exam := NULL;
    IF target_submission IS NOT NULL THEN
      SELECT exam_id INTO target_exam FROM submission
      WHERE tenant_id = target_tenant AND id = target_submission;
    ELSIF target_page IS NOT NULL THEN
      SELECT s.exam_id INTO target_exam
      FROM submission_page p
      JOIN submission s ON s.tenant_id = p.tenant_id AND s.id = p.submission_id
      WHERE p.tenant_id = target_tenant AND p.id = target_page;
    END IF;
    PERFORM request_processing_projection_refresh(target_tenant, target_exam);
  END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_processing_projection_capture_page ON capture_page;
CREATE TRIGGER trg_processing_projection_capture_page
AFTER INSERT OR UPDATE OR DELETE ON capture_page
FOR EACH ROW EXECUTE FUNCTION request_processing_projection_from_capture_page();

-- Queue a one-time refresh for pre-existing exams. Subsequent changes are
-- versioned by the triggers above.
INSERT INTO processing_projection_cursor (tenant_id, exam_id)
SELECT DISTINCT tenant_id, exam_id
FROM submission
WHERE deleted_at IS NULL
ON CONFLICT (tenant_id, exam_id) DO NOTHING;
