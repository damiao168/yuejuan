ALTER TABLE file_asset DROP CONSTRAINT IF EXISTS file_asset_lifecycle_status_check;
ALTER TABLE file_asset
  ADD CONSTRAINT file_asset_lifecycle_status_check
  CHECK (lifecycle_status IN (
    'pending_upload','active','quarantined','upload_failed',
    'pending_delete','delete_failed','deleted','missing_object','orphan_recovered'
  ));

ALTER TABLE file_asset
  ADD COLUMN IF NOT EXISTS delete_attempts INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_storage_error_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS retention_until TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS legal_hold BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

ALTER TABLE file_asset
  DROP CONSTRAINT IF EXISTS file_asset_delete_attempts_check;
ALTER TABLE file_asset
  ADD CONSTRAINT file_asset_delete_attempts_check CHECK (delete_attempts >= 0);

CREATE UNIQUE INDEX IF NOT EXISTS uq_file_asset_logical_content
ON file_asset (tenant_id, owner_type, owner_id, hash_sha256) NULLS NOT DISTINCT
WHERE deleted_at IS NULL AND lifecycle_status NOT IN ('deleted','upload_failed');

CREATE INDEX IF NOT EXISTS idx_file_asset_reconciliation
ON file_asset (id)
WHERE deleted_at IS NULL AND lifecycle_status <> 'deleted';

CREATE TABLE IF NOT EXISTS file_reconciliation_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  mode TEXT NOT NULL DEFAULT 'report',
  status TEXT NOT NULL DEFAULT 'running',
  scanned_assets INT NOT NULL DEFAULT 0,
  scanned_objects INT NOT NULL DEFAULT 0,
  finding_count INT NOT NULL DEFAULT 0,
  repaired_count INT NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  error_detail TEXT,
  CONSTRAINT file_reconciliation_mode_check CHECK (mode IN ('report','repair')),
  CONSTRAINT file_reconciliation_status_check CHECK (status IN ('running','completed','failed'))
);

CREATE TABLE IF NOT EXISTS file_reconciliation_finding (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES file_reconciliation_run(id) ON DELETE CASCADE,
  tenant_id UUID REFERENCES tenant(id),
  file_asset_id UUID REFERENCES file_asset(id),
  storage_bucket TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  code TEXT NOT NULL,
  detail JSONB NOT NULL DEFAULT '{}',
  repaired BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_file_reconciliation_run_latest
ON file_reconciliation_run (started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_file_reconciliation_finding_run
ON file_reconciliation_finding (run_id, created_at, id);

COMMENT ON TABLE file_reconciliation_finding IS
'Read-only-by-default evidence comparing file_asset metadata with object storage. Repair must be explicitly requested.';

