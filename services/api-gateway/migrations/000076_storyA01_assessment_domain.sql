CREATE TABLE subject_profile (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  code TEXT NOT NULL,
  education_stage TEXT NOT NULL,
  subject_code TEXT NOT NULL,
  version INT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  parser_policy_json JSONB NOT NULL DEFAULT '{}',
  evidence_policy_json JSONB NOT NULL DEFAULT '{}',
  scoring_default_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, code, version),
  CHECK (length(btrim(code)) BETWEEN 1 AND 120),
  CHECK (education_stage IN ('junior', 'senior')),
  CHECK (subject_code IN (
    'chinese', 'mathematics', 'english', 'physics', 'chemistry',
    'biology', 'history', 'geography', 'ethics_politics'
  )),
  CHECK (version > 0),
  CHECK (status IN ('active', 'retired')),
  CHECK (jsonb_typeof(parser_policy_json) = 'object'),
  CHECK (jsonb_typeof(evidence_policy_json) = 'object'),
  CHECK (jsonb_typeof(COALESCE(evidence_policy_json -> 'allowed_types', '[]'::jsonb)) = 'array'),
  CHECK (jsonb_typeof(scoring_default_json) = 'object')
);

CREATE INDEX idx_subject_profile_lookup
ON subject_profile (tenant_id, education_stage, subject_code, status, version DESC);

CREATE TABLE question_archetype (
  code TEXT PRIMARY KEY,
  response_schema_json JSONB NOT NULL DEFAULT '{}',
  evidence_types_json JSONB NOT NULL DEFAULT '[]',
  default_scoring_mode TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (code IN (
    'selected_response', 'exact_text', 'numeric_expression', 'structured_steps',
    'short_constructed', 'extended_response', 'diagram_graph', 'table_experiment'
  )),
  CHECK (jsonb_typeof(response_schema_json) = 'object'),
  CHECK (jsonb_typeof(evidence_types_json) = 'array'),
  CHECK (default_scoring_mode IN (
    'RULE_AUTO', 'AI_ASSIST', 'AI_FAST_CONFIRM',
    'HUMAN_PRIMARY', 'DUAL_HUMAN', 'MANUAL_ONLY'
  ))
);

INSERT INTO question_archetype (
  code, response_schema_json, evidence_types_json, default_scoring_mode
)
VALUES
  ('selected_response', '{"type":"selected_response"}', '["selected_option"]', 'RULE_AUTO'),
  ('exact_text', '{"type":"text","comparison":"normalized_exact"}', '["exact_text","text_span"]', 'RULE_AUTO'),
  ('numeric_expression', '{"type":"numeric_expression"}', '["numeric_value","math_expression","unit_value"]', 'RULE_AUTO'),
  ('structured_steps', '{"type":"ordered_steps"}', '["text_span","math_expression","math_step","unit_value","chemical_equation"]', 'AI_ASSIST'),
  ('short_constructed', '{"type":"constructed_response","length":"short"}', '["text_span","concept","relation"]', 'AI_ASSIST'),
  ('extended_response', '{"type":"constructed_response","length":"extended","rubric":"multi_trait"}', '["text_span","concept","relation"]', 'HUMAN_PRIMARY'),
  ('diagram_graph', '{"type":"diagram_or_graph"}', '["diagram_feature","text_span","numeric_value","unit_value"]', 'HUMAN_PRIMARY'),
  ('table_experiment', '{"type":"table_or_experiment"}', '["table_cell","diagram_feature","text_span","numeric_value","unit_value"]', 'HUMAN_PRIMARY');

