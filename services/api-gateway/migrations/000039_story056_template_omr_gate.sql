-- STORY-056 implementation-review fix: snapshot the server-side permission to turn an OMR suggestion
-- into an automatic grade. A worker result must never be able to grant this
-- permission by claiming a high confidence value or a profile version.
ALTER TABLE omr_run
  ADD COLUMN auto_confirm_eligible BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN auto_confirm_reason TEXT NOT NULL DEFAULT 'template_profile_unverified';

ALTER TABLE omr_run
  ADD CONSTRAINT chk_omr_run_auto_confirm_reason
  CHECK (length(btrim(auto_confirm_reason)) BETWEEN 1 AND 128);
