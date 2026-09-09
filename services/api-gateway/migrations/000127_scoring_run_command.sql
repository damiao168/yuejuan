-- Historical runs keep their original identity; do not fabricate request hashes.
ALTER TABLE scoring_run ADD COLUMN command_request_hash TEXT;
CREATE UNIQUE INDEX uq_scoring_run_actor_command ON scoring_run(tenant_id,started_by,idempotency_key)
WHERE command_request_hash IS NOT NULL;
ALTER TABLE scoring_run ADD CONSTRAINT scoring_run_command_hash CHECK(command_request_hash IS NULL OR length(command_request_hash)=64);
