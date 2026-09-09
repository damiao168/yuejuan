-- Preserve accepted material configurations and prevent in-place mutation of
-- already queued parser inputs. Unknown historical configurations stay NULL.
ALTER TABLE paper_import_run ADD COLUMN source_snapshot JSONB;
ALTER TABLE paper_import_run
  ADD CONSTRAINT chk_paper_import_source_snapshot CHECK (source_snapshot IS NULL OR jsonb_typeof(source_snapshot)='array'),
  ADD CONSTRAINT uq_paper_import_run_identity UNIQUE (tenant_id,id,generation);

UPDATE paper_import_run r SET source_snapshot=(
  SELECT COALESCE(jsonb_agg(jsonb_build_object('id',s.id,'file_asset_id',s.file_asset_id,
    'document_index',s.document_index,'role_hint',s.role_hint) ORDER BY s.document_index,s.id),'[]'::jsonb)
  FROM paper_import_source s WHERE s.tenant_id=r.tenant_id AND s.paper_import_id=r.paper_import_id AND s.deleted_at IS NULL
)
FROM paper_import_job j
WHERE j.tenant_id=r.tenant_id AND j.id=r.paper_import_id AND j.current_generation=r.generation
  AND j.source_revision=r.source_revision AND r.source_revision<>'legacy-unverified';

ALTER TABLE agent_worker_task ADD CONSTRAINT fk_paper_import_task_run_generation
  FOREIGN KEY (tenant_id,paper_import_run_id,paper_import_generation) REFERENCES paper_import_run(tenant_id,id,generation);
ALTER TABLE paper_import_parse_input ADD CONSTRAINT fk_paper_import_input_run_generation
  FOREIGN KEY (tenant_id,run_id,generation) REFERENCES paper_import_run(tenant_id,id,generation);

CREATE FUNCTION guard_paper_import_input_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF to_jsonb(NEW) IS DISTINCT FROM to_jsonb(OLD) THEN
    RAISE EXCEPTION 'paper import stage input is immutable' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_paper_import_input_immutable BEFORE UPDATE ON paper_import_parse_input
  FOR EACH ROW EXECUTE FUNCTION guard_paper_import_input_immutable();

CREATE FUNCTION guard_paper_import_source_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.source_snapshot IS NOT NULL AND NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN
    RAISE EXCEPTION 'accepted paper import source snapshot is immutable' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_paper_import_source_snapshot BEFORE UPDATE ON paper_import_run
  FOR EACH ROW EXECUTE FUNCTION guard_paper_import_source_snapshot();

COMMENT ON COLUMN paper_import_run.source_snapshot IS
'Material configuration accepted with this run; NULL means historical configuration cannot be proven.';
