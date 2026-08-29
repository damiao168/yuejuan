-- Make every aligned grading observation capable of explaining which layer
-- failed. Raw answers and images remain outside this evidence-only subsystem.

ALTER TABLE grading_evaluation_observation
  ADD COLUMN page_match_correct BOOLEAN,
  ADD COLUMN crop_iou DOUBLE PRECISION,
  ADD COLUMN transcription_cer DOUBLE PRECISION,
  ADD COLUMN formula_exact BOOLEAN,
  ADD COLUMN rubric_criterion_agreement DOUBLE PRECISION,
  ADD COLUMN error_source TEXT NOT NULL DEFAULT 'none',
  ADD COLUMN needs_human_review BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN reference_reviewer_count INT NOT NULL DEFAULT 1,
  ADD COLUMN reference_adjudicated BOOLEAN NOT NULL DEFAULT false;

UPDATE grading_evaluation_observation
SET error_source='unattributed'
WHERE abs(model_score-reference_score) > 0.000000001;

ALTER TABLE grading_evaluation_observation
  ADD CONSTRAINT chk_grading_evaluation_pipeline_evidence CHECK (
    (crop_iou IS NULL OR crop_iou BETWEEN 0 AND 1)
    AND (transcription_cer IS NULL OR transcription_cer BETWEEN 0 AND 1)
    AND (rubric_criterion_agreement IS NULL OR rubric_criterion_agreement BETWEEN 0 AND 1)
    AND reference_reviewer_count >= 1
    AND error_source IN (
      'none','image_quality','page_matching','answer_crop','handwriting_ocr',
      'formula_recognition','answer_structuring','rubric','model_scoring',
      'score_calculation','system','unattributed'
    )
  );

ALTER TABLE grading_evaluation_slice_metric
  DROP CONSTRAINT chk_grading_evaluation_slice_dimension;
ALTER TABLE grading_evaluation_slice_metric
  ADD CONSTRAINT chk_grading_evaluation_slice_dimension CHECK (
    dimension IN ('subject','archetype','score_band','ocr_quality','answer_length','rubric_complexity','error_source')
  );

