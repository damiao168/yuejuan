INSERT INTO permission (tenant_id, code, name, resource, action, description)
SELECT t.id, p.code, p.name, p.resource, p.action, p.description
FROM tenant t
CROSS JOIN (VALUES
  ('orchestrator:manage', 'Manage orchestrator', 'orchestrator', 'manage', '编排和管理 Agent 任务')
) AS p(code, name, resource, action, description)
WHERE t.code IN ('platform', 'demo')
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO role_permission (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM role r
JOIN permission p ON p.tenant_id = r.tenant_id
WHERE r.code IN ('platform_admin', 'tenant_admin', 'school_admin', 'teacher')
  AND p.code = 'orchestrator:manage'
ON CONFLICT (tenant_id, role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS orchestration_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  workflow_type TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id UUID NOT NULL,
  status TEXT NOT NULL,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (workflow_type IN ('ocr_pipeline', 'segmentation_pipeline', 'grading_pipeline', 'review_pipeline', 'custom')),
  CHECK (target_type IN ('exam', 'submission', 'answer_segment', 'ocr_task')),
  CHECK (status IN ('created', 'running', 'completed', 'failed', 'requires_human_review'))
);

CREATE TABLE IF NOT EXISTS agent_task (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  orchestration_run_id UUID NOT NULL REFERENCES orchestration_run(id),
  agent_type TEXT NOT NULL,
  status TEXT NOT NULL,
  input_ref JSONB NOT NULL DEFAULT '{}',
  output_ref JSONB NOT NULL DEFAULT '{}',
  evidence_ref JSONB NOT NULL DEFAULT '{}',
  confidence NUMERIC(5,4),
  attempt_no INT NOT NULL DEFAULT 1,
  max_attempts INT NOT NULL DEFAULT 3,
  error_message TEXT,
  created_by UUID NOT NULL REFERENCES app_user(id),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (agent_type IN ('ocr_agent', 'layout_agent', 'segmentation_agent', 'objective_grading_agent', 'fill_blank_agent', 'subjective_grading_agent', 'essay_grading_agent', 'evidence_check_agent', 'consistency_check_agent', 'anomaly_detection_agent', 'fairness_check_agent', 'analytics_agent', 'audit_agent')),
  CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'requires_human_review')),
  CHECK (attempt_no > 0),
  CHECK (max_attempts > 0),
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1))
);

CREATE INDEX IF NOT EXISTS idx_orchestration_target ON orchestration_run (tenant_id, target_type, target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_task_run ON agent_task (tenant_id, orchestration_run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_task_status ON agent_task (tenant_id, status, created_at);
