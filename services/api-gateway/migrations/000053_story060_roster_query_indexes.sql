-- STORY-060 roster reconciliation reads submissions by exam and student.
-- Keep the hot path index-only for active records, including duplicate
-- detection and the 500-student release gate.

CREATE INDEX IF NOT EXISTS idx_submission_exam_student_active
ON submission (tenant_id, exam_id, student_id)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_student_class_active_roster
ON student (tenant_id, class_id, id)
INCLUDE (student_no, name)
WHERE deleted_at IS NULL AND status = 'active';