CREATE OR REPLACE FUNCTION assessment_seed_subject_profiles(p_tenant_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
  WITH subject_seed(subject_code, parsers, evidence_types) AS (
    VALUES
      ('chinese', '["layout_text","chinese_handwriting"]'::jsonb, '["selected_option","exact_text","text_span","concept","relation"]'::jsonb),
      ('mathematics', '["layout_text","math_expression"]'::jsonb, '["selected_option","exact_text","text_span","numeric_value","math_expression","math_step","unit_value","diagram_feature"]'::jsonb),
      ('english', '["layout_text","latin_handwriting"]'::jsonb, '["selected_option","exact_text","text_span","concept","relation"]'::jsonb),
      ('physics', '["layout_text","math_expression","diagram"]'::jsonb, '["selected_option","exact_text","text_span","numeric_value","math_expression","math_step","unit_value","diagram_feature","table_cell"]'::jsonb),
      ('chemistry', '["layout_text","chemical_equation","math_expression"]'::jsonb, '["selected_option","exact_text","text_span","numeric_value","math_expression","math_step","unit_value","chemical_equation","concept","relation","diagram_feature","table_cell"]'::jsonb),
      ('biology', '["layout_text","diagram","table"]'::jsonb, '["selected_option","exact_text","text_span","numeric_value","unit_value","concept","relation","diagram_feature","table_cell"]'::jsonb),
      ('history', '["layout_text","chinese_handwriting"]'::jsonb, '["selected_option","exact_text","text_span","concept","relation"]'::jsonb),
      ('geography', '["layout_text","map","diagram","table"]'::jsonb, '["selected_option","exact_text","text_span","numeric_value","unit_value","concept","relation","diagram_feature","table_cell"]'::jsonb),
      ('ethics_politics', '["layout_text","chinese_handwriting"]'::jsonb, '["selected_option","exact_text","text_span","concept","relation"]'::jsonb)
  ), stage_seed(education_stage) AS (VALUES ('junior'), ('senior'))
  INSERT INTO subject_profile (
    tenant_id, code, education_stage, subject_code, version, status,
    parser_policy_json, evidence_policy_json, scoring_default_json
  )
  SELECT p_tenant_id,
         stage_seed.education_stage || '.' || subject_seed.subject_code || '.standard',
         stage_seed.education_stage,
         subject_seed.subject_code,
         1,
         'active',
         jsonb_build_object('parsers', subject_seed.parsers),
         jsonb_build_object('allowed_types', subject_seed.evidence_types),
         '{"mode":"AI_ASSIST","require_evidence":true,"human_review_below_confidence":true}'::jsonb
  FROM subject_seed
  CROSS JOIN stage_seed
  ON CONFLICT (tenant_id, code, version) DO NOTHING;
END
$$;

SELECT assessment_seed_subject_profiles(id) FROM tenant;

CREATE OR REPLACE FUNCTION assessment_seed_subject_profiles_for_new_tenant()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM assessment_seed_subject_profiles(NEW.id);
  RETURN NEW;
END
$$;

CREATE TRIGGER trg_assessment_seed_subject_profiles
AFTER INSERT ON tenant
FOR EACH ROW EXECUTE FUNCTION assessment_seed_subject_profiles_for_new_tenant();

CREATE TABLE question_assessment_config (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  subject_profile_id UUID NOT NULL,
  archetype_code TEXT NOT NULL REFERENCES question_archetype(code),
  allowed_evidence_types JSONB NOT NULL DEFAULT '[]',
  risk_tier TEXT NOT NULL,
  scoring_policy_json JSONB NOT NULL,
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, exam_id, question_id),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, subject_profile_id) REFERENCES subject_profile(tenant_id, id),
  CHECK (jsonb_typeof(allowed_evidence_types) = 'array'),
  CHECK (jsonb_typeof(scoring_policy_json) = 'object'),
  CHECK (risk_tier IN ('R1', 'R2', 'R3')),
  CHECK (scoring_policy_json ->> 'mode' IN (
    'RULE_AUTO', 'AI_ASSIST', 'AI_FAST_CONFIRM',
    'HUMAN_PRIMARY', 'DUAL_HUMAN', 'MANUAL_ONLY'
  )),
  CHECK (CASE
    WHEN NOT (scoring_policy_json ? 'confidence_threshold') THEN true
    WHEN jsonb_typeof(scoring_policy_json -> 'confidence_threshold') = 'number'
      THEN (scoring_policy_json ->> 'confidence_threshold')::numeric BETWEEN 0 AND 1
    ELSE false
  END),
  CHECK (revision > 0)
);

CREATE INDEX idx_question_assessment_config_exam
ON question_assessment_config (tenant_id, exam_id, question_id);

