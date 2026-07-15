ALTER TABLE scoring_run
  DROP CONSTRAINT IF EXISTS scoring_run_status_check;

ALTER TABLE scoring_run
  ADD CONSTRAINT scoring_run_status_check
  CHECK (status IN ('queued', 'processing', 'needs_review', 'completed', 'failed', 'cancelling', 'cancelled'));

ALTER TABLE review_task
  DROP CONSTRAINT IF EXISTS review_task_status_check;

ALTER TABLE review_task
  ADD CONSTRAINT review_task_status_check
  CHECK (status IN ('pending', 'assigned', 'in_progress', 'submitted', 'returned', 'completed', 'cancelled'));

CREATE INDEX IF NOT EXISTS idx_omr_run_scoring_status
ON omr_run (tenant_id, scoring_run_id, status, created_at)
WHERE deleted_at IS NULL;
