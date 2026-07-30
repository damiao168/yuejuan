-- STORY-061C1: immutable offline evaluation evidence. Protocol fixtures are
-- explicitly distinguished from authorized frozen-set quality evidence.

CREATE TABLE model_evaluation_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  run_key TEXT NOT NULL,
  display_name TEXT NOT NULL,
  dataset_reference TEXT NOT NULL,
  dataset_sha256 TEXT NOT NULL,
  authorization_reference TEXT NOT NULL DEFAULT '',
  evidence_class TEXT NOT NULL,
  subject TEXT NOT NULL,
  grade TEXT NOT NULL,
  question_type TEXT NOT NULL,
  modality TEXT NOT NULL,
  sample_count INT NOT NULL,
  repeat_count INT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_by UUID,
  completed_at TIMESTAMPTZ,
  invalidated_by UUID,
  invalidated_at TIMESTAMPTZ,
  invalidation_reason TEXT NOT NULL DEFAULT '',
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, run_key),
  CONSTRAINT fk_model_evaluation_run_creator
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT fk_model_evaluation_run_completer
    FOREIGN KEY (tenant_id, completed_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT fk_model_evaluation_run_invalidator
    FOREIGN KEY (tenant_id, invalidated_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT chk_model_evaluation_run_keys
    CHECK (
      run_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND dataset_reference ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND question_type ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND dataset_sha256 ~ '^[a-f0-9]{64}$'
    ),
  CONSTRAINT chk_model_evaluation_run_text
    CHECK (
      btrim(display_name) <> ''
      AND btrim(subject) <> ''
      AND btrim(grade) <> ''
      AND char_length(display_name) <= 128
      AND char_length(subject) <= 128
      AND char_length(grade) <= 128
    ),
  CONSTRAINT chk_model_evaluation_run_evidence
    CHECK (
      (evidence_class = 'protocol_fixture' AND authorization_reference = '')
      OR
      (evidence_class = 'authorized_frozen_set'
        AND authorization_reference ~ '^[a-z0-9][a-z0-9._-]{0,127}$')
    ),
  CONSTRAINT chk_model_evaluation_run_modality
    CHECK (modality IN ('text', 'image')),
  CONSTRAINT chk_model_evaluation_run_size
    CHECK (sample_count BETWEEN 1 AND 100000 AND repeat_count BETWEEN 1 AND 20),
  CONSTRAINT chk_model_evaluation_run_status
    CHECK (status IN ('draft', 'completed', 'invalidated')),
  CONSTRAINT chk_model_evaluation_run_transitions
    CHECK (
      (status = 'draft'
        AND completed_at IS NULL
        AND invalidated_at IS NULL
        AND invalidation_reason = '')
      OR
      (status = 'completed'
        AND completed_at IS NOT NULL
        AND invalidated_at IS NULL
        AND invalidation_reason = '')
      OR
      (status = 'invalidated'
        AND invalidated_at IS NOT NULL
        AND btrim(invalidation_reason) <> '')
    )
);

CREATE TABLE model_evaluation_candidate (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  run_id UUID NOT NULL,
  deployment_id UUID NOT NULL,
  provider_key TEXT NOT NULL,
  deployment_key TEXT NOT NULL,
  model_version TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  rubric_version TEXT NOT NULL,
  evaluated_samples INT NOT NULL,
  teacher_reviewed_samples INT NOT NULL,
  teacher_accepted_samples INT NOT NULL,
  serious_error_samples INT NOT NULL,
  evidence_valid_samples INT NOT NULL,
  repeat_comparisons INT NOT NULL,
  stable_repeat_samples INT NOT NULL,
  p95_latency_ms BIGINT NOT NULL,
  total_cost_micros BIGINT NOT NULL,
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, run_id, deployment_id),
  CONSTRAINT fk_model_evaluation_candidate_run
    FOREIGN KEY (tenant_id, run_id) REFERENCES model_evaluation_run(tenant_id, id),
  CONSTRAINT fk_model_evaluation_candidate_deployment
    FOREIGN KEY (tenant_id, deployment_id) REFERENCES model_deployment(tenant_id, id),
  CONSTRAINT fk_model_evaluation_candidate_creator
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT chk_model_evaluation_candidate_identity
    CHECK (
      provider_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND deployment_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND btrim(model_version) <> ''
      AND btrim(prompt_version) <> ''
      AND btrim(rubric_version) <> ''
      AND char_length(model_version) <= 128
      AND char_length(prompt_version) <= 128
      AND char_length(rubric_version) <= 128
    ),
  CONSTRAINT chk_model_evaluation_candidate_counts
    CHECK (
      evaluated_samples > 0
      AND teacher_reviewed_samples BETWEEN 0 AND evaluated_samples
      AND teacher_accepted_samples BETWEEN 0 AND teacher_reviewed_samples
      AND serious_error_samples BETWEEN 0 AND evaluated_samples
      AND evidence_valid_samples BETWEEN 0 AND evaluated_samples
      AND repeat_comparisons >= 0
      AND stable_repeat_samples BETWEEN 0 AND repeat_comparisons
      AND p95_latency_ms >= 0
      AND total_cost_micros >= 0
    )
);

