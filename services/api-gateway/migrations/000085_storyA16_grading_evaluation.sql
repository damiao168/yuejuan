-- STORY-A16: empirical offline scoring evaluation. These tables hold opaque
-- aligned score observations and derived metrics only; no answer content,
-- images, OCR text, model credentials, or production decisions are stored.

CREATE TABLE grading_evaluation_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  run_key TEXT NOT NULL,
  display_name TEXT NOT NULL,
  model_reference TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  rubric_version TEXT NOT NULL,
  dataset_reference TEXT NOT NULL,
  dataset_sha256 TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  invalidated_at TIMESTAMPTZ,
  invalidation_reason TEXT NOT NULL DEFAULT '',
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, run_key),
  CONSTRAINT chk_grading_evaluation_run_key CHECK (run_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CONSTRAINT chk_grading_evaluation_run_text CHECK (
    btrim(display_name) <> '' AND char_length(display_name) <= 128
    AND btrim(model_reference) <> '' AND char_length(model_reference) <= 256
    AND btrim(prompt_version) <> '' AND char_length(prompt_version) <= 128
    AND btrim(rubric_version) <> '' AND char_length(rubric_version) <= 128
    AND dataset_reference ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
    AND dataset_sha256 ~ '^[a-f0-9]{64}$'
  ),
  CONSTRAINT chk_grading_evaluation_run_state CHECK (
    (status = 'draft' AND completed_at IS NULL AND invalidated_at IS NULL AND invalidation_reason = '')
    OR (status = 'completed' AND completed_at IS NOT NULL AND invalidated_at IS NULL AND invalidation_reason = '')
    OR (status = 'invalidated' AND invalidated_at IS NOT NULL AND btrim(invalidation_reason) <> '')
  )
);

CREATE TABLE grading_evaluation_observation (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  run_id UUID NOT NULL,
  response_key TEXT NOT NULL,
  response_fingerprint TEXT NOT NULL,
  reference_kind TEXT NOT NULL,
  subject TEXT NOT NULL,
  archetype TEXT NOT NULL,
  ocr_quality TEXT NOT NULL,
  answer_length TEXT NOT NULL,
  rubric_complexity TEXT NOT NULL,
  reference_score DOUBLE PRECISION NOT NULL,
  model_score DOUBLE PRECISION NOT NULL,
  max_score DOUBLE PRECISION NOT NULL,
  reference_score_band TEXT NOT NULL,
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, run_id, response_key),
  FOREIGN KEY (tenant_id, run_id) REFERENCES grading_evaluation_run(tenant_id, id),
  CONSTRAINT chk_grading_evaluation_observation_key CHECK (
    response_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
    AND response_fingerprint ~ '^[a-f0-9]{64}$'
    AND subject ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
    AND archetype ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
  ),
  CONSTRAINT chk_grading_evaluation_observation_labels CHECK (
    reference_kind IN ('gold','human_adjudicated')
    AND ocr_quality IN ('high','medium','low','unknown')
    AND answer_length IN ('short','medium','long','unknown')
    AND rubric_complexity IN ('low','medium','high','unknown')
    AND reference_score_band IN ('zero','partial','full')
  ),
  CONSTRAINT chk_grading_evaluation_observation_scores CHECK (
    max_score > 0 AND reference_score >= 0 AND model_score >= 0
    AND reference_score <= max_score AND model_score <= max_score
  )
);

CREATE TABLE grading_evaluation_slice_metric (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  run_id UUID NOT NULL,
  dimension TEXT NOT NULL,
  slice_value TEXT NOT NULL,
  sample_count INT NOT NULL,
  mae DOUBLE PRECISION NOT NULL,
  exact_rate DOUBLE PRECISION NOT NULL,
  within_one_rate DOUBLE PRECISION NOT NULL,
  severe_error_rate DOUBLE PRECISION NOT NULL,
  false_zero_rate DOUBLE PRECISION NOT NULL,
  false_full_rate DOUBLE PRECISION NOT NULL,
  qwk DOUBLE PRECISION,
  qwk_unavailable_reason TEXT NOT NULL DEFAULT '',
  computed_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, run_id, dimension, slice_value),
  FOREIGN KEY (tenant_id, run_id) REFERENCES grading_evaluation_run(tenant_id, id),
  CONSTRAINT chk_grading_evaluation_slice_dimension CHECK (
    dimension IN ('subject','archetype','score_band','ocr_quality','answer_length','rubric_complexity')
  ),
  CONSTRAINT chk_grading_evaluation_slice_values CHECK (slice_value <> '' AND char_length(slice_value) <= 128),
  CONSTRAINT chk_grading_evaluation_slice_count CHECK (sample_count > 0),
  CONSTRAINT chk_grading_evaluation_slice_rates CHECK (
    mae >= 0 AND exact_rate BETWEEN 0 AND 1 AND within_one_rate BETWEEN 0 AND 1
    AND severe_error_rate BETWEEN 0 AND 1 AND false_zero_rate BETWEEN 0 AND 1 AND false_full_rate BETWEEN 0 AND 1
    AND (qwk IS NULL OR qwk BETWEEN -1 AND 1)
  )
);

