CREATE TABLE IF NOT EXISTS school (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  status TEXT NOT NULL,
  settings JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS grade (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  name TEXT NOT NULL,
  level_no INT,
  academic_year TEXT,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS school_class (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  grade_id UUID NOT NULL REFERENCES grade(id),
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  status TEXT NOT NULL,
  homeroom_teacher_id UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, grade_id, code)
);

CREATE TABLE IF NOT EXISTS student (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  class_id UUID NOT NULL REFERENCES school_class(id),
  student_no TEXT NOT NULL,
  name TEXT NOT NULL,
  gender TEXT,
  status TEXT NOT NULL,
  sensitive_profile JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, school_id, student_no)
);

CREATE TABLE IF NOT EXISTS teacher_class (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  teacher_id UUID NOT NULL REFERENCES app_user(id),
  class_id UUID NOT NULL REFERENCES school_class(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, teacher_id, class_id)
);

INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('tenant:manage', 'Manage tenants', 'tenant', 'manage', '管理租户'),
  ('org:manage', 'Manage organization', 'organization', 'manage', '管理学校、年级、班级和学生'),
  ('student:import', 'Import students', 'student', 'import', '批量导入学生')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin')
  AND p.code IN ('tenant:manage', 'org:manage', 'student:import')
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'teacher'
  AND p.code = 'org:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_school_tenant ON school (tenant_id);
CREATE INDEX IF NOT EXISTS idx_grade_school ON grade (tenant_id, school_id);
CREATE INDEX IF NOT EXISTS idx_class_grade ON school_class (tenant_id, grade_id);
CREATE INDEX IF NOT EXISTS idx_student_class ON student (tenant_id, class_id);
CREATE INDEX IF NOT EXISTS idx_teacher_class_teacher ON teacher_class (tenant_id, teacher_id);