CREATE INDEX idx_model_evaluation_run_history
  ON model_evaluation_run (tenant_id, created_at DESC);

CREATE INDEX idx_model_evaluation_candidate_comparison
  ON model_evaluation_candidate (tenant_id, run_id, deployment_key);

CREATE OR REPLACE FUNCTION enforce_model_evaluation_candidate_scope()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  evaluation model_evaluation_run%ROWTYPE;
  deployment_modalities JSONB;
  governed_provider_key TEXT;
  governed_deployment_key TEXT;
  governed_model_version TEXT;
BEGIN
  SELECT *
  INTO evaluation
  FROM model_evaluation_run
  WHERE tenant_id = NEW.tenant_id AND id = NEW.run_id;

  SELECT provider.provider_key, deployment.deployment_key,
         deployment.model_version, deployment.modalities
  INTO governed_provider_key, governed_deployment_key,
       governed_model_version, deployment_modalities
  FROM model_deployment deployment
  JOIN model_provider provider
    ON provider.tenant_id = deployment.tenant_id
   AND provider.id = deployment.provider_id
   AND provider.deleted_at IS NULL
  WHERE deployment.tenant_id = NEW.tenant_id
    AND deployment.id = NEW.deployment_id
    AND deployment.deleted_at IS NULL;

  IF NOT FOUND
     OR evaluation.status <> 'draft'
     OR NEW.provider_key <> governed_provider_key
     OR NEW.deployment_key <> governed_deployment_key
     OR NEW.model_version <> governed_model_version
     OR NEW.evaluated_samples <> evaluation.sample_count
     OR NOT (deployment_modalities ? evaluation.modality)
     OR NEW.repeat_comparisons > evaluation.sample_count * (evaluation.repeat_count - 1)
     OR (evaluation.repeat_count = 1
         AND (NEW.repeat_comparisons <> 0 OR NEW.stable_repeat_samples <> 0))
     OR (evaluation.repeat_count > 1 AND NEW.repeat_comparisons = 0)
     OR (evaluation.evidence_class = 'authorized_frozen_set'
         AND NEW.teacher_reviewed_samples <> NEW.evaluated_samples)
     OR (evaluation.evidence_class = 'protocol_fixture'
         AND (NEW.teacher_reviewed_samples <> 0 OR NEW.teacher_accepted_samples <> 0))
  THEN
    RAISE EXCEPTION 'invalid model evaluation candidate scope'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_model_evaluation_candidate_scope
BEFORE INSERT ON model_evaluation_candidate
FOR EACH ROW
EXECUTE FUNCTION enforce_model_evaluation_candidate_scope();

CREATE OR REPLACE FUNCTION enforce_model_evaluation_completion()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  candidate_count INT;
  local_candidate_count INT;
