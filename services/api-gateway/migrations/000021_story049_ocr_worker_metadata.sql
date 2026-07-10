ALTER TABLE ocr_task
  ADD COLUMN IF NOT EXISTS model_version TEXT,
  ADD COLUMN IF NOT EXISTS config_hash TEXT,
  ADD COLUMN IF NOT EXISTS input_hash TEXT,
  ADD COLUMN IF NOT EXISTS duration_ms INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS worker_id TEXT,
  ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'chk_ocr_task_duration_non_negative'
  ) THEN
    ALTER TABLE ocr_task
      ADD CONSTRAINT chk_ocr_task_duration_non_negative CHECK (duration_ms >= 0);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'chk_ocr_task_attempt_non_negative'
  ) THEN
    ALTER TABLE ocr_task
      ADD CONSTRAINT chk_ocr_task_attempt_non_negative CHECK (attempt_count >= 0);
  END IF;
END
$$;

ALTER TABLE ocr_result
  ADD COLUMN IF NOT EXISTS model_version TEXT,
  ADD COLUMN IF NOT EXISTS config_hash TEXT,
  ADD COLUMN IF NOT EXISTS input_hash TEXT,
  ADD COLUMN IF NOT EXISTS preprocess_profile TEXT;
