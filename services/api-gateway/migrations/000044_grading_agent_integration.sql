ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS rubric_version TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS delivery_mode TEXT NOT NULL DEFAULT 'teacher_review';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS capability_profile TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS adapter_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS adapter_name TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS adapter_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS adapter_latency_ms BIGINT NOT NULL DEFAULT 0;
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS adapter_repair_attempted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ck_ai_grade_delivery_mode;
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_grade_delivery_mode
  CHECK (delivery_mode IN ('teacher_review', 'teacher_suggestion', 'shadow_only'));

ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ck_ai_grade_adapter_attempts;
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_grade_adapter_attempts
  CHECK (adapter_attempts >= 0 AND adapter_attempts <= 2);

ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ck_ai_grade_adapter_latency_ms;
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_grade_adapter_latency_ms
  CHECK (adapter_latency_ms >= 0);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_grade_adapter_request
  ON ai_grade (tenant_id, adapter_request_id)
  WHERE adapter_request_id <> '' AND deleted_at IS NULL;
