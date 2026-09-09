-- Reliability protocol v2: explicit paper-import runs, durable command facts,
-- and recoverable exam-session commands.  Older migrations are intentionally
-- left untouched so an installed 000122 database can be upgraded in place.

ALTER TABLE paper_import_job
  ADD COLUMN current_generation BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN source_revision TEXT NOT NULL DEFAULT 'legacy-unverified',
  ADD COLUMN result_generation BIGINT,
  ADD COLUMN result_task_id UUID,
  ADD COLUMN result_payload_hash TEXT;

ALTER TABLE paper_import_job
  ADD CONSTRAINT uq_paper_import_job_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE paper_import_job
  ADD CONSTRAINT chk_paper_import_generation CHECK (current_generation > 0),
  ADD CONSTRAINT chk_paper_import_result_generation
    CHECK (result_generation IS NULL OR (result_generation > 0 AND result_generation <= current_generation)),
  ADD CONSTRAINT fk_paper_import_result_task
    FOREIGN KEY (tenant_id, result_task_id) REFERENCES agent_worker_task(tenant_id, id);

CREATE TABLE paper_import_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  paper_import_id UUID NOT NULL REFERENCES paper_import_job(id),
  generation BIGINT NOT NULL,
  command_id TEXT NOT NULL,
  command_type TEXT NOT NULL,
  command_request_hash TEXT NOT NULL,
  actor_id UUID NOT NULL REFERENCES app_user(id),
  source_revision TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'processing',
  dispatch_status TEXT NOT NULL DEFAULT 'pending',
  dispatch_lease_owner TEXT,
  dispatch_lease_expires_at TIMESTAMPTZ,
  dispatch_available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  dispatch_attempt_count INT NOT NULL DEFAULT 0,
  initial_task_id UUID,
  result_task_id UUID,
  result_payload_hash TEXT,
  error_code TEXT,
  error_detail JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  CONSTRAINT uq_paper_import_run_tenant_id UNIQUE (tenant_id, id),
  CONSTRAINT uq_paper_import_run_generation UNIQUE (tenant_id, paper_import_id, generation),
  CONSTRAINT uq_paper_import_run_command UNIQUE (tenant_id, actor_id, command_id),
  CONSTRAINT fk_paper_import_run_job_tenant
    FOREIGN KEY (tenant_id, paper_import_id) REFERENCES paper_import_job(tenant_id, id),
  CONSTRAINT fk_paper_import_run_actor_tenant
    FOREIGN KEY (tenant_id, actor_id) REFERENCES app_user(tenant_id, id),
  CONSTRAINT fk_paper_import_run_initial_task
    FOREIGN KEY (tenant_id, initial_task_id) REFERENCES agent_worker_task(tenant_id, id),
  CONSTRAINT fk_paper_import_run_result_task
    FOREIGN KEY (tenant_id, result_task_id) REFERENCES agent_worker_task(tenant_id, id),
  CONSTRAINT chk_paper_import_run_generation CHECK (generation > 0),
  CONSTRAINT chk_paper_import_run_command_id CHECK (length(command_id) BETWEEN 8 AND 128),
  CONSTRAINT chk_paper_import_run_command_type CHECK (command_type IN ('create','add_sources','replace_sources','rerun','repair')),
  CONSTRAINT chk_paper_import_run_status CHECK (status IN ('processing','review_required','failed','cancelled','superseded','applied','needs_review')),
  CONSTRAINT chk_paper_import_run_dispatch CHECK (dispatch_status IN ('pending','queued','not_required','failed')),
  CONSTRAINT chk_paper_import_run_dispatch_lease CHECK ((dispatch_lease_owner IS NULL)=(dispatch_lease_expires_at IS NULL))
);

CREATE INDEX idx_paper_import_run_current
  ON paper_import_run (tenant_id, paper_import_id, generation DESC);
CREATE INDEX idx_paper_import_run_dispatch
  ON paper_import_run (dispatch_status, created_at)
  WHERE status='processing' AND dispatch_status IN ('pending','failed');

-- Preserve historical state without claiming provenance that 000122 did not
-- record.  Such rows are deliberately marked needs_review or failed below.
INSERT INTO paper_import_run (
  tenant_id,paper_import_id,generation,command_id,command_type,actor_id,
  source_revision,status,dispatch_status,error_code,error_detail,created_at,updated_at,completed_at,command_request_hash
)
SELECT tenant_id,id,1,'legacy-' || id::text,'repair',created_by,
       'legacy-unverified',
       CASE WHEN status='processing' THEN 'failed' ELSE
         CASE WHEN status IN ('review_required','failed','cancelled','applied') THEN status ELSE 'needs_review' END
       END,
       CASE WHEN status='processing' THEN 'failed' ELSE 'not_required' END,
       CASE WHEN status='processing' THEN 'paper_import_protocol_upgrade_required' ELSE NULL END,
       CASE WHEN status='processing' THEN '{"reason":"legacy processing run has no provable generation-bound task"}'::jsonb ELSE '{}'::jsonb END,
       created_at,updated_at,
       CASE WHEN status='processing' THEN now() WHEN status<>'processing' THEN updated_at ELSE NULL END,
       'legacy-unverified'
