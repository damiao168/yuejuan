-- STORY-A24: durable, resumable hand-off from the desktop scan spool.
-- Chunks are untrusted staging data.  A capture_file is created only after
-- the server has verified the full immutable SHA-256 and activated the asset.

CREATE TABLE capture_upload_session (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  capture_batch_id UUID NOT NULL,
  idempotency_key TEXT NOT NULL,
  original_name TEXT NOT NULL,
  content_type TEXT NOT NULL,
  expected_sha256 TEXT NOT NULL,
  total_size BIGINT NOT NULL,
  chunk_size BIGINT NOT NULL,
  confirmed_offset BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'uploading',
  file_asset_id UUID,
  capture_file_id UUID,
  error_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, capture_batch_id, idempotency_key),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, capture_batch_id) REFERENCES capture_batch(tenant_id, id),
  FOREIGN KEY (tenant_id, file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, capture_file_id) REFERENCES capture_file(tenant_id, id),
  CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 200),
  CHECK (length(expected_sha256) = 64 AND expected_sha256 ~ '^[0-9a-f]{64}$'),
  CHECK (total_size > 0),
  CHECK (chunk_size > 0),
  CHECK (confirmed_offset >= 0 AND confirmed_offset <= total_size),
  CHECK (status IN ('uploading', 'finalizing', 'completed', 'failed')),
  CHECK (
    (status = 'completed' AND file_asset_id IS NOT NULL AND capture_file_id IS NOT NULL AND completed_at IS NOT NULL)
    OR status <> 'completed'
  )
);

CREATE INDEX idx_capture_upload_session_resumable
ON capture_upload_session (tenant_id, capture_batch_id, status, updated_at DESC)
WHERE status IN ('uploading', 'finalizing');

CREATE TABLE capture_upload_chunk (
  tenant_id UUID NOT NULL,
  capture_upload_session_id UUID NOT NULL,
  offset_bytes BIGINT NOT NULL,
  size_bytes INT NOT NULL,
  chunk_sha256 TEXT NOT NULL,
  payload BYTEA NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, capture_upload_session_id, offset_bytes),
  FOREIGN KEY (tenant_id, capture_upload_session_id) REFERENCES capture_upload_session(tenant_id, id) ON DELETE CASCADE,
  CHECK (offset_bytes >= 0),
  CHECK (size_bytes > 0),
  CHECK (octet_length(payload) = size_bytes),
  CHECK (length(chunk_sha256) = 64 AND chunk_sha256 ~ '^[0-9a-f]{64}$')
);
