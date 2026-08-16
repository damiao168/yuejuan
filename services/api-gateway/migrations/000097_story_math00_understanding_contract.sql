-- STORY-MATH-00: versioned, tenant-isolated mathematical answer understanding
-- artifacts. These are evidence only and never constitute a final grade.

CREATE TABLE math_understanding_artifact (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  subject_code TEXT NOT NULL,
  answer_segment_id UUID NOT NULL,
  exam_question_snapshot_id UUID NOT NULL,
  version BIGINT NOT NULL,
  input_hash TEXT NOT NULL,
  engine_version TEXT NOT NULL,
  blocks_json JSONB NOT NULL,
  formulas_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  relations_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  solution_graph_json JSONB NOT NULL,
  verifications_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  rubric_evidence_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  is_current BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, answer_segment_id, version),
  FOREIGN KEY (tenant_id, answer_segment_id)
    REFERENCES answer_segment(tenant_id, id),
  FOREIGN KEY (tenant_id, exam_question_snapshot_id)
    REFERENCES exam_question_snapshot(tenant_id, id),
  CHECK (version > 0),
  CHECK (subject_code IN ('mathematics', 'physics', 'chemistry')),
  CHECK (length(input_hash) BETWEEN 1 AND 200),
  CHECK (length(engine_version) BETWEEN 1 AND 120),
  CHECK (jsonb_typeof(blocks_json) = 'array' AND jsonb_array_length(blocks_json) > 0),
  CHECK (jsonb_typeof(formulas_json) = 'array'),
  CHECK (jsonb_typeof(relations_json) = 'array'),
  CHECK (jsonb_typeof(solution_graph_json) = 'object'),
  CHECK (jsonb_typeof(verifications_json) = 'array'),
  CHECK (jsonb_typeof(rubric_evidence_json) = 'array')
);

CREATE UNIQUE INDEX uq_math_understanding_current
  ON math_understanding_artifact(tenant_id, answer_segment_id)
  WHERE is_current;

CREATE INDEX idx_math_understanding_snapshot
  ON math_understanding_artifact(tenant_id, exam_question_snapshot_id, created_at DESC);

CREATE FUNCTION reject_math_understanding_content_update()
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
     OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'math understanding artifact content is immutable';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_math_understanding_immutable
BEFORE UPDATE ON math_understanding_artifact
FOR EACH ROW EXECUTE FUNCTION reject_math_understanding_content_update();
