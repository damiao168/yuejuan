-- Strengthen managed model API state invariants and persist validation health.

ALTER TABLE managed_model_api_config
  ADD COLUMN last_test_latency_ms BIGINT,
  ADD COLUMN config_source TEXT NOT NULL DEFAULT 'manual',
  ADD COLUMN provider_registry_version TEXT NOT NULL DEFAULT '';

UPDATE managed_model_api_config
SET is_default = FALSE,
    updated_at = now()
WHERE is_default AND status <> 'active' AND deleted_at IS NULL;

ALTER TABLE managed_model_api_config
  ADD CONSTRAINT chk_managed_model_api_default_active
    CHECK (NOT is_default OR status = 'active'),
  ADD CONSTRAINT chk_managed_model_api_latency
    CHECK (last_test_latency_ms IS NULL OR last_test_latency_ms >= 0),
  ADD CONSTRAINT chk_managed_model_api_config_source
    CHECK (config_source IN ('auto', 'manual', 'imported'));
