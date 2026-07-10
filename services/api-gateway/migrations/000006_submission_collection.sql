INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('submission:manage', 'Manage submissions', 'submission', 'manage', '采集和管理答卷')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'submission:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS submission (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  student_id UUID REFERENCES student(id),
  candidate_no TEXT,
  source_type TEXT NOT NULL,
  status TEXT NOT NULL,
  expected_page_count INT NOT NULL DEFAULT 0,
  actual_page_count INT NOT NULL DEFAULT 0,
  quality_status TEXT NOT NULL DEFAULT 'unchecked',
  quality_issues JSONB NOT NULL DEFAULT '[]',
  collected_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (expected_page_count >= 0),
  CHECK (actual_page_count >= 0),
  CHECK (source_type IN ('scanner_upload', 'image_upload', 'pdf_upload', 'mobile_capture', 'lms_import', 'manual_import')),
  CHECK (status IN ('created', 'pages_uploaded', 'quality_checked', 'ready_for_ocr', 'rejected')),
  CHECK (quality_status IN ('unchecked', 'passed', 'failed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_submission_candidate
ON submission (tenant_id, exam_id, candidate_no)
WHERE candidate_no IS NOT NULL AND candidate_no <> '' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS submission_page (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  file_asset_id UUID NOT NULL REFERENCES file_asset(id),
  page_no INT NOT NULL,
  status TEXT NOT NULL,
  quality_issues JSONB NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, submission_id, page_no),
  CHECK (page_no > 0),
  CHECK (status IN ('uploaded', 'accepted', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_submission_exam ON submission (tenant_id, exam_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_submission_page_submission ON submission_page (tenant_id, submission_id, page_no);
