WITH completed_ocr AS (
  SELECT
    o.tenant_id,
    o.id,
    o.result_count,
    COALESCE(o.model_version, '') AS model_version,
    COALESCE(o.config_hash, '') AS config_hash,
    COALESCE(o.input_hash, '') AS input_hash,
    COALESCE(o.duration_ms, 0) AS duration_ms,
    jsonb_build_object(
      'ocr_task_id', o.id::text,
      'result_count', o.result_count,
      'model_version', COALESCE(o.model_version, ''),
      'config_hash', COALESCE(o.config_hash, ''),
      'input_hash', COALESCE(o.input_hash, '')
    ) AS runtime_result,
    concat(
      '{"schema":"ocr-result.v1","payload":{"config_hash":',
      to_json(COALESCE(o.config_hash, ''))::text,
      ',"input_hash":', to_json(COALESCE(o.input_hash, ''))::text,
      ',"model_version":', to_json(COALESCE(o.model_version, ''))::text,
      ',"ocr_task_id":', to_json(o.id::text)::text,
      ',"result_count":', o.result_count::text,
      '}}'
    ) AS hash_input
  FROM ocr_task AS o
  WHERE o.status = 'completed'
    AND o.deleted_at IS NULL
), reconciled AS (
  UPDATE agent_worker_task AS runtime
  SET status = 'succeeded',
      result = source.runtime_result,
      result_schema_version = 'ocr-result.v1',
      result_payload_hash = encode(digest(source.hash_input, 'sha256'), 'hex'),
      not_before = NULL,
      duration_ms = source.duration_ms,
      completed_at = now(),
      cancelled_at = NULL,
      error_code = NULL,
      error_detail = '{}'::jsonb,
      updated_at = now()
  FROM completed_ocr AS source
  WHERE runtime.tenant_id = source.tenant_id
    AND runtime.task_type = 'ocr'
    AND runtime.source_type = 'ocr_task'
    AND runtime.source_id = source.id
    AND runtime.status IN ('queued', 'leased', 'running')
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
