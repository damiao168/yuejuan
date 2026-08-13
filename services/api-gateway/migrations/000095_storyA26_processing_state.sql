-- STORY-A26: a stable, tenant-scoped operational projection over the
-- capture, image-quality, OCR, registration and segmentation pipelines.
-- These records hold no answer text or image bytes; the source systems remain
-- the evidence of record.

CREATE TABLE IF NOT EXISTS submission_page_processing_state (
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  page_id UUID NOT NULL REFERENCES submission_page(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  current_stage TEXT NOT NULL,
  blocking BOOLEAN NOT NULL DEFAULT false,
  issue_code TEXT,
  retryable BOOLEAN NOT NULL DEFAULT false,
  retry_source_type TEXT,
  retry_source_id UUID,
  parser_quality_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, page_id),
  CHECK (current_stage IN ('RECEIVED', 'VALIDATED', 'QUALITY_CHECKED', 'REGISTERED', 'IDENTIFIED', 'PARSED', 'SEGMENTED', 'READY')),
  CHECK (issue_code IS NULL OR issue_code IN (
    'BLOCKED_MISSING_IDENTITY', 'BLOCKED_MISSING_PAGE', 'BLOCKED_BAD_ALIGNMENT',
    'BLOCKED_LOW_IMAGE_QUALITY', 'OCR_LOW_CONFIDENCE', 'MATH_PARSE_FAILED',
    'CHEMISTRY_PARSE_FAILED', 'TABLE_PARSE_FAILED', 'DIAGRAM_PARSE_FAILED', 'SEGMENTATION_FAILED'
  ))
);

CREATE INDEX IF NOT EXISTS idx_submission_page_processing_state_exam_stage
  ON submission_page_processing_state (tenant_id, exam_id, current_stage, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_submission_page_processing_state_exam_issue
  ON submission_page_processing_state (tenant_id, exam_id, issue_code, updated_at DESC)
  WHERE issue_code IS NOT NULL;

CREATE TABLE IF NOT EXISTS operational_exception (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  page_id UUID NOT NULL REFERENCES submission_page(id),
  code TEXT NOT NULL,
  severity TEXT NOT NULL,
  blocking BOOLEAN NOT NULL DEFAULT false,
  status TEXT NOT NULL DEFAULT 'open',
  assigned_to UUID REFERENCES app_user(id),
  details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  resolution TEXT,
  UNIQUE (tenant_id, source_type, source_id, code),
  CHECK (severity IN ('P0', 'P1', 'P2', 'P3')),
  CHECK (status IN ('open', 'assigned', 'resolved')),
  CHECK ((status = 'resolved') = (resolved_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_operational_exception_list
  ON operational_exception (tenant_id, exam_id, status, severity, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_operational_exception_page
  ON operational_exception (tenant_id, page_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_operational_exception_assignee
  ON operational_exception (tenant_id, assigned_to, status, updated_at DESC)
  WHERE assigned_to IS NOT NULL;
