ALTER TABLE subjective_grading_batch ADD COLUMN command_request_hash TEXT;
ALTER TABLE subjective_grading_batch ADD CONSTRAINT uq_subjective_batch_tenant_id UNIQUE(tenant_id,id);
ALTER TABLE subjective_grading_batch ADD CONSTRAINT subjective_batch_command_hash CHECK(command_request_hash IS NULL OR length(command_request_hash)=64);
CREATE UNIQUE INDEX uq_subjective_batch_actor_command ON subjective_grading_batch(tenant_id,created_by,idempotency_key) WHERE command_request_hash IS NOT NULL;
-- The first enqueue freezes all request identities before creating any run/task.
CREATE TABLE subjective_batch_enqueue_plan (
 tenant_id UUID NOT NULL,
 batch_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 command_id TEXT NOT NULL,
 request_hash TEXT NOT NULL CHECK(length(request_hash)=64),
 runs JSONB NOT NULL CHECK(jsonb_typeof(runs)='array'),
	completed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,batch_id),
 UNIQUE(tenant_id,actor_id,command_id),
 FOREIGN KEY(tenant_id,batch_id) REFERENCES subjective_grading_batch(tenant_id,id)
);
