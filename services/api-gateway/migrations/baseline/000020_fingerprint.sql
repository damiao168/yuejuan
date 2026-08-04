DO $$
DECLARE
  missing TEXT[];
BEGIN
  SELECT array_agg(expected.name ORDER BY expected.name)
  INTO missing
  FROM (VALUES
    ('tenant'), ('app_user'), ('auth_session'), ('audit_log'), ('school'),
    ('school_class'), ('student'), ('exam'), ('file_asset'), ('submission'),
    ('review_task'), ('arbitration_task'), ('submission_grade'), ('appeal')
  ) AS expected(name)
  WHERE to_regclass('public.' || expected.name) IS NULL;
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'migration baseline mismatch: missing tables %', missing;
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (VALUES
      ('tenant', 'id', 'uuid', 'NO'),
      ('tenant', 'tenant_id', 'uuid', 'NO'),
      ('app_user', 'password_hash', 'text', 'NO'),
      ('auth_session', 'token_hash', 'text', 'NO'),
      ('exam', 'status', 'text', 'NO'),
      ('file_asset', 'storage_key', 'text', 'NO'),
      ('submission', 'exam_id', 'uuid', 'NO'),
      ('audit_log', 'created_at', 'timestamp with time zone', 'NO')
    ) AS expected(table_name, column_name, data_type, is_nullable)
    LEFT JOIN information_schema.columns actual
      ON actual.table_schema = 'public'
     AND actual.table_name = expected.table_name
     AND actual.column_name = expected.column_name
    WHERE actual.column_name IS NULL
       OR actual.data_type <> expected.data_type
       OR actual.is_nullable <> expected.is_nullable
  ) THEN
    RAISE EXCEPTION 'migration baseline mismatch: required column shape differs from 000020';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='public' AND tablename='exam' AND indexname='idx_exam_tenant_status')
     OR NOT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='public' AND tablename='audit_log' AND indexname='idx_audit_log_actor_time')
     OR NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_exam_tenant_id_id' AND contype='u')
     OR NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_submission_exam_tenant' AND contype='f') THEN
    RAISE EXCEPTION 'migration baseline mismatch: required index or tenant constraint is missing';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='user_role' AND column_name='data_scope'
      AND data_type='jsonb' AND is_nullable='NO' AND column_default IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'migration baseline mismatch: user_role.data_scope default/not-null differs';
  END IF;
END $$;
