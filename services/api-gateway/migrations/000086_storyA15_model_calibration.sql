-- STORY-A15: version-bound confidence calibration and selective risk coverage.
-- These records are evidence artifacts only. They never contain answer text,
-- images, OCR output, prompts, credentials, or a released score.

CREATE TABLE model_calibration (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  calibration_key TEXT NOT NULL,
  evaluation_run_id UUID NOT NULL,
  model_reference TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  rubric_version TEXT NOT NULL,
  subject_code TEXT NOT NULL,
  archetype_code TEXT NOT NULL,
  slice_key TEXT NOT NULL DEFAULT 'all',
  method TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  calibration_n INTEGER NOT NULL DEFAULT 0,
  artifact_uri TEXT NOT NULL DEFAULT '',
  artifact_sha256 TEXT NOT NULL DEFAULT '',
  artifact_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  approved_at TIMESTAMPTZ,
  approved_by UUID REFERENCES app_user(id),
  invalidated_at TIMESTAMPTZ,
  invalidated_by UUID REFERENCES app_user(id),
  invalidation_reason TEXT NOT NULL DEFAULT '',
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, calibration_key),
  FOREIGN KEY (tenant_id, evaluation_run_id) REFERENCES grading_evaluation_run(tenant_id, id),
  CHECK (calibration_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CHECK (btrim(model_reference) <> '' AND char_length(model_reference) <= 256),
  CHECK (btrim(prompt_version) <> '' AND char_length(prompt_version) <= 128),
  CHECK (btrim(rubric_version) <> '' AND char_length(rubric_version) <= 128),
  CHECK (subject_code ~ '^[a-z0-9][a-z0-9._-]{0,127}$' AND archetype_code ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CHECK (slice_key = 'all' OR slice_key ~ '^score_band:(zero|partial|full)$' OR slice_key ~ '^ocr_quality:(high|medium|low|unknown)$'),
  CHECK (method IN ('auto','isotonic','logistic','conformal')),
  CHECK (status IN ('draft','completed','approved','invalidated')),
  CHECK (calibration_n >= 0),
  CHECK (jsonb_typeof(artifact_json) = 'object'),
  CHECK (
    (status = 'draft' AND calibration_n = 0 AND artifact_uri = '' AND artifact_sha256 = '' AND artifact_json = '{}'::jsonb
      AND completed_at IS NULL AND approved_at IS NULL AND approved_by IS NULL AND invalidated_at IS NULL AND invalidated_by IS NULL AND invalidation_reason = '')
    OR (status = 'completed' AND calibration_n > 0 AND artifact_uri <> '' AND artifact_sha256 ~ '^[a-f0-9]{64}$'
      AND artifact_json <> '{}'::jsonb AND completed_at IS NOT NULL AND approved_at IS NULL AND approved_by IS NULL
      AND invalidated_at IS NULL AND invalidated_by IS NULL AND invalidation_reason = '')
    OR (status = 'approved' AND calibration_n > 0 AND artifact_uri <> '' AND artifact_sha256 ~ '^[a-f0-9]{64}$'
      AND artifact_json <> '{}'::jsonb AND completed_at IS NOT NULL AND approved_at IS NOT NULL AND approved_by IS NOT NULL
      AND invalidated_at IS NULL AND invalidated_by IS NULL AND invalidation_reason = '')
    OR (status = 'invalidated' AND calibration_n > 0 AND artifact_uri <> '' AND artifact_sha256 ~ '^[a-f0-9]{64}$'
      AND artifact_json <> '{}'::jsonb AND completed_at IS NOT NULL AND invalidated_at IS NOT NULL
      AND invalidated_by IS NOT NULL AND btrim(invalidation_reason) <> '')
  )
);

CREATE TABLE model_calibration_evidence (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  calibration_id UUID NOT NULL,
  evaluation_run_id UUID NOT NULL,
  response_key TEXT NOT NULL,
  raw_confidence NUMERIC(7,6) NOT NULL,
  correct BOOLEAN NOT NULL,
  severe_error BOOLEAN NOT NULL,
  score_band TEXT NOT NULL,
  ocr_quality TEXT NOT NULL,
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, calibration_id, response_key),
  FOREIGN KEY (tenant_id, calibration_id) REFERENCES model_calibration(tenant_id, id),
  FOREIGN KEY (tenant_id, evaluation_run_id, response_key) REFERENCES grading_evaluation_observation(tenant_id, run_id, response_key),
  CHECK (response_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CHECK (raw_confidence BETWEEN 0 AND 1),
  CHECK (score_band IN ('zero','partial','full')),
  CHECK (ocr_quality IN ('high','medium','low','unknown'))
);

CREATE TABLE model_score_candidate (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  candidate_key TEXT NOT NULL,
  model_reference TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  rubric_version TEXT NOT NULL,
  subject_code TEXT NOT NULL,
  archetype_code TEXT NOT NULL,
  slice_key TEXT NOT NULL DEFAULT 'all',
  raw_confidence NUMERIC(7,6) NOT NULL,
  calibrated_confidence NUMERIC(7,6),
  calibration_id UUID,
  target_risk NUMERIC(7,6),
  abstain_reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, candidate_key),
  FOREIGN KEY (tenant_id, calibration_id) REFERENCES model_calibration(tenant_id, id),
  CHECK (candidate_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CHECK (btrim(model_reference) <> '' AND char_length(model_reference) <= 256),
  CHECK (btrim(prompt_version) <> '' AND char_length(prompt_version) <= 128),
  CHECK (btrim(rubric_version) <> '' AND char_length(rubric_version) <= 128),
  CHECK (subject_code ~ '^[a-z0-9][a-z0-9._-]{0,127}$' AND archetype_code ~ '^[a-z0-9][a-z0-9._-]{0,127}$'),
  CHECK (slice_key = 'all' OR slice_key ~ '^score_band:(zero|partial|full)$' OR slice_key ~ '^ocr_quality:(high|medium|low|unknown)$'),
  CHECK (raw_confidence BETWEEN 0 AND 1),
  CHECK (calibrated_confidence IS NULL OR calibrated_confidence BETWEEN 0 AND 1),
  CHECK (target_risk IS NULL OR target_risk BETWEEN 0 AND 1),
  CHECK ((calibration_id IS NULL AND calibrated_confidence IS NULL AND abstain_reason <> '')
      OR (calibration_id IS NOT NULL AND calibrated_confidence IS NOT NULL))
);

CREATE INDEX idx_model_calibration_axis ON model_calibration
  (tenant_id, model_reference, prompt_version, rubric_version, subject_code, archetype_code, slice_key, approved_at DESC);
CREATE INDEX idx_model_calibration_evidence ON model_calibration_evidence (tenant_id, calibration_id, observed_at, id);

-- Evidence has to be a genuine completed A16 observation on the exact
-- deployment/rubric axis. Correctness and severe error are re-derived in the
-- database so neither an API caller nor a worker can fabricate a curve.
CREATE OR REPLACE FUNCTION validate_model_calibration_evidence()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
  calibration_status TEXT;
  expected_run UUID;
  expected_model TEXT;
  expected_prompt TEXT;
  expected_rubric TEXT;
  expected_subject TEXT;
  expected_archetype TEXT;
  expected_slice TEXT;
  run_status TEXT;
  run_model TEXT;
  run_prompt TEXT;
  run_rubric TEXT;
  observed_subject TEXT;
  observed_archetype TEXT;
  observed_ocr TEXT;
  observed_band TEXT;
  reference_score DOUBLE PRECISION;
  model_score DOUBLE PRECISION;
  max_score DOUBLE PRECISION;
  expected_correct BOOLEAN;
  expected_severe BOOLEAN;
BEGIN
  SELECT status,evaluation_run_id,model_reference,prompt_version,rubric_version,subject_code,archetype_code,slice_key
    INTO calibration_status,expected_run,expected_model,expected_prompt,expected_rubric,expected_subject,expected_archetype,expected_slice
    FROM model_calibration WHERE tenant_id=NEW.tenant_id AND id=NEW.calibration_id;
  IF NOT FOUND OR calibration_status <> 'draft' OR expected_run <> NEW.evaluation_run_id THEN
    RAISE EXCEPTION 'calibration evidence requires a matching draft artifact' USING ERRCODE='23514';
  END IF;
  SELECT status,model_reference,prompt_version,rubric_version INTO run_status,run_model,run_prompt,run_rubric
    FROM grading_evaluation_run WHERE tenant_id=NEW.tenant_id AND id=NEW.evaluation_run_id;
  IF NOT FOUND OR run_status <> 'completed' OR run_model <> expected_model OR run_prompt <> expected_prompt OR run_rubric <> expected_rubric THEN
    RAISE EXCEPTION 'calibration evidence requires completed matching evaluation provenance' USING ERRCODE='23514';
  END IF;
  SELECT subject,archetype,ocr_quality,reference_score_band,reference_score,model_score,max_score
    INTO observed_subject,observed_archetype,observed_ocr,observed_band,reference_score,model_score,max_score
    FROM grading_evaluation_observation WHERE tenant_id=NEW.tenant_id AND run_id=NEW.evaluation_run_id AND response_key=NEW.response_key;
  IF NOT FOUND OR observed_subject <> expected_subject OR observed_archetype <> expected_archetype
     OR (expected_slice LIKE 'score_band:%' AND expected_slice <> 'score_band:' || observed_band)
     OR (expected_slice LIKE 'ocr_quality:%' AND expected_slice <> 'ocr_quality:' || observed_ocr) THEN
    RAISE EXCEPTION 'calibration evidence response does not match artifact axis' USING ERRCODE='23514';
  END IF;
  expected_correct := abs(model_score-reference_score) < 0.000000001;
  expected_severe := abs(model_score-reference_score) >= greatest(1.0,max_score*.4)-0.000000001;
  IF NEW.correct <> expected_correct OR NEW.severe_error <> expected_severe OR NEW.score_band <> observed_band OR NEW.ocr_quality <> observed_ocr THEN
    RAISE EXCEPTION 'calibration evidence facts must equal aligned evaluation facts' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_model_calibration_evidence_validate
BEFORE INSERT ON model_calibration_evidence
FOR EACH ROW EXECUTE FUNCTION validate_model_calibration_evidence();

CREATE OR REPLACE FUNCTION reject_model_calibration_evidence_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'model calibration evidence is immutable' USING ERRCODE='23514';
END;
$$;
CREATE TRIGGER trg_model_calibration_evidence_immutable
BEFORE UPDATE OR DELETE ON model_calibration_evidence
FOR EACH ROW EXECUTE FUNCTION reject_model_calibration_evidence_mutation();

CREATE OR REPLACE FUNCTION enforce_model_calibration_transition()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.calibration_key IS DISTINCT FROM OLD.calibration_key OR NEW.evaluation_run_id IS DISTINCT FROM OLD.evaluation_run_id
     OR NEW.model_reference IS DISTINCT FROM OLD.model_reference OR NEW.prompt_version IS DISTINCT FROM OLD.prompt_version
     OR NEW.rubric_version IS DISTINCT FROM OLD.rubric_version OR NEW.subject_code IS DISTINCT FROM OLD.subject_code
     OR NEW.archetype_code IS DISTINCT FROM OLD.archetype_code OR NEW.slice_key IS DISTINCT FROM OLD.slice_key
     OR NEW.created_by IS DISTINCT FROM OLD.created_by OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'model calibration provenance is immutable' USING ERRCODE='23514';
  END IF;
  IF OLD.status = 'draft' AND NEW.status <> 'completed' THEN
    RAISE EXCEPTION 'draft calibration can only be completed' USING ERRCODE='23514';
  ELSIF OLD.status = 'completed' AND NEW.status NOT IN ('approved','invalidated') THEN
    RAISE EXCEPTION 'completed calibration can only be approved or invalidated' USING ERRCODE='23514';
  ELSIF OLD.status = 'approved' AND NEW.status <> 'invalidated' THEN
    RAISE EXCEPTION 'approved calibration can only be invalidated' USING ERRCODE='23514';
  ELSIF OLD.status = 'invalidated' THEN
    RAISE EXCEPTION 'invalidated calibration is immutable' USING ERRCODE='23514';
  END IF;
  IF OLD.status <> 'draft' AND (NEW.method IS DISTINCT FROM OLD.method OR NEW.calibration_n IS DISTINCT FROM OLD.calibration_n
     OR NEW.artifact_uri IS DISTINCT FROM OLD.artifact_uri OR NEW.artifact_sha256 IS DISTINCT FROM OLD.artifact_sha256
     OR NEW.artifact_json IS DISTINCT FROM OLD.artifact_json OR NEW.completed_at IS DISTINCT FROM OLD.completed_at) THEN
    RAISE EXCEPTION 'completed calibration artifact is immutable' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_model_calibration_transition
BEFORE UPDATE ON model_calibration
FOR EACH ROW EXECUTE FUNCTION enforce_model_calibration_transition();

CREATE OR REPLACE FUNCTION reject_model_score_candidate_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'model score candidate confidence record is immutable' USING ERRCODE='23514';
END;
$$;
CREATE TRIGGER trg_model_score_candidate_immutable
BEFORE UPDATE OR DELETE ON model_score_candidate
FOR EACH ROW EXECUTE FUNCTION reject_model_score_candidate_mutation();
