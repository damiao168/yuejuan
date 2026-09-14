-- Historical exam questions may be selected only from immutable content facts
-- captured with a readiness confirmation. Existing hash-only records remain
-- explicit unavailable sources.
ALTER TABLE exam_readiness_snapshot
  ADD COLUMN import_snapshot_schema_version SMALLINT,
  ADD COLUMN import_snapshot_hash TEXT,
  ADD COLUMN import_snapshot_json JSONB,
  ADD CONSTRAINT exam_readiness_snapshot_import_shape CHECK (
    (import_snapshot_schema_version IS NULL AND import_snapshot_hash IS NULL AND import_snapshot_json IS NULL)
    OR
    (import_snapshot_schema_version = 1
      AND import_snapshot_hash ~ '^[0-9a-f]{64}$'
      AND jsonb_typeof(import_snapshot_json) = 'object'
      AND jsonb_typeof(import_snapshot_json->'questions') = 'array')
  );

ALTER TABLE exam_readiness_snapshot
  ADD CONSTRAINT exam_readiness_snapshot_tenant_id UNIQUE (tenant_id, id);

CREATE OR REPLACE FUNCTION guard_exam_readiness_import_snapshot()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'readiness snapshots must be retained' USING ERRCODE = '23514';
  END IF;
  IF (NEW.id, NEW.tenant_id, NEW.exam_id, NEW.configuration_hash,
      NEW.import_snapshot_schema_version, NEW.import_snapshot_hash, NEW.import_snapshot_json,
      NEW.confirmed_by, NEW.confirmed_at)
     IS DISTINCT FROM
     (OLD.id, OLD.tenant_id, OLD.exam_id, OLD.configuration_hash,
      OLD.import_snapshot_schema_version, OLD.import_snapshot_hash, OLD.import_snapshot_json,
      OLD.confirmed_by, OLD.confirmed_at) THEN
    RAISE EXCEPTION 'readiness source facts are immutable' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END $$;

CREATE TRIGGER exam_readiness_import_snapshot_guard
BEFORE UPDATE OR DELETE ON exam_readiness_snapshot
FOR EACH ROW EXECUTE FUNCTION guard_exam_readiness_import_snapshot();

CREATE TABLE question_bank_import (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  target_bank_id UUID NOT NULL,
  target_item_id UUID NOT NULL,
  target_version_id UUID NOT NULL,
  target_schema_version INTEGER NOT NULL CHECK (target_schema_version > 0),
  source_exam_id UUID NOT NULL,
  source_question_id UUID NOT NULL,
  source_question_no TEXT NOT NULL CHECK (length(source_question_no) <= 32),
  source_readiness_snapshot_id UUID NOT NULL,
  source_configuration_hash TEXT NOT NULL CHECK (source_configuration_hash ~ '^[0-9a-f]{64}$'),
  source_snapshot_hash TEXT NOT NULL CHECK (source_snapshot_hash ~ '^[0-9a-f]{64}$'),
  source_assessment_snapshot_id UUID NOT NULL,
  source_assessment_hash TEXT NOT NULL CHECK (source_assessment_hash ~ '^[0-9a-f]{64}$'),
  imported_content_hash TEXT NOT NULL CHECK (imported_content_hash ~ '^[0-9a-f]{64}$'),
  imported_bundle_hash TEXT NOT NULL CHECK (imported_bundle_hash ~ '^[0-9a-f]{64}$'),
  scoring_source TEXT NOT NULL CHECK (scoring_source = 'original_exam'),
  dedup_decision TEXT NOT NULL CHECK (dedup_decision IN ('new_item','new_version')),
  linked_item_id UUID,
  mapping JSONB NOT NULL CHECK (jsonb_typeof(mapping) = 'object'),
  provenance JSONB NOT NULL CHECK (jsonb_typeof(provenance) = 'object'),
  issues JSONB NOT NULL CHECK (jsonb_typeof(issues) = 'array'),
  command_id TEXT NOT NULL CHECK (length(btrim(command_id)) BETWEEN 1 AND 200),
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, target_version_id),
  FOREIGN KEY (tenant_id, target_bank_id) REFERENCES question_bank(tenant_id, id),
  FOREIGN KEY (tenant_id, target_item_id, target_version_id) REFERENCES question_bank_item_version(tenant_id, item_id, id),
  FOREIGN KEY (tenant_id, source_exam_id) REFERENCES exam(tenant_id, id),
  FOREIGN KEY (tenant_id, source_question_id) REFERENCES question(tenant_id, id),
  FOREIGN KEY (tenant_id, source_readiness_snapshot_id) REFERENCES exam_readiness_snapshot(tenant_id, id),
  FOREIGN KEY (tenant_id, source_assessment_snapshot_id) REFERENCES exam_question_snapshot(tenant_id, id),
  FOREIGN KEY (tenant_id, linked_item_id) REFERENCES question_bank_item(tenant_id, id),
  FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id)
);

CREATE FUNCTION question_bank_import_source_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS (
  SELECT 1 FROM exam_readiness_snapshot r
  JOIN exam_question_snapshot a ON a.tenant_id=r.tenant_id AND a.exam_id=r.exam_id AND a.id=NEW.source_assessment_snapshot_id
  JOIN question_bank_item_version v ON v.tenant_id=NEW.tenant_id AND v.id=NEW.target_version_id AND v.item_id=NEW.target_item_id
  JOIN question_bank_item i ON i.tenant_id=v.tenant_id AND i.id=v.item_id AND i.bank_id=NEW.target_bank_id
  WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.source_readiness_snapshot_id AND r.exam_id=NEW.source_exam_id
   AND r.configuration_hash=NEW.source_configuration_hash AND r.import_snapshot_hash=NEW.source_snapshot_hash
   AND a.question_id=NEW.source_question_id AND a.content_hash=NEW.source_assessment_hash
   AND v.schema_version=NEW.target_schema_version AND v.workflow_status='draft'
   AND v.content_hash=NEW.imported_content_hash AND v.bundle_hash=NEW.imported_bundle_hash
   AND EXISTS (SELECT 1 FROM jsonb_array_elements(r.import_snapshot_json->'questions') q
    WHERE q->>'id'=NEW.source_question_id::text AND q->>'assessment_snapshot_id'=a.id::text AND q->>'assessment_snapshot_hash'=a.content_hash)
 ) THEN RAISE EXCEPTION 'import source and target facts must match' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER question_bank_import_source_guard BEFORE INSERT ON question_bank_import
 FOR EACH ROW EXECUTE FUNCTION question_bank_import_source_guard();

CREATE INDEX question_bank_import_source_idx
  ON question_bank_import (tenant_id, source_exam_id, source_question_id, created_at DESC);

CREATE OR REPLACE FUNCTION question_bank_import_immutable()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'question bank import provenance is immutable' USING ERRCODE = '23514';
END $$;
CREATE TRIGGER question_bank_import_immutable
BEFORE UPDATE OR DELETE ON question_bank_import
FOR EACH ROW EXECUTE FUNCTION question_bank_import_immutable();

ALTER TABLE question_bank_import ENABLE ROW LEVEL SECURITY;
ALTER TABLE question_bank_import FORCE ROW LEVEL SECURITY;
CREATE POLICY edugrade_tenant_isolation ON question_bank_import
FOR ALL USING (edugrade_tenant_matches(tenant_id))
WITH CHECK (edugrade_tenant_matches(tenant_id));
