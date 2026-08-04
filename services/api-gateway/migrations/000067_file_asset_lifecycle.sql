ALTER TABLE file_asset
  ADD COLUMN IF NOT EXISTS lifecycle_status TEXT NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS last_storage_error TEXT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'file_asset_lifecycle_status_check'
  ) THEN
    ALTER TABLE file_asset
      ADD CONSTRAINT file_asset_lifecycle_status_check
      CHECK (lifecycle_status IN (
        'pending_upload', 'active', 'upload_failed',
        'pending_delete', 'delete_failed', 'deleted'
      ));
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'file_asset_revision_positive'
  ) THEN
    ALTER TABLE file_asset
      ADD CONSTRAINT file_asset_revision_positive CHECK (revision > 0);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_file_asset_storage_recovery
ON file_asset (tenant_id, lifecycle_status, updated_at)
WHERE deleted_at IS NULL
  AND lifecycle_status IN ('pending_upload', 'upload_failed', 'pending_delete', 'delete_failed');

COMMENT ON COLUMN file_asset.lifecycle_status IS
'Durable object-storage saga state. Failed upload/delete operations remain retryable instead of becoming silent orphans.';
