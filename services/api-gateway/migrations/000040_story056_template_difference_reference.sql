-- STORY-056 implementation-review fix: persist the exact blank-page reference used by a
-- template-difference OMR run. The template content hash already includes
-- this snapshot, and explicit run columns make the evidence chain queryable and
-- prevent a worker callback from substituting another reference.
ALTER TABLE omr_run
  ADD COLUMN reference_file_asset_id UUID,
  ADD COLUMN reference_sha256 TEXT;

ALTER TABLE omr_run
  ADD CONSTRAINT fk_omr_run_reference_file_asset_tenant
  FOREIGN KEY (tenant_id, reference_file_asset_id)
  REFERENCES file_asset (tenant_id, id);

ALTER TABLE omr_run
  ADD CONSTRAINT chk_omr_run_reference_snapshot
  CHECK (
    (reference_file_asset_id IS NULL AND reference_sha256 IS NULL) OR
    (reference_file_asset_id IS NOT NULL AND length(btrim(reference_sha256)) > 0)
  );
