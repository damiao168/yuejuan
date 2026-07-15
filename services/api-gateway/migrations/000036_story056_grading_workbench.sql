ALTER TABLE agent_worker_task
  DROP CONSTRAINT chk_agent_worker_task_type;

ALTER TABLE agent_worker_task
  ADD CONSTRAINT chk_agent_worker_task_type
  CHECK (task_type IN (
    'ocr', 'layout', 'preprocess', 'image_quality', 'ai_grade',
    'evidence_verify', 'report_generate', 'export', 'desktop_sync',
    'capture_file_decode', 'page_registration', 'answer_segment_crop',
    'page_registration_correction_preview', 'omr_extract'
  ));

ALTER TABLE answer_segment_answer
  DROP CONSTRAINT answer_segment_answer_source_check;

ALTER TABLE answer_segment_answer
  ADD CONSTRAINT answer_segment_answer_source_check
  CHECK (source IN ('manual_entry', 'ocr_text', 'imported_answer', 'omr'));

ALTER TABLE review_task
  DROP CONSTRAINT review_task_source_check;

ALTER TABLE review_task
  ADD CONSTRAINT review_task_source_check
  CHECK (source IN (
    'ai_low_confidence', 'ocr_low_confidence', 'subjective_default_review',
    'evidence_verification_failed', 'double_mark_required', 'score_anomaly',
    'manual_sample', 'omr_ambiguous', 'rule_review_required', 'grading_failure'
  ));

ALTER TABLE review_task
  ADD COLUMN revision INT NOT NULL DEFAULT 1,
  ADD COLUMN reason_code TEXT NOT NULL DEFAULT '',
  ADD COLUMN claimed_at TIMESTAMPTZ,
  ADD COLUMN claim_expires_at TIMESTAMPTZ,
  ADD COLUMN last_opened_at TIMESTAMPTZ,
  ADD COLUMN current_grade_id UUID;

ALTER TABLE review_task
  ADD CONSTRAINT chk_review_task_revision CHECK (revision > 0),
  ADD CONSTRAINT chk_review_task_claim_window CHECK (
    claim_expires_at IS NULL OR claimed_at IS NULL OR claim_expires_at > claimed_at
  );

CREATE UNIQUE INDEX uq_review_task_active_source
ON review_task (tenant_id, answer_segment_id, source)
WHERE status IN ('pending', 'assigned', 'in_progress', 'returned') AND deleted_at IS NULL;

CREATE TABLE scoring_rule (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  version INT NOT NULL,
  rule_type TEXT NOT NULL,
  config JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'draft',
  revision INT NOT NULL DEFAULT 1,
  content_hash TEXT NOT NULL,
  created_by UUID NOT NULL,
  published_by UUID,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, question_id, version),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, published_by) REFERENCES app_user(tenant_id, id),
  CHECK (version > 0 AND revision > 0),
  CHECK (rule_type IN ('single_choice', 'true_false', 'multiple_choice', 'fill_blank', 'numeric', 'manual')),
  CHECK (status IN ('draft', 'published', 'retired')),
  CHECK (length(content_hash) >= 16),
  CHECK (jsonb_typeof(config) = 'object'),
  CHECK (status <> 'published' OR (published_by IS NOT NULL AND published_at IS NOT NULL)),
  CHECK (status <> 'draft' OR (published_by IS NULL AND published_at IS NULL))
);

CREATE UNIQUE INDEX uq_scoring_rule_current_draft
ON scoring_rule (tenant_id, question_id)
WHERE status = 'draft' AND deleted_at IS NULL;

CREATE UNIQUE INDEX uq_scoring_rule_current_published
ON scoring_rule (tenant_id, question_id)
WHERE status = 'published' AND deleted_at IS NULL;

CREATE TABLE scoring_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  total_count INT NOT NULL DEFAULT 0,
  queued_count INT NOT NULL DEFAULT 0,
  auto_confirmed_count INT NOT NULL DEFAULT 0,
  review_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  rule_snapshot JSONB NOT NULL DEFAULT '{}',
  started_by UUID NOT NULL,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  cancelled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, exam_id, idempotency_key),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, started_by) REFERENCES app_user(tenant_id, id),
  CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 160),
  CHECK (status IN ('queued', 'processing', 'needs_review', 'completed', 'failed', 'cancelled')),
  CHECK (total_count >= 0 AND queued_count >= 0 AND auto_confirmed_count >= 0 AND review_count >= 0 AND failed_count >= 0),
  CHECK (auto_confirmed_count + review_count + failed_count <= total_count)
);

