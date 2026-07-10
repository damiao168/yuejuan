CREATE TABLE IF NOT EXISTS agent_worker_task (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  task_type TEXT NOT NULL,
  queue_name TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  priority INT NOT NULL DEFAULT 100,
  payload JSONB NOT NULL DEFAULT '{}',
  payload_schema_version TEXT NOT NULL,
  result JSONB NOT NULL DEFAULT '{}',
  result_schema_version TEXT,
  result_payload_hash TEXT,
  idempotency_key TEXT NOT NULL,
  dedupe_key TEXT,
  max_attempts INT NOT NULL DEFAULT 3,
  attempt_count INT NOT NULL DEFAULT 0,
  retry_backoff_seconds INT NOT NULL DEFAULT 60,
  not_before TIMESTAMPTZ,
  lease_token TEXT,
  lease_expires_at TIMESTAMPTZ,
  leased_by TEXT,
  worker_service TEXT,
  worker_instance_id TEXT,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  cancelled_at TIMESTAMPTZ,
  duration_ms INT,
  error_code TEXT,
  error_detail JSONB NOT NULL DEFAULT '{}',
  created_by UUID REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_agent_worker_task_tenant_id UNIQUE (tenant_id, id),
  CONSTRAINT uq_agent_worker_task_idempotency UNIQUE (tenant_id, task_type, idempotency_key),
  CONSTRAINT fk_agent_worker_task_creator_tenant FOREIGN KEY (tenant_id, created_by)
    REFERENCES app_user(tenant_id, id),
  CONSTRAINT chk_agent_worker_task_type CHECK (task_type IN (
    'ocr', 'layout', 'preprocess', 'image_quality', 'ai_grade',
    'evidence_verify', 'report_generate', 'export', 'desktop_sync'
  )),
  CONSTRAINT chk_agent_worker_task_status CHECK (status IN (
    'queued', 'leased', 'running', 'succeeded', 'failed', 'dead_letter', 'cancelled'
  )),
  CONSTRAINT chk_agent_worker_task_attempts CHECK (max_attempts >= 1 AND attempt_count >= 0),
  CONSTRAINT chk_agent_worker_task_duration CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE TABLE IF NOT EXISTS agent_worker_task_attempt (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  task_id UUID NOT NULL,
  attempt_no INT NOT NULL,
  worker_service TEXT NOT NULL,
  worker_instance_id TEXT NOT NULL,
  lease_token TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  heartbeat_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  duration_ms INT,
  error_code TEXT,
  error_detail JSONB NOT NULL DEFAULT '{}',
  CONSTRAINT fk_agent_worker_attempt_task FOREIGN KEY (tenant_id, task_id)
    REFERENCES agent_worker_task(tenant_id, id),
  CONSTRAINT uq_agent_worker_attempt_no UNIQUE (tenant_id, task_id, attempt_no),
  CONSTRAINT chk_agent_worker_attempt_no CHECK (attempt_no >= 1),
  CONSTRAINT chk_agent_worker_attempt_duration CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE TABLE IF NOT EXISTS agent_worker_heartbeat (
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  worker_service TEXT NOT NULL,
  worker_instance_id TEXT NOT NULL,
  queue_name TEXT NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  PRIMARY KEY (tenant_id, worker_service, worker_instance_id, queue_name)
);

CREATE INDEX IF NOT EXISTS idx_agent_worker_task_claim
ON agent_worker_task (tenant_id, queue_name, status, priority, not_before, created_at);

CREATE INDEX IF NOT EXISTS idx_agent_worker_task_expired_lease
ON agent_worker_task (tenant_id, queue_name, lease_expires_at)
WHERE status IN ('leased', 'running');

CREATE INDEX IF NOT EXISTS idx_agent_worker_task_source
ON agent_worker_task (tenant_id, source_type, source_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_worker_attempt_task
ON agent_worker_task_attempt (tenant_id, task_id, attempt_no DESC);

COMMENT ON TABLE agent_worker_task IS
'Business-visible worker runtime task. Payloads contain references and version metadata, not full student answers.';