CREATE TABLE exam_question_snapshot (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  snapshot_version INT NOT NULL DEFAULT 1,
  subject_profile_id UUID NOT NULL,
  subject_profile_version INT NOT NULL,
  archetype_code TEXT NOT NULL REFERENCES question_archetype(code),
  allowed_evidence_types JSONB NOT NULL,
  risk_tier TEXT NOT NULL,
  profile_snapshot_json JSONB NOT NULL,
  archetype_snapshot_json JSONB NOT NULL,
  rubric_snapshot_json JSONB NOT NULL,
  scoring_policy_snapshot_json JSONB NOT NULL,
  content_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, exam_id, question_id, snapshot_version),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, subject_profile_id) REFERENCES subject_profile(tenant_id, id),
  CHECK (snapshot_version > 0 AND subject_profile_version > 0),
  CHECK (jsonb_typeof(allowed_evidence_types) = 'array'),
  CHECK (risk_tier IN ('R1', 'R2', 'R3')),
  CHECK (jsonb_typeof(profile_snapshot_json) = 'object'),
  CHECK (jsonb_typeof(archetype_snapshot_json) = 'object'),
  CHECK (jsonb_typeof(rubric_snapshot_json) = 'object'),
  CHECK (jsonb_typeof(scoring_policy_snapshot_json) = 'object'),
  CHECK (length(content_hash) = 64)
);

CREATE INDEX idx_exam_question_snapshot_latest
ON exam_question_snapshot (tenant_id, exam_id, question_id, snapshot_version DESC);

ALTER TABLE answer_candidate
  ADD COLUMN exam_question_snapshot_id UUID;

ALTER TABLE answer_candidate
  ADD CONSTRAINT fk_answer_candidate_assessment_snapshot
  FOREIGN KEY (tenant_id, exam_question_snapshot_id)
  REFERENCES exam_question_snapshot(tenant_id, id);

CREATE INDEX idx_answer_candidate_assessment_snapshot
ON answer_candidate (tenant_id, exam_question_snapshot_id)
WHERE exam_question_snapshot_id IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE question_grade
  ADD COLUMN exam_question_snapshot_id UUID;

ALTER TABLE question_grade
  ADD CONSTRAINT fk_question_grade_assessment_snapshot
  FOREIGN KEY (tenant_id, exam_question_snapshot_id)
  REFERENCES exam_question_snapshot(tenant_id, id);

CREATE INDEX idx_question_grade_assessment_snapshot
ON question_grade (tenant_id, exam_question_snapshot_id)
WHERE exam_question_snapshot_id IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE ai_grade
  ADD COLUMN exam_question_snapshot_id UUID;

ALTER TABLE ai_grade
  ADD CONSTRAINT fk_ai_grade_assessment_snapshot
  FOREIGN KEY (tenant_id, exam_question_snapshot_id)
  REFERENCES exam_question_snapshot(tenant_id, id);

CREATE INDEX idx_ai_grade_assessment_snapshot
ON ai_grade (tenant_id, exam_question_snapshot_id)
WHERE exam_question_snapshot_id IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE human_grade
  ADD COLUMN exam_question_snapshot_id UUID;

ALTER TABLE human_grade
  ADD CONSTRAINT fk_human_grade_assessment_snapshot
  FOREIGN KEY (tenant_id, exam_question_snapshot_id)
  REFERENCES exam_question_snapshot(tenant_id, id);

CREATE INDEX idx_human_grade_assessment_snapshot
ON human_grade (tenant_id, exam_question_snapshot_id)
WHERE exam_question_snapshot_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE scoring_evidence (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  submission_id UUID NOT NULL,
  question_id UUID NOT NULL,
  exam_question_snapshot_id UUID NOT NULL,
  evidence_type TEXT NOT NULL,
  source_artifact_id UUID NOT NULL,
  rubric_criterion_key TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  bbox_json JSONB,
  quality NUMERIC(6,5),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, exam_question_snapshot_id) REFERENCES exam_question_snapshot(tenant_id, id),
  FOREIGN KEY (tenant_id, source_artifact_id) REFERENCES file_asset(tenant_id, id),
  CHECK (evidence_type IN (
    'selected_option', 'exact_text', 'text_span', 'numeric_value',
    'math_expression', 'math_step', 'unit_value', 'chemical_equation',
    'concept', 'relation', 'diagram_feature', 'table_cell'
  )),
  CHECK (jsonb_typeof(payload_json) = 'object'),
  CHECK (bbox_json IS NULL OR jsonb_typeof(bbox_json) = 'object'),
  CHECK (quality IS NULL OR (quality >= 0 AND quality <= 1))
);

CREATE INDEX idx_scoring_evidence_answer
ON scoring_evidence (tenant_id, submission_id, question_id, created_at, id);

CREATE OR REPLACE FUNCTION assessment_validate_question_config()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  current_exam_status TEXT;
  current_exam_subject TEXT;
  profile_subject TEXT;
  normalized_exam_subject TEXT;
  profile_evidence JSONB;
  archetype_evidence JSONB;
