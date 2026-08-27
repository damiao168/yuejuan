-- Queue math-understanding work only after a trustworthy answer crop exists.
-- The formula branch is restricted to the three enabled subjects and to
-- snapshots which explicitly request mathematical expression/step evidence.
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
    WHEN snapshot_record.allowed_evidence_types ?| ARRAY['math_expression','math_step','numeric_value','unit_value','chemical_equation']
      THEN 'formula'
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
    'math-understanding-task-v1',task_key,task_key,3,30
  )
  ON CONFLICT (tenant_id,task_type,idempotency_key) DO NOTHING;
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_enqueue_math_understanding ON answer_segment;
CREATE TRIGGER trg_enqueue_math_understanding
AFTER INSERT OR UPDATE OF processing_status,crop_file_asset_id,crop_sha256,deleted_at
ON answer_segment
FOR EACH ROW EXECUTE FUNCTION enqueue_math_understanding_for_segment();

-- Backfill ready mathematical crops without duplicating an existing task.
INSERT INTO agent_worker_task(
  tenant_id,task_type,queue_name,source_type,source_id,status,priority,payload,
  payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds
)
SELECT seg.tenant_id,'evidence_verify','math-understanding','answer_segment',seg.id,
       'queued',80,
       jsonb_build_object(
         'answer_segment_id',seg.id::text,
         'exam_question_snapshot_id',snapshot.id::text,
         'subject_code',snapshot.profile_snapshot_json->>'subject_code',
         'region_kind',CASE
           WHEN snapshot.allowed_evidence_types ?| ARRAY['math_expression','math_step','numeric_value','unit_value','chemical_equation']
             THEN 'formula'
           ELSE 'text'
         END,
         'input_hash',seg.crop_sha256
       ),
       'math-understanding-task-v1',
       'math-understanding:' || seg.id::text || ':' || seg.crop_sha256,
       'math-understanding:' || seg.id::text || ':' || seg.crop_sha256,
       3,30
FROM answer_segment seg
JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id AND q.deleted_at IS NULL
JOIN LATERAL (
  SELECT candidate.* FROM exam_question_snapshot candidate
  WHERE candidate.tenant_id=q.tenant_id
    AND candidate.exam_id=q.exam_id
    AND candidate.question_id=q.id
  ORDER BY candidate.snapshot_version DESC
  LIMIT 1
) snapshot ON true
WHERE seg.deleted_at IS NULL
  AND seg.processing_status='completed'
  AND seg.crop_file_asset_id IS NOT NULL
  AND COALESCE(seg.crop_sha256,'') <> ''
  AND snapshot.profile_snapshot_json->>'subject_code' IN ('mathematics','physics','chemistry')
ON CONFLICT (tenant_id,task_type,idempotency_key) DO NOTHING;
