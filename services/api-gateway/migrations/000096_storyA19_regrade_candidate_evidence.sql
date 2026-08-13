-- STORY-A19: retain the new-rubric evidence behind a candidate score. These
-- remain isolated regrade facts and never rewrite historic grades/releases.

ALTER TABLE regrade_item
  ADD COLUMN candidate_rubric_selections JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN candidate_comment TEXT NOT NULL DEFAULT '';

ALTER TABLE regrade_item
  ADD CONSTRAINT chk_regrade_item_candidate_rubric_selections
  CHECK (jsonb_typeof(candidate_rubric_selections) = 'array');

ALTER TABLE regrade_item
  ADD CONSTRAINT chk_regrade_item_candidate_comment
  CHECK (octet_length(candidate_comment) <= 2000);