BEGIN
  SELECT status, subject INTO current_exam_status, current_exam_subject
  FROM exam
  WHERE tenant_id = NEW.tenant_id AND id = NEW.exam_id AND deleted_at IS NULL;

  IF current_exam_status IS NULL THEN
    RAISE EXCEPTION USING ERRCODE = '23503', MESSAGE = 'assessment exam not found';
  END IF;
  IF current_exam_status NOT IN ('draft', 'configured') THEN
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'assessment configuration is frozen';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM question
    WHERE tenant_id = NEW.tenant_id AND exam_id = NEW.exam_id
      AND id = NEW.question_id AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION USING ERRCODE = '23503', MESSAGE = 'assessment question does not belong to exam';
  END IF;

  SELECT COALESCE(evidence_policy_json -> 'allowed_types', '[]'::jsonb), subject_code
  INTO profile_evidence, profile_subject
  FROM subject_profile
  WHERE tenant_id = NEW.tenant_id AND id = NEW.subject_profile_id AND status = 'active';

  SELECT evidence_types_json INTO archetype_evidence
  FROM question_archetype WHERE code = NEW.archetype_code;

  IF profile_evidence IS NULL OR archetype_evidence IS NULL THEN
    RAISE EXCEPTION USING ERRCODE = '23514', MESSAGE = 'assessment profile or archetype is not active';
  END IF;

  normalized_exam_subject := CASE lower(btrim(current_exam_subject))
    WHEN 'math' THEN 'mathematics'
    WHEN '数学' THEN 'mathematics'
    WHEN 'politics' THEN 'ethics_politics'
    WHEN 'civics' THEN 'ethics_politics'
    WHEN '政治' THEN 'ethics_politics'
    WHEN '道德与法治' THEN 'ethics_politics'
    WHEN '思想政治' THEN 'ethics_politics'
    WHEN '语文' THEN 'chinese'
    WHEN '英语' THEN 'english'
    WHEN '物理' THEN 'physics'
    WHEN '化学' THEN 'chemistry'
    WHEN '生物' THEN 'biology'
    WHEN '历史' THEN 'history'
    WHEN '地理' THEN 'geography'
    ELSE lower(btrim(current_exam_subject))
  END;
  IF normalized_exam_subject IN (
       'chinese', 'mathematics', 'english', 'physics', 'chemistry',
       'biology', 'history', 'geography', 'ethics_politics'
     ) AND normalized_exam_subject <> profile_subject THEN
    RAISE EXCEPTION USING ERRCODE = '23514', MESSAGE = 'assessment profile subject does not match exam';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM jsonb_array_elements_text(NEW.allowed_evidence_types) AS selected(value)
    WHERE NOT (profile_evidence ? selected.value)
       OR NOT (archetype_evidence ? selected.value)
  ) THEN
    RAISE EXCEPTION USING ERRCODE = '23514', MESSAGE = 'assessment evidence type is not allowed';
  END IF;

  IF NEW.risk_tier = 'R3'
     AND NEW.archetype_code = 'extended_response'
     AND NEW.scoring_policy_json ->> 'mode' = 'AI_FAST_CONFIRM' THEN
    RAISE EXCEPTION USING ERRCODE = '23514', MESSAGE = 'R3 extended response cannot use AI_FAST_CONFIRM';
  END IF;

  RETURN NEW;
END
$$;

CREATE OR REPLACE FUNCTION assessment_guard_config_delete()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE current_exam_status TEXT;
BEGIN
  SELECT status INTO current_exam_status
  FROM exam WHERE tenant_id = OLD.tenant_id AND id = OLD.exam_id;
  IF current_exam_status NOT IN ('draft', 'configured') THEN
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'assessment configuration is frozen';
  END IF;
  RETURN OLD;
END
$$;

CREATE TRIGGER trg_assessment_guard_config_delete
BEFORE DELETE ON question_assessment_config
FOR EACH ROW EXECUTE FUNCTION assessment_guard_config_delete();

