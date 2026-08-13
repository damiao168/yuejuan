CREATE TABLE review_annotation (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  review_task_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  submission_page_id UUID NOT NULL,
  annotation_type TEXT NOT NULL,
  coordinate_space TEXT NOT NULL DEFAULT 'canonical_image_normalized',
  x NUMERIC(12,9) NOT NULL,
  y NUMERIC(12,9) NOT NULL,
  width NUMERIC(12,9) NOT NULL,
  height NUMERIC(12,9) NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  content TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL DEFAULT 'private',
  revision BIGINT NOT NULL DEFAULT 1,
  created_by UUID NOT NULL,
  updated_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, review_task_id) REFERENCES review_task(tenant_id, id),
  FOREIGN KEY (tenant_id, answer_segment_id) REFERENCES answer_segment(tenant_id, id),
  FOREIGN KEY (tenant_id, submission_page_id) REFERENCES submission_page(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, updated_by) REFERENCES app_user(tenant_id, id),
  CHECK (annotation_type IN ('note', 'highlight', 'rectangle', 'freehand')),
  CHECK (coordinate_space = 'canonical_image_normalized'),
  CHECK (x >= 0 AND x <= 1 AND y >= 0 AND y <= 1),
  CHECK (width >= 0 AND height >= 0 AND x + width <= 1 AND y + height <= 1),
  CHECK (jsonb_typeof(payload) = 'object'),
  CHECK (visibility IN ('private', 'student_after_publish')),
  CHECK (revision > 0)
);

CREATE INDEX idx_review_annotation_task
ON review_annotation (tenant_id, review_task_id, created_at, id)
WHERE deleted_at IS NULL;

CREATE INDEX idx_review_annotation_student
ON review_annotation (tenant_id, answer_segment_id, created_at, id)
WHERE visibility = 'student_after_publish' AND deleted_at IS NULL;

CREATE TABLE review_comment_template (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  owner_id UUID NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  shortcut TEXT NOT NULL,
  usage_count BIGINT NOT NULL DEFAULT 0,
  revision BIGINT NOT NULL DEFAULT 1,
  created_by UUID NOT NULL,
  updated_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, owner_id) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (tenant_id, updated_by) REFERENCES app_user(tenant_id, id),
  CHECK (length(btrim(title)) BETWEEN 1 AND 120),
  CHECK (length(btrim(content)) BETWEEN 1 AND 4000),
  CHECK (shortcut = lower(shortcut)),
  CHECK (shortcut ~ '^[a-z0-9][a-z0-9._-]{0,31}$'),
  CHECK (usage_count >= 0),
  CHECK (revision > 0)
);

CREATE UNIQUE INDEX uq_review_comment_template_active_shortcut
ON review_comment_template (tenant_id, owner_id, shortcut)
WHERE deleted_at IS NULL;

CREATE INDEX idx_review_comment_template_owner_usage
ON review_comment_template (tenant_id, owner_id, usage_count DESC, updated_at DESC)
WHERE deleted_at IS NULL;
