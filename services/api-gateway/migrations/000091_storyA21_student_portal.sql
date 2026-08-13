-- STORY-A21: the student portal reads only the current immutable, published
-- release.  This is a security-barrier projection, not a second score fact:
-- totals and item scores remain owned by score_release_* (STORY-A18).
CREATE OR REPLACE VIEW student_published_exam
WITH (security_barrier = true)
AS
SELECT
  current_release.tenant_id,
  current_release.exam_id,
  item.student_id,
  release.version AS release_version,
  release.published_at,
  exam.name AS exam_name,
  exam.subject AS subject
FROM score_release_current current_release
JOIN score_release release
  ON release.tenant_id = current_release.tenant_id
 AND release.id = current_release.release_id
 AND release.exam_id = current_release.exam_id
 AND release.status = 'published'
JOIN score_release_item item
  ON item.tenant_id = release.tenant_id
 AND item.release_id = release.id
JOIN exam
  ON exam.tenant_id = current_release.tenant_id
 AND exam.id = current_release.exam_id
 AND exam.deleted_at IS NULL
WHERE item.student_id IS NOT NULL;

COMMENT ON VIEW student_published_exam IS
  'Student-safe index of current published score releases. It contains no scores, review facts, model output or private annotations.';
