-- STORY-A12: quality incidents can trigger a separate back-marking queue.
-- These records are deliberately not question_grade/final_grade facts: a
-- correction still has to travel through confirmation/arbitration/regrade.

-- human_grade predates the tenant-scoped foreign-key hardening and does not
-- yet expose a composite identity. Add it before a backmark item can bind an
-- original grade without allowing a cross-tenant reference.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_human_grade_tenant_id_id'
  ) THEN
    ALTER TABLE human_grade
      ADD CONSTRAINT uq_human_grade_tenant_id_id UNIQUE (tenant_id, id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS backmark_batch (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  question_id UUID NOT NULL REFERENCES question(id),
  source_incident_id TEXT NOT NULL,
  selector_json JSONB NOT NULL DEFAULT '{}',
  policy_json JSONB NOT NULL DEFAULT '{}',
  affected_count INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open',
  created_by UUID NOT NULL REFERENCES app_user(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CHECK (affected_count >= 0),
  CHECK (status IN ('open', 'in_progress', 'ready_for_confirmation', 'completed', 'cancelled')),
  CHECK (jsonb_typeof(selector_json) = 'object'),
  CHECK (jsonb_typeof(policy_json) = 'object')
);

CREATE TABLE IF NOT EXISTS backmark_item (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  batch_id UUID NOT NULL REFERENCES backmark_batch(id) ON DELETE CASCADE,
  review_task_id UUID NOT NULL REFERENCES review_task(id),
  original_grade_id UUID NOT NULL REFERENCES human_grade(id),
  reassigned_task_id UUID REFERENCES review_task(id),
  new_grade_id UUID,
  original_reviewer_id UUID NOT NULL REFERENCES app_user(id),
  reassigned_to UUID NOT NULL REFERENCES app_user(id),
  original_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  new_score NUMERIC(8,2),
  diff NUMERIC(8,2),
  status TEXT NOT NULL DEFAULT 'pending',
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, batch_id, review_task_id),
  UNIQUE (tenant_id, id),
  CHECK (original_score >= 0 AND original_score <= max_score),
  CHECK (new_score IS NULL OR (new_score >= 0 AND new_score <= max_score)),
  CHECK ((new_score IS NULL AND diff IS NULL AND new_grade_id IS NULL) OR
         (new_score IS NOT NULL AND diff IS NOT NULL AND new_grade_id IS NOT NULL)),
  CHECK (status IN ('pending', 'in_progress', 'diff_ready', 'arbitration_required', 'regrade_required', 'cancelled')),
  CHECK (revision > 0),
  FOREIGN KEY (tenant_id, batch_id) REFERENCES backmark_batch(tenant_id, id),
  FOREIGN KEY (tenant_id, review_task_id) REFERENCES review_task(tenant_id, id),
  FOREIGN KEY (tenant_id, original_grade_id) REFERENCES human_grade(tenant_id, id),
  FOREIGN KEY (tenant_id, original_reviewer_id) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, reassigned_to) REFERENCES app_user(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS backmark_grade (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  backmark_item_id UUID NOT NULL REFERENCES backmark_item(id) ON DELETE CASCADE,
  reviewer_id UUID NOT NULL REFERENCES app_user(id),
  score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  rubric_selections JSONB NOT NULL DEFAULT '[]',
  comments TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CHECK (score >= 0 AND score <= max_score),
  CHECK (jsonb_typeof(rubric_selections) = 'array'),
  UNIQUE (tenant_id, backmark_item_id)
);

ALTER TABLE backmark_item
  ADD CONSTRAINT fk_backmark_item_new_grade_tenant
  FOREIGN KEY (tenant_id, new_grade_id) REFERENCES backmark_grade(tenant_id, id);

CREATE INDEX IF NOT EXISTS idx_backmark_batch_exam
  ON backmark_batch (tenant_id, exam_id, question_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_backmark_item_assignee
  ON backmark_item (tenant_id, reassigned_to, status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_backmark_item_batch
  ON backmark_item (tenant_id, batch_id, status, id);

-- No existing review-task source or human-grade round is changed by this
-- migration. Backmark submissions live in backmark_grade until a later A19
-- regrade/release decision explicitly promotes a correction.
