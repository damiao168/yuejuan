ALTER TABLE exam
  ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_teacher_class_active_scope
  ON teacher_class (tenant_id, teacher_id, class_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_exam_class_active_scope
  ON exam_class (tenant_id, class_id, exam_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_review_task_assigned_scope
  ON review_task (tenant_id, assigned_to, status, id)
  WHERE deleted_at IS NULL AND assigned_to IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_arbitration_task_assigned_scope
  ON arbitration_task (tenant_id, assigned_to, status, id)
  WHERE deleted_at IS NULL AND assigned_to IS NOT NULL;

