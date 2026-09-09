-- Preserve command identity after soft deletion. Legacy rows remain unhashed:
-- replay checks their immutable creation fields instead of inventing a hash.
ALTER TABLE capture_batch ADD COLUMN command_request_hash TEXT;
CREATE UNIQUE INDEX uq_capture_batch_actor_command
ON capture_batch(tenant_id,operator_id,idempotency_key)
WHERE command_request_hash IS NOT NULL;
ALTER TABLE capture_batch ADD CONSTRAINT capture_batch_command_identity
CHECK (command_request_hash IS NULL OR (length(command_request_hash)=64 AND idempotency_key IS NOT NULL AND length(btrim(idempotency_key))>0));
