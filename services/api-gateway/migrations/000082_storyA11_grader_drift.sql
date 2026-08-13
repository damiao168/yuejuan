-- STORY-A11: Seed observations are aggregated into reviewer-quality windows.
-- The tables intentionally store aggregate/error evidence only. They never
-- duplicate a Gold answer, its image, or its reference rubric.

CREATE TABLE grader_quality_window (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  grader_id UUID NOT NULL,
  window_size INT NOT NULL,
  window_start TIMESTAMPTZ NOT NULL,
  window_end TIMESTAMPTZ NOT NULL,
  sample_count INT NOT NULL,
  mean_error NUMERIC(10,4) NOT NULL,
  mae NUMERIC(10,4) NOT NULL,
  exact_agreement NUMERIC(7,6) NOT NULL,
  rubric_agreement NUMERIC(7,6),
  severe_rate NUMERIC(7,6) NOT NULL,
  middle_score_sample_count INT NOT NULL DEFAULT 0,
  middle_score_mae NUMERIC(10,4),
  middle_score_exact_agreement NUMERIC(7,6),
  ewma_bias NUMERIC(10,4),
  status TEXT NOT NULL,
  computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, exam_id, question_id, grader_id, window_size, window_start, window_end),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, grader_id) REFERENCES app_user(tenant_id, id),
  CHECK (window_size IN (20, 50)),
  CHECK (window_start <= window_end),
  CHECK (sample_count >= 0 AND sample_count <= window_size),
  CHECK (mean_error = mean_error AND mae >= 0 AND mae = mae),
  CHECK (exact_agreement BETWEEN 0 AND 1),
  CHECK (rubric_agreement IS NULL OR rubric_agreement BETWEEN 0 AND 1),
  CHECK (severe_rate BETWEEN 0 AND 1),
  CHECK (middle_score_sample_count >= 0 AND middle_score_sample_count <= sample_count),
  CHECK (middle_score_mae IS NULL OR (middle_score_mae >= 0 AND middle_score_mae = middle_score_mae)),
  CHECK (middle_score_exact_agreement IS NULL OR middle_score_exact_agreement BETWEEN 0 AND 1),
  CHECK (ewma_bias IS NULL OR ewma_bias = ewma_bias),
  CHECK (status IN ('insufficient_data','stable','warning','critical'))
);

CREATE INDEX idx_grader_quality_window_lookup
ON grader_quality_window (tenant_id, exam_id, question_id, grader_id, computed_at DESC);

CREATE TABLE grading_quality_incident (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  grader_id UUID NOT NULL,
  source_window_id UUID NOT NULL,
  incident_type TEXT NOT NULL,
  severity TEXT NOT NULL,
  metric_snapshot_json JSONB NOT NULL,
  affected_range_json JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  UNIQUE (tenant_id, source_window_id, incident_type),
  FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, grader_id) REFERENCES app_user(tenant_id, id),
  FOREIGN KEY (source_window_id) REFERENCES grader_quality_window(id),
  CHECK (incident_type IN (
    'grader_bias_high','grader_bias_low','high_inconsistency','severe_seed_failure',
    'rubric_disagreement_spike','suspicious_speed'
  )),
  CHECK (severity IN ('warning','critical')),
  CHECK (jsonb_typeof(metric_snapshot_json) = 'object'),
  CHECK (jsonb_typeof(affected_range_json) = 'object'),
  CHECK (status IN ('open','acknowledged','resolved')),
  CHECK ((status = 'resolved') = (resolved_at IS NOT NULL))
);

CREATE INDEX idx_grading_quality_incident_open
ON grading_quality_incident (tenant_id, exam_id, question_id, grader_id, created_at DESC)
WHERE status IN ('open','acknowledged');
