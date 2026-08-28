ALTER TABLE grade
  ADD COLUMN IF NOT EXISTS education_stage TEXT;

UPDATE grade
SET education_stage = CASE
  WHEN name LIKE '%高%' OR level_no >= 10 THEN 'senior'
  ELSE 'junior'
END
WHERE education_stage IS NULL OR education_stage = '';

ALTER TABLE grade
  ALTER COLUMN education_stage SET DEFAULT 'junior',
  ALTER COLUMN education_stage SET NOT NULL;

ALTER TABLE grade
  DROP CONSTRAINT IF EXISTS grade_education_stage_check;

ALTER TABLE grade
  ADD CONSTRAINT grade_education_stage_check
  CHECK (education_stage IN ('junior', 'senior'));

CREATE INDEX IF NOT EXISTS idx_grade_stage
  ON grade (tenant_id, school_id, education_stage, status)
  WHERE deleted_at IS NULL;
