INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('report:read', 'Read learning reports', 'report', 'read', '查看学情报告和考试质量分析'),
  ('report:export', 'Export learning reports', 'report', 'export', '导出学情报告'),
  ('student:report:read', 'Read own learning report', 'student_report', 'read', '学生查看自己的学情报告')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher', 'auditor')
  AND p.code = 'report:read'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'report:export'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'student'
  AND p.code = 'student:report:read'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS report (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  report_type TEXT NOT NULL,
  scope_type TEXT NOT NULL,
  scope_id UUID,
  status TEXT NOT NULL,
  file_asset_id UUID REFERENCES file_asset(id),
  data JSONB NOT NULL DEFAULT '{}',
  generated_by UUID REFERENCES app_user(id),
  generated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (report_type IN ('overview', 'classes', 'questions', 'grading_quality', 'student', 'export')),
  CHECK (scope_type IN ('exam', 'class', 'grade', 'student')),
  CHECK (status IN ('pending', 'generating', 'succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_report_exam_type ON report (tenant_id, exam_id, report_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_report_scope ON report (tenant_id, scope_type, scope_id, created_at DESC);
