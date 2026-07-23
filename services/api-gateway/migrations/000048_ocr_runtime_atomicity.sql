ALTER TABLE ocr_task
ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

UPDATE ocr_task
SET idempotency_key = 'legacy:' || id::text,
    updated_at = now()
WHERE idempotency_key IS NULL OR btrim(idempotency_key) = '';

ALTER TABLE ocr_task
ALTER COLUMN idempotency_key SET DEFAULT ('legacy:' || gen_random_uuid()::text),
ALTER COLUMN idempotency_key SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_ocr_task_idempotency_key'
  ) THEN
    ALTER TABLE ocr_task
    ADD CONSTRAINT chk_ocr_task_idempotency_key
    CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 200);
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ocr_task_active_idempotency
ON ocr_task (tenant_id, idempotency_key)
WHERE deleted_at IS NULL;

-- Go's encoding/json escapes HTML-sensitive runes and the two JavaScript line
-- separators even though PostgreSQL's to_json(text) leaves them literal. Keep
-- this migration's hash input byte-for-byte compatible with
-- workerruntime.CanonicalResultJSON.
CREATE OR REPLACE FUNCTION pg_temp.go_json_string(value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
STRICT
PARALLEL SAFE
AS $function$
  SELECT replace(
    replace(
      replace(
        replace(
          replace(to_json(value)::text, '&', E'\\u0026'),
          '<', E'\\u003c'
        ),
        '>', E'\\u003e'
      ),
      chr(8232), E'\\u2028'
    ),
    chr(8233), E'\\u2029'
  )
$function$;

WITH completed_ocr AS (
  SELECT
    source.tenant_id,
    source.id,
    source.result_count,
    COALESCE(source.model_version, '') AS model_version,
    COALESCE(source.config_hash, '') AS config_hash,
    COALESCE(source.input_hash, '') AS input_hash,
    COALESCE(source.duration_ms, 0) AS duration_ms,
    jsonb_build_object(
      'ocr_task_id', source.id::text,
      'result_count', source.result_count,
      'model_version', COALESCE(source.model_version, ''),
      'config_hash', COALESCE(source.config_hash, ''),
      'input_hash', COALESCE(source.input_hash, '')
    ) AS runtime_result,
    concat(
      '{"schema":"ocr-result.v1","payload":{"config_hash":',
      pg_temp.go_json_string(COALESCE(source.config_hash, '')),
      ',"input_hash":', pg_temp.go_json_string(COALESCE(source.input_hash, '')),
      ',"model_version":', pg_temp.go_json_string(COALESCE(source.model_version, '')),
      ',"ocr_task_id":', pg_temp.go_json_string(source.id::text),
      ',"result_count":', source.result_count::text,
      '}}'
    ) AS canonical_hash_input
  FROM ocr_task AS source
  WHERE source.status = 'completed'
    AND source.deleted_at IS NULL
), reconciled AS (
  UPDATE agent_worker_task AS runtime
  SET status = 'succeeded',
      result = source.runtime_result,
      result_schema_version = 'ocr-result.v1',
      result_payload_hash = encode(digest(source.canonical_hash_input, 'sha256'), 'hex'),
      not_before = NULL,
      duration_ms = source.duration_ms,
      completed_at = COALESCE(runtime.completed_at, now()),
      cancelled_at = NULL,
      error_code = NULL,
      error_detail = '{}'::jsonb,
      updated_at = now()
  FROM completed_ocr AS source
  WHERE runtime.tenant_id = source.tenant_id
    AND runtime.task_type = 'ocr'
    AND runtime.source_type = 'ocr_task'
    AND runtime.source_id = source.id
    AND runtime.status IN ('queued', 'leased', 'running', 'succeeded')
  RETURNING runtime.tenant_id, runtime.id, runtime.duration_ms
)
UPDATE agent_worker_task_attempt AS attempt
SET status = 'succeeded',
    duration_ms = reconciled.duration_ms,
    completed_at = now(),
    error_code = NULL,
    error_detail = '{}'::jsonb
FROM reconciled
WHERE attempt.tenant_id = reconciled.tenant_id
  AND attempt.task_id = reconciled.id
  AND attempt.completed_at IS NULL;
