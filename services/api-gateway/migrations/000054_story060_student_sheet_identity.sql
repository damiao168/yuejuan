-- STORY-060A: make student-bound answer-sheet identity durable.
-- Signed payloads alone cannot prove that a serial was actually issued, nor
-- can JSON-only observations safely detect the same physical page appearing
-- in another capture file. These tables freeze issuance and the typed capture
-- columns make duplicate detection queryable and tenant constrained.

CREATE TABLE answer_sheet_print_batch (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  template_id UUID NOT NULL,
  template_content_hash TEXT NOT NULL,
  key_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  sheet_count INT NOT NULL,
  page_count INT NOT NULL,
  issued_by UUID NOT NULL,
  issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, template_id, idempotency_key),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam (tenant_id, id),
  FOREIGN KEY (tenant_id, template_id) REFERENCES answer_sheet_template (tenant_id, id),
  FOREIGN KEY (tenant_id, issued_by) REFERENCES app_user (tenant_id, id),
  CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 120),
  CHECK (length(btrim(template_content_hash)) > 0),
  CHECK (length(btrim(key_id)) > 0),
  CHECK (sheet_count BETWEEN 1 AND 2000),
  CHECK (page_count BETWEEN 1 AND 100)
);

CREATE INDEX idx_answer_sheet_print_batch_exam
ON answer_sheet_print_batch (tenant_id, exam_id, issued_at DESC);

CREATE TABLE answer_sheet_print_sheet (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  print_batch_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  template_id UUID NOT NULL,
  template_content_hash TEXT NOT NULL,
  student_id UUID NOT NULL,
  ordinal INT NOT NULL,
  status TEXT NOT NULL DEFAULT 'issued',
  first_observed_at TIMESTAMPTZ,
  last_observed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, print_batch_id, student_id),
  UNIQUE (tenant_id, print_batch_id, ordinal),
  FOREIGN KEY (tenant_id, print_batch_id) REFERENCES answer_sheet_print_batch (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam (tenant_id, id),
  FOREIGN KEY (tenant_id, template_id) REFERENCES answer_sheet_template (tenant_id, id),
  FOREIGN KEY (tenant_id, student_id) REFERENCES student (tenant_id, id),
  CHECK (ordinal > 0),
  CHECK (status IN ('issued', 'observed', 'conflict', 'revoked'))
);

CREATE INDEX idx_answer_sheet_print_sheet_exam_student
ON answer_sheet_print_sheet (tenant_id, exam_id, student_id);

CREATE TABLE answer_sheet_print_page (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  print_sheet_id UUID NOT NULL,
  page_no INT NOT NULL,
  nonce UUID NOT NULL,
  barcode_value TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, print_sheet_id, page_no),
  UNIQUE (tenant_id, barcode_value),
  FOREIGN KEY (tenant_id, print_sheet_id) REFERENCES answer_sheet_print_sheet (tenant_id, id),
  CHECK (page_no > 0),
  CHECK (length(barcode_value) BETWEEN 1 AND 2048)
);

ALTER TABLE capture_page
  ADD COLUMN barcode_student_id UUID,
  ADD COLUMN barcode_template_id UUID,
  ADD COLUMN sheet_serial UUID;

ALTER TABLE capture_page
  ADD CONSTRAINT fk_capture_page_barcode_student_tenant
    FOREIGN KEY (tenant_id, barcode_student_id) REFERENCES student (tenant_id, id),
  ADD CONSTRAINT fk_capture_page_barcode_template_tenant
    FOREIGN KEY (tenant_id, barcode_template_id) REFERENCES answer_sheet_template (tenant_id, id),
  ADD CONSTRAINT fk_capture_page_sheet_serial_tenant
    FOREIGN KEY (tenant_id, sheet_serial) REFERENCES answer_sheet_print_sheet (tenant_id, id);

CREATE INDEX idx_capture_page_sheet_page
ON capture_page (tenant_id, sheet_serial, assigned_page_no, created_at)
WHERE sheet_serial IS NOT NULL AND deleted_at IS NULL AND status <> 'deleted';
