-- Separate zero-token connectivity checks from paid model capability checks.

ALTER TABLE managed_model_api_config
  ADD COLUMN last_probe_mode TEXT NOT NULL DEFAULT '',
  ADD COLUMN last_capability_status TEXT NOT NULL DEFAULT 'untested',
  ADD COLUMN last_capability_message TEXT NOT NULL DEFAULT '',
  ADD COLUMN last_capability_tested_at TIMESTAMPTZ,
  ADD COLUMN last_capability_probe_version TEXT NOT NULL DEFAULT '',
  ADD COLUMN last_capability_usage JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Every probe recorded before this migration was a full capability probe.
UPDATE managed_model_api_config
SET last_probe_mode = CASE WHEN last_tested_at IS NULL THEN '' ELSE 'capability' END,
    last_capability_status = last_test_status,
    last_capability_message = last_test_message,
    last_capability_tested_at = last_tested_at,
    last_capability_probe_version = CASE WHEN last_tested_at IS NULL THEN '' ELSE 'legacy-v1' END;

ALTER TABLE managed_model_api_config
  ADD CONSTRAINT chk_managed_model_api_probe_mode
    CHECK (last_probe_mode IN ('', 'quick', 'capability')),
  ADD CONSTRAINT chk_managed_model_api_capability_status
    CHECK (last_capability_status IN ('untested', 'success', 'failed')),
  ADD CONSTRAINT chk_managed_model_api_capability_usage
    CHECK (jsonb_typeof(last_capability_usage) = 'object');
