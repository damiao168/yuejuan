ALTER TABLE event_outbox
  ADD COLUMN IF NOT EXISTS locked_by TEXT,
  ADD COLUMN IF NOT EXISTS lock_expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS dead_lettered_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS max_attempts INT NOT NULL DEFAULT 8;

ALTER TABLE event_outbox
  DROP CONSTRAINT IF EXISTS event_outbox_max_attempts_check;
ALTER TABLE event_outbox
  ADD CONSTRAINT event_outbox_max_attempts_check CHECK (max_attempts BETWEEN 1 AND 100);

CREATE INDEX IF NOT EXISTS idx_event_outbox_claimable
ON event_outbox (next_attempt_at, occurred_at, id)
WHERE published_at IS NULL AND dead_lettered_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_event_outbox_dead_letter
ON event_outbox (dead_lettered_at DESC, tenant_id)
WHERE dead_lettered_at IS NOT NULL;

COMMENT ON COLUMN event_outbox.lock_expires_at IS
'Lease expiry for at-least-once consumers; expired claims are safe to reclaim.';

