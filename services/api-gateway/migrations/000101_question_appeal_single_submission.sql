-- A student gets one review request for the same question in one published
-- score version. This is a workflow invariant, not merely a UI restriction.
CREATE UNIQUE INDEX IF NOT EXISTS uq_question_appeal_student_release_question
ON question_appeal (tenant_id, student_id, source_release_id, question_id);
