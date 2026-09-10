-- The answer-sheet template version selected for an exam is independent from
-- the template's immutable lifecycle state. Absence of a row means UNBOUND.
CREATE TABLE exam_answer_sheet_template_binding (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  template_id UUID NOT NULL,
  template_content_hash TEXT NOT NULL,
  mode TEXT NOT NULL DEFAULT 'locked_with_guard',
  source TEXT NOT NULL DEFAULT 'manual',
  revision INT NOT NULL DEFAULT 1,
  bound_by UUID NOT NULL,
  bound_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, exam_id),
  UNIQUE (tenant_id, id),
  CONSTRAINT fk_exam_answer_sheet_template_binding_exam FOREIGN KEY (tenant_id, exam_id)
    REFERENCES exam (tenant_id, id),
  CONSTRAINT fk_exam_answer_sheet_template_binding_template FOREIGN KEY (tenant_id, template_id)
    REFERENCES answer_sheet_template (tenant_id, id),
  CONSTRAINT fk_exam_answer_sheet_template_binding_actor FOREIGN KEY (tenant_id, bound_by)
    REFERENCES app_user (tenant_id, id),
  CHECK (mode IN ('bound_auto', 'locked_with_guard')),
  CHECK (source IN ('automatic', 'manual')),
  CHECK (revision > 0),
  CHECK (length(template_content_hash) > 0)
);

CREATE INDEX idx_exam_answer_sheet_template_binding_template
ON exam_answer_sheet_template_binding (tenant_id, template_id);

ALTER TABLE page_registration_run
  ADD COLUMN routing_mode TEXT NOT NULL DEFAULT 'single_template',
  ADD COLUMN guard_report JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD CONSTRAINT ck_page_registration_routing_mode CHECK (
    routing_mode IN ('barcode', 'single_template', 'bound_auto', 'locked_with_guard')
  );

CREATE TABLE page_template_match_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  capture_page_id UUID NOT NULL,
  submission_page_id UUID NOT NULL,
  source_file_asset_id UUID NOT NULL,
  source_sha256 TEXT NOT NULL,
  source_page_revision INT NOT NULL,
  processing_status TEXT NOT NULL DEFAULT 'processing',
  decision TEXT,
  selected_template_id UUID,
  selected_template_content_hash TEXT,
  score NUMERIC(8,6),
  margin NUMERIC(8,6),
  candidates JSONB NOT NULL DEFAULT '[]'::jsonb,
  profile_version TEXT NOT NULL DEFAULT 'phash-orb-ransac-v1',
  runtime_task_id UUID,
  result_version TEXT,
  error_code TEXT,
  error_detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  duration_ms INT,
  created_by UUID NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  CONSTRAINT fk_page_template_match_exam FOREIGN KEY (tenant_id, exam_id) REFERENCES exam (tenant_id, id),
  CONSTRAINT fk_page_template_match_capture_page FOREIGN KEY (tenant_id, capture_page_id) REFERENCES capture_page (tenant_id, id),
  CONSTRAINT fk_page_template_match_submission_page FOREIGN KEY (tenant_id, submission_page_id) REFERENCES submission_page (tenant_id, id),
  CONSTRAINT fk_page_template_match_source_asset FOREIGN KEY (tenant_id, source_file_asset_id) REFERENCES file_asset (tenant_id, id),
  CONSTRAINT fk_page_template_match_selected_template FOREIGN KEY (tenant_id, selected_template_id) REFERENCES answer_sheet_template (tenant_id, id),
  CONSTRAINT fk_page_template_match_runtime_task FOREIGN KEY (tenant_id, runtime_task_id) REFERENCES agent_worker_task (tenant_id, id),
  CONSTRAINT fk_page_template_match_actor FOREIGN KEY (tenant_id, created_by) REFERENCES app_user (tenant_id, id),
  CHECK (processing_status IN ('processing', 'completed', 'retryable_error', 'terminal_error')),
  CHECK (decision IS NULL OR decision IN ('matched', 'ambiguous', 'unknown', 'conflict')),
  CHECK (score IS NULL OR (score >= 0 AND score <= 1)),
  CHECK (margin IS NULL OR (margin >= 0 AND margin <= 1)),
  CHECK (source_page_revision > 0),
  CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE UNIQUE INDEX uq_page_template_match_active_source
ON page_template_match_run (tenant_id, capture_page_id, source_sha256, source_page_revision)
WHERE processing_status IN ('processing', 'completed') AND deleted_at IS NULL;

CREATE INDEX idx_page_template_match_review
ON page_template_match_run (tenant_id, exam_id, decision, created_at DESC)
WHERE deleted_at IS NULL;
