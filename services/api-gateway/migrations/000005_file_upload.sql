INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('file:manage', 'Manage files', 'file', 'manage', '上传、下载和删除私有文件')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'file:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'file_asset_size_positive'
  ) THEN
    ALTER TABLE file_asset
      ADD CONSTRAINT file_asset_size_positive CHECK (size_bytes > 0);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'file_asset_visibility_check'
  ) THEN
    ALTER TABLE file_asset
      ADD CONSTRAINT file_asset_visibility_check CHECK (visibility IN ('private', 'tenant', 'public'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_file_asset_exam ON file_asset (tenant_id, exam_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_file_asset_hash ON file_asset (tenant_id, hash_sha256);
