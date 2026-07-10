ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'succeeded';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS failure_reason TEXT;
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS model_version TEXT NOT NULL DEFAULT 'rule-based-objective';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS prompt_version TEXT NOT NULL DEFAULT 'rule-prompt-none';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS student_feedback TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_grade ADD COLUMN IF NOT EXISTS teacher_note TEXT NOT NULL DEFAULT '';

ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ai_grade_question_type_check;
ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ck_ai_grade_question_type;
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_grade_question_type CHECK (
  question_type IN (
    'single_choice', 'multiple_choice', 'true_false', 'fill_blank', 'numeric',
    'short_answer', 'calculation', 'essay', 'discussion'
  )
);

ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ai_grade_grader_type_check;
ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ck_ai_grade_grader_type;
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_grade_grader_type CHECK (
  grader_type IN ('rule_based_objective', 'mock_llm_subjective', 'llm_subjective')
);

ALTER TABLE ai_grade DROP CONSTRAINT IF EXISTS ck_ai_grade_status;
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_grade_status CHECK (status IN ('succeeded', 'failed'));

CREATE INDEX IF NOT EXISTS idx_ai_grade_status ON ai_grade (tenant_id, status, created_at DESC);
