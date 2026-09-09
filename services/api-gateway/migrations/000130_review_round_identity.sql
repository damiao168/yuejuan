-- First and second marking are distinct tasks for the same answer and source.
DROP INDEX uq_review_task_active_source;
CREATE UNIQUE INDEX uq_review_task_active_source
ON review_task(tenant_id,answer_segment_id,source,grade_round)
WHERE status IN ('pending','assigned','in_progress','returned') AND deleted_at IS NULL;
