DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM file_asset fa
    LEFT JOIN submission s ON s.id = fa.submission_id
    WHERE fa.submission_id IS NOT NULL
      AND (s.id IS NULL OR s.tenant_id <> fa.tenant_id)
  ) THEN
    RAISE EXCEPTION 'file_asset contains invalid cross-tenant submission references';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM file_asset fa
    WHERE (fa.owner_type = 'exam' AND (fa.owner_id IS NULL OR fa.exam_id IS NULL OR fa.owner_id <> fa.exam_id))
       OR (fa.owner_type IN ('submission', 'answer_page') AND (fa.owner_id IS NULL OR fa.submission_id IS NULL OR fa.owner_id <> fa.submission_id))
       OR (fa.owner_type IN ('submission_page_original', 'submission_page_normalized') AND (fa.owner_id IS NULL OR fa.submission_id IS NULL))
  ) THEN
    RAISE EXCEPTION 'file_asset contains inconsistent owner references';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fk_file_asset_submission_tenant'
  ) THEN
    ALTER TABLE file_asset
      ADD CONSTRAINT fk_file_asset_submission_tenant
      FOREIGN KEY (tenant_id, submission_id)
      REFERENCES submission (tenant_id, id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_file_asset_owner_reference'
  ) THEN
    ALTER TABLE file_asset
      ADD CONSTRAINT chk_file_asset_owner_reference CHECK (
        (owner_type <> 'exam' OR (owner_id IS NOT NULL AND exam_id IS NOT NULL AND owner_id = exam_id))
        AND (owner_type NOT IN ('submission', 'answer_page') OR (owner_id IS NOT NULL AND submission_id IS NOT NULL AND owner_id = submission_id))
        AND (owner_type NOT IN ('submission_page_original', 'submission_page_normalized') OR (owner_id IS NOT NULL AND submission_id IS NOT NULL))
      ) NOT VALID;
  END IF;

  ALTER TABLE file_asset VALIDATE CONSTRAINT chk_file_asset_owner_reference;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM submission_page_quality_run qr
    LEFT JOIN submission_page sp
      ON sp.tenant_id = qr.tenant_id
     AND sp.submission_id = qr.submission_id
     AND sp.id = qr.submission_page_id
    LEFT JOIN file_asset source_file
      ON source_file.tenant_id = qr.tenant_id
     AND source_file.submission_id = qr.submission_id
     AND source_file.id = qr.source_file_asset_id
    LEFT JOIN file_asset normalized_file
      ON normalized_file.tenant_id = qr.tenant_id
     AND normalized_file.submission_id = qr.submission_id
     AND normalized_file.id = qr.normalized_file_asset_id
    WHERE sp.id IS NULL
       OR source_file.id IS NULL
       OR (qr.normalized_file_asset_id IS NOT NULL AND normalized_file.id IS NULL)
  ) THEN
    RAISE EXCEPTION 'submission_page_quality_run contains invalid cross-tenant references';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM submission_page sp
    LEFT JOIN submission_page_quality_run qr
      ON qr.tenant_id = sp.tenant_id
     AND qr.submission_id = sp.submission_id
     AND qr.submission_page_id = sp.id
     AND qr.id = sp.latest_quality_run_id
    WHERE sp.latest_quality_run_id IS NOT NULL AND qr.id IS NULL
  ) THEN
    RAISE EXCEPTION 'submission_page contains an invalid cross-tenant latest quality run reference';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM submission_page sp
    LEFT JOIN file_asset normalized_file
      ON normalized_file.tenant_id = sp.tenant_id
     AND normalized_file.submission_id = sp.submission_id
     AND normalized_file.id = sp.normalized_file_asset_id
    WHERE sp.normalized_file_asset_id IS NOT NULL AND normalized_file.id IS NULL
  ) THEN
    RAISE EXCEPTION 'submission_page contains an invalid normalized file reference';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_submission_page_tenant_submission_id_id') THEN
    ALTER TABLE submission_page
      ADD CONSTRAINT uq_submission_page_tenant_submission_id_id UNIQUE (tenant_id, submission_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_quality_run_tenant_id_id') THEN
    ALTER TABLE submission_page_quality_run
      ADD CONSTRAINT uq_quality_run_tenant_id_id UNIQUE (tenant_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_file_asset_tenant_submission_id_id') THEN
    ALTER TABLE file_asset
      ADD CONSTRAINT uq_file_asset_tenant_submission_id_id UNIQUE (tenant_id, submission_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_quality_run_tenant_submission_page_id') THEN
    ALTER TABLE submission_page_quality_run
      ADD CONSTRAINT uq_quality_run_tenant_submission_page_id
      UNIQUE (tenant_id, submission_id, submission_page_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_quality_run_submission_tenant') THEN
    ALTER TABLE submission_page_quality_run
      ADD CONSTRAINT fk_quality_run_submission_tenant
      FOREIGN KEY (tenant_id, submission_id) REFERENCES submission (tenant_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_quality_run_page_tenant') THEN
    ALTER TABLE submission_page_quality_run
      ADD CONSTRAINT fk_quality_run_page_tenant
      FOREIGN KEY (tenant_id, submission_id, submission_page_id)
      REFERENCES submission_page (tenant_id, submission_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_quality_run_source_file_tenant') THEN
    ALTER TABLE submission_page_quality_run
      ADD CONSTRAINT fk_quality_run_source_file_tenant
      FOREIGN KEY (tenant_id, submission_id, source_file_asset_id)
      REFERENCES file_asset (tenant_id, submission_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_quality_run_normalized_file_tenant') THEN
    ALTER TABLE submission_page_quality_run
      ADD CONSTRAINT fk_quality_run_normalized_file_tenant
      FOREIGN KEY (tenant_id, submission_id, normalized_file_asset_id)
      REFERENCES file_asset (tenant_id, submission_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_submission_page_normalized_file_tenant') THEN
    ALTER TABLE submission_page
      ADD CONSTRAINT fk_submission_page_normalized_file_tenant
      FOREIGN KEY (tenant_id, submission_id, normalized_file_asset_id)
      REFERENCES file_asset (tenant_id, submission_id, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_submission_page_latest_quality_run_tenant') THEN
    ALTER TABLE submission_page
      ADD CONSTRAINT fk_submission_page_latest_quality_run_tenant
      FOREIGN KEY (tenant_id, submission_id, id, latest_quality_run_id)
      REFERENCES submission_page_quality_run (tenant_id, submission_id, submission_page_id, id);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_file_asset_submission
ON file_asset (tenant_id, submission_id, created_at DESC)
WHERE submission_id IS NOT NULL AND deleted_at IS NULL;
