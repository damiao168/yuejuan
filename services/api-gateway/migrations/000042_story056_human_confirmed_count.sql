ALTER TABLE scoring_run
  ADD COLUMN human_confirmed_count INT NOT NULL DEFAULT 0;

ALTER TABLE scoring_run
  DROP CONSTRAINT scoring_run_check,
  DROP CONSTRAINT scoring_run_check1,
  ADD CONSTRAINT chk_scoring_run_nonnegative_counts
    CHECK (
      total_count >= 0
      AND queued_count >= 0
      AND auto_confirmed_count >= 0
      AND human_confirmed_count >= 0
      AND review_count >= 0
      AND failed_count >= 0
    ),
  ADD CONSTRAINT chk_scoring_run_count_capacity
    CHECK (
      auto_confirmed_count + human_confirmed_count + review_count + failed_count <= total_count
    );
