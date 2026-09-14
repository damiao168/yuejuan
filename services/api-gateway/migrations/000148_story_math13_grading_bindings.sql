-- STORY-MATH-13: bind advisory math grading runs and suggestions to the exact
-- verified evidence projection used by the deterministic server scorer.

ALTER TABLE math_understanding_artifact
  ADD CONSTRAINT uq_math_artifact_grading_binding
  UNIQUE (tenant_id, id, version, answer_segment_id);

ALTER TABLE subjective_grading_run
  ADD COLUMN IF NOT EXISTS math_artifact_id UUID,
  ADD COLUMN IF NOT EXISTS math_artifact_version BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS math_correction_revision BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS math_scoring_version TEXT NOT NULL DEFAULT '';

ALTER TABLE ai_grade
  ADD COLUMN IF NOT EXISTS math_artifact_id UUID,
  ADD COLUMN IF NOT EXISTS math_artifact_version BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS math_correction_revision BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS math_scoring_version TEXT NOT NULL DEFAULT '';

ALTER TABLE subjective_grading_run
  ADD CONSTRAINT fk_subjective_run_math_artifact
  FOREIGN KEY (tenant_id, math_artifact_id, math_artifact_version, answer_segment_id)
  REFERENCES math_understanding_artifact(tenant_id, id, version, answer_segment_id),
  ADD CONSTRAINT chk_subjective_run_math_binding CHECK (
    (math_artifact_id IS NULL AND math_artifact_version = 0 AND math_correction_revision = 0 AND math_scoring_version = '') OR
    (math_artifact_id IS NOT NULL AND math_artifact_version > 0 AND math_correction_revision >= 0 AND math_scoring_version <> '')
  );

ALTER TABLE ai_grade
  ADD CONSTRAINT fk_ai_grade_math_artifact
  FOREIGN KEY (tenant_id, math_artifact_id, math_artifact_version, answer_segment_id)
  REFERENCES math_understanding_artifact(tenant_id, id, version, answer_segment_id),
  ADD CONSTRAINT chk_ai_grade_math_binding CHECK (
    (math_artifact_id IS NULL AND math_artifact_version = 0 AND math_correction_revision = 0 AND math_scoring_version = '') OR
    (math_artifact_id IS NOT NULL AND math_artifact_version > 0 AND math_correction_revision >= 0 AND math_scoring_version <> '')
  );

CREATE INDEX IF NOT EXISTS idx_subjective_run_math_artifact
  ON subjective_grading_run (tenant_id, math_artifact_id, math_artifact_version, math_correction_revision)
  WHERE math_artifact_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ai_grade_math_artifact
  ON ai_grade (tenant_id, math_artifact_id, math_artifact_version, math_correction_revision)
  WHERE math_artifact_id IS NOT NULL AND deleted_at IS NULL;

COMMENT ON COLUMN ai_grade.math_scoring_version IS
'Server-owned deterministic math scorer version. Model responses never populate suggested_score directly.';