FROM paper_import_job;

UPDATE paper_import_job
SET status='failed',error_code='paper_import_protocol_upgrade_required',
    issues='["旧版处理中任务缺少可验证的运行身份，已安全停止；请重新解析"]'::jsonb,
    updated_at=now()
WHERE status='processing';

ALTER TABLE paper_import_parse_input
  ADD COLUMN run_id UUID,
  ADD COLUMN generation BIGINT,
  ADD COLUMN protocol_version INT NOT NULL DEFAULT 1;

UPDATE paper_import_parse_input input
SET run_id=run.id,generation=run.generation
FROM paper_import_run run
WHERE run.tenant_id=input.tenant_id AND run.paper_import_id=input.paper_import_id AND run.generation=1;

ALTER TABLE paper_import_parse_input
  ALTER COLUMN run_id SET NOT NULL,
  ALTER COLUMN generation SET NOT NULL,
  ADD CONSTRAINT fk_paper_import_parse_input_run
    FOREIGN KEY (tenant_id, run_id) REFERENCES paper_import_run(tenant_id, id),
  ADD CONSTRAINT chk_paper_import_parse_input_generation CHECK (generation > 0),
  DROP CONSTRAINT uq_paper_import_parse_input_revision;

ALTER TABLE paper_import_parse_input
  ADD CONSTRAINT uq_paper_import_parse_input_run_hash
    UNIQUE (tenant_id, run_id, input_hash);

ALTER TABLE agent_worker_task
  ADD COLUMN paper_import_run_id UUID,
  ADD COLUMN paper_import_generation BIGINT,
  ADD COLUMN task_protocol_version INT NOT NULL DEFAULT 1,
  ADD CONSTRAINT fk_agent_worker_task_paper_import_run
    FOREIGN KEY (tenant_id, paper_import_run_id) REFERENCES paper_import_run(tenant_id, id),
  ADD CONSTRAINT chk_agent_worker_task_paper_generation
    CHECK ((paper_import_run_id IS NULL AND paper_import_generation IS NULL)
        OR (paper_import_run_id IS NOT NULL AND paper_import_generation > 0));

-- Protocol-v1 tasks cannot safely publish after this migration.  Preserve the
-- task/attempt history, but revoke every active lease and queue entry.
UPDATE agent_worker_task_attempt attempt
SET status='cancelled',completed_at=now(),error_code='protocol_superseded'
FROM agent_worker_task task
WHERE attempt.tenant_id=task.tenant_id AND attempt.task_id=task.id
  AND attempt.completed_at IS NULL
  AND ((task.source_type='paper_import_job') OR (task.source_type='paper_import_parse'));

UPDATE agent_worker_task
SET status='cancelled',cancelled_at=now(),completed_at=now(),updated_at=now(),revision=revision+1,
    error_code='protocol_superseded'
WHERE status IN ('queued','leased','running')
  AND (source_type='paper_import_job' OR source_type='paper_import_parse');

ALTER TABLE exam_session
  ADD COLUMN command_request_hash TEXT,
  ADD COLUMN command_status TEXT NOT NULL DEFAULT 'completed',
  ADD COLUMN command_completed_at TIMESTAMPTZ;

UPDATE exam_session
SET command_request_hash='legacy-unverified',command_completed_at=created_at
WHERE command_id IS NOT NULL;

ALTER TABLE exam_session
  ADD CONSTRAINT chk_exam_session_command_status CHECK (command_status IN ('completed','needs_review')),
  ADD CONSTRAINT chk_exam_session_command_fact CHECK (
    (command_id IS NULL AND command_request_hash IS NULL AND command_completed_at IS NULL)
    OR
    (command_id IS NOT NULL AND command_request_hash IS NOT NULL AND command_completed_at IS NOT NULL)
  );

CREATE INDEX idx_exam_session_command_recovery
  ON exam_session (tenant_id,created_by,command_id)
  WHERE command_id IS NOT NULL;

COMMENT ON TABLE paper_import_run IS
'One immutable user-visible parsing generation. Automatic attempts remain in the same run; explicit reruns create a new row.';
COMMENT ON COLUMN agent_worker_task.task_protocol_version IS
'Paper import callbacks require protocol v2; v1 tasks are historical and cannot publish after migration 000123.';
