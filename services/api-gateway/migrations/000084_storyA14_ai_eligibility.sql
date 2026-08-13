-- STORY-A14: server-side, tenant-isolated admission control for external AI
-- grading. The decision table is an immutable audit record; it stores only
-- quality/evaluation facts, never an answer image, answer text or model trace.

CREATE TABLE IF NOT EXISTS ai_eligibility_policy (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  subject_code TEXT NOT NULL,
  education_stage TEXT NOT NULL,
  archetype_code TEXT NOT NULL REFERENCES question_archetype(code),
  risk_tier TEXT NOT NULL,
  min_ocr_quality NUMERIC(7,6) NOT NULL,
  min_parser_quality NUMERIC(7,6) NOT NULL,
  min_eval_n INTEGER NOT NULL,
  max_severe_error_rate NUMERIC(7,6) NOT NULL,
  allowed_modes_json JSONB NOT NULL,
  version INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, subject_code, education_stage, archetype_code, risk_tier, version),
  CHECK (subject_code IN ('chinese','mathematics','english','physics','chemistry','biology','history','geography','ethics_politics')),
  CHECK (education_stage IN ('junior','senior')),
  CHECK (risk_tier IN ('R1','R2','R3')),
  CHECK (min_ocr_quality BETWEEN 0 AND 1),
  CHECK (min_parser_quality BETWEEN 0 AND 1),
  CHECK (min_eval_n > 0),
  CHECK (max_severe_error_rate BETWEEN 0 AND 1),
  CHECK (version > 0),
  CHECK (status IN ('active','disabled')),
  CHECK (jsonb_typeof(allowed_modes_json) = 'array'),
  CHECK (NOT (risk_tier = 'R3' AND archetype_code = 'extended_response' AND
    (allowed_modes_json ? 'AI_FAST_CONFIRM' OR allowed_modes_json ? 'RULE_AUTO')))
);

CREATE INDEX IF NOT EXISTS idx_ai_eligibility_policy_lookup
  ON ai_eligibility_policy (tenant_id, subject_code, education_stage, archetype_code, risk_tier, version DESC);

CREATE TABLE IF NOT EXISTS ai_eligibility_decision (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  run_item_id TEXT NOT NULL,
  policy_id UUID REFERENCES ai_eligibility_policy(id),
  policy_version INTEGER NOT NULL DEFAULT 0,
  input_snapshot_json JSONB NOT NULL,
  decision TEXT NOT NULL,
  reasons_json JSONB NOT NULL,
  external_ai_allowed BOOLEAN NOT NULL DEFAULT FALSE,
  output_constraint_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, run_item_id),
  FOREIGN KEY (tenant_id, policy_id) REFERENCES ai_eligibility_policy(tenant_id, id),
  CHECK (policy_version >= 0),
  CHECK (decision IN ('RULE_AUTO','AI_ASSIST','AI_FAST_CONFIRM','HUMAN_PRIMARY','DUAL_HUMAN','MANUAL_ONLY')),
  CHECK (jsonb_typeof(input_snapshot_json) = 'object'),
  CHECK (jsonb_typeof(reasons_json) = 'array'),
  CHECK (jsonb_typeof(output_constraint_json) = 'object'),
  CHECK (NOT external_ai_allowed OR decision IN ('AI_ASSIST','AI_FAST_CONFIRM')),
  CHECK (COALESCE((output_constraint_json ->> 'allow_model_final_score')::boolean, FALSE) = FALSE)
);

CREATE INDEX IF NOT EXISTS idx_ai_eligibility_decision_run
  ON ai_eligibility_decision (tenant_id, run_item_id, created_at DESC);

-- Decisions are facts. A retry must read the previous decision instead of
-- overwriting it under a later policy/evaluation state.
CREATE OR REPLACE FUNCTION prevent_ai_eligibility_decision_mutation()
RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'ai eligibility decision is immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ai_eligibility_decision_immutable ON ai_eligibility_decision;
CREATE TRIGGER trg_ai_eligibility_decision_immutable
BEFORE UPDATE OR DELETE ON ai_eligibility_decision
FOR EACH ROW EXECUTE FUNCTION prevent_ai_eligibility_decision_mutation();
