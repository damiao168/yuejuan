-- Student calculation regions contain prose and multiple expressions. Route
-- those immutable crops through the mixed OCR + formula-layout pipeline rather
-- than treating the entire answer segment as one formula image.
CREATE OR REPLACE FUNCTION enqueue_math_understanding_for_segment()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  snapshot_record RECORD;
  region_kind_value TEXT;
  task_key TEXT;
BEGIN
  IF NEW.deleted_at IS NOT NULL
     OR NEW.processing_status <> 'completed'
     OR COALESCE(NEW.crop_file_asset_id::text, '') = ''
     OR COALESCE(NEW.crop_sha256, '') = '' THEN
    RETURN NEW;
  END IF;

  SELECT snapshot.id,
         snapshot.profile_snapshot_json->>'subject_code' AS subject_code,
         snapshot.archetype_snapshot_json->>'code' AS archetype_code,
         snapshot.profile_snapshot_json->>'question_type' AS question_type,
         snapshot.allowed_evidence_types
    INTO snapshot_record
  FROM question q
  JOIN exam_question_snapshot snapshot
    ON snapshot.tenant_id=q.tenant_id
   AND snapshot.exam_id=q.exam_id
   AND snapshot.question_id=q.id
  WHERE q.tenant_id=NEW.tenant_id
    AND q.id=NEW.question_id
    AND q.deleted_at IS NULL
  ORDER BY snapshot.snapshot_version DESC
  LIMIT 1;

  IF snapshot_record.id IS NULL
     OR snapshot_record.subject_code NOT IN ('mathematics','physics','chemistry') THEN
    RETURN NEW;
  END IF;

  region_kind_value := CASE
    WHEN snapshot_record.archetype_code = 'structured_steps'
      OR snapshot_record.question_type = 'calculation'
      OR snapshot_record.allowed_evidence_types ?| ARRAY['math_expression','math_step','numeric_value','unit_value','chemical_equation']
      THEN 'mixed'
    ELSE 'text'
  END;
  task_key := 'math-understanding:' || NEW.id::text || ':' || NEW.crop_sha256;

  INSERT INTO agent_worker_task(
    tenant_id,task_type,queue_name,source_type,source_id,status,priority,payload,
    payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds
  ) VALUES (
    NEW.tenant_id,'evidence_verify','math-understanding','answer_segment',NEW.id,
    'queued',80,
    jsonb_build_object(
      'answer_segment_id',NEW.id::text,
      'exam_question_snapshot_id',snapshot_record.id::text,
      'subject_code',snapshot_record.subject_code,
      'region_kind',region_kind_value,
      'input_hash',NEW.crop_sha256
    ),
    'math-understanding-task-v2',task_key,task_key,3,30
  )
  ON CONFLICT (tenant_id,task_type,idempotency_key) DO NOTHING;
  RETURN NEW;
END
$$;

-- A queued v1 task has not produced immutable evidence yet, so it is safe to
-- upgrade in place. Running/completed/history rows remain untouched.
UPDATE agent_worker_task
SET payload=jsonb_set(payload, '{region_kind}', '"mixed"'::jsonb, false),
    payload_schema_version='math-understanding-task-v2',
    updated_at=now(),
    revision=revision+1
WHERE queue_name='math-understanding'
  AND source_type='answer_segment'
  AND status='queued'
  AND payload->>'region_kind'='formula'
  AND payload->>'subject_code' IN ('mathematics','physics','chemistry');
