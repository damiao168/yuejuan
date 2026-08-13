CREATE TABLE grader_calibration_policy (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  archetype_code TEXT NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  minimum_samples INT NOT NULL,
  maximum_mae NUMERIC(8,4) NOT NULL,
  minimum_exact_agreement NUMERIC(7,6) NOT NULL,
  minimum_within_one_agreement NUMERIC(7,6) NOT NULL,
  minimum_criterion_agreement NUMERIC(7,6),
  maximum_severe_rate NUMERIC(7,6) NOT NULL,
  severe_error_threshold NUMERIC(8,2) NOT NULL,
  qualification_validity_days INT NOT NULL,
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, exam_id, question_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  CHECK (length(btrim(archetype_code)) > 0),
  CHECK (max_score > 0),
  CHECK (minimum_samples > 0),
  CHECK (maximum_mae >= 0),
  CHECK (minimum_exact_agreement BETWEEN 0 AND 1),
  CHECK (minimum_within_one_agreement BETWEEN 0 AND 1),
  CHECK (minimum_criterion_agreement IS NULL OR minimum_criterion_agreement BETWEEN 0 AND 1),
  CHECK (maximum_severe_rate BETWEEN 0 AND 1),
  CHECK (severe_error_threshold > 0 AND severe_error_threshold <= max_score),
  CHECK (qualification_validity_days BETWEEN 1 AND 3650),
  CHECK (revision > 0)
);

CREATE TABLE grader_calibration_session (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  grader_id UUID NOT NULL,
  exam_question_snapshot_id UUID NOT NULL,
  gold_version TEXT NOT NULL,
  policy_snapshot_json JSONB NOT NULL,
  sample_manifest_json JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'in_progress',
  metrics_json JSONB,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  invalidated_at TIMESTAMPTZ,
  invalidation_reason TEXT NOT NULL DEFAULT '',
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, grader_id) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, exam_question_snapshot_id) REFERENCES exam_question_snapshot(tenant_id, id),
  CHECK (length(gold_version) = 64),
  CHECK (jsonb_typeof(policy_snapshot_json) = 'object'),
  CHECK (jsonb_typeof(sample_manifest_json) = 'array' AND jsonb_array_length(sample_manifest_json) > 0),
  CHECK (metrics_json IS NULL OR jsonb_typeof(metrics_json) = 'object'),
  CHECK (status IN ('in_progress','passed','failed','invalidated')),
  CHECK ((status = 'in_progress' AND completed_at IS NULL AND invalidated_at IS NULL) OR status <> 'in_progress'),
  CHECK ((status IN ('passed','failed') AND completed_at IS NOT NULL AND metrics_json IS NOT NULL) OR status NOT IN ('passed','failed')),
  CHECK ((status = 'invalidated' AND invalidated_at IS NOT NULL AND length(btrim(invalidation_reason)) > 0) OR status <> 'invalidated')
);

CREATE UNIQUE INDEX uq_grader_calibration_in_progress
ON grader_calibration_session (tenant_id, exam_id, question_id, grader_id)
WHERE status = 'in_progress';

CREATE INDEX idx_grader_calibration_session_grader
ON grader_calibration_session (tenant_id, grader_id, started_at DESC);

CREATE TABLE grader_calibration_attempt (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  session_id UUID NOT NULL,
  gold_paper_id UUID NOT NULL,
  gold_version INT NOT NULL,
  submitted_score NUMERIC(8,2) NOT NULL,
  reference_score NUMERIC(8,2) NOT NULL,
  rubric_selection_json JSONB NOT NULL DEFAULT '{}',
  criterion_differences_json JSONB NOT NULL DEFAULT '[]',
  criterion_correct INT NOT NULL DEFAULT 0,
  criterion_count INT NOT NULL DEFAULT 0,
  exact_match BOOLEAN NOT NULL,
  within_one BOOLEAN NOT NULL,
  absolute_error NUMERIC(8,2) NOT NULL,
  severe_disagreement BOOLEAN NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, session_id, gold_paper_id),
  FOREIGN KEY (tenant_id, session_id) REFERENCES grader_calibration_session(tenant_id, id),
  FOREIGN KEY (tenant_id, gold_paper_id, gold_version) REFERENCES grading_gold_paper_version(tenant_id, gold_paper_id, version),
  CHECK (gold_version > 0),
  CHECK (submitted_score >= 0 AND reference_score >= 0 AND absolute_error >= 0),
  CHECK (criterion_correct >= 0 AND criterion_count >= criterion_correct),
  CHECK (jsonb_typeof(rubric_selection_json) = 'object'),
  CHECK (jsonb_typeof(criterion_differences_json) = 'array')
);

CREATE INDEX idx_grader_calibration_attempt_session
ON grader_calibration_attempt (tenant_id, session_id, created_at, id);

CREATE TABLE grader_question_qualification (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  grader_id UUID NOT NULL,
  status TEXT NOT NULL,
  valid_until TIMESTAMPTZ NOT NULL,
  calibration_session_id UUID NOT NULL,
  gold_version TEXT NOT NULL,
  metric_snapshot_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, calibration_session_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, grader_id) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, calibration_session_id) REFERENCES grader_calibration_session(tenant_id, id),
  CHECK (status IN ('qualified','expired','revoked')),
  CHECK (length(gold_version) = 64),
  CHECK (jsonb_typeof(metric_snapshot_json) = 'object')
);

CREATE INDEX idx_grader_question_qualification_lookup
ON grader_question_qualification (tenant_id, exam_id, question_id, grader_id, created_at DESC);

-- A newly approved or retired Gold version changes the calibration contract.
-- Revoke matching qualifications immediately; the service also compares the
-- current fingerprint to protect deployments where changes arrive indirectly.
CREATE OR REPLACE FUNCTION revoke_qualification_on_gold_set_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.active_version IS DISTINCT FROM NEW.active_version OR OLD.status IS DISTINCT FROM NEW.status THEN
    UPDATE grader_question_qualification
       SET status = 'revoked', updated_at = now()
     WHERE tenant_id = NEW.tenant_id
       AND exam_id = NEW.exam_id
       AND question_id = NEW.question_id
       AND status = 'qualified';
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER trg_revoke_qualification_on_gold_set_change
AFTER UPDATE OF active_version, status ON grading_gold_paper
FOR EACH ROW EXECUTE FUNCTION revoke_qualification_on_gold_set_change();