CREATE OR REPLACE FUNCTION assessment_freeze_question_snapshot(
  p_tenant_id UUID,
  p_exam_id UUID,
  p_question_id UUID
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
  config_record RECORD;
  rubric_snapshot JSONB;
  profile_snapshot JSONB;
  archetype_snapshot JSONB;
  snapshot_hash TEXT;
  snapshot_id UUID;
BEGIN
  SELECT config.*, profile.code AS profile_code, profile.education_stage,
         profile.subject_code, profile.version AS profile_version,
         profile.parser_policy_json, profile.evidence_policy_json,
         profile.scoring_default_json, archetype.response_schema_json,
         archetype.evidence_types_json, archetype.default_scoring_mode
  INTO config_record
  FROM question_assessment_config config
  JOIN subject_profile profile
    ON profile.tenant_id = config.tenant_id AND profile.id = config.subject_profile_id
  JOIN question_archetype archetype ON archetype.code = config.archetype_code
  JOIN question
    ON question.tenant_id = config.tenant_id AND question.id = config.question_id
   AND question.exam_id = config.exam_id AND question.deleted_at IS NULL
  WHERE config.tenant_id = p_tenant_id
    AND config.exam_id = p_exam_id
    AND config.question_id = p_question_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING ERRCODE = 'P0001', MESSAGE = 'assessment profile missing for question';
  END IF;

  SELECT jsonb_build_object(
           'id', rubric.id,
           'version_id', rubric_version.id,
           'version', rubric_version.version,
           'status', rubric.status,
           'max_score', rubric.max_score,
           'points', rubric.points,
           'deductions', rubric.deductions,
           'examples', rubric.examples,
           'content_hash', rubric_version.content_hash
         )
  INTO rubric_snapshot
  FROM question_rubric rubric
  JOIN rubric_version
    ON rubric_version.tenant_id = rubric.tenant_id
   AND rubric_version.id = rubric.rubric_version_id
  WHERE rubric.tenant_id = p_tenant_id
    AND rubric.question_id = p_question_id
    AND rubric.deleted_at IS NULL
    AND rubric_version.deleted_at IS NULL
  ORDER BY rubric.created_at DESC, rubric.id DESC
  LIMIT 1;

  rubric_snapshot := COALESCE(rubric_snapshot, '{}'::jsonb);
  profile_snapshot := jsonb_build_object(
    'id', config_record.subject_profile_id,
    'code', config_record.profile_code,
    'education_stage', config_record.education_stage,
    'subject_code', config_record.subject_code,
    'version', config_record.profile_version,
    'parser_policy', config_record.parser_policy_json,
    'evidence_policy', config_record.evidence_policy_json,
    'scoring_default', config_record.scoring_default_json
  );
  archetype_snapshot := jsonb_build_object(
    'code', config_record.archetype_code,
    'response_schema', config_record.response_schema_json,
    'evidence_types', config_record.evidence_types_json,
    'default_scoring_mode', config_record.default_scoring_mode
  );
  snapshot_hash := encode(digest(
    profile_snapshot::text || archetype_snapshot::text || rubric_snapshot::text ||
    config_record.allowed_evidence_types::text || config_record.risk_tier ||
    config_record.scoring_policy_json::text,
    'sha256'
  ), 'hex');

  INSERT INTO exam_question_snapshot (
    tenant_id, exam_id, question_id, snapshot_version,
    subject_profile_id, subject_profile_version, archetype_code,
    allowed_evidence_types, risk_tier, profile_snapshot_json,
    archetype_snapshot_json, rubric_snapshot_json,
    scoring_policy_snapshot_json, content_hash
  )
  VALUES (
    p_tenant_id, p_exam_id, p_question_id, 1,
    config_record.subject_profile_id, config_record.profile_version,
    config_record.archetype_code, config_record.allowed_evidence_types,
    config_record.risk_tier, profile_snapshot, archetype_snapshot,
    rubric_snapshot, config_record.scoring_policy_json, snapshot_hash
  )
  ON CONFLICT (tenant_id, exam_id, question_id, snapshot_version) DO NOTHING
  RETURNING id INTO snapshot_id;

  IF snapshot_id IS NULL THEN
    SELECT id INTO snapshot_id
    FROM exam_question_snapshot
    WHERE tenant_id = p_tenant_id AND exam_id = p_exam_id
      AND question_id = p_question_id AND snapshot_version = 1;
  END IF;
  RETURN snapshot_id;
END
$$;

CREATE OR REPLACE FUNCTION assessment_apply_default_question_config(
  p_tenant_id UUID,
  p_exam_id UUID,
  p_question_id UUID
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
  exam_subject TEXT;
  exam_grading_mode TEXT;
  question_type_value TEXT;
  canonical_subject TEXT;
  stage_value TEXT;
  archetype_value TEXT;
  profile_id_value UUID;
  profile_evidence JSONB;
  archetype_evidence JSONB;
  scoring_mode_value TEXT;
  allowed_evidence JSONB;
BEGIN
  IF EXISTS (
    SELECT 1 FROM question_assessment_config
    WHERE tenant_id = p_tenant_id AND exam_id = p_exam_id AND question_id = p_question_id
  ) THEN
    RETURN;
  END IF;

  SELECT exam.subject, exam.grading_mode, question.question_type,
         CASE WHEN EXISTS (
           SELECT 1
           FROM exam_class
           JOIN school_class
             ON school_class.tenant_id = exam_class.tenant_id
            AND school_class.id = exam_class.class_id
           JOIN grade
             ON grade.tenant_id = school_class.tenant_id
            AND grade.id = school_class.grade_id
           WHERE exam_class.tenant_id = exam.tenant_id
             AND exam_class.exam_id = exam.id
             AND exam_class.deleted_at IS NULL
             AND school_class.deleted_at IS NULL
             AND grade.deleted_at IS NULL
             AND grade.level_no >= 10
         ) THEN 'senior' ELSE 'junior' END
  INTO exam_subject, exam_grading_mode, question_type_value, stage_value
  FROM exam
  JOIN question
    ON question.tenant_id = exam.tenant_id AND question.exam_id = exam.id
  WHERE exam.tenant_id = p_tenant_id AND exam.id = p_exam_id
    AND question.id = p_question_id
    AND exam.deleted_at IS NULL AND question.deleted_at IS NULL
    AND question.status <> 'deleted';

  IF NOT FOUND THEN
    RETURN;
  END IF;

  canonical_subject := CASE lower(btrim(exam_subject))
    WHEN 'math' THEN 'mathematics'
    WHEN '数学' THEN 'mathematics'
    WHEN 'politics' THEN 'ethics_politics'
    WHEN 'civics' THEN 'ethics_politics'
    WHEN '政治' THEN 'ethics_politics'
    WHEN '道德与法治' THEN 'ethics_politics'
    WHEN '思想政治' THEN 'ethics_politics'
    WHEN '语文' THEN 'chinese'
    WHEN '英语' THEN 'english'
    WHEN '物理' THEN 'physics'
    WHEN '化学' THEN 'chemistry'
    WHEN '生物' THEN 'biology'
    WHEN '历史' THEN 'history'
    WHEN '地理' THEN 'geography'
    ELSE lower(btrim(exam_subject))
  END;

  archetype_value := CASE question_type_value
    WHEN 'single_choice' THEN 'selected_response'
    WHEN 'multiple_choice' THEN 'selected_response'
    WHEN 'true_false' THEN 'selected_response'
    WHEN 'fill_blank' THEN 'exact_text'
    WHEN 'numeric' THEN 'numeric_expression'
    WHEN 'formula' THEN 'numeric_expression'
    WHEN 'calculation' THEN 'structured_steps'
    WHEN 'short_answer' THEN 'short_constructed'
    WHEN 'essay' THEN 'extended_response'
    WHEN 'discussion' THEN 'extended_response'
    ELSE 'structured_steps'
  END;

  SELECT profile.id, profile.evidence_policy_json -> 'allowed_types',
         archetype.evidence_types_json,
         CASE
           WHEN exam_grading_mode IN ('double_mark', 'blind_double_mark') THEN 'DUAL_HUMAN'
           ELSE archetype.default_scoring_mode
         END
  INTO profile_id_value, profile_evidence, archetype_evidence, scoring_mode_value
  FROM subject_profile profile
  JOIN question_archetype archetype ON archetype.code = archetype_value
  WHERE profile.tenant_id = p_tenant_id
    AND profile.education_stage = stage_value
    AND profile.subject_code = canonical_subject
    AND profile.status = 'active'
  ORDER BY profile.version DESC
  LIMIT 1;

  IF profile_id_value IS NULL THEN
    RETURN;
  END IF;

  SELECT COALESCE(jsonb_agg(value ORDER BY value), '[]'::jsonb)
  INTO allowed_evidence
  FROM jsonb_array_elements_text(archetype_evidence) evidence(value)
  WHERE profile_evidence ? evidence.value;

  INSERT INTO question_assessment_config (
    tenant_id, exam_id, question_id, subject_profile_id, archetype_code,
    allowed_evidence_types, risk_tier, scoring_policy_json
  )
  VALUES (
    p_tenant_id, p_exam_id, p_question_id, profile_id_value, archetype_value,
    allowed_evidence, 'R2', jsonb_build_object(
      'mode', scoring_mode_value,
      'require_evidence', true,
      'human_review_below_confidence', true
    )
  )
  ON CONFLICT (tenant_id, exam_id, question_id) DO NOTHING;
END
$$;

CREATE OR REPLACE FUNCTION assessment_freeze_exam_on_ready()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE question_record RECORD;
BEGIN
  IF NEW.status = 'ready' AND OLD.status IS DISTINCT FROM NEW.status THEN
    FOR question_record IN
      SELECT id FROM question
      WHERE tenant_id = NEW.tenant_id AND exam_id = NEW.id
        AND deleted_at IS NULL AND status <> 'deleted'
      ORDER BY sort_order, question_no
    LOOP
      PERFORM assessment_apply_default_question_config(NEW.tenant_id, NEW.id, question_record.id);
    END LOOP;

    IF EXISTS (
      SELECT 1
      FROM question question_row
      LEFT JOIN question_assessment_config config
        ON config.tenant_id = question_row.tenant_id
       AND config.exam_id = question_row.exam_id
       AND config.question_id = question_row.id
      WHERE question_row.tenant_id = NEW.tenant_id
        AND question_row.exam_id = NEW.id
        AND question_row.deleted_at IS NULL
        AND question_row.status <> 'deleted'
        AND config.id IS NULL
    ) THEN
      RAISE EXCEPTION USING ERRCODE = 'P0001', MESSAGE = 'assessment profile missing for exam question';
    END IF;

    FOR question_record IN
      SELECT id FROM question
      WHERE tenant_id = NEW.tenant_id AND exam_id = NEW.id
        AND deleted_at IS NULL AND status <> 'deleted'
      ORDER BY sort_order, question_no
    LOOP
      PERFORM assessment_freeze_question_snapshot(NEW.tenant_id, NEW.id, question_record.id);
    END LOOP;
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER trg_assessment_freeze_exam_on_ready
BEFORE UPDATE OF status ON exam
FOR EACH ROW EXECUTE FUNCTION assessment_freeze_exam_on_ready();

CREATE OR REPLACE FUNCTION assessment_reject_snapshot_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'assessment snapshot is immutable';
END
$$;

CREATE TRIGGER trg_assessment_snapshot_immutable
BEFORE UPDATE OR DELETE ON exam_question_snapshot
FOR EACH ROW EXECUTE FUNCTION assessment_reject_snapshot_mutation();

CREATE OR REPLACE FUNCTION assessment_guard_question_definition()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  row_data JSONB;
  target_question_id UUID;
  current_exam_status TEXT;
BEGIN
  row_data := CASE WHEN TG_OP = 'DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
  target_question_id := CASE
    WHEN TG_TABLE_NAME = 'question' THEN (row_data ->> 'id')::uuid
    ELSE (row_data ->> 'question_id')::uuid
  END;

  SELECT exam.status INTO current_exam_status
  FROM question
  JOIN exam ON exam.tenant_id = question.tenant_id AND exam.id = question.exam_id
  WHERE question.tenant_id = (row_data ->> 'tenant_id')::uuid
    AND question.id = target_question_id;

  IF current_exam_status IS NOT NULL AND current_exam_status NOT IN ('draft', 'configured') THEN
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'assessment question definition is frozen';
  END IF;
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER trg_assessment_guard_question
BEFORE UPDATE OR DELETE ON question
FOR EACH ROW EXECUTE FUNCTION assessment_guard_question_definition();

CREATE TRIGGER trg_assessment_guard_rubric_version
BEFORE INSERT OR UPDATE OR DELETE ON rubric_version
FOR EACH ROW EXECUTE FUNCTION assessment_guard_question_definition();

CREATE TRIGGER trg_assessment_guard_question_rubric
BEFORE INSERT OR UPDATE OR DELETE ON question_rubric
FOR EACH ROW EXECUTE FUNCTION assessment_guard_question_definition();

CREATE OR REPLACE FUNCTION assessment_reject_active_profile_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.status = 'active' THEN
    IF TG_OP = 'UPDATE'
       AND NEW.status = 'retired'
       AND (to_jsonb(NEW) - 'status' - 'updated_at') = (to_jsonb(OLD) - 'status' - 'updated_at') THEN
      RETURN NEW;
    END IF;
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'active subject profile content is immutable';
  END IF;
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER trg_assessment_subject_profile_immutable
BEFORE UPDATE OR DELETE ON subject_profile
FOR EACH ROW EXECUTE FUNCTION assessment_reject_active_profile_mutation();

-- Preserve pre-A01 exams without rewriting their historical question facts. New
-- exams still require explicit assessment configuration before entering ready.
WITH legacy_question AS (
  SELECT question.tenant_id, question.exam_id, question.id AS question_id,
         CASE lower(btrim(exam.subject))
           WHEN 'math' THEN 'mathematics'
           WHEN '数学' THEN 'mathematics'
           WHEN 'politics' THEN 'ethics_politics'
           WHEN 'civics' THEN 'ethics_politics'
           WHEN '政治' THEN 'ethics_politics'
           WHEN '道德与法治' THEN 'ethics_politics'
           WHEN '思想政治' THEN 'ethics_politics'
           WHEN '语文' THEN 'chinese'
           WHEN '英语' THEN 'english'
           WHEN '物理' THEN 'physics'
           WHEN '化学' THEN 'chemistry'
           WHEN '生物' THEN 'biology'
           WHEN '历史' THEN 'history'
           WHEN '地理' THEN 'geography'
           ELSE lower(btrim(exam.subject))
         END AS subject_code,
         CASE question.question_type
           WHEN 'single_choice' THEN 'selected_response'
           WHEN 'multiple_choice' THEN 'selected_response'
           WHEN 'true_false' THEN 'selected_response'
           WHEN 'fill_blank' THEN 'exact_text'
           WHEN 'numeric' THEN 'numeric_expression'
           WHEN 'formula' THEN 'numeric_expression'
           WHEN 'calculation' THEN 'structured_steps'
           WHEN 'short_answer' THEN 'short_constructed'
           WHEN 'essay' THEN 'extended_response'
           WHEN 'discussion' THEN 'extended_response'
           ELSE 'structured_steps'
         END AS archetype_code,
         exam.grading_mode
  FROM question
  JOIN exam ON exam.tenant_id = question.tenant_id AND exam.id = question.exam_id
  WHERE question.deleted_at IS NULL AND question.status <> 'deleted'
), legacy_config AS (
  SELECT legacy_question.*, profile.id AS profile_id,
         archetype.evidence_types_json,
         profile.evidence_policy_json -> 'allowed_types' AS profile_evidence,
         archetype.default_scoring_mode
  FROM legacy_question
  JOIN subject_profile profile
    ON profile.tenant_id = legacy_question.tenant_id
   AND profile.education_stage = 'junior'
   AND profile.subject_code = legacy_question.subject_code
   AND profile.version = 1 AND profile.status = 'active'
  JOIN question_archetype archetype ON archetype.code = legacy_question.archetype_code
)
INSERT INTO question_assessment_config (
  tenant_id, exam_id, question_id, subject_profile_id, archetype_code,
  allowed_evidence_types, risk_tier, scoring_policy_json
)
SELECT legacy_config.tenant_id, legacy_config.exam_id, legacy_config.question_id,
       legacy_config.profile_id, legacy_config.archetype_code,
       COALESCE((
         SELECT jsonb_agg(value ORDER BY value)
         FROM jsonb_array_elements_text(legacy_config.evidence_types_json) evidence(value)
         WHERE legacy_config.profile_evidence ? evidence.value
       ), '[]'::jsonb),
       'R2',
       jsonb_build_object(
         'mode', CASE
           WHEN legacy_config.grading_mode IN ('double_mark', 'blind_double_mark') THEN 'DUAL_HUMAN'
           ELSE legacy_config.default_scoring_mode
         END,
         'require_evidence', true,
         'human_review_below_confidence', true
       )
FROM legacy_config
ON CONFLICT (tenant_id, exam_id, question_id) DO NOTHING;

CREATE TRIGGER trg_assessment_validate_question_config
BEFORE INSERT OR UPDATE ON question_assessment_config
FOR EACH ROW EXECUTE FUNCTION assessment_validate_question_config();

DO $$
DECLARE item RECORD;
BEGIN
  FOR item IN
    SELECT question.tenant_id, question.exam_id, question.id AS question_id
    FROM question
    JOIN exam ON exam.tenant_id = question.tenant_id AND exam.id = question.exam_id
    JOIN question_assessment_config config
      ON config.tenant_id = question.tenant_id
     AND config.exam_id = question.exam_id
     AND config.question_id = question.id
    WHERE question.deleted_at IS NULL AND question.status <> 'deleted'
      AND exam.status IN ('ready', 'collecting', 'grading', 'reviewing', 'finalized', 'published', 'archived')
  LOOP
    PERFORM assessment_freeze_question_snapshot(item.tenant_id, item.exam_id, item.question_id);
  END LOOP;
END
$$;
