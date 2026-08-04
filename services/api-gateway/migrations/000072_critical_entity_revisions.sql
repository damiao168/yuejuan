ALTER TABLE submission ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE arbitration_task ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE appeal ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE submission_grade ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE model_approval ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE review_draft ADD COLUMN IF NOT EXISTS client_updated_at TIMESTAMPTZ;

ALTER TABLE capture_batch ALTER COLUMN revision TYPE BIGINT;
ALTER TABLE review_task ALTER COLUMN revision TYPE BIGINT;
ALTER TABLE agent_worker_task ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;

DO $$
DECLARE table_name TEXT;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'submission','arbitration_task','appeal','submission_grade',
    'model_approval','agent_worker_task'
  ] LOOP
    EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', table_name, table_name || '_revision_positive');
    EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK (revision > 0)', table_name, table_name || '_revision_positive');
  END LOOP;
END $$;
