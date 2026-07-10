ALTER TABLE submission_page
  ADD COLUMN IF NOT EXISTS latest_quality_run_id UUID NULL,
  ADD COLUMN IF NOT EXISTS normalized_file_asset_id UUID NULL REFERENCES file_asset(id),
  ADD COLUMN IF NOT EXISTS quality_status TEXT NOT NULL DEFAULT 'unchecked',
  ADD COLUMN IF NOT EXISTS quality_override JSONB NOT NULL DEFAULT '{}';

DO $$
DECLARE
  constraint_name TEXT;
BEGIN
  FOR constraint_name IN
    SELECT c.conname
    FROM pg_constraint c
    JOIN pg_class r ON r.oid = c.conrelid
    WHERE r.relname = 'submission'
      AND c.contype = 'c'
      AND pg_get_constraintdef(c.oid) ILIKE '%quality_status%'
  LOOP
    EXECUTE format('ALTER TABLE submission DROP CONSTRAINT IF EXISTS %I', constraint_name);
  END LOOP;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_submission_quality_status_story050'
  ) THEN
    ALTER TABLE submission
      ADD CONSTRAINT chk_submission_quality_status_story050
      CHECK (quality_status IN ('unchecked', 'passed', 'review', 'failed'));
  END IF;
END
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_submission_page_quality_status_story050'
  ) THEN
    ALTER TABLE submission_page
      ADD CONSTRAINT chk_submission_page_quality_status_story050
      CHECK (quality_status IN ('unchecked', 'passed', 'review', 'failed'));
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS submission_page_quality_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  submission_page_id UUID NOT NULL REFERENCES submission_page(id),
  source_file_asset_id UUID NOT NULL REFERENCES file_asset(id),
  source_sha256 TEXT NOT NULL,
  normalized_file_asset_id UUID NULL REFERENCES file_asset(id),
  processing_status TEXT NOT NULL DEFAULT 'pending',
  quality_status TEXT NULL,
  profile_name TEXT NOT NULL,
  profile_version TEXT NOT NULL,
  profile_config_hash TEXT NOT NULL,
  metric_schema_version TEXT NOT NULL,
  report_schema_version TEXT NOT NULL,
  quality_report JSONB NOT NULL DEFAULT '{}',
  quality_issues JSONB NOT NULL DEFAULT '[]',
  normalization_transform JSONB NOT NULL DEFAULT '{}',
  worker_service TEXT,
  worker_instance_id TEXT,
  attempt_no INT NOT NULL DEFAULT 0,
  result_version TEXT,
  result_payload_hash TEXT,
  lease_token TEXT,
  lease_expires_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  duration_ms INT,
  error_code TEXT,
  error_detail JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (processing_status IN ('pending', 'processing', 'completed', 'retryable_error', 'terminal_error')),
  CHECK (quality_status IS NULL OR quality_status IN ('passed', 'review', 'failed')),
  CHECK (attempt_no >= 0),
  CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fk_submission_page_latest_quality_run_story050'
  ) THEN
    ALTER TABLE submission_page
      ADD CONSTRAINT fk_submission_page_latest_quality_run_story050
      FOREIGN KEY (latest_quality_run_id) REFERENCES submission_page_quality_run(id);
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_quality_run_claim
ON submission_page_quality_run (tenant_id, processing_status, lease_expires_at, created_at)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_quality_run_page
ON submission_page_quality_run (tenant_id, submission_page_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_quality_run_submission
ON submission_page_quality_run (tenant_id, submission_id, created_at DESC)
WHERE deleted_at IS NULL;

COMMENT ON COLUMN submission_page_quality_run.normalization_transform IS
'JSON object containing source dimensions, normalized dimensions, crop_box_source_pixels, and source_to_normalized_matrix.';

