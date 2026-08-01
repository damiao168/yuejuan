CREATE TABLE IF NOT EXISTS subjective_grading_batch (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'planned',
  segment_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  total_count INT NOT NULL,
  queued_count INT NOT NULL DEFAULT 0,
  processing_count INT NOT NULL DEFAULT 0,
  succeeded_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT uq_subjective_grading_batch_request UNIQUE (tenant_id, idempotency_key),
  CONSTRAINT chk_subjective_grading_batch_status CHECK (status IN ('planned', 'processing', 'completed', 'failed', 'cancelled')),
  CONSTRAINT chk_subjective_grading_batch_counts CHECK (
    total_count > 0 AND queued_count >= 0 AND processing_count >= 0 AND succeeded_count >= 0 AND failed_count >= 0
    AND queued_count + processing_count + succeeded_count + failed_count <= total_count
  )
);

CREATE INDEX IF NOT EXISTS idx_subjective_grading_batch_tenant_time
  ON subjective_grading_batch (tenant_id, created_at DESC);

COMMENT ON TABLE subjective_grading_batch IS
'Durable operator batch for governed subjective grading. Segment IDs only; student answers stay behind protected context loaders.';

ALTER TABLE subjective_grading_run
  ADD COLUMN IF NOT EXISTS batch_id UUID REFERENCES subjective_grading_batch(id);

CREATE INDEX IF NOT EXISTS idx_subjective_grading_run_batch
  ON subjective_grading_run (tenant_id, batch_id, created_at DESC)
  WHERE batch_id IS NOT NULL;
