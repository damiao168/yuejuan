INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('segment:manage', 'Manage answer segments', 'segment', 'manage', '生成和修正答题区域')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'segment:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS answer_segment (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  submission_id UUID NOT NULL REFERENCES submission(id),
  submission_page_id UUID NOT NULL REFERENCES submission_page(id),
  question_id UUID NOT NULL REFERENCES question(id),
  question_no TEXT NOT NULL,
  bbox JSONB NOT NULL,
  source TEXT NOT NULL,
  status TEXT NOT NULL,
  review_notes TEXT,
  reviewed_by UUID REFERENCES app_user(id),
  reviewed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, submission_id, question_id),
  CHECK (source IN ('configured_answer_area', 'manual')),
  CHECK (status IN ('generated', 'accepted', 'needs_manual_review', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_answer_segment_submission ON answer_segment (tenant_id, submission_id);
CREATE INDEX IF NOT EXISTS idx_answer_segment_question ON answer_segment (tenant_id, question_id);
