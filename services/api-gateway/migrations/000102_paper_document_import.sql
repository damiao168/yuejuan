CREATE TABLE IF NOT EXISTS paper_import_job (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenant(id),
    exam_id uuid NOT NULL REFERENCES exam(id),
    exam_paper_id uuid NOT NULL REFERENCES exam_paper(id),
    paper_file_asset_id uuid NOT NULL REFERENCES file_asset(id),
    answer_file_asset_id uuid NOT NULL REFERENCES file_asset(id),
    status text NOT NULL CHECK (status IN ('processing', 'review_required', 'failed', 'applied')),
    subject text NOT NULL DEFAULT '',
    draft_questions jsonb NOT NULL DEFAULT '[]'::jsonb,
    issues jsonb NOT NULL DEFAULT '[]'::jsonb,
    error_code text NOT NULL DEFAULT '',
    created_by uuid NOT NULL REFERENCES app_user(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    applied_at timestamptz,
    deleted_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_paper_import_job_exam
ON paper_import_job (tenant_id, exam_id, created_at DESC)
WHERE deleted_at IS NULL;
