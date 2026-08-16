-- STORY-MATH-08: append-only pilot gate evidence. Passing never enables automatic final scoring.
CREATE TABLE math_pilot_gate_evaluation (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  subject_code TEXT NOT NULL,
  benchmark_ref TEXT NOT NULL,
  metrics_json JSONB NOT NULL,
  policy_json JSONB NOT NULL,
  decision_json JSONB NOT NULL,
  evaluated_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(tenant_id,id),
  FOREIGN KEY(tenant_id,evaluated_by) REFERENCES app_user(tenant_id,id),
  CHECK(subject_code IN ('mathematics','physics','chemistry')),
  CHECK(octet_length(benchmark_ref) BETWEEN 1 AND 512),
  CHECK(jsonb_typeof(metrics_json)='object'),
  CHECK(jsonb_typeof(policy_json)='object'),
  CHECK(jsonb_typeof(decision_json)='object'),
  CHECK(decision_json->>'scope'='teacher_suggestion_only')
);
CREATE INDEX idx_math_pilot_gate_subject ON math_pilot_gate_evaluation(tenant_id,subject_code,created_at DESC,id DESC);
CREATE TRIGGER trg_math_pilot_gate_no_update BEFORE UPDATE OR DELETE ON math_pilot_gate_evaluation FOR EACH ROW EXECUTE FUNCTION reject_math_correction_mutation();
