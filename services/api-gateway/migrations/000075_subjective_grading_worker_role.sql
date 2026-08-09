INSERT INTO role (tenant_id, code, name, scope_type, description)
SELECT t.id,
       'subjective_grading_worker',
       'Subjective Grading Worker',
       'tenant',
       '主观题评分后台工作账号'
FROM tenant t
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code = 'subjective_grading_worker'
  AND p.code = 'orchestrator:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;
