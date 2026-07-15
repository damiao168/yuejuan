ALTER TABLE review_task
  ADD COLUMN scoring_run_id UUID;

ALTER TABLE review_task
  ADD CONSTRAINT fk_review_task_scoring_run_tenant
  FOREIGN KEY (tenant_id, scoring_run_id) REFERENCES scoring_run(tenant_id, id);

CREATE INDEX idx_review_task_scoring_run
ON review_task (tenant_id, scoring_run_id, status)
WHERE scoring_run_id IS NOT NULL AND deleted_at IS NULL;
