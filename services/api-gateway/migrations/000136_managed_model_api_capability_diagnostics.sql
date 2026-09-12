-- Persist safe capability diagnostics and ensure only the current probe policy
-- can select a third-party model as the school's active default.

ALTER TABLE managed_model_api_config
  ADD COLUMN IF NOT EXISTS last_capability_diagnostic JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE managed_model_api_config
SET is_default = FALSE,
    updated_at = now()
WHERE is_default
  AND deleted_at IS NULL
  AND (
    last_capability_status <> 'success'
    OR last_capability_probe_version <> 'structured-json-v3'
  );

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_managed_model_api_capability_diagnostic'
  ) THEN
    ALTER TABLE managed_model_api_config
      ADD CONSTRAINT chk_managed_model_api_capability_diagnostic
        CHECK (jsonb_typeof(last_capability_diagnostic) = 'object');
  END IF;
END
$$;
