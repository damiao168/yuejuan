-- STORY-060A: keep physical-sheet replacement and revocation explainable.
-- A reprint creates a new immutable serial and the old serial is never reused.

ALTER TABLE answer_sheet_print_batch
  ADD COLUMN operation TEXT NOT NULL DEFAULT 'initial',
  ADD COLUMN reason TEXT;

ALTER TABLE answer_sheet_print_batch
  ADD CONSTRAINT ck_answer_sheet_print_batch_operation
    CHECK (operation IN ('initial', 'reprint')),
  ADD CONSTRAINT ck_answer_sheet_print_batch_reason
    CHECK (
      (operation='initial' AND reason IS NULL)
      OR (operation='reprint' AND length(btrim(reason)) > 0)
    );

ALTER TABLE answer_sheet_print_sheet
  ADD COLUMN supersedes_sheet_id UUID,
  ADD COLUMN revoked_by UUID,
  ADD COLUMN revoked_at TIMESTAMPTZ,
  ADD COLUMN revoke_reason TEXT;

ALTER TABLE answer_sheet_print_sheet
  ADD CONSTRAINT fk_answer_sheet_print_sheet_supersedes_tenant
    FOREIGN KEY (tenant_id, supersedes_sheet_id)
    REFERENCES answer_sheet_print_sheet (tenant_id, id),
  ADD CONSTRAINT fk_answer_sheet_print_sheet_revoked_by_tenant
    FOREIGN KEY (tenant_id, revoked_by)
    REFERENCES app_user (tenant_id, id),
  ADD CONSTRAINT ck_answer_sheet_print_sheet_revocation
    CHECK (
      (status <> 'revoked' AND revoked_by IS NULL AND revoked_at IS NULL AND revoke_reason IS NULL)
      OR (
        status = 'revoked'
        AND revoked_by IS NOT NULL
        AND revoked_at IS NOT NULL
        AND length(btrim(revoke_reason)) > 0
      )
    ) NOT VALID;

CREATE UNIQUE INDEX uq_answer_sheet_print_sheet_replacement
ON answer_sheet_print_sheet (tenant_id, supersedes_sheet_id)
WHERE supersedes_sheet_id IS NOT NULL;
