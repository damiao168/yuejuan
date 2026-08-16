-- STORY-MATH-07: append-only teacher corrections for mathematical evidence.
CREATE TABLE math_understanding_correction (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  artifact_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  revision BIGINT NOT NULL,
  operations_json JSONB NOT NULL,
  corrected_contract_json JSONB NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(tenant_id,id), UNIQUE(tenant_id,artifact_id,revision),
  FOREIGN KEY(tenant_id,artifact_id) REFERENCES math_understanding_artifact(tenant_id,id),
  FOREIGN KEY(tenant_id,answer_segment_id) REFERENCES answer_segment(tenant_id,id),
  FOREIGN KEY(tenant_id,created_by) REFERENCES app_user(tenant_id,id),
  CHECK(revision>0), CHECK(jsonb_typeof(operations_json)='array' AND jsonb_array_length(operations_json)>0),
  CHECK(jsonb_typeof(corrected_contract_json)='object'), CHECK(octet_length(reason)<=1000)
);
CREATE INDEX idx_math_correction_training ON math_understanding_correction(tenant_id,created_at,id);
CREATE FUNCTION reject_math_correction_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'math understanding corrections are append-only'; END; $$;
CREATE TRIGGER trg_math_correction_no_update BEFORE UPDATE OR DELETE ON math_understanding_correction FOR EACH ROW EXECUTE FUNCTION reject_math_correction_mutation();
