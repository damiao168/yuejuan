-- Result snapshots survive target deletion, HTTP replay TTL and artifact expiry.
CREATE TABLE business_command_receipt (
 tenant_id UUID NOT NULL,
 actor_id UUID NOT NULL,
 command_id TEXT NOT NULL CHECK(length(command_id) BETWEEN 1 AND 128),
 operation TEXT NOT NULL,
 target_id TEXT NOT NULL,
 request_hash TEXT NOT NULL CHECK(length(request_hash)=64),
 result JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,actor_id,command_id)
);
