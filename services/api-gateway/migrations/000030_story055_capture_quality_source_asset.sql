ALTER TABLE submission_page_quality_run
  DROP CONSTRAINT IF EXISTS fk_quality_run_source_file_tenant;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'fk_quality_run_source_file_asset_tenant'
  ) THEN
    ALTER TABLE submission_page_quality_run
      ADD CONSTRAINT fk_quality_run_source_file_asset_tenant
      FOREIGN KEY (tenant_id, source_file_asset_id)
      REFERENCES file_asset (tenant_id, id);
  END IF;
END
$$;

COMMENT ON CONSTRAINT fk_quality_run_source_file_asset_tenant
ON submission_page_quality_run IS
'Decoded capture assets may be content-deduplicated before a submission exists; tenant ownership remains mandatory.';