CREATE INDEX idx_scoring_run_exam
ON scoring_run (tenant_id, exam_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE TABLE answer_candidate (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  scoring_run_id UUID,
  source TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  display_text TEXT NOT NULL DEFAULT '',
  confidence DOUBLE PRECISION,
  decision TEXT NOT NULL,
  evidence JSONB NOT NULL DEFAULT '{}',
  engine_version TEXT NOT NULL,
  profile_version TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  is_current BOOLEAN NOT NULL DEFAULT false,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, answer_segment_id) REFERENCES answer_segment(tenant_id, id),
  FOREIGN KEY (tenant_id, scoring_run_id) REFERENCES scoring_run(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CHECK (source IN ('omr', 'ocr', 'manual', 'imported')),
  CHECK (decision IN ('selected', 'blank', 'multiple', 'ambiguous', 'parse_failed', 'confirmed')),
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
  CHECK (length(input_hash) >= 16),
  CHECK (jsonb_typeof(payload) = 'object' AND jsonb_typeof(evidence) = 'object')
);

CREATE UNIQUE INDEX uq_answer_candidate_current
ON answer_candidate (tenant_id, answer_segment_id)
WHERE is_current AND deleted_at IS NULL;

CREATE INDEX idx_answer_candidate_segment
ON answer_candidate (tenant_id, answer_segment_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE TABLE omr_run (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  scoring_run_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  crop_file_asset_id UUID NOT NULL,
  crop_sha256 TEXT NOT NULL,
  template_id UUID NOT NULL,
  template_content_hash TEXT NOT NULL,
  profile_version TEXT NOT NULL,
  profile_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  decision TEXT,
  confidence DOUBLE PRECISION,
  measurements JSONB NOT NULL DEFAULT '[]',
  selected_options JSONB NOT NULL DEFAULT '[]',
  risk_flags JSONB NOT NULL DEFAULT '[]',
  overlay_file_asset_id UUID,
  runtime_task_id UUID,
  attempt_no INT NOT NULL DEFAULT 0,
  result_version TEXT,
  error_code TEXT,
  error_detail JSONB NOT NULL DEFAULT '{}',
  duration_ms INT,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, scoring_run_id) REFERENCES scoring_run(tenant_id, id),
  FOREIGN KEY (tenant_id, answer_segment_id) REFERENCES answer_segment(tenant_id, id),
  FOREIGN KEY (tenant_id, crop_file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, template_id) REFERENCES answer_sheet_template(tenant_id, id),
  FOREIGN KEY (tenant_id, overlay_file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, runtime_task_id) REFERENCES agent_worker_task(tenant_id, id),
  CHECK (status IN ('queued', 'processing', 'completed', 'retryable_error', 'terminal_error', 'invalidated')),
  CHECK (decision IS NULL OR decision IN ('selected', 'blank', 'multiple', 'ambiguous')),
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
  CHECK (attempt_no >= 0 AND (duration_ms IS NULL OR duration_ms >= 0)),
  CHECK (jsonb_typeof(measurements) = 'array' AND jsonb_typeof(selected_options) = 'array' AND jsonb_typeof(risk_flags) = 'array')
);

CREATE INDEX idx_omr_run_segment
ON omr_run (tenant_id, answer_segment_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE TABLE question_grade (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  question_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  scoring_run_id UUID,
  answer_candidate_id UUID,
  scoring_rule_id UUID,
  review_task_id UUID,
  source TEXT NOT NULL,
  status TEXT NOT NULL,
  score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  evidence JSONB NOT NULL DEFAULT '{}',
  version INT NOT NULL DEFAULT 1,
  supersedes_id UUID,
  is_current BOOLEAN NOT NULL DEFAULT true,
  confirmed_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, answer_segment_id) REFERENCES answer_segment(tenant_id, id),
  FOREIGN KEY (tenant_id, scoring_run_id) REFERENCES scoring_run(tenant_id, id),
  FOREIGN KEY (tenant_id, answer_candidate_id) REFERENCES answer_candidate(tenant_id, id),
  FOREIGN KEY (tenant_id, scoring_rule_id) REFERENCES scoring_rule(tenant_id, id),
  FOREIGN KEY (tenant_id, review_task_id) REFERENCES review_task(tenant_id, id),
  FOREIGN KEY (tenant_id, supersedes_id) REFERENCES question_grade(tenant_id, id),
  FOREIGN KEY (tenant_id, confirmed_by) REFERENCES app_user(tenant_id, id),
  CHECK (source IN ('rule_confirmed', 'human')),
  CHECK (status IN ('confirmed', 'superseded', 'invalidated')),
  CHECK (score >= 0 AND score <= max_score AND max_score >= 0),
  CHECK (version > 0 AND jsonb_typeof(evidence) = 'object')
);

CREATE UNIQUE INDEX uq_question_grade_current
ON question_grade (tenant_id, answer_segment_id)
WHERE is_current AND deleted_at IS NULL;

CREATE INDEX idx_question_grade_exam
ON question_grade (tenant_id, exam_id, question_id, created_at DESC)
WHERE deleted_at IS NULL;

ALTER TABLE review_task
  ADD CONSTRAINT fk_review_task_current_grade_tenant
  FOREIGN KEY (tenant_id, current_grade_id) REFERENCES question_grade(tenant_id, id);

CREATE TABLE review_draft (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  review_task_id UUID NOT NULL,
  reviewer_id UUID NOT NULL,
  score NUMERIC(8,2),
  rubric_selections JSONB NOT NULL DEFAULT '[]',
  comments TEXT NOT NULL DEFAULT '',
  private_note TEXT NOT NULL DEFAULT '',
  student_feedback TEXT NOT NULL DEFAULT '',
  viewer_state JSONB NOT NULL DEFAULT '{}',
  revision INT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, review_task_id, reviewer_id),
  FOREIGN KEY (tenant_id, review_task_id) REFERENCES review_task(tenant_id, id),
  FOREIGN KEY (tenant_id, reviewer_id) REFERENCES app_user(tenant_id, id),
  CHECK (score IS NULL OR score >= 0),
  CHECK (revision > 0),
  CHECK (jsonb_typeof(rubric_selections) = 'array' AND jsonb_typeof(viewer_state) = 'object')
);

CREATE INDEX idx_review_draft_reviewer
ON review_draft (tenant_id, reviewer_id, updated_at DESC)
WHERE deleted_at IS NULL;
