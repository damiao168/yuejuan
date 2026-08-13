-- STORY-A10: hidden Seed Papers are structurally isolated from student grades.
-- No Seed table contains submission_id, answer_segment_id, final_grade_id or a
-- score-release foreign key, so a Seed completion cannot enter grade facts.

CREATE TABLE seed_sampling_policy (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  rate NUMERIC(7,6) NOT NULL,
  min_interval INT NOT NULL,
  max_interval INT NOT NULL,
  active_gold_fingerprint TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'paused',
  revision BIGINT NOT NULL DEFAULT 1,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, exam_id, question_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  CHECK (rate > 0 AND rate <= 1),
  CHECK (min_interval > 0 AND max_interval >= min_interval AND max_interval <= 100000),
  CHECK (length(active_gold_fingerprint) = 64),
  CHECK (status IN ('active','paused')),
  CHECK (revision > 0)
);

CREATE TABLE seed_sampling_cursor (
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  grader_id UUID NOT NULL,
  claims_since_seed INT NOT NULL DEFAULT 0,
  force_at_interval INT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, exam_id, question_id, grader_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, grader_id) REFERENCES app_user(tenant_id, id),
  CHECK (claims_since_seed >= 0),
  CHECK (force_at_interval > 0)
);

CREATE TABLE seed_task (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  question_no TEXT NOT NULL DEFAULT '',
  grader_id UUID NOT NULL,
  gold_paper_id UUID NOT NULL,
  gold_version INT NOT NULL,
  exam_question_snapshot_id UUID NOT NULL,
  anonymous_code TEXT NOT NULL,
  source_image_url TEXT NOT NULL,
  reference_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  archetype_code TEXT NOT NULL,
  expected_criteria_json JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'in_progress',
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, grader_id) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, gold_paper_id, gold_version)
    REFERENCES grading_gold_paper_version(tenant_id, gold_paper_id, version),
  FOREIGN KEY (tenant_id, exam_question_snapshot_id)
    REFERENCES exam_question_snapshot(tenant_id, id),
  CHECK (gold_version > 0),
  CHECK (reference_score >= 0 AND max_score >= reference_score),
  CHECK (jsonb_typeof(expected_criteria_json) = 'object'),
  CHECK (status IN ('in_progress','completed','expired')),
  CHECK (revision > 0)
);

CREATE UNIQUE INDEX uq_seed_task_open_for_grader_question
ON seed_task (tenant_id, exam_id, question_id, grader_id)
WHERE status = 'in_progress';

CREATE INDEX idx_seed_task_grader
ON seed_task (tenant_id, grader_id, created_at DESC);

CREATE TABLE seed_observation (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  seed_task_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  grader_id UUID NOT NULL,
  gold_paper_id UUID NOT NULL,
  gold_version INT NOT NULL,
  submitted_score NUMERIC(8,2) NOT NULL,
  reference_score NUMERIC(8,2) NOT NULL,
  rubric_selection_json JSONB NOT NULL DEFAULT '{}',
  error NUMERIC(8,2) NOT NULL,
  rubric_agreement NUMERIC(7,6),
  observation_kind TEXT NOT NULL,
  trait_observation_json JSONB,
  criterion_observation_json JSONB,
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, seed_task_id),
  FOREIGN KEY (tenant_id, seed_task_id) REFERENCES seed_task(tenant_id, id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, grader_id) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, gold_paper_id, gold_version)
    REFERENCES grading_gold_paper_version(tenant_id, gold_paper_id, version),
  CHECK (gold_version > 0),
  CHECK (submitted_score >= 0 AND reference_score >= 0 AND error >= 0),
  CHECK (jsonb_typeof(rubric_selection_json) = 'object'),
  CHECK (rubric_agreement IS NULL OR rubric_agreement BETWEEN 0 AND 1),
  CHECK (observation_kind IN ('trait','criterion')),
  CHECK (trait_observation_json IS NULL OR jsonb_typeof(trait_observation_json) = 'object'),
  CHECK (criterion_observation_json IS NULL OR jsonb_typeof(criterion_observation_json) = 'object'),
  CHECK ((observation_kind = 'trait' AND trait_observation_json IS NOT NULL AND criterion_observation_json IS NULL)
      OR (observation_kind = 'criterion' AND criterion_observation_json IS NOT NULL AND trait_observation_json IS NULL))
);

CREATE INDEX idx_seed_observation_quality
ON seed_observation (tenant_id, exam_id, question_id, grader_id, observed_at DESC);

CREATE OR REPLACE FUNCTION protect_seed_observation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'Seed observations are immutable quality evidence'
    USING ERRCODE = '55000';
END
$$;

CREATE TRIGGER trg_protect_seed_observation
BEFORE UPDATE OR DELETE ON seed_observation
FOR EACH ROW EXECUTE FUNCTION protect_seed_observation();
