DO $$
DECLARE
  migration_count INT;
  grade_count INT;
  audit_count INT;
  final_count INT;
BEGIN
  SELECT count(*) INTO migration_count FROM schema_migration;
  IF migration_count <> 44 THEN
    RAISE EXCEPTION 'expected 44 migrations, found %', migration_count;
  END IF;

  SELECT count(*) INTO grade_count
  FROM ai_grade
  WHERE tenant_id = '00000000-0000-0000-0000-000000000001'
    AND answer_segment_id = '00000000-0000-0000-0000-000000000614'
    AND status = 'succeeded'
    AND grader_type = 'llm_subjective'
    AND mock = false
    AND needs_human_review = true
    AND confidence = 0
    AND answer_version = '00000000-0000-0000-0000-000000000615'
    AND model_version = 'Qwen/Qwen3-4B-GGUF:Q4_K_M'
    AND prompt_version = 'subjective-local-structured-v2'
    AND rubric_version = 'rubric-v3'
    AND delivery_mode = 'teacher_suggestion'
    AND capability_profile = 'local-pilot-v1'
    AND adapter_name = 'local_llama_cpp'
    AND adapter_attempts BETWEEN 1 AND 2
    AND adapter_latency_ms >= 0;
  IF grade_count <> 1 THEN
    RAISE EXCEPTION 'expected one governed AI suggestion, found %', grade_count;
  END IF;

  SELECT count(*) INTO audit_count
  FROM audit_log
  WHERE tenant_id = '00000000-0000-0000-0000-000000000001'
    AND action = 'subjective.ai_grade_created'
    AND target_type = 'ai_grade';
  IF audit_count <> 1 THEN
    RAISE EXCEPTION 'expected one AI-grade audit event, found %', audit_count;
  END IF;

  SELECT count(*) INTO final_count
  FROM final_grade
  WHERE tenant_id = '00000000-0000-0000-0000-000000000001'
    AND answer_segment_id = '00000000-0000-0000-0000-000000000614';
  IF final_count <> 0 THEN
    RAISE EXCEPTION 'grading agent must not create final grades';
  END IF;
END $$;

SELECT 'STORY-060 PostgreSQL governance verification passed' AS result;
