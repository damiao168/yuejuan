-- STORY-060B: calibrate one immutable OMR profile for the whole answer-sheet
-- template. Legacy question-scoped approvals remain readable and fail closed.
ALTER TABLE omr_calibration_session
  ADD COLUMN scope_type TEXT NOT NULL DEFAULT 'question',
  ADD COLUMN question_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN minimum_samples_per_stratum INT NOT NULL DEFAULT 10,
  ADD COLUMN inherited_from_session_id UUID;

UPDATE omr_calibration_session
SET question_ids = jsonb_build_array(question_id::text)
WHERE question_id IS NOT NULL AND question_ids = '[]'::jsonb;

ALTER TABLE omr_calibration_session
  ALTER COLUMN question_id DROP NOT NULL;

ALTER TABLE omr_calibration_session
  DROP CONSTRAINT omr_calibration_session_question_type_check,
  ADD CONSTRAINT chk_omr_calibration_session_scope
    CHECK (
      (scope_type = 'question' AND question_id IS NOT NULL AND question_type IN ('single_choice', 'true_false')) OR
      (scope_type = 'template' AND question_id IS NULL AND question_type = 'template')
    ),
  ADD CONSTRAINT chk_omr_calibration_session_question_ids
    CHECK (
      jsonb_typeof(question_ids) = 'array'
      AND jsonb_array_length(question_ids) >= 1
      AND minimum_samples_per_stratum >= 10
    ),
  ADD CONSTRAINT fk_omr_calibration_session_inherited_from
    FOREIGN KEY (tenant_id, inherited_from_session_id)
    REFERENCES omr_calibration_session(tenant_id, id);

DROP INDEX uq_omr_calibration_session_approved_scope;

CREATE UNIQUE INDEX uq_omr_calibration_session_approved_question_scope
ON omr_calibration_session (
  tenant_id, template_id, template_content_hash, question_id,
  profile_version, profile_hash, reference_file_asset_id, reference_sha256
)
WHERE scope_type = 'question' AND status = 'approved' AND deleted_at IS NULL;

CREATE UNIQUE INDEX uq_omr_calibration_session_approved_template_scope
ON omr_calibration_session (
  tenant_id, template_id, template_content_hash,
  profile_version, profile_hash, reference_file_asset_id, reference_sha256
)
WHERE scope_type = 'template' AND status = 'approved' AND deleted_at IS NULL;

ALTER TABLE omr_calibration_case
  ADD COLUMN question_id UUID,
  ADD COLUMN question_no TEXT NOT NULL DEFAULT '',
  ADD COLUMN question_type TEXT NOT NULL DEFAULT 'single_choice',
  ADD COLUMN option_labels JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN sample_stratum TEXT NOT NULL DEFAULT 'selected_high';

UPDATE omr_calibration_case c
SET question_id = s.question_id,
    question_no = q.question_no,
    question_type = s.question_type,
    option_labels = s.option_labels
FROM omr_calibration_session s
JOIN question q ON q.tenant_id = s.tenant_id AND q.id = s.question_id
WHERE c.tenant_id = s.tenant_id
  AND c.calibration_session_id = s.id
  AND c.question_id IS NULL;

ALTER TABLE omr_calibration_case
  ALTER COLUMN question_id SET NOT NULL,
  DROP CONSTRAINT omr_calibration_case_observed_decision_check,
  DROP CONSTRAINT omr_calibration_case_observed_options_check,
  DROP CONSTRAINT omr_calibration_case_check,
  ADD CONSTRAINT fk_omr_calibration_case_question
    FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  ADD CONSTRAINT chk_omr_calibration_case_observed_decision
    CHECK (observed_decision IN ('selected', 'blank', 'multiple', 'ambiguous')),
  ADD CONSTRAINT chk_omr_calibration_case_observed_options
    CHECK (jsonb_typeof(observed_options) = 'array'),
  ADD CONSTRAINT chk_omr_calibration_case_option_labels
    CHECK (jsonb_typeof(option_labels) = 'array' AND jsonb_array_length(option_labels) >= 2),
  ADD CONSTRAINT chk_omr_calibration_case_sample_stratum
    CHECK (sample_stratum IN ('selected_high', 'selected_low', 'blank', 'ambiguous')),
  ADD CONSTRAINT chk_omr_calibration_case_label
    CHECK (
      (expected_options IS NULL AND matches IS NULL AND labeled_by IS NULL AND labeled_at IS NULL) OR
      (
        expected_options IS NOT NULL
        AND jsonb_typeof(expected_options) = 'array'
        AND matches IS NOT NULL
        AND labeled_by IS NOT NULL
        AND labeled_at IS NOT NULL
      )
    );

CREATE INDEX idx_omr_calibration_case_template_sample
ON omr_calibration_case (tenant_id, calibration_session_id, sample_stratum, question_id);
