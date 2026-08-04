-- Support stable cursor pagination without scanning an exam or submission history.
CREATE INDEX IF NOT EXISTS idx_submission_grade_exam_cursor
  ON submission_grade (tenant_id, exam_id, anonymous_code, id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ocr_task_submission_cursor
  ON ocr_task (tenant_id, submission_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL;
