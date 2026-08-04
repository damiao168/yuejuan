CREATE OR REPLACE FUNCTION enqueue_business_mutation_outbox()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  before_data JSONB := CASE WHEN TG_OP = 'INSERT' THEN NULL ELSE to_jsonb(OLD) END;
  after_data JSONB := CASE WHEN TG_OP = 'DELETE' THEN NULL ELSE to_jsonb(NEW) END;
  row_data JSONB := COALESCE(after_data, before_data);
  safe_before JSONB;
  safe_after JSONB;
  row_id UUID;
  row_tenant_id UUID;
  actor_text TEXT;
  actor_uuid UUID;
  mutation_reason TEXT;
  request_id TEXT := NULLIF(current_setting('edugrade.request_id', true), '');
  strong_audit BOOLEAN := true;
BEGIN
  row_id := (row_data->>'id')::uuid;
  row_tenant_id := (row_data->>'tenant_id')::uuid;
  safe_before := before_data - ARRAY[
    'password_hash', 'token_hash', 'api_key', 'secret', 'secret_value',
    'access_token', 'refresh_token', 'original_content', 'answer_content'
  ];
  safe_after := after_data - ARRAY[
    'password_hash', 'token_hash', 'api_key', 'secret', 'secret_value',
    'access_token', 'refresh_token', 'original_content', 'answer_content'
  ];

  actor_text := COALESCE(
    NULLIF(row_data->>'published_by', ''),
    NULLIF(row_data->>'confirmed_by', ''),
    NULLIF(row_data->>'reviewed_by', ''),
    NULLIF(row_data->>'arbitrated_by', ''),
    NULLIF(row_data->>'approved_by', ''),
    NULLIF(row_data->>'reviewer_id', ''),
    NULLIF(row_data->>'generated_by', ''),
    NULLIF(row_data->>'created_by', ''),
    NULLIF(row_data->>'updated_by', ''),
    NULLIF(row_data->>'operator_id', '')
  );
  IF actor_text ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' THEN
    actor_uuid := actor_text::uuid;
  END IF;
  mutation_reason := COALESCE(
    NULLIF(row_data->>'reason', ''),
    NULLIF(row_data->>'review_reason', ''),
    NULLIF(row_data->>'decision_reason', ''),
    'transactional database mutation'
  );

  -- Preserve the existing outbox for every protected mutation, but reserve
  -- the hash-chained strong audit stream for security or grading decisions.
  -- This excludes login timestamps, ordinary file creation and submission
  -- pipeline churn that would otherwise drown the non-repudiation trail.
  IF TG_TABLE_NAME = 'submission' THEN
    strong_audit := false;
  ELSIF TG_TABLE_NAME = 'app_user' AND TG_OP = 'UPDATE' THEN
    strong_audit :=
      before_data->>'status' IS DISTINCT FROM after_data->>'status' OR
      before_data->>'password_hash' IS DISTINCT FROM after_data->>'password_hash' OR
      before_data->>'school_id' IS DISTINCT FROM after_data->>'school_id' OR
      before_data->>'deleted_at' IS DISTINCT FROM after_data->>'deleted_at';
  ELSIF TG_TABLE_NAME = 'file_asset' THEN
    strong_audit := TG_OP = 'DELETE' OR (
      TG_OP = 'UPDATE' AND (
        before_data->>'lifecycle_status' IS DISTINCT FROM after_data->>'lifecycle_status' AND
        after_data->>'lifecycle_status' IN ('pending_delete', 'delete_failed', 'deleted', 'missing_object', 'orphan_recovered')
      )
    ) OR (
      TG_OP = 'UPDATE' AND (
        before_data->>'legal_hold' IS DISTINCT FROM after_data->>'legal_hold' OR
        before_data->>'retention_until' IS DISTINCT FROM after_data->>'retention_until'
      )
    );
  ELSIF TG_TABLE_NAME = 'review_task' THEN
    strong_audit := TG_OP = 'UPDATE' AND
      before_data->>'status' IS DISTINCT FROM after_data->>'status' AND
      after_data->>'status' IN ('submitted', 'completed', 'returned');
  END IF;

  IF strong_audit THEN
    INSERT INTO audit_log (
      tenant_id, actor_id, action, target_type, target_id,
      before_value, after_value, reason, request_id
    ) VALUES (
      row_tenant_id, actor_uuid,
      'strong.' || TG_TABLE_NAME || '.' || lower(TG_OP),
      TG_TABLE_NAME, row_id, safe_before, safe_after, mutation_reason, request_id
    );
  END IF;

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
      'request_correlation', request_id
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
    'tenant', 'app_user', 'user_role', 'role_permission',
    'exam', 'file_asset', 'submission', 'review_task', 'arbitration_task',
    'human_grade', 'final_grade', 'submission_grade', 'appeal', 'report',
    'model_provider', 'model_deployment', 'tenant_model_policy', 'model_approval'
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

COMMENT ON FUNCTION enqueue_business_mutation_outbox() IS
'Writes a redacted append-only audit record and an outbox event in the same transaction as each protected business mutation.';
