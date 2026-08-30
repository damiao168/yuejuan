-- Paper imports now accept any ordered collection of question, answer, solution,
-- or mixed documents. Legacy columns remain readable during the transition.
ALTER TABLE paper_import_job
    ALTER COLUMN exam_paper_id DROP NOT NULL,
    ALTER COLUMN paper_file_asset_id DROP NOT NULL,
    ALTER COLUMN answer_file_asset_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS question_candidates jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS answer_candidates jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS solution_candidates jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS structured_issues jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS paper_import_source (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenant(id),
    paper_import_id uuid NOT NULL REFERENCES paper_import_job(id),
    file_asset_id uuid NOT NULL REFERENCES file_asset(id),
    document_index integer NOT NULL CHECK (document_index >= 0),
    role_hint text NOT NULL DEFAULT 'auto' CHECK (role_hint IN ('auto','question','answer','solution','mixed','unknown')),
    detected_role text NOT NULL DEFAULT 'unknown' CHECK (detected_role IN ('question','answer','solution','mixed','unknown')),
    role_confidence numeric(5,4) NOT NULL DEFAULT 0 CHECK (role_confidence >= 0 AND role_confidence <= 1),
    processing_status text NOT NULL DEFAULT 'pending' CHECK (processing_status IN ('pending','processing','processed','failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

-- Early development versions of this migration used an unconditional table
-- constraint. Drop it so a soft-deleted source can release its order slot.
ALTER TABLE paper_import_source
    DROP CONSTRAINT IF EXISTS paper_import_source_paper_import_id_document_index_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_paper_import_source_order_active
ON paper_import_source (tenant_id, paper_import_id, document_index)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_paper_import_source_asset_active
ON paper_import_source (tenant_id, paper_import_id, file_asset_id)
WHERE deleted_at IS NULL;

-- Keep machine extraction provenance attached to canonical records after apply.
ALTER TABLE question
    ADD COLUMN IF NOT EXISTS paper_import_id uuid REFERENCES paper_import_job(id),
    ADD COLUMN IF NOT EXISTS paper_import_candidate_id text,
    ADD COLUMN IF NOT EXISTS paper_import_source_refs jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE question_answer_key
    ADD COLUMN IF NOT EXISTS paper_import_id uuid REFERENCES paper_import_job(id),
    ADD COLUMN IF NOT EXISTS paper_import_candidate_id text,
    ADD COLUMN IF NOT EXISTS paper_import_source_refs jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE question_rubric
    ADD COLUMN IF NOT EXISTS paper_import_id uuid REFERENCES paper_import_job(id),
    ADD COLUMN IF NOT EXISTS paper_import_source_refs jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS question_solution (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenant(id),
    question_id uuid NOT NULL REFERENCES question(id),
    paper_import_id uuid REFERENCES paper_import_job(id),
    solution_version text NOT NULL,
    raw_text text NOT NULL,
    steps jsonb NOT NULL DEFAULT '[]'::jsonb,
    source_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    verification_status text NOT NULL DEFAULT 'machine' CHECK (verification_status IN ('machine','human_confirmed','conflicted')),
    created_by uuid NOT NULL REFERENCES app_user(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (tenant_id, question_id, solution_version)
);

CREATE INDEX IF NOT EXISTS idx_question_solution_current
ON question_solution (tenant_id, question_id, created_at DESC)
WHERE deleted_at IS NULL;

-- Backfill old jobs without changing their historical meaning or ordering.
INSERT INTO paper_import_source
    (tenant_id, paper_import_id, file_asset_id, document_index, role_hint, detected_role, role_confidence, processing_status)
SELECT tenant_id, id, paper_file_asset_id, 0, 'question', 'question', 1, 'processed'
FROM paper_import_job
WHERE paper_file_asset_id IS NOT NULL AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO paper_import_source
    (tenant_id, paper_import_id, file_asset_id, document_index, role_hint, detected_role, role_confidence, processing_status)
SELECT tenant_id, id, answer_file_asset_id, 1, 'answer', 'answer', 1, 'processed'
FROM paper_import_job
WHERE answer_file_asset_id IS NOT NULL
  AND answer_file_asset_id IS DISTINCT FROM paper_file_asset_id
  AND deleted_at IS NULL
ON CONFLICT DO NOTHING;
