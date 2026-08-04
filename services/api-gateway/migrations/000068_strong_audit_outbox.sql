CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS event_outbox (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  aggregate_type TEXT NOT NULL,
  aggregate_id UUID NOT NULL,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at TIMESTAMPTZ,
  attempt_count INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error TEXT,
  CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_event_outbox_pending
ON event_outbox (next_attempt_at, occurred_at, id)
WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_event_outbox_aggregate
ON event_outbox (tenant_id, aggregate_type, aggregate_id, occurred_at DESC);

CREATE OR REPLACE FUNCTION enqueue_business_mutation_outbox()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  row_data JSONB;
  row_id UUID;
  row_tenant_id UUID;
BEGIN
  row_data := CASE WHEN TG_OP = 'DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
  row_id := (row_data->>'id')::uuid;
  row_tenant_id := (row_data->>'tenant_id')::uuid;
  INSERT INTO event_outbox (
    tenant_id, aggregate_type, aggregate_id, event_type, payload
  ) VALUES (
    row_tenant_id,
    TG_TABLE_NAME,
    row_id,
    TG_TABLE_NAME || '.' || lower(TG_OP),
    jsonb_strip_nulls(jsonb_build_object(
      'operation', lower(TG_OP),
      'status', row_data->>'status',
      'revision', row_data->>'revision',
      'request_correlation', current_setting('edugrade.request_id', true)
    ))
  );
  PERFORM pg_notify('edugrade_outbox', row_tenant_id::text);
  RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END
$$;

DO $$
DECLARE
  table_name TEXT;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'exam', 'file_asset', 'submission', 'review_task', 'arbitration_task',
    'final_grade', 'submission_grade', 'appeal', 'model_provider',
    'model_deployment', 'tenant_model_policy', 'model_approval'
  ]
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%I_outbox ON %I', table_name, table_name);
    EXECUTE format(
      'CREATE TRIGGER trg_%I_outbox AFTER INSERT OR UPDATE OR DELETE ON %I '
      'FOR EACH ROW EXECUTE FUNCTION enqueue_business_mutation_outbox()',
      table_name, table_name
    );
  END LOOP;
END
$$;

ALTER TABLE audit_log
  ADD COLUMN IF NOT EXISTS previous_hash TEXT,
  ADD COLUMN IF NOT EXISTS record_hash TEXT,
  ADD COLUMN IF NOT EXISTS chain_version INT NOT NULL DEFAULT 1;

CREATE OR REPLACE FUNCTION audit_log_chain_before_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  prior_hash TEXT;
  canonical TEXT;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id::text, 0));
  SELECT record_hash
    INTO prior_hash
  FROM audit_log
  WHERE tenant_id = NEW.tenant_id AND record_hash IS NOT NULL
  ORDER BY created_at DESC, id DESC
  LIMIT 1;

  NEW.previous_hash := COALESCE(prior_hash, repeat('0', 64));
  canonical := concat_ws('|',
    NEW.previous_hash,
    NEW.tenant_id::text,
    COALESCE(NEW.actor_id::text, ''),
    NEW.action,
    NEW.target_type,
    COALESCE(NEW.target_id::text, ''),
    COALESCE(NEW.before_value::text, ''),
    COALESCE(NEW.after_value::text, ''),
    COALESCE(NEW.reason, ''),
    COALESCE(NEW.request_id, ''),
    NEW.created_at::text,
    NEW.chain_version::text
  );
  NEW.record_hash := encode(digest(convert_to(canonical, 'UTF8'), 'sha256'), 'hex');
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_audit_log_hash_chain ON audit_log;
CREATE TRIGGER trg_audit_log_hash_chain
BEFORE INSERT ON audit_log
FOR EACH ROW EXECUTE FUNCTION audit_log_chain_before_insert();

CREATE OR REPLACE FUNCTION reject_audit_log_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'audit_log is append-only';
END
$$;

DROP TRIGGER IF EXISTS trg_audit_log_append_only ON audit_log;
CREATE TRIGGER trg_audit_log_append_only
BEFORE UPDATE OR DELETE ON audit_log
FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();

CREATE UNIQUE INDEX IF NOT EXISTS uq_audit_log_tenant_record_hash
ON audit_log (tenant_id, record_hash)
WHERE record_hash IS NOT NULL;

COMMENT ON TABLE event_outbox IS
'Transactional outbox populated by database triggers in the same transaction as critical business mutations.';
COMMENT ON COLUMN audit_log.record_hash IS
'Per-tenant SHA-256 hash chain for new append-only audit rows; legacy rows remain unhashed.';
