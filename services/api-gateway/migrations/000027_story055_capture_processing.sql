INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, 'capture:manage', 'Manage capture batches', 'capture', 'manage', '创建采集批次并处理答卷页面'
FROM tenant t
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id AND p.code = 'capture:manage'
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE capture_batch (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  name TEXT NOT NULL,
  source_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  revision INT NOT NULL DEFAULT 1,
  operator_id UUID NOT NULL,
  scanner_device TEXT,
  idempotency_key TEXT,
  file_count INT NOT NULL DEFAULT 0,
  page_count INT NOT NULL DEFAULT 0,
  submission_count INT NOT NULL DEFAULT 0,
  normal_count INT NOT NULL DEFAULT 0,
  review_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, operator_id) REFERENCES app_user(tenant_id, id),
  CHECK (length(btrim(name)) BETWEEN 1 AND 120),
  CHECK (source_type IN ('web_upload', 'scanner_upload', 'folder_import', 'desktop_sync')),
  CHECK (status IN ('draft', 'uploading', 'matching', 'processing', 'needs_review', 'ready', 'completed', 'cancelled')),
  CHECK (revision > 0),
  CHECK (file_count >= 0 AND page_count >= 0 AND submission_count >= 0 AND normal_count >= 0 AND review_count >= 0 AND failed_count >= 0)
);

CREATE UNIQUE INDEX idx_capture_batch_idempotency
ON capture_batch (tenant_id, exam_id, idempotency_key)
WHERE idempotency_key IS NOT NULL AND idempotency_key <> '' AND deleted_at IS NULL;

CREATE INDEX idx_capture_batch_exam
ON capture_batch (tenant_id, exam_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE TABLE capture_file (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  capture_batch_id UUID NOT NULL,
  file_asset_id UUID NOT NULL,
  original_name TEXT NOT NULL,
  content_type TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  byte_size BIGINT NOT NULL,
  page_count INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'uploaded',
  error_code TEXT,
  idempotency_key TEXT NOT NULL,
  uploaded_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, capture_batch_id, idempotency_key),
  FOREIGN KEY (tenant_id, capture_batch_id) REFERENCES capture_batch(tenant_id, id),
  FOREIGN KEY (tenant_id, file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, uploaded_by) REFERENCES app_user(tenant_id, id),
  CHECK (byte_size > 0),
  CHECK (page_count >= 0),
  CHECK (status IN ('uploaded', 'queued', 'processing', 'completed', 'duplicate', 'failed'))
);

CREATE INDEX idx_capture_file_batch
ON capture_file (tenant_id, capture_batch_id, created_at)
WHERE deleted_at IS NULL;

CREATE TABLE capture_page (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  capture_batch_id UUID NOT NULL,
  capture_file_id UUID NOT NULL,
  source_index INT NOT NULL,
  submission_id UUID,
  submission_page_id UUID,
  assigned_page_no INT,
  sequence_no INT NOT NULL,
  rotation_degrees INT NOT NULL DEFAULT 0,
  decoded_file_asset_id UUID NOT NULL,
  status TEXT NOT NULL DEFAULT 'decoded',
  duplicate_of_page_id UUID,
  revision INT NOT NULL DEFAULT 1,
  page_identity JSONB NOT NULL DEFAULT '{}',
  match_candidates JSONB NOT NULL DEFAULT '[]',
  manual_override JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, capture_file_id, source_index),
  FOREIGN KEY (tenant_id, capture_batch_id) REFERENCES capture_batch(tenant_id, id),
  FOREIGN KEY (tenant_id, capture_file_id) REFERENCES capture_file(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_page_id) REFERENCES submission_page(tenant_id, id),
  FOREIGN KEY (tenant_id, decoded_file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, duplicate_of_page_id) REFERENCES capture_page(tenant_id, id),
  CHECK (source_index > 0 AND sequence_no > 0),
  CHECK (assigned_page_no IS NULL OR assigned_page_no > 0),
  CHECK (rotation_degrees IN (0, 90, 180, 270)),
  CHECK (revision > 0),
  CHECK (status IN ('decoded', 'grouped', 'quality_checking', 'normalized', 'page_matching', 'registration', 'segmenting', 'ready', 'needs_review', 'quality_rejected', 'failed', 'deleted'))
);

CREATE INDEX idx_capture_page_batch
ON capture_page (tenant_id, capture_batch_id, sequence_no)
WHERE deleted_at IS NULL;

CREATE INDEX idx_capture_page_submission
ON capture_page (tenant_id, submission_id, assigned_page_no)
WHERE deleted_at IS NULL AND submission_id IS NOT NULL;

