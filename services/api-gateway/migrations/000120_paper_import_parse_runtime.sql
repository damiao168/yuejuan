ALTER TABLE agent_worker_task
  DROP CONSTRAINT chk_agent_worker_task_type;

ALTER TABLE agent_worker_task
  ADD CONSTRAINT chk_agent_worker_task_type
  CHECK (task_type IN (
    'ocr', 'layout', 'preprocess', 'image_quality', 'ai_grade',
    'evidence_verify', 'report_generate', 'export', 'desktop_sync',
    'capture_file_decode', 'page_registration', 'answer_segment_crop',
    'page_registration_correction_preview', 'omr_extract', 'paper_parse'
  ));

CREATE TABLE paper_import_parse_input (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  paper_import_id UUID NOT NULL REFERENCES paper_import_job(id),
  source_revision TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  documents JSONB NOT NULL,
  extra_issues JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_paper_import_parse_input_tenant_id UNIQUE (tenant_id, id),
  CONSTRAINT uq_paper_import_parse_input_revision
    UNIQUE (tenant_id, paper_import_id, source_revision, input_hash),
  CONSTRAINT fk_paper_import_parse_input_creator_tenant
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id)
);

CREATE INDEX idx_paper_import_parse_input_job
ON paper_import_parse_input (tenant_id, paper_import_id, created_at DESC);

COMMENT ON TABLE paper_import_parse_input IS
'Immutable, version-bound input for restart-safe paper parsing. Worker task payloads carry only this row reference.';
