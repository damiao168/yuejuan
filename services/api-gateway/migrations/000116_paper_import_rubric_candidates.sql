ALTER TABLE paper_import_job
  ADD COLUMN IF NOT EXISTS rubric_candidates jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE paper_import_source DROP CONSTRAINT IF EXISTS paper_import_source_role_hint_check;
ALTER TABLE paper_import_source DROP CONSTRAINT IF EXISTS paper_import_source_detected_role_check;
ALTER TABLE paper_import_source ADD CONSTRAINT paper_import_source_role_hint_check CHECK (role_hint IN ('auto','question','answer','solution','rubric','mixed','unknown'));
ALTER TABLE paper_import_source ADD CONSTRAINT paper_import_source_detected_role_check CHECK (detected_role IN ('question','answer','solution','rubric','mixed','unknown'));

ALTER TABLE question_rubric
  ADD COLUMN IF NOT EXISTS paper_import_candidate_id text;
