CREATE TABLE IF NOT EXISTS idempotency_record (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  actor_id UUID NOT NULL REFERENCES app_user(id),
  method TEXT NOT NULL,
  route TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'processing',
  response_status INTEGER,
  response_headers JSONB NOT NULL DEFAULT '{}',
  response_body BYTEA,
  resource_type TEXT,
  resource_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT idempotency_record_state_check CHECK (state IN ('processing', 'completed')),
  CONSTRAINT idempotency_record_status_check CHECK (response_status IS NULL OR response_status BETWEEN 100 AND 599),
  CONSTRAINT idempotency_record_key_check CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  UNIQUE (tenant_id, actor_id, method, route, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_record_expiry
ON idempotency_record (expires_at);

CREATE INDEX IF NOT EXISTS idx_idempotency_record_processing
ON idempotency_record (tenant_id, state, updated_at)
WHERE state = 'processing';

COMMENT ON COLUMN idempotency_record.request_hash IS
'SHA-256 over method, matched route, path/query and request bytes. Request bodies are never persisted.';