BEGIN
  IF NEW.run_key IS DISTINCT FROM OLD.run_key
     OR NEW.display_name IS DISTINCT FROM OLD.display_name
     OR NEW.dataset_reference IS DISTINCT FROM OLD.dataset_reference
     OR NEW.dataset_sha256 IS DISTINCT FROM OLD.dataset_sha256
     OR NEW.authorization_reference IS DISTINCT FROM OLD.authorization_reference
     OR NEW.evidence_class IS DISTINCT FROM OLD.evidence_class
     OR NEW.subject IS DISTINCT FROM OLD.subject
     OR NEW.grade IS DISTINCT FROM OLD.grade
     OR NEW.question_type IS DISTINCT FROM OLD.question_type
     OR NEW.modality IS DISTINCT FROM OLD.modality
     OR NEW.sample_count IS DISTINCT FROM OLD.sample_count
     OR NEW.repeat_count IS DISTINCT FROM OLD.repeat_count
     OR NEW.created_by IS DISTINCT FROM OLD.created_by
     OR NEW.created_at IS DISTINCT FROM OLD.created_at
  THEN
    RAISE EXCEPTION 'model evaluation evidence is immutable'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.status = OLD.status
     OR OLD.status = 'invalidated'
     OR (OLD.status = 'completed' AND NEW.status NOT IN ('completed', 'invalidated'))
     OR (OLD.status = 'draft' AND NEW.status NOT IN ('draft', 'completed', 'invalidated'))
  THEN
    RAISE EXCEPTION 'invalid model evaluation status transition'
      USING ERRCODE = '23514';
  END IF;

  IF OLD.status = 'completed'
     AND (NEW.completed_at IS DISTINCT FROM OLD.completed_at
          OR NEW.completed_by IS DISTINCT FROM OLD.completed_by)
  THEN
    RAISE EXCEPTION 'completed model evaluation provenance is immutable'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.status = 'completed' AND OLD.status = 'draft' THEN
    SELECT
      count(*)::INT,
      (count(*) FILTER (WHERE provider.provider_kind = 'local'))::INT
    INTO candidate_count, local_candidate_count
    FROM model_evaluation_candidate candidate
    JOIN model_deployment deployment
      ON deployment.tenant_id = candidate.tenant_id
     AND deployment.id = candidate.deployment_id
    JOIN model_provider provider
      ON provider.tenant_id = deployment.tenant_id
     AND provider.id = deployment.provider_id
    WHERE candidate.tenant_id = NEW.tenant_id
      AND candidate.run_id = NEW.id;

    IF candidate_count < 2 OR local_candidate_count < 1 THEN
      RAISE EXCEPTION 'model evaluation completion requires two candidates and a local baseline'
        USING ERRCODE = '23514';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_model_evaluation_completion
BEFORE UPDATE ON model_evaluation_run
FOR EACH ROW
EXECUTE FUNCTION enforce_model_evaluation_completion();

CREATE OR REPLACE FUNCTION reject_model_evaluation_candidate_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'model evaluation candidates are immutable'
    USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER trg_model_evaluation_candidate_immutable
BEFORE UPDATE OR DELETE ON model_evaluation_candidate
FOR EACH ROW
EXECUTE FUNCTION reject_model_evaluation_candidate_mutation();

INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT tenant.id, 'model:evaluation:manage', 'Manage model evaluations',
       'model_evaluation', 'manage', '管理离线冻结集模型评测与证据失效'
FROM tenant
WHERE tenant.deleted_at IS NULL
ON CONFLICT (tenant_id, code) DO UPDATE
SET name = EXCLUDED.name,
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    description = EXCLUDED.description,
    deleted_at = NULL,
    updated_at = now();

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT role.tenant_id, role.id, permission.id
FROM role
JOIN permission
  ON permission.tenant_id = role.tenant_id
WHERE role.code IN ('platform_admin', 'tenant_admin')
  AND permission.code = 'model:evaluation:manage'
  AND role.deleted_at IS NULL
  AND permission.deleted_at IS NULL
ON CONFLICT (tenant_id, role_id, permission_id) DO UPDATE
SET deleted_at = NULL,
    updated_at = now();
