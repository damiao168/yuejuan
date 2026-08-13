-- STORY-A17: immutable-source Human/AI disagreement observations. The table
-- stores IDs and aggregate evidence counts only; answer text, image URLs,
-- Gold, blind Seeds and reviewer comments never become part of this dataset.

ALTER TABLE review_task DROP CONSTRAINT IF EXISTS review_task_source_check;
ALTER TABLE review_task ADD CONSTRAINT review_task_source_check CHECK (source IN (
  'ai_low_confidence', 'ocr_low_confidence', 'subjective_default_review',
  'evidence_verification_failed', 'double_mark_required', 'score_anomaly',
  'manual_sample', 'omr_ambiguous', 'rule_review_required', 'grading_failure',
  'answer_group_outlier', 'ai_human_disagreement'
));

CREATE TABLE ai_human_disagreement (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  ai_candidate_id UUID NOT NULL REFERENCES ai_grade(id),
  human_grade_id UUID NOT NULL REFERENCES human_grade(id),
  ai_candidate_score NUMERIC(8,2) NOT NULL,
  human_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  delta NUMERIC(8,2) NOT NULL,
  absolute_delta NUMERIC(8,2) NOT NULL,
  difference_type TEXT NOT NULL,
  severity TEXT NOT NULL,
  risk_tier TEXT NOT NULL,
  trigger_rules_json JSONB NOT NULL DEFAULT '[]',
  evidence_summary_json JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'needs_review',
  taxonomy TEXT,
  reviewer_id UUID REFERENCES app_user(id),
  reviewed_at TIMESTAMPTZ,
  notes TEXT NOT NULL DEFAULT '',
  routed_review_task_id UUID REFERENCES review_task(id),
  routed_by UUID REFERENCES app_user(id),
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, ai_candidate_id, human_grade_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, answer_segment_id) REFERENCES answer_segment(tenant_id, id),
  CHECK (ai_candidate_score >= 0 AND human_score >= 0 AND max_score > 0
    AND ai_candidate_score <= max_score AND human_score <= max_score),
  CHECK (absolute_delta >= 0 AND absolute_delta = abs(delta)),
  CHECK (difference_type IN ('score','criterion','evidence','score_and_criterion','score_and_evidence','combined')),
  CHECK (severity IN ('warning','severe')),
  CHECK (risk_tier IN ('R1','R2','R3')),
  CHECK (jsonb_typeof(trigger_rules_json) = 'array'),
  CHECK (jsonb_typeof(evidence_summary_json) = 'object'),
  CHECK (status IN ('needs_review','classified','routed')),
  CHECK (taxonomy IS NULL OR taxonomy IN (
    'ai_scoring_error','human_scoring_error','ocr_error','parser_error',
    'rubric_ambiguity','reference_answer_issue','question_issue',
    'insufficient_evidence','acceptable_variation'
  )),
  CHECK ((status = 'needs_review' AND taxonomy IS NULL AND reviewer_id IS NULL AND reviewed_at IS NULL AND routed_review_task_id IS NULL)
    OR (status = 'classified' AND taxonomy IS NOT NULL AND reviewer_id IS NOT NULL AND reviewed_at IS NOT NULL AND routed_review_task_id IS NULL)
    OR (status = 'routed' AND routed_review_task_id IS NOT NULL AND routed_by IS NOT NULL)),
  CHECK (revision > 0),
  CHECK (char_length(notes) <= 2000)
);

CREATE INDEX idx_ai_human_disagreement_queue
  ON ai_human_disagreement (tenant_id, status, severity DESC, created_at ASC);
CREATE INDEX idx_ai_human_disagreement_exam_question
  ON ai_human_disagreement (tenant_id, exam_id, question_id, created_at DESC);
CREATE INDEX idx_ai_human_disagreement_taxonomy
  ON ai_human_disagreement (tenant_id, taxonomy, created_at DESC)
  WHERE taxonomy IS NOT NULL;

-- Source lineage is append-only. Classification and human task routing remain
-- mutable under an optimistic revision; neither operation can change scores.
CREATE OR REPLACE FUNCTION protect_ai_human_disagreement_source()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'AI-human disagreement records cannot be deleted' USING ERRCODE = '23514';
  END IF;
  IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.exam_id IS DISTINCT FROM OLD.exam_id
    OR NEW.question_id IS DISTINCT FROM OLD.question_id OR NEW.submission_id IS DISTINCT FROM OLD.submission_id
    OR NEW.answer_segment_id IS DISTINCT FROM OLD.answer_segment_id OR NEW.ai_candidate_id IS DISTINCT FROM OLD.ai_candidate_id
    OR NEW.human_grade_id IS DISTINCT FROM OLD.human_grade_id OR NEW.ai_candidate_score IS DISTINCT FROM OLD.ai_candidate_score
    OR NEW.human_score IS DISTINCT FROM OLD.human_score OR NEW.max_score IS DISTINCT FROM OLD.max_score
    OR NEW.delta IS DISTINCT FROM OLD.delta OR NEW.absolute_delta IS DISTINCT FROM OLD.absolute_delta
    OR NEW.difference_type IS DISTINCT FROM OLD.difference_type OR NEW.severity IS DISTINCT FROM OLD.severity
    OR NEW.risk_tier IS DISTINCT FROM OLD.risk_tier OR NEW.trigger_rules_json IS DISTINCT FROM OLD.trigger_rules_json
    OR NEW.evidence_summary_json IS DISTINCT FROM OLD.evidence_summary_json OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'AI-human disagreement source facts are immutable' USING ERRCODE = '23514';
  END IF;
  IF NEW.revision <> OLD.revision + 1 OR NEW.updated_at < OLD.updated_at THEN
    RAISE EXCEPTION 'AI-human disagreement update requires next revision' USING ERRCODE = '23514';
  END IF;
  IF OLD.status = 'routed' THEN
    RAISE EXCEPTION 'routed AI-human disagreement is immutable' USING ERRCODE = '23514';
  END IF;
  IF OLD.status = 'needs_review' AND NEW.status NOT IN ('classified','routed') THEN
    RAISE EXCEPTION 'invalid AI-human disagreement state transition' USING ERRCODE = '23514';
  END IF;
  IF OLD.status = 'classified' AND NEW.status NOT IN ('classified','routed') THEN
    RAISE EXCEPTION 'invalid AI-human disagreement state transition' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_ai_human_disagreement_source
BEFORE UPDATE OR DELETE ON ai_human_disagreement
FOR EACH ROW EXECUTE FUNCTION protect_ai_human_disagreement_source();
