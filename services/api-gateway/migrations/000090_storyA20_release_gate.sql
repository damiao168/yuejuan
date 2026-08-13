-- STORY-A20: versioned release-gate policy and append-only evidence. A18
-- remains the only table that exposes a published score; these records prove
-- how its pre-publication quality decision was reached.

CREATE TABLE IF NOT EXISTS release_gate_policy (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  policy JSONB NOT NULL DEFAULT '{}',
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, exam_id, version),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CHECK (status IN ('active', 'retired')),
  CHECK (length(btrim(version)) BETWEEN 3 AND 120),
  CHECK (jsonb_typeof(policy) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_release_gate_policy_one_active
ON release_gate_policy (tenant_id, exam_id) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS release_gate_evidence (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  release_id UUID,
  policy_id UUID,
  phase TEXT NOT NULL,
  evaluation JSONB NOT NULL,
  evidence_hash TEXT NOT NULL,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, release_id) REFERENCES score_release(tenant_id, id),
  FOREIGN KEY (tenant_id, policy_id) REFERENCES release_gate_policy(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CHECK (phase IN ('preview', 'publish')),
  CHECK (jsonb_typeof(evaluation) = 'object'),
  CHECK (evidence_hash ~ '^[0-9a-f]{64}$')
);

CREATE INDEX IF NOT EXISTS idx_release_gate_evidence_exam_created
ON release_gate_evidence (tenant_id, exam_id, created_at DESC);

CREATE TABLE IF NOT EXISTS release_gate_waiver_request (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  policy_id UUID,
  evidence_id UUID NOT NULL,
  issue_code TEXT NOT NULL,
  reason TEXT NOT NULL,
  requested_by UUID NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, policy_id) REFERENCES release_gate_policy(tenant_id, id),
  FOREIGN KEY (tenant_id, evidence_id) REFERENCES release_gate_evidence(tenant_id, id),
  FOREIGN KEY (tenant_id, requested_by) REFERENCES app_user(tenant_id, id),
  CHECK (length(btrim(issue_code)) BETWEEN 1 AND 120),
  CHECK (length(btrim(reason)) BETWEEN 1 AND 2000)
);

CREATE TABLE IF NOT EXISTS release_gate_waiver_decision (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  waiver_id UUID NOT NULL,
  status TEXT NOT NULL,
  reason TEXT NOT NULL,
  decided_by UUID NOT NULL,
  decided_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, waiver_id),
  FOREIGN KEY (tenant_id, waiver_id) REFERENCES release_gate_waiver_request(tenant_id, id),
  FOREIGN KEY (tenant_id, decided_by) REFERENCES app_user(tenant_id, id),
  CHECK (status IN ('approved', 'rejected')),
  CHECK (length(btrim(reason)) BETWEEN 1 AND 2000)
);

-- Policies may only transition active -> retired when a newer policy is
-- created. Their version/configuration and all evidence/request/decision
-- records are otherwise append-only forensic facts.
CREATE OR REPLACE FUNCTION enforce_release_gate_append_only()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_TABLE_NAME = 'release_gate_policy' THEN
    IF TG_OP = 'DELETE' OR NEW.id <> OLD.id OR NEW.tenant_id <> OLD.tenant_id
      OR NEW.exam_id <> OLD.exam_id OR NEW.version <> OLD.version
      OR NEW.policy <> OLD.policy OR NEW.created_by <> OLD.created_by
      OR NEW.created_at <> OLD.created_at
      OR NOT (OLD.status = 'active' AND NEW.status = 'retired') THEN
      RAISE EXCEPTION 'release gate policy is immutable except active-to-retired transition';
    END IF;
    RETURN NEW;
  END IF;
  IF TG_OP IN ('UPDATE', 'DELETE') THEN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
  END IF;
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_release_gate_policy_append_only ON release_gate_policy;
CREATE TRIGGER trg_release_gate_policy_append_only
BEFORE UPDATE OR DELETE ON release_gate_policy
FOR EACH ROW EXECUTE FUNCTION enforce_release_gate_append_only();

DROP TRIGGER IF EXISTS trg_release_gate_evidence_append_only ON release_gate_evidence;
CREATE TRIGGER trg_release_gate_evidence_append_only
BEFORE UPDATE OR DELETE ON release_gate_evidence
FOR EACH ROW EXECUTE FUNCTION enforce_release_gate_append_only();

DROP TRIGGER IF EXISTS trg_release_gate_waiver_request_append_only ON release_gate_waiver_request;
CREATE TRIGGER trg_release_gate_waiver_request_append_only
BEFORE UPDATE OR DELETE ON release_gate_waiver_request
FOR EACH ROW EXECUTE FUNCTION enforce_release_gate_append_only();

DROP TRIGGER IF EXISTS trg_release_gate_waiver_decision_append_only ON release_gate_waiver_decision;
CREATE TRIGGER trg_release_gate_waiver_decision_append_only
BEFORE UPDATE OR DELETE ON release_gate_waiver_decision
FOR EACH ROW EXECUTE FUNCTION enforce_release_gate_append_only();

COMMENT ON TABLE release_gate_evidence IS
'Append-only A20 release-gate evaluation evidence. A publish phase is recalculated immediately before score-release publication.';
