UPDATE exam AS e
SET status = 'published', updated_at = now()
WHERE e.deleted_at IS NULL
  AND e.status IN ('draft', 'configured', 'ready', 'collecting', 'grading', 'reviewing', 'finalized')
  AND EXISTS (
    SELECT 1
    FROM submission_grade AS sg
    WHERE sg.tenant_id = e.tenant_id
      AND sg.exam_id = e.id
      AND sg.deleted_at IS NULL
      AND sg.status = 'published'
      AND sg.locked = TRUE
      AND sg.published_at IS NOT NULL
  );
