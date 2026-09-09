-- Keep historical exceptions associated with their original exam when a page
-- moves. The destination gets its own issue lifecycle and manual decisions.
ALTER TABLE operational_exception
  DROP CONSTRAINT operational_exception_tenant_id_source_type_source_id_code_key,
  ADD CONSTRAINT uq_operational_exception_exam_source UNIQUE (tenant_id,exam_id,source_type,source_id,code);

CREATE VIEW processing_active_submission_page AS
SELECT p.tenant_id,p.id AS page_id,p.submission_id,s.exam_id
FROM submission_page p JOIN submission s ON s.tenant_id=p.tenant_id AND s.id=p.submission_id
WHERE p.deleted_at IS NULL AND s.deleted_at IS NULL AND (
  NOT EXISTS(SELECT 1 FROM capture_page cp WHERE cp.tenant_id=p.tenant_id AND cp.submission_page_id=p.id)
  OR EXISTS(SELECT 1 FROM capture_page cp WHERE cp.tenant_id=p.tenant_id AND cp.submission_page_id=p.id AND cp.deleted_at IS NULL AND cp.status<>'deleted')
);

COMMENT ON VIEW processing_active_submission_page IS
'Current effective processing page set, excluding deleted submissions, pages and withdrawn capture pages.';
