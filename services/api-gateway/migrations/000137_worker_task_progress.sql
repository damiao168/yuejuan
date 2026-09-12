ALTER TABLE agent_worker_task
    ADD COLUMN IF NOT EXISTS progress JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN agent_worker_task.progress IS
    'Latest task-local progress heartbeat. Counters are factual worker measurements, never estimated percentages.';
