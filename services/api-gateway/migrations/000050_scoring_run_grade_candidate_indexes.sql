-- Support the scoring-run hot paths that filter by (tenant_id, scoring_run_id,
-- answer_segment_id): refreshScoringRunTx counters (executed on every OMR
-- result), GetScoringSummary EXISTS probes, GetScoringRunDetail scoped joins,
-- and ProcessRuleCandidates batch scans. Without these the planner falls back
-- to the tenant-wide prefixes of the is_current / segment indexes.

CREATE INDEX IF NOT EXISTS idx_question_grade_scoring_run
ON question_grade (tenant_id, scoring_run_id, answer_segment_id)
WHERE scoring_run_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_answer_candidate_scoring_run
ON answer_candidate (tenant_id, scoring_run_id, answer_segment_id)
WHERE scoring_run_id IS NOT NULL AND deleted_at IS NULL;
