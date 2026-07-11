ALTER TABLE agent_worker_task
  DROP CONSTRAINT chk_agent_worker_task_type;

ALTER TABLE agent_worker_task
  ADD CONSTRAINT chk_agent_worker_task_type
  CHECK (task_type IN (
    'ocr', 'layout', 'preprocess', 'image_quality', 'ai_grade',
    'evidence_verify', 'report_generate', 'export', 'desktop_sync',
    'capture_file_decode', 'page_registration', 'answer_segment_crop',
    'page_registration_correction_preview'
  ));

ALTER TABLE capture_operation
  DROP CONSTRAINT capture_operation_operation_check;

ALTER TABLE capture_operation
  ADD CONSTRAINT capture_operation_operation_check
  CHECK (operation IN (
    'rotate', 'reorder', 'split', 'merge', 'delete', 'restore',
    'student_match', 'page_match', 'override', 'reopen', 'cancel', 'complete',
    'registration_correction_apply', 'registration_correction_undo'
  ));

CREATE TABLE page_registration_correction (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  capture_page_id UUID NOT NULL,
  base_registration_run_id UUID NOT NULL,
  source_page_revision INT NOT NULL,
  template_id UUID NOT NULL,
  template_content_hash TEXT NOT NULL,
  page_no INT NOT NULL,
  source_points JSONB NOT NULL,
  template_points JSONB NOT NULL,
  advanced_anchor_mode BOOLEAN NOT NULL DEFAULT false,
  status TEXT NOT NULL DEFAULT 'draft',
  revision INT NOT NULL DEFAULT 1,
  attempt_count INT NOT NULL DEFAULT 0,
  points_hash TEXT NOT NULL,
  preview_registered_file_asset_id UUID,
  preview_segments JSONB NOT NULL DEFAULT '[]',
  source_to_template_matrix JSONB NOT NULL DEFAULT '[]',
  template_to_source_matrix JSONB NOT NULL DEFAULT '[]',
  coverage DOUBLE PRECISION,
  reprojection_error DOUBLE PRECISION,
  validation_report JSONB NOT NULL DEFAULT '{}',
  runtime_task_id UUID,
  error_code TEXT,
  created_by UUID NOT NULL,
  applied_by UUID,
  reason TEXT,
  previous_registration_snapshot JSONB NOT NULL DEFAULT '{}',
  expires_at TIMESTAMPTZ NOT NULL,
  applied_at TIMESTAMPTZ,
  undone_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, capture_page_id) REFERENCES capture_page(tenant_id, id),
  FOREIGN KEY (tenant_id, base_registration_run_id) REFERENCES page_registration_run(tenant_id, id),
  FOREIGN KEY (tenant_id, template_id) REFERENCES answer_sheet_template(tenant_id, id),
  FOREIGN KEY (tenant_id, preview_registered_file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, runtime_task_id) REFERENCES agent_worker_task(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, applied_by) REFERENCES app_user(tenant_id, id),
  CHECK (source_page_revision > 0 AND page_no > 0 AND revision > 0),
  CHECK (attempt_count >= 0 AND attempt_count <= 8),
  CHECK (jsonb_typeof(source_points) = 'array' AND jsonb_array_length(source_points) = 4),
  CHECK (jsonb_typeof(template_points) = 'array' AND jsonb_array_length(template_points) = 4),
  CHECK (status IN ('draft', 'queued', 'preview_ready', 'failed', 'expired', 'superseded', 'applied', 'undone')),
  CHECK (coverage IS NULL OR (coverage >= 0 AND coverage <= 1.25)),
  CHECK (reprojection_error IS NULL OR reprojection_error >= 0)
);

CREATE INDEX idx_registration_correction_page
ON page_registration_correction (tenant_id, capture_page_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_registration_correction_active_operator
ON page_registration_correction (tenant_id, capture_page_id, created_by)
WHERE status IN ('draft', 'queued', 'preview_ready') AND deleted_at IS NULL;
