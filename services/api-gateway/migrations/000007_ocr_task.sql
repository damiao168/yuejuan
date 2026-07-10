INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('ocr:manage', 'Manage OCR tasks', 'ocr', 'manage', '创建和管理 OCR 任务')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'ocr:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS ocr_task (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  status TEXT NOT NULL,
  engine TEXT NOT NULL,
  engine_version TEXT NOT NULL,
  min_confidence NUMERIC(5,4) NOT NULL,
  result_count INT NOT NULL DEFAULT 0,
  requires_human_review BOOLEAN NOT NULL DEFAULT false,
  error_message TEXT,
  requested_by UUID NOT NULL REFERENCES app_user(id),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (status IN ('queued', 'processing', 'completed', 'failed', 'canceled')),
  CHECK (min_confidence >= 0 AND min_confidence <= 1),
  CHECK (result_count >= 0)
);

CREATE TABLE IF NOT EXISTS ocr_result (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  ocr_task_id UUID NOT NULL REFERENCES ocr_task(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  submission_page_id UUID NOT NULL REFERENCES submission_page(id),
  text TEXT NOT NULL,
  bbox JSONB NOT NULL,
  confidence NUMERIC(5,4) NOT NULL,
  ocr_engine TEXT NOT NULL,
  ocr_version TEXT NOT NULL,
  source_image_file_id UUID REFERENCES file_asset(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (confidence >= 0 AND confidence <= 1)
);

CREATE INDEX IF NOT EXISTS idx_ocr_task_submission ON ocr_task (tenant_id, submission_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ocr_task_status ON ocr_task (tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ocr_result_task ON ocr_result (tenant_id, ocr_task_id);
CREATE INDEX IF NOT EXISTS idx_ocr_result_page ON ocr_result (tenant_id, submission_page_id);