CREATE TABLE grading_evaluation_response_difficulty (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  run_id UUID NOT NULL,
  response_key TEXT NOT NULL,
  response_fingerprint TEXT NOT NULL,
  difficulty_score DOUBLE PRECISION NOT NULL,
  difficulty_band TEXT NOT NULL,
  normalized_error DOUBLE PRECISION NOT NULL,
  severe_error BOOLEAN NOT NULL,
  ocr_quality TEXT NOT NULL,
  answer_length TEXT NOT NULL,
  rubric_complexity TEXT NOT NULL,
  evidence_note TEXT NOT NULL,
  computed_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, run_id, response_key),
  FOREIGN KEY (tenant_id, run_id) REFERENCES grading_evaluation_run(tenant_id, id),
  CONSTRAINT chk_grading_evaluation_difficulty CHECK (
    difficulty_score BETWEEN 0 AND 1 AND normalized_error BETWEEN 0 AND 1
    AND difficulty_band IN ('low','medium','high')
    AND ocr_quality IN ('high','medium','low','unknown')
    AND answer_length IN ('short','medium','long','unknown')
    AND rubric_complexity IN ('low','medium','high','unknown')
    AND evidence_note = 'offline_aligned_score_difference'
  )
);

CREATE INDEX idx_grading_evaluation_run_history ON grading_evaluation_run (tenant_id, created_at DESC);
CREATE INDEX idx_grading_evaluation_observation_run ON grading_evaluation_observation (tenant_id, run_id, observed_at, id);
CREATE INDEX idx_grading_evaluation_slice_metric_run ON grading_evaluation_slice_metric (tenant_id, run_id, dimension, slice_value);
CREATE INDEX idx_grading_evaluation_difficulty_run ON grading_evaluation_response_difficulty (tenant_id, run_id, difficulty_score DESC);

-- Raw observations cannot change once stored. A model run is mutable only by
-- adding genuine aligned observations while draft, then it is completed or
-- invalidated. This prevents retrospective metric fabrication.
CREATE OR REPLACE FUNCTION enforce_grading_evaluation_observation_write()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE run_status TEXT;
BEGIN
  SELECT status INTO run_status FROM grading_evaluation_run WHERE tenant_id = NEW.tenant_id AND id = NEW.run_id;
  IF NOT FOUND OR run_status <> 'draft' THEN
    RAISE EXCEPTION 'evaluation observations may only be added to a draft run' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_grading_evaluation_observation_write
BEFORE INSERT ON grading_evaluation_observation
FOR EACH ROW EXECUTE FUNCTION enforce_grading_evaluation_observation_write();

CREATE OR REPLACE FUNCTION reject_grading_evaluation_observation_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'evaluation observations are immutable' USING ERRCODE = '23514';
END;
$$;
CREATE TRIGGER trg_grading_evaluation_observation_immutable
BEFORE UPDATE OR DELETE ON grading_evaluation_observation
FOR EACH ROW EXECUTE FUNCTION reject_grading_evaluation_observation_mutation();

CREATE OR REPLACE FUNCTION enforce_grading_evaluation_run_transition()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.run_key IS DISTINCT FROM OLD.run_key OR NEW.display_name IS DISTINCT FROM OLD.display_name
     OR NEW.model_reference IS DISTINCT FROM OLD.model_reference OR NEW.prompt_version IS DISTINCT FROM OLD.prompt_version
     OR NEW.rubric_version IS DISTINCT FROM OLD.rubric_version OR NEW.dataset_reference IS DISTINCT FROM OLD.dataset_reference
     OR NEW.dataset_sha256 IS DISTINCT FROM OLD.dataset_sha256 OR NEW.created_by IS DISTINCT FROM OLD.created_by
     OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'evaluation run provenance is immutable' USING ERRCODE = '23514';
  END IF;
  IF OLD.status = 'invalidated' OR NEW.status = OLD.status
     OR (OLD.status = 'draft' AND NEW.status NOT IN ('completed','invalidated'))
     OR (OLD.status = 'completed' AND NEW.status <> 'invalidated') THEN
    RAISE EXCEPTION 'invalid evaluation run state transition' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_grading_evaluation_run_transition
BEFORE UPDATE ON grading_evaluation_run
FOR EACH ROW EXECUTE FUNCTION enforce_grading_evaluation_run_transition();

CREATE OR REPLACE FUNCTION enforce_grading_evaluation_computed_write()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE run_status TEXT;
BEGIN
  IF TG_OP = 'DELETE' THEN
    SELECT status INTO run_status FROM grading_evaluation_run WHERE tenant_id = OLD.tenant_id AND id = OLD.run_id;
  ELSE
    SELECT status INTO run_status FROM grading_evaluation_run WHERE tenant_id = NEW.tenant_id AND id = NEW.run_id;
  END IF;
  IF NOT FOUND OR run_status <> 'draft' THEN
    RAISE EXCEPTION 'derived metrics may only be written while completing a draft run' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_grading_evaluation_slice_metric_write
BEFORE INSERT OR UPDATE OR DELETE ON grading_evaluation_slice_metric
FOR EACH ROW EXECUTE FUNCTION enforce_grading_evaluation_computed_write();
CREATE TRIGGER trg_grading_evaluation_difficulty_write
BEFORE INSERT OR UPDATE OR DELETE ON grading_evaluation_response_difficulty
FOR EACH ROW EXECUTE FUNCTION enforce_grading_evaluation_computed_write();
