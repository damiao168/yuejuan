ALTER TABLE paper_import_run
  ADD COLUMN authoritative_subject_code TEXT NOT NULL DEFAULT '',
  ADD COLUMN recognition_policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN recognition_policy_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE paper_import_formula_input (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  paper_import_id UUID NOT NULL REFERENCES paper_import_job(id),
  run_id UUID NOT NULL REFERENCES paper_import_run(id),
  generation BIGINT NOT NULL,
  source_revision TEXT NOT NULL,
  policy_hash TEXT NOT NULL,
  pages JSONB NOT NULL,
  documents JSONB NOT NULL,
  extra_issues JSONB NOT NULL DEFAULT '[]'::jsonb,
  result JSONB,
  status TEXT NOT NULL DEFAULT 'pending',
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  CONSTRAINT chk_paper_import_formula_input_status CHECK (status IN ('pending','running','succeeded','review_required','failed')),
  CONSTRAINT uq_paper_import_formula_input_run UNIQUE (tenant_id, run_id),
  CONSTRAINT fk_paper_import_formula_input_creator_tenant FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id)
);

CREATE INDEX idx_paper_import_formula_input_job
ON paper_import_formula_input (tenant_id, paper_import_id, generation DESC);

ALTER TABLE agent_worker_task DROP CONSTRAINT chk_agent_worker_task_type;
ALTER TABLE agent_worker_task ADD CONSTRAINT chk_agent_worker_task_type CHECK (task_type IN (
  'ocr', 'layout', 'preprocess', 'image_quality', 'ai_grade', 'evidence_verify',
  'report_generate', 'export', 'desktop_sync', 'capture_file_decode',
  'page_registration', 'answer_segment_crop', 'page_registration_correction_preview',
  'omr_extract', 'paper_parse', 'paper_formula'
));

COMMENT ON TABLE paper_import_formula_input IS
'Immutable formula-stage barrier input and auditable detector/M/L candidate output for one paper import generation.';
