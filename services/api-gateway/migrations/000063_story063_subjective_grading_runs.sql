CREATE TABLE IF NOT EXISTS subjective_grading_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  answer_segment_id UUID NOT NULL REFERENCES answer_segment(id),
  answer_version TEXT NOT NULL,
  question_id UUID NOT NULL REFERENCES question(id),
  rubric_version TEXT NOT NULL,
  model_version TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  min_confidence NUMERIC(5,4) NOT NULL DEFAULT 0,
  request_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  attempt_count INT NOT NULL DEFAULT 0,
  grade_id UUID REFERENCES ai_grade(id),
  error_code TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT uq_subjective_grading_run_request UNIQUE (tenant_id, request_id),
  CONSTRAINT chk_subjective_grading_run_status CHECK (status IN ('queued', 'processing', 'succeeded', 'failed', 'conflict')),
  CONSTRAINT chk_subjective_grading_run_attempts CHECK (attempt_count >= 0),
  CONSTRAINT chk_subjective_grading_run_confidence CHECK (min_confidence >= 0 AND min_confidence <= 1)
);

CREATE INDEX IF NOT EXISTS idx_subjective_grading_run_segment
  ON subjective_grading_run (tenant_id, answer_segment_id, created_at DESC);

ALTER TABLE ai_grade
  ADD COLUMN IF NOT EXISTS subjective_grading_run_id UUID;

CREATE INDEX IF NOT EXISTS idx_ai_grade_subjective_run
  ON ai_grade (tenant_id, subjective_grading_run_id)
  WHERE subjective_grading_run_id IS NOT NULL AND deleted_at IS NULL;

COMMENT ON TABLE subjective_grading_run IS
'Durable source record for governed subjective AI suggestions. It never represents a final teacher-confirmed score.';
