ALTER TABLE submission
  ADD COLUMN IF NOT EXISTS identity_status TEXT NOT NULL DEFAULT 'unassigned',
  ADD COLUMN IF NOT EXISTS identity_revision INT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS identity_evidence JSONB NOT NULL DEFAULT '{}';

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_submission_identity_status_story055') THEN
    ALTER TABLE submission ADD CONSTRAINT chk_submission_identity_status_story055
      CHECK (identity_status IN ('unassigned', 'matched', 'unknown', 'conflict'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_submission_identity_revision_story055') THEN
    ALTER TABLE submission ADD CONSTRAINT chk_submission_identity_revision_story055
      CHECK (identity_revision > 0);
  END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_submission_exam_student_active
ON submission (tenant_id, exam_id, student_id)
WHERE student_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_submission_identity_queue
ON submission (tenant_id, exam_id, identity_status, created_at)
WHERE deleted_at IS NULL;

COMMENT ON COLUMN submission.identity_evidence IS
'Operator-confirmed candidate/page evidence only; raw OCR or barcode candidates remain untrusted until confirmed.';
