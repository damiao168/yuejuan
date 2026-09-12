-- High-risk student and grading data is constrained by the authenticated
-- tenant context installed by the API connection wrapper. Missing context is
-- denied; cross-tenant maintenance requires an explicit process-only marker.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'edugrade_tenant_runtime') THEN
    CREATE ROLE edugrade_tenant_runtime
      NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
  ELSE
    ALTER ROLE edugrade_tenant_runtime
      NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
  END IF;
END $$;

GRANT edugrade_tenant_runtime TO CURRENT_USER;
GRANT USAGE ON SCHEMA public TO edugrade_tenant_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO edugrade_tenant_runtime;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO edugrade_tenant_runtime;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO edugrade_tenant_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO edugrade_tenant_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO edugrade_tenant_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT EXECUTE ON FUNCTIONS TO edugrade_tenant_runtime;

DO $$
BEGIN
  IF to_regclass('public.schema_migration') IS NOT NULL THEN
    REVOKE INSERT, UPDATE, DELETE ON schema_migration FROM edugrade_tenant_runtime;
  END IF;
END $$;

CREATE OR REPLACE FUNCTION edugrade_tenant_matches(row_tenant UUID)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
PARALLEL SAFE
AS $$
  SELECT current_setting('edugrade.tenant_id', true) = 'maintenance'
      OR current_setting('edugrade.tenant_id', true) = row_tenant::text
$$;

COMMENT ON FUNCTION edugrade_tenant_matches(UUID) IS
'Matches rows to the authenticated API tenant; only an explicit maintenance setting permits cross-tenant work.';

REVOKE ALL ON FUNCTION edugrade_tenant_matches(UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION edugrade_tenant_matches(UUID) TO edugrade_tenant_runtime;

DO $$
DECLARE
  protected_table TEXT;
BEGIN
  FOREACH protected_table IN ARRAY ARRAY[
    'student',
    'submission',
    'submission_page',
    'answer_segment',
    'ocr_result',
    'human_grade',
    'final_grade',
    'appeal',
    'file_asset'
  ]
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', protected_table);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', protected_table);
    EXECUTE format('DROP POLICY IF EXISTS edugrade_tenant_isolation ON %I', protected_table);
    EXECUTE format(
      'CREATE POLICY edugrade_tenant_isolation ON %I FOR ALL USING (edugrade_tenant_matches(tenant_id)) WITH CHECK (edugrade_tenant_matches(tenant_id))',
      protected_table
    );
  END LOOP;
END $$;
