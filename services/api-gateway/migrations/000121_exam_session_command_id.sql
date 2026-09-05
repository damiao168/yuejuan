ALTER TABLE exam_session
  ADD COLUMN command_id TEXT;

CREATE UNIQUE INDEX uq_exam_session_command
ON exam_session (tenant_id, created_by, command_id)
WHERE command_id IS NOT NULL;

ALTER TABLE exam_session
  ADD CONSTRAINT exam_session_command_id_check
  CHECK (command_id IS NULL OR length(command_id) BETWEEN 8 AND 128);

COMMENT ON COLUMN exam_session.command_id IS
'Stable user-operation identity reused after a lost response; committed with the complete multi-subject session.';
