-- STORY-A19: a question-level correction is a versioned workflow.  It stores
-- its own candidate/review facts and never mutates human_grade, final_grade,
-- submission_grade, or a published score_release in place.

CREATE TABLE IF NOT EXISTS regrade_job (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  source_release_id UUID NOT NULL,
  reason_code TEXT NOT NULL,
  reason_text TEXT NOT NULL,
  strategy TEXT NOT NULL,
  selector_json JSONB NOT NULL DEFAULT '{}',
  new_rubric_snapshot_id UUID,
  new_policy_version TEXT,
  severity_delta NUMERIC(8,2) NOT NULL DEFAULT 2,
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'awaiting_approval',
  affected_count INT NOT NULL DEFAULT 0,
  created_by UUID NOT NULL,
  approved_by UUID,
  approved_at TIMESTAMPTZ,
  finalized_by UUID,
  finalized_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, id, exam_id),
  UNIQUE (tenant_id, exam_id, idempotency_key),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, source_release_id) REFERENCES score_release(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, approved_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, finalized_by) REFERENCES app_user(tenant_id, id),
  CHECK (reason_code IN ('answer_key_error','rubric_error','ocr_correction','parser_bug','model_issue','quality_incident','appeal_pattern','other')),
  CHECK (strategy IN ('rule_recompute','ai_recompute_then_review','human_recheck','backmark_import')),
  CHECK (status IN ('awaiting_approval','approved','running','paused','diff_review','ready_for_release','cancelled')),
  CHECK (affected_count >= 0),
  CHECK (severity_delta >= 0),
  CHECK (length(btrim(reason_text)) > 0 AND octet_length(reason_text) <= 2000),
  CHECK (length(btrim(idempotency_key)) BETWEEN 8 AND 200),
  CHECK (jsonb_typeof(selector_json) = 'object'),
  CHECK ((approved_at IS NULL AND approved_by IS NULL) OR (approved_at IS NOT NULL AND approved_by IS NOT NULL)),
  CHECK ((finalized_at IS NULL AND finalized_by IS NULL) OR (finalized_at IS NOT NULL AND finalized_by IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS regrade_item (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  job_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  old_final_grade_id UUID NOT NULL,
  old_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  candidate_grade_id UUID,
  candidate_score NUMERIC(8,2),
  reviewed_grade_id UUID,
  reviewed_score NUMERIC(8,2),
  delta NUMERIC(8,2),
  status TEXT NOT NULL DEFAULT 'pending',
  assigned_to UUID,
  claimed_by UUID,
  reviewed_by UUID,
  review_note TEXT NOT NULL DEFAULT '',
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, job_id, submission_id),
  FOREIGN KEY (tenant_id, job_id) REFERENCES regrade_job(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, old_final_grade_id) REFERENCES final_grade(tenant_id, id),
  FOREIGN KEY (tenant_id, assigned_to) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, claimed_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, reviewed_by) REFERENCES app_user(tenant_id, id),
  CHECK (old_score >= 0 AND old_score <= max_score),
  CHECK (max_score >= 0),
  CHECK (candidate_score IS NULL OR (candidate_score >= 0 AND candidate_score <= max_score)),
  CHECK (reviewed_score IS NULL OR (reviewed_score >= 0 AND reviewed_score <= max_score)),
  CHECK (delta IS NULL OR reviewed_score IS NOT NULL),
  CHECK (status IN ('pending','claimed','candidate_ready','awaiting_review','resolved','exception','cancelled')),
  CHECK (revision > 0)
);

-- The event trail is intentionally separate from generic audit retention. It
-- makes the population, approval and every decision reproducible when A18
-- snapshots a later release or A20 decides whether it may be published.
CREATE TABLE IF NOT EXISTS regrade_event (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  job_id UUID NOT NULL,
  item_id UUID,
  event_type TEXT NOT NULL,
  actor_id UUID NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, job_id) REFERENCES regrade_job(tenant_id, id),
  FOREIGN KEY (tenant_id, item_id) REFERENCES regrade_item(tenant_id, id),
  FOREIGN KEY (tenant_id, actor_id) REFERENCES app_user(tenant_id, id),
  CHECK (event_type IN ('created','approved','started','paused','resumed','item_claimed','candidate_recorded','item_reviewed','finalized','cancelled')),
  CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_regrade_job_exam_status
ON regrade_job (tenant_id, exam_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_regrade_item_job_status
ON regrade_item (tenant_id, job_id, status, created_at);

CREATE INDEX IF NOT EXISTS idx_regrade_item_assignee
ON regrade_item (tenant_id, assigned_to, status, created_at);

CREATE INDEX IF NOT EXISTS idx_regrade_event_job
ON regrade_event (tenant_id, job_id, created_at, id);

DROP TRIGGER IF EXISTS trg_regrade_job_outbox ON regrade_job;
CREATE TRIGGER trg_regrade_job_outbox
AFTER INSERT OR UPDATE ON regrade_job
FOR EACH ROW EXECUTE FUNCTION enqueue_business_mutation_outbox();

COMMENT ON TABLE regrade_job IS
'A versioned, approved question-level correction workflow. Completion requires a new score release; no historical grade or release is updated in place.';
COMMENT ON TABLE regrade_item IS
'Frozen source score plus isolated proposed/reviewed correction. These are workflow facts, never the current final_grade.';
