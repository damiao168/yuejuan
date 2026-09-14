-- Symbolic verification is a second immutable stage. Recognition artifacts are
-- never updated in place; each verification result creates a derived artifact
-- bound to its exact parent and correction revision.
ALTER TABLE math_understanding_artifact
  ADD COLUMN stage TEXT NOT NULL DEFAULT 'recognition',
  ADD COLUMN parent_artifact_id UUID,
  ADD COLUMN correction_revision BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN quality_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE math_understanding_artifact
  ADD CONSTRAINT chk_math_understanding_artifact_stage
    CHECK (stage IN ('recognition','verified')),
  ADD CONSTRAINT chk_math_understanding_correction_revision
    CHECK (correction_revision >= 0),
  ADD CONSTRAINT chk_math_understanding_stage_parent
    CHECK ((stage='recognition' AND parent_artifact_id IS NULL AND correction_revision=0)
        OR (stage='verified' AND parent_artifact_id IS NOT NULL AND parent_artifact_id<>id)),
  ADD CONSTRAINT chk_math_understanding_quality_summary
    CHECK (jsonb_typeof(quality_summary_json) = 'object'),
  ADD CONSTRAINT fk_math_understanding_parent
    FOREIGN KEY (tenant_id,parent_artifact_id)
    REFERENCES math_understanding_artifact(tenant_id,id);

CREATE UNIQUE INDEX uq_math_understanding_derived_stage
  ON math_understanding_artifact(tenant_id,parent_artifact_id,stage,correction_revision)
  WHERE parent_artifact_id IS NOT NULL;

CREATE INDEX idx_math_understanding_parent
  ON math_understanding_artifact(tenant_id,parent_artifact_id,created_at DESC)
  WHERE parent_artifact_id IS NOT NULL;

CREATE FUNCTION enqueue_math_verification_for_artifact()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.stage <> 'recognition' OR jsonb_typeof(NEW.formulas_json) IS DISTINCT FROM 'array'
     OR jsonb_array_length(NEW.formulas_json) = 0 THEN
    RETURN NEW;
  END IF;
  INSERT INTO agent_worker_task(
    tenant_id,task_type,queue_name,source_type,source_id,status,priority,payload,
    payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds
  ) VALUES (
    NEW.tenant_id,'evidence_verify','math-verification','math_understanding_artifact',NEW.id,
    'queued',75,
    jsonb_build_object('artifact_id',NEW.id::text,'artifact_version',NEW.version,'input_hash',NEW.input_hash,'correction_revision',0),
    'math-verification-task-v1',
    'math-verification:' || NEW.id::text || ':' || NEW.version::text || ':0',
    'math-verification:' || NEW.id::text || ':' || NEW.version::text || ':0',
    3,30
  ) ON CONFLICT (tenant_id,task_type,idempotency_key) DO NOTHING;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_enqueue_math_verification_artifact
AFTER INSERT ON math_understanding_artifact
FOR EACH ROW EXECUTE FUNCTION enqueue_math_verification_for_artifact();

CREATE FUNCTION enqueue_math_verification_for_correction()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  artifact_version BIGINT;
  artifact_hash TEXT;
BEGIN
  IF jsonb_typeof(NEW.corrected_contract_json->'formulas') IS DISTINCT FROM 'array'
     OR jsonb_array_length(NEW.corrected_contract_json->'formulas') = 0 THEN
    RETURN NEW;
  END IF;
  SELECT version,input_hash INTO artifact_version,artifact_hash
  FROM math_understanding_artifact
  WHERE tenant_id=NEW.tenant_id AND id=NEW.artifact_id;

  INSERT INTO agent_worker_task(
    tenant_id,task_type,queue_name,source_type,source_id,status,priority,payload,
    payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by
  ) VALUES (
    NEW.tenant_id,'evidence_verify','math-verification','math_understanding_artifact',NEW.artifact_id,
    'queued',70,
    jsonb_build_object('artifact_id',NEW.artifact_id::text,'artifact_version',artifact_version,'input_hash',artifact_hash,'correction_revision',NEW.revision),
    'math-verification-task-v1',
    'math-verification:' || NEW.artifact_id::text || ':' || artifact_version::text || ':' || NEW.revision::text,
    'math-verification:' || NEW.artifact_id::text || ':' || artifact_version::text || ':' || NEW.revision::text,
    3,30,NEW.created_by
  ) ON CONFLICT (tenant_id,task_type,idempotency_key) DO NOTHING;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_enqueue_math_verification_correction
AFTER INSERT ON math_understanding_correction
FOR EACH ROW EXECUTE FUNCTION enqueue_math_verification_for_correction();

-- Existing current artifacts predate stage metadata. Backfill one task bound
-- to the latest correction revision without changing any evidence row.
INSERT INTO agent_worker_task(
  tenant_id,task_type,queue_name,source_type,source_id,status,priority,payload,
  payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds
)
SELECT artifact.tenant_id,'evidence_verify','math-verification','math_understanding_artifact',artifact.id,
       'queued',75,
       jsonb_build_object(
         'artifact_id',artifact.id::text,'artifact_version',artifact.version,'input_hash',artifact.input_hash,
         'correction_revision',COALESCE(latest.revision,0)
       ),
       'math-verification-task-v1',
       'math-verification:' || artifact.id::text || ':' || artifact.version::text || ':' || COALESCE(latest.revision,0)::text,
       'math-verification:' || artifact.id::text || ':' || artifact.version::text || ':' || COALESCE(latest.revision,0)::text,
       3,30
FROM math_understanding_artifact artifact
LEFT JOIN LATERAL (
  SELECT revision FROM math_understanding_correction correction
  WHERE correction.tenant_id=artifact.tenant_id AND correction.artifact_id=artifact.id
  ORDER BY revision DESC LIMIT 1
) latest ON true
WHERE artifact.is_current AND jsonb_array_length(artifact.formulas_json) > 0
ON CONFLICT (tenant_id,task_type,idempotency_key) DO NOTHING;

CREATE OR REPLACE FUNCTION reject_math_understanding_content_update()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
     OR NEW.subject_code IS DISTINCT FROM OLD.subject_code
     OR NEW.answer_segment_id IS DISTINCT FROM OLD.answer_segment_id
     OR NEW.exam_question_snapshot_id IS DISTINCT FROM OLD.exam_question_snapshot_id
     OR NEW.version IS DISTINCT FROM OLD.version
     OR NEW.input_hash IS DISTINCT FROM OLD.input_hash
     OR NEW.engine_version IS DISTINCT FROM OLD.engine_version
     OR NEW.blocks_json IS DISTINCT FROM OLD.blocks_json
     OR NEW.formulas_json IS DISTINCT FROM OLD.formulas_json
     OR NEW.relations_json IS DISTINCT FROM OLD.relations_json
     OR NEW.solution_graph_json IS DISTINCT FROM OLD.solution_graph_json
     OR NEW.verifications_json IS DISTINCT FROM OLD.verifications_json
     OR NEW.rubric_evidence_json IS DISTINCT FROM OLD.rubric_evidence_json
     OR NEW.stage IS DISTINCT FROM OLD.stage
     OR NEW.parent_artifact_id IS DISTINCT FROM OLD.parent_artifact_id
     OR NEW.correction_revision IS DISTINCT FROM OLD.correction_revision
     OR NEW.quality_summary_json IS DISTINCT FROM OLD.quality_summary_json
     OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'math understanding artifact content is immutable';
  END IF;
  RETURN NEW;
END;
$$;