CREATE TABLE page_registration_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  capture_page_id UUID NOT NULL,
  submission_page_id UUID NOT NULL,
  source_file_asset_id UUID NOT NULL,
  source_sha256 TEXT NOT NULL,
  template_id UUID NOT NULL,
  template_content_hash TEXT NOT NULL,
  page_no INT NOT NULL,
  processing_status TEXT NOT NULL DEFAULT 'pending',
  match_status TEXT,
  confidence DOUBLE PRECISION,
  method TEXT,
  profile_version TEXT NOT NULL,
  source_to_template_matrix JSONB NOT NULL DEFAULT '[]',
  template_to_source_matrix JSONB NOT NULL DEFAULT '[]',
  feature_count INT NOT NULL DEFAULT 0,
  match_count INT NOT NULL DEFAULT 0,
  inlier_count INT NOT NULL DEFAULT 0,
  inlier_ratio DOUBLE PRECISION,
  reprojection_error DOUBLE PRECISION,
  registered_file_asset_id UUID,
  runtime_task_id UUID,
  attempt_no INT NOT NULL DEFAULT 0,
  result_version TEXT,
  error_code TEXT,
  error_detail JSONB NOT NULL DEFAULT '{}',
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  duration_ms INT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, capture_page_id) REFERENCES capture_page(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_page_id) REFERENCES submission_page(tenant_id, id),
  FOREIGN KEY (tenant_id, source_file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, template_id) REFERENCES answer_sheet_template(tenant_id, id),
  FOREIGN KEY (tenant_id, registered_file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, runtime_task_id) REFERENCES agent_worker_task(tenant_id, id),
  CHECK (page_no > 0),
  CHECK (processing_status IN ('pending', 'processing', 'completed', 'retryable_error', 'terminal_error', 'invalidated')),
  CHECK (match_status IS NULL OR match_status IN ('matched', 'needs_review', 'failed')),
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
  CHECK (feature_count >= 0 AND match_count >= 0 AND inlier_count >= 0),
  CHECK (inlier_ratio IS NULL OR (inlier_ratio >= 0 AND inlier_ratio <= 1)),
  CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE INDEX idx_page_registration_current
ON page_registration_run (tenant_id, capture_page_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE TABLE capture_operation (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  capture_batch_id UUID NOT NULL,
  operation TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id UUID NOT NULL,
  actor_id UUID NOT NULL,
  reason TEXT,
  before_state JSONB NOT NULL DEFAULT '{}',
  after_state JSONB NOT NULL DEFAULT '{}',
  request_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, capture_batch_id) REFERENCES capture_batch(tenant_id, id),
  FOREIGN KEY (tenant_id, actor_id) REFERENCES app_user(tenant_id, id),
  CHECK (operation IN ('rotate', 'reorder', 'split', 'merge', 'delete', 'restore', 'student_match', 'page_match', 'override', 'reopen', 'cancel'))
);

ALTER TABLE answer_segment
  ADD COLUMN template_id UUID,
  ADD COLUMN template_content_hash TEXT,
  ADD COLUMN registration_run_id UUID,
  ADD COLUMN normalized_bbox JSONB,
  ADD COLUMN pixel_bbox JSONB,
  ADD COLUMN crop_file_asset_id UUID,
  ADD COLUMN crop_sha256 TEXT,
  ADD COLUMN question_version INT,
  ADD COLUMN processing_status TEXT NOT NULL DEFAULT 'completed',
  ADD COLUMN confidence DOUBLE PRECISION;

ALTER TABLE answer_segment
  ADD CONSTRAINT fk_answer_segment_template_tenant
    FOREIGN KEY (tenant_id, template_id) REFERENCES answer_sheet_template(tenant_id, id),
  ADD CONSTRAINT fk_answer_segment_registration_tenant
    FOREIGN KEY (tenant_id, registration_run_id) REFERENCES page_registration_run(tenant_id, id),
  ADD CONSTRAINT fk_answer_segment_crop_file_tenant
    FOREIGN KEY (tenant_id, crop_file_asset_id) REFERENCES file_asset(tenant_id, id),
  ADD CONSTRAINT chk_answer_segment_processing_status
    CHECK (processing_status IN ('pending', 'processing', 'completed', 'retryable_error', 'terminal_error', 'invalidated')),
  ADD CONSTRAINT chk_answer_segment_confidence
    CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1));

CREATE INDEX idx_answer_segment_registration
ON answer_segment (tenant_id, registration_run_id)
WHERE registration_run_id IS NOT NULL AND deleted_at IS NULL;
