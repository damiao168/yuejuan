-- STORY-056 implementation-review fix: a template-difference OMR run can become automatically
-- confirmable only through a separately auditable calibration approval.
CREATE TABLE omr_calibration_session (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  template_id UUID NOT NULL,
  template_content_hash TEXT NOT NULL,
  question_id UUID NOT NULL,
  question_type TEXT NOT NULL,
  profile_version TEXT NOT NULL,
  profile_hash TEXT NOT NULL,
  reference_file_asset_id UUID NOT NULL,
  reference_sha256 TEXT NOT NULL,
  option_labels JSONB NOT NULL,
  sample_seed UUID NOT NULL DEFAULT gen_random_uuid(),
  sample_count INT NOT NULL DEFAULT 0,
  minimum_samples INT NOT NULL,
  minimum_samples_per_option INT NOT NULL,
  minimum_confidence DOUBLE PRECISION NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  approved_by UUID,
  approved_at TIMESTAMPTZ,
  approval_note TEXT NOT NULL DEFAULT '',
  evidence_hash TEXT,
  revoked_by UUID,
  revoked_at TIMESTAMPTZ,
  revoke_reason TEXT NOT NULL DEFAULT '',
  discarded_by UUID,
  discarded_at TIMESTAMPTZ,
  discard_reason TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, template_id) REFERENCES answer_sheet_template(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, reference_file_asset_id) REFERENCES file_asset(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, approved_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, revoked_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, discarded_by) REFERENCES app_user(tenant_id, id),
  CHECK (question_type IN ('single_choice', 'true_false')),
  CHECK (jsonb_typeof(option_labels) = 'array' AND jsonb_array_length(option_labels) >= 2),
  CHECK (sample_count >= 0 AND sample_count <= minimum_samples AND minimum_samples >= 100 AND minimum_samples_per_option >= 10),
  CHECK (minimum_confidence >= 0.98 AND minimum_confidence <= 1),
  CHECK (status IN ('draft', 'approved', 'revoked', 'discarded')),
  CHECK (length(btrim(template_content_hash)) >= 16 AND length(btrim(profile_hash)) >= 16 AND length(btrim(reference_sha256)) >= 16),
  CHECK (
    (status = 'draft' AND approved_by IS NULL AND approved_at IS NULL AND evidence_hash IS NULL AND revoked_by IS NULL AND revoked_at IS NULL AND discarded_by IS NULL AND discarded_at IS NULL) OR
    (status = 'approved' AND approved_by IS NOT NULL AND approved_at IS NOT NULL AND length(btrim(approval_note)) >= 10 AND length(btrim(evidence_hash)) >= 16 AND revoked_by IS NULL AND revoked_at IS NULL AND discarded_by IS NULL AND discarded_at IS NULL) OR
    (status = 'revoked' AND approved_by IS NOT NULL AND approved_at IS NOT NULL AND length(btrim(evidence_hash)) >= 16 AND revoked_by IS NOT NULL AND revoked_at IS NOT NULL AND length(btrim(revoke_reason)) >= 10 AND discarded_by IS NULL AND discarded_at IS NULL) OR
    (status = 'discarded' AND approved_by IS NULL AND approved_at IS NULL AND evidence_hash IS NULL AND revoked_by IS NULL AND revoked_at IS NULL AND discarded_by IS NOT NULL AND discarded_at IS NOT NULL AND length(btrim(discard_reason)) >= 10)
  )
);

CREATE INDEX idx_omr_calibration_session_template
ON omr_calibration_session (tenant_id, template_id, question_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_omr_calibration_session_approved_scope
ON omr_calibration_session (tenant_id, template_id, template_content_hash, question_id, profile_version, profile_hash, reference_file_asset_id, reference_sha256)
WHERE status = 'approved' AND deleted_at IS NULL;

CREATE TABLE omr_calibration_case (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  calibration_session_id UUID NOT NULL,
  omr_run_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  crop_sha256 TEXT NOT NULL,
  observed_decision TEXT NOT NULL,
  observed_options JSONB NOT NULL,
  observed_confidence DOUBLE PRECISION NOT NULL,
  measurements JSONB NOT NULL,
  expected_options JSONB,
  matches BOOLEAN,
  labeled_by UUID,
  labeled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, calibration_session_id, omr_run_id),
  UNIQUE (tenant_id, calibration_session_id, answer_segment_id),
  FOREIGN KEY (tenant_id, calibration_session_id) REFERENCES omr_calibration_session(tenant_id, id),
  FOREIGN KEY (tenant_id, omr_run_id) REFERENCES omr_run(tenant_id, id),
  FOREIGN KEY (tenant_id, answer_segment_id) REFERENCES answer_segment(tenant_id, id),
  FOREIGN KEY (tenant_id, labeled_by) REFERENCES app_user(tenant_id, id),
  CHECK (observed_decision = 'selected'),
  CHECK (observed_confidence >= 0 AND observed_confidence <= 1),
  CHECK (jsonb_typeof(observed_options) = 'array' AND jsonb_array_length(observed_options) = 1),
  CHECK (jsonb_typeof(measurements) = 'array'),
  CHECK (
    (expected_options IS NULL AND matches IS NULL AND labeled_by IS NULL AND labeled_at IS NULL) OR
    (expected_options IS NOT NULL AND jsonb_typeof(expected_options) = 'array' AND jsonb_array_length(expected_options) = 1 AND matches IS NOT NULL AND labeled_by IS NOT NULL AND labeled_at IS NOT NULL)
  )
);

CREATE INDEX idx_omr_calibration_case_session
ON omr_calibration_case (tenant_id, calibration_session_id, labeled_at, created_at);

ALTER TABLE omr_run
  ADD COLUMN calibration_session_id UUID,
  ADD COLUMN calibration_evidence_hash TEXT,
  ADD COLUMN auto_confirm_min_confidence DOUBLE PRECISION NOT NULL DEFAULT 0.9;

ALTER TABLE omr_run
  ADD CONSTRAINT fk_omr_run_calibration_session_tenant
  FOREIGN KEY (tenant_id, calibration_session_id)
  REFERENCES omr_calibration_session (tenant_id, id);

ALTER TABLE omr_run
  ADD CONSTRAINT chk_omr_run_calibration_snapshot
  CHECK (
    (calibration_session_id IS NULL AND calibration_evidence_hash IS NULL) OR
    (calibration_session_id IS NOT NULL AND length(btrim(calibration_evidence_hash)) >= 16)
  ),
  ADD CONSTRAINT chk_omr_run_auto_confirm_min_confidence
  CHECK (
    auto_confirm_min_confidence >= 0 AND auto_confirm_min_confidence <= 1
    AND (calibration_session_id IS NULL OR auto_confirm_min_confidence >= 0.98)
  );
