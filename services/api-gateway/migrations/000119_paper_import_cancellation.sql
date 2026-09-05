ALTER TABLE paper_import_job
  DROP CONSTRAINT IF EXISTS paper_import_job_status_check;

ALTER TABLE paper_import_job
  ADD CONSTRAINT paper_import_job_status_check
  CHECK (status IN ('processing', 'review_required', 'failed', 'cancelled', 'applied'));
