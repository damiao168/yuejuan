-- STORY-061C3: immutable, version-bound model approvals derived only from
-- completed authorized frozen-set evaluation evidence.

CREATE TABLE model_approval (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  evaluation_run_id UUID NOT NULL,
  evaluation_candidate_id UUID NOT NULL,
  deployment_id UUID NOT NULL,
  provider_key TEXT NOT NULL,
  deployment_key TEXT NOT NULL,
  model_version TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  rubric_version TEXT NOT NULL,
  dataset_reference TEXT NOT NULL,
  dataset_sha256 TEXT NOT NULL,
  authorization_reference TEXT NOT NULL,
  subject TEXT NOT NULL,
  grade TEXT NOT NULL,
  question_type TEXT NOT NULL,
  modality TEXT NOT NULL,
  manual_review_rate DOUBLE PRECISION NOT NULL,
  decision_reference TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_by UUID,
  revoked_at TIMESTAMPTZ,
  revocation_reason TEXT NOT NULL DEFAULT '',
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, decision_reference),
  CONSTRAINT fk_model_approval_evaluation
    FOREIGN KEY (tenant_id, evaluation_run_id) REFERENCES model_evaluation_run(tenant_id, id),
  CONSTRAINT fk_model_approval_candidate
    FOREIGN KEY (tenant_id, evaluation_candidate_id) REFERENCES model_evaluation_candidate(tenant_id, id),
  CONSTRAINT fk_model_approval_deployment
    FOREIGN KEY (tenant_id, deployment_id) REFERENCES model_deployment(tenant_id, id),
  CONSTRAINT fk_model_approval_creator
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT fk_model_approval_revoker
    FOREIGN KEY (tenant_id, revoked_by) REFERENCES app_user(tenant_id, id),
  CONSTRAINT chk_model_approval_keys
    CHECK (
      provider_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND deployment_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND dataset_reference ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND authorization_reference ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND question_type ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND decision_reference ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
      AND dataset_sha256 ~ '^[a-f0-9]{64}$'
    ),
  CONSTRAINT chk_model_approval_versions
    CHECK (
      btrim(model_version) <> '' AND char_length(model_version) <= 128
      AND btrim(prompt_version) <> '' AND char_length(prompt_version) <= 128
      AND btrim(rubric_version) <> '' AND char_length(rubric_version) <= 128
      AND btrim(subject) <> '' AND char_length(subject) <= 128
      AND btrim(grade) <> '' AND char_length(grade) <= 128
    ),
  CONSTRAINT chk_model_approval_scope
    CHECK (
      modality IN ('text', 'image')
      AND manual_review_rate BETWEEN 0 AND 1
      AND expires_at > created_at
      AND expires_at <= created_at + INTERVAL '365 days'
    ),
  CONSTRAINT chk_model_approval_revocation
    CHECK (
      (revoked_at IS NULL AND revoked_by IS NULL AND revocation_reason = '')
      OR
      (revoked_at IS NOT NULL AND btrim(revocation_reason) <> '')
    )
);

CREATE INDEX idx_model_approval_active_scope
  ON model_approval (
    tenant_id, deployment_id, subject, grade, question_type, modality,
    model_version, prompt_version, rubric_version, expires_at DESC
  )
  WHERE revoked_at IS NULL;

CREATE OR REPLACE FUNCTION enforce_model_approval_evidence()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  evaluation model_evaluation_run%ROWTYPE;
  candidate model_evaluation_candidate%ROWTYPE;
BEGIN
  SELECT * INTO evaluation
  FROM model_evaluation_run
  WHERE tenant_id = NEW.tenant_id AND id = NEW.evaluation_run_id;

  SELECT * INTO candidate
  FROM model_evaluation_candidate
  WHERE tenant_id = NEW.tenant_id
    AND id = NEW.evaluation_candidate_id
    AND run_id = NEW.evaluation_run_id;

  IF evaluation.id IS NULL
     OR candidate.id IS NULL
     OR evaluation.status <> 'completed'
     OR evaluation.evidence_class <> 'authorized_frozen_set'
     OR candidate.deployment_id <> NEW.deployment_id
     OR NEW.provider_key <> candidate.provider_key
     OR NEW.deployment_key <> candidate.deployment_key
     OR NEW.model_version <> candidate.model_version
     OR NEW.prompt_version <> candidate.prompt_version
     OR NEW.rubric_version <> candidate.rubric_version
     OR NEW.dataset_reference <> evaluation.dataset_reference
     OR NEW.dataset_sha256 <> evaluation.dataset_sha256
     OR NEW.authorization_reference <> evaluation.authorization_reference
     OR NEW.subject <> evaluation.subject
     OR NEW.grade <> evaluation.grade
     OR NEW.question_type <> evaluation.question_type
     OR NEW.modality <> evaluation.modality
  THEN
    RAISE EXCEPTION 'model approval must exactly snapshot authorized evaluation evidence'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_model_approval_evidence
BEFORE INSERT ON model_approval
FOR EACH ROW
EXECUTE FUNCTION enforce_model_approval_evidence();

CREATE OR REPLACE FUNCTION enforce_model_approval_immutable()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
     OR NEW.evaluation_run_id IS DISTINCT FROM OLD.evaluation_run_id
     OR NEW.evaluation_candidate_id IS DISTINCT FROM OLD.evaluation_candidate_id
     OR NEW.deployment_id IS DISTINCT FROM OLD.deployment_id
     OR NEW.provider_key IS DISTINCT FROM OLD.provider_key
     OR NEW.deployment_key IS DISTINCT FROM OLD.deployment_key
     OR NEW.model_version IS DISTINCT FROM OLD.model_version
     OR NEW.prompt_version IS DISTINCT FROM OLD.prompt_version
     OR NEW.rubric_version IS DISTINCT FROM OLD.rubric_version
     OR NEW.dataset_reference IS DISTINCT FROM OLD.dataset_reference
     OR NEW.dataset_sha256 IS DISTINCT FROM OLD.dataset_sha256
     OR NEW.authorization_reference IS DISTINCT FROM OLD.authorization_reference
     OR NEW.subject IS DISTINCT FROM OLD.subject
     OR NEW.grade IS DISTINCT FROM OLD.grade
     OR NEW.question_type IS DISTINCT FROM OLD.question_type
     OR NEW.modality IS DISTINCT FROM OLD.modality
     OR NEW.manual_review_rate IS DISTINCT FROM OLD.manual_review_rate
     OR NEW.decision_reference IS DISTINCT FROM OLD.decision_reference
     OR NEW.expires_at IS DISTINCT FROM OLD.expires_at
     OR NEW.created_by IS DISTINCT FROM OLD.created_by
     OR NEW.created_at IS DISTINCT FROM OLD.created_at
     OR OLD.revoked_at IS NOT NULL
     OR NEW.revoked_at IS NULL
     OR btrim(NEW.revocation_reason) = ''
  THEN
    RAISE EXCEPTION 'model approval evidence is immutable'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_model_approval_immutable
BEFORE UPDATE ON model_approval
FOR EACH ROW
EXECUTE FUNCTION enforce_model_approval_immutable();

CREATE OR REPLACE FUNCTION reject_model_approval_delete()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'model approval history cannot be deleted'
    USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER trg_model_approval_no_delete
BEFORE DELETE ON model_approval
FOR EACH ROW
EXECUTE FUNCTION reject_model_approval_delete();
