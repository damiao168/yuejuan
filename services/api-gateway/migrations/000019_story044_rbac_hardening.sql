INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('review:work', 'Work assigned human review tasks', 'review', 'work', '处理分配给自己的人工阅卷任务'),
  ('arbitration:work', 'Work assigned arbitration tasks', 'arbitration', 'work', '处理分配给自己的双评仲裁任务')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'grader'
  AND p.code = 'review:work'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'arbitrator'
  AND p.code = 'arbitration:work'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

UPDATE role_permission rp
SET deleted_at = now(), updated_at = now()
FROM role r, permission p
WHERE rp.tenant_id = r.tenant_id
  AND rp.role_id = r.id
  AND rp.tenant_id = p.tenant_id
  AND rp.permission_id = p.id
  AND rp.deleted_at IS NULL
  AND r.code IN ('grader', 'auditor')
  AND p.code = 'review:manage';

UPDATE role_permission rp
SET deleted_at = now(), updated_at = now()
FROM role r, permission p
WHERE rp.tenant_id = r.tenant_id
  AND rp.role_id = r.id
  AND rp.tenant_id = p.tenant_id
  AND rp.permission_id = p.id
  AND rp.deleted_at IS NULL
  AND r.code IN ('arbitrator', 'auditor')
  AND p.code = 'arbitration:manage';

UPDATE app_user u
SET password_hash = crypt(gen_random_uuid()::text, gen_salt('bf')),
    status = 'disabled',
    updated_at = now()
FROM tenant t
WHERE u.tenant_id = t.id
  AND u.deleted_at IS NULL
  AND u.password_hash = crypt('ChangeMe123!', u.password_hash)
  AND (
    (t.code = 'platform' AND u.username = 'platform_admin')
    OR (t.code = 'demo' AND u.username IN ('tenant_admin', 'school_admin', 'teacher', 'grader', 'auditor', 'arbitrator', 'student'))
  );
