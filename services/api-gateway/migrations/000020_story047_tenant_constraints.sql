DO $$
BEGIN
  CREATE OR REPLACE FUNCTION pg_temp.ensure_tenant_identity(table_name TEXT, constraint_name TEXT)
  RETURNS VOID
  LANGUAGE plpgsql
  AS $fn$
  BEGIN
    IF NOT EXISTS (
      SELECT 1
      FROM pg_constraint
      WHERE conname = constraint_name
        AND conrelid = table_name::regclass
    ) THEN
      EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I UNIQUE (tenant_id, id)', table_name, constraint_name);
    END IF;
  END;
  $fn$;

  CREATE OR REPLACE FUNCTION pg_temp.ensure_tenant_fk(child_table TEXT, child_column TEXT, parent_table TEXT, constraint_name TEXT)
  RETURNS VOID
  LANGUAGE plpgsql
  AS $fn$
  DECLARE
    has_bad_reference BOOLEAN;
  BEGIN
    EXECUTE format(
      'SELECT EXISTS (
         SELECT 1
         FROM %I c
         WHERE c.%I IS NOT NULL
           AND NOT EXISTS (
             SELECT 1
             FROM %I p
             WHERE p.tenant_id = c.tenant_id
               AND p.id = c.%I
           )
       )',
      child_table,
      child_column,
      parent_table,
      child_column
    ) INTO has_bad_reference;

    IF has_bad_reference THEN
      RAISE EXCEPTION 'tenant scoped foreign key violation before adding %', constraint_name;
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM pg_constraint
      WHERE conname = constraint_name
        AND conrelid = child_table::regclass
    ) THEN
      EXECUTE format(
        'ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (tenant_id, %I) REFERENCES %I (tenant_id, id)',
        child_table,
        constraint_name,
        child_column,
        parent_table
      );
    END IF;
  END;
  $fn$;

  PERFORM pg_temp.ensure_tenant_identity('tenant', 'uq_tenant_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('app_user', 'uq_app_user_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('role', 'uq_role_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('permission', 'uq_permission_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('school', 'uq_school_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('grade', 'uq_grade_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('school_class', 'uq_school_class_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('student', 'uq_student_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('exam', 'uq_exam_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('file_asset', 'uq_file_asset_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('exam_paper', 'uq_exam_paper_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('question', 'uq_question_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('rubric_version', 'uq_rubric_version_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('answer_segment', 'uq_answer_segment_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('submission', 'uq_submission_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('submission_page', 'uq_submission_page_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('ocr_task', 'uq_ocr_task_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('review_task', 'uq_review_task_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('double_mark_session', 'uq_double_mark_session_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('arbitration_task', 'uq_arbitration_task_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('final_grade', 'uq_final_grade_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('submission_grade', 'uq_submission_grade_tenant_id_id');
  PERFORM pg_temp.ensure_tenant_identity('appeal', 'uq_appeal_tenant_id_id');

  PERFORM pg_temp.ensure_tenant_fk('app_user', 'school_id', 'school', 'fk_app_user_school_tenant');
  PERFORM pg_temp.ensure_tenant_fk('user_role', 'user_id', 'app_user', 'fk_user_role_app_user_tenant');
  PERFORM pg_temp.ensure_tenant_fk('user_role', 'role_id', 'role', 'fk_user_role_role_tenant');
  PERFORM pg_temp.ensure_tenant_fk('role_permission', 'role_id', 'role', 'fk_role_permission_role_tenant');
  PERFORM pg_temp.ensure_tenant_fk('role_permission', 'permission_id', 'permission', 'fk_role_permission_permission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('auth_session', 'user_id', 'app_user', 'fk_auth_session_app_user_tenant');
  PERFORM pg_temp.ensure_tenant_fk('audit_log', 'actor_id', 'app_user', 'fk_audit_log_actor_tenant');

  PERFORM pg_temp.ensure_tenant_fk('grade', 'school_id', 'school', 'fk_grade_school_tenant');
  PERFORM pg_temp.ensure_tenant_fk('school_class', 'school_id', 'school', 'fk_school_class_school_tenant');
  PERFORM pg_temp.ensure_tenant_fk('school_class', 'grade_id', 'grade', 'fk_school_class_grade_tenant');
  PERFORM pg_temp.ensure_tenant_fk('school_class', 'homeroom_teacher_id', 'app_user', 'fk_school_class_homeroom_teacher_tenant');
  PERFORM pg_temp.ensure_tenant_fk('student', 'school_id', 'school', 'fk_student_school_tenant');
  PERFORM pg_temp.ensure_tenant_fk('student', 'class_id', 'school_class', 'fk_student_school_class_tenant');
  PERFORM pg_temp.ensure_tenant_fk('teacher_class', 'teacher_id', 'app_user', 'fk_teacher_class_teacher_tenant');
  PERFORM pg_temp.ensure_tenant_fk('teacher_class', 'class_id', 'school_class', 'fk_teacher_class_school_class_tenant');

  PERFORM pg_temp.ensure_tenant_fk('exam', 'school_id', 'school', 'fk_exam_school_tenant');
  PERFORM pg_temp.ensure_tenant_fk('exam', 'created_by', 'app_user', 'fk_exam_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('exam_class', 'exam_id', 'exam', 'fk_exam_class_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('exam_class', 'class_id', 'school_class', 'fk_exam_class_school_class_tenant');

  PERFORM pg_temp.ensure_tenant_fk('file_asset', 'school_id', 'school', 'fk_file_asset_school_tenant');
  PERFORM pg_temp.ensure_tenant_fk('file_asset', 'exam_id', 'exam', 'fk_file_asset_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('file_asset', 'uploaded_by', 'app_user', 'fk_file_asset_uploaded_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('exam_paper', 'exam_id', 'exam', 'fk_exam_paper_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('exam_paper', 'file_asset_id', 'file_asset', 'fk_exam_paper_file_asset_tenant');
  PERFORM pg_temp.ensure_tenant_fk('exam_paper', 'uploaded_by', 'app_user', 'fk_exam_paper_uploaded_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question', 'exam_id', 'exam', 'fk_question_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question', 'exam_paper_id', 'exam_paper', 'fk_question_exam_paper_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question_answer_key', 'question_id', 'question', 'fk_question_answer_key_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question_answer_key', 'created_by', 'app_user', 'fk_question_answer_key_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('rubric_version', 'question_id', 'question', 'fk_rubric_version_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('rubric_version', 'created_by', 'app_user', 'fk_rubric_version_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('rubric_version', 'approved_by', 'app_user', 'fk_rubric_version_approved_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question_rubric', 'question_id', 'question', 'fk_question_rubric_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question_rubric', 'rubric_version_id', 'rubric_version', 'fk_question_rubric_rubric_version_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question_rubric', 'created_by', 'app_user', 'fk_question_rubric_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('question_rubric', 'approved_by', 'app_user', 'fk_question_rubric_approved_by_tenant');

  PERFORM pg_temp.ensure_tenant_fk('submission', 'exam_id', 'exam', 'fk_submission_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission', 'student_id', 'student', 'fk_submission_student_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission', 'collected_by', 'app_user', 'fk_submission_collected_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_page', 'submission_id', 'submission', 'fk_submission_page_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_page', 'file_asset_id', 'file_asset', 'fk_submission_page_file_asset_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ocr_task', 'submission_id', 'submission', 'fk_ocr_task_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ocr_task', 'requested_by', 'app_user', 'fk_ocr_task_requested_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ocr_result', 'ocr_task_id', 'ocr_task', 'fk_ocr_result_ocr_task_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ocr_result', 'submission_id', 'submission', 'fk_ocr_result_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ocr_result', 'submission_page_id', 'submission_page', 'fk_ocr_result_submission_page_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ocr_result', 'source_image_file_id', 'file_asset', 'fk_ocr_result_source_image_file_tenant');
  PERFORM pg_temp.ensure_tenant_fk('answer_segment', 'submission_id', 'submission', 'fk_answer_segment_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('answer_segment', 'submission_page_id', 'submission_page', 'fk_answer_segment_submission_page_tenant');
  PERFORM pg_temp.ensure_tenant_fk('answer_segment', 'question_id', 'question', 'fk_answer_segment_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('answer_segment', 'reviewed_by', 'app_user', 'fk_answer_segment_reviewed_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('answer_segment_answer', 'answer_segment_id', 'answer_segment', 'fk_answer_segment_answer_answer_segment_tenant');
  PERFORM pg_temp.ensure_tenant_fk('answer_segment_answer', 'recorded_by', 'app_user', 'fk_answer_segment_answer_recorded_by_tenant');

  PERFORM pg_temp.ensure_tenant_fk('ai_grade', 'answer_segment_id', 'answer_segment', 'fk_ai_grade_answer_segment_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ai_grade', 'question_id', 'question', 'fk_ai_grade_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ai_grade', 'rubric_version_id', 'rubric_version', 'fk_ai_grade_rubric_version_tenant');
  PERFORM pg_temp.ensure_tenant_fk('ai_grade', 'created_by', 'app_user', 'fk_ai_grade_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('review_task', 'exam_id', 'exam', 'fk_review_task_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('review_task', 'question_id', 'question', 'fk_review_task_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('review_task', 'answer_segment_id', 'answer_segment', 'fk_review_task_answer_segment_tenant');
  PERFORM pg_temp.ensure_tenant_fk('review_task', 'submission_id', 'submission', 'fk_review_task_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('review_task', 'assigned_to', 'app_user', 'fk_review_task_assigned_to_tenant');
  PERFORM pg_temp.ensure_tenant_fk('review_task', 'created_by', 'app_user', 'fk_review_task_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('human_grade', 'review_task_id', 'review_task', 'fk_human_grade_review_task_tenant');
  PERFORM pg_temp.ensure_tenant_fk('human_grade', 'answer_segment_id', 'answer_segment', 'fk_human_grade_answer_segment_tenant');
  PERFORM pg_temp.ensure_tenant_fk('human_grade', 'reviewer_id', 'app_user', 'fk_human_grade_reviewer_tenant');

  PERFORM pg_temp.ensure_tenant_fk('double_mark_policy', 'exam_id', 'exam', 'fk_double_mark_policy_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_policy', 'question_id', 'question', 'fk_double_mark_policy_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_policy', 'created_by', 'app_user', 'fk_double_mark_policy_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'exam_id', 'exam', 'fk_double_mark_session_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'question_id', 'question', 'fk_double_mark_session_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'answer_segment_id', 'answer_segment', 'fk_double_mark_session_answer_segment_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'submission_id', 'submission', 'fk_double_mark_session_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'first_review_task_id', 'review_task', 'fk_double_mark_session_first_review_task_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'second_review_task_id', 'review_task', 'fk_double_mark_session_second_review_task_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'first_reviewer_id', 'app_user', 'fk_double_mark_session_first_reviewer_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'second_reviewer_id', 'app_user', 'fk_double_mark_session_second_reviewer_tenant');
  PERFORM pg_temp.ensure_tenant_fk('double_mark_session', 'created_by', 'app_user', 'fk_double_mark_session_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'double_mark_session_id', 'double_mark_session', 'fk_arbitration_task_double_mark_session_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'exam_id', 'exam', 'fk_arbitration_task_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'question_id', 'question', 'fk_arbitration_task_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'answer_segment_id', 'answer_segment', 'fk_arbitration_task_answer_segment_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'submission_id', 'submission', 'fk_arbitration_task_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'first_reviewer_id', 'app_user', 'fk_arbitration_task_first_reviewer_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'second_reviewer_id', 'app_user', 'fk_arbitration_task_second_reviewer_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'assigned_to', 'app_user', 'fk_arbitration_task_assigned_to_tenant');
  PERFORM pg_temp.ensure_tenant_fk('arbitration_task', 'created_by', 'app_user', 'fk_arbitration_task_created_by_tenant');

  PERFORM pg_temp.ensure_tenant_fk('final_grade', 'exam_id', 'exam', 'fk_final_grade_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('final_grade', 'question_id', 'question', 'fk_final_grade_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('final_grade', 'answer_segment_id', 'answer_segment', 'fk_final_grade_answer_segment_tenant');
  PERFORM pg_temp.ensure_tenant_fk('final_grade', 'submission_id', 'submission', 'fk_final_grade_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('final_grade', 'double_mark_session_id', 'double_mark_session', 'fk_final_grade_double_mark_session_tenant');
  PERFORM pg_temp.ensure_tenant_fk('final_grade', 'arbitration_task_id', 'arbitration_task', 'fk_final_grade_arbitration_task_tenant');
  PERFORM pg_temp.ensure_tenant_fk('final_grade', 'created_by', 'app_user', 'fk_final_grade_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_grade', 'exam_id', 'exam', 'fk_submission_grade_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_grade', 'submission_id', 'submission', 'fk_submission_grade_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_grade', 'student_id', 'student', 'fk_submission_grade_student_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_grade', 'confirmed_by', 'app_user', 'fk_submission_grade_confirmed_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_grade', 'published_by', 'app_user', 'fk_submission_grade_published_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('submission_grade', 'created_by', 'app_user', 'fk_submission_grade_created_by_tenant');

  PERFORM pg_temp.ensure_tenant_fk('appeal', 'exam_id', 'exam', 'fk_appeal_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'submission_id', 'submission', 'fk_appeal_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'submission_grade_id', 'submission_grade', 'fk_appeal_submission_grade_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'student_id', 'student', 'fk_appeal_student_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'final_grade_id', 'final_grade', 'fk_appeal_final_grade_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'question_id', 'question', 'fk_appeal_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'assigned_to', 'app_user', 'fk_appeal_assigned_to_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'reviewed_by', 'app_user', 'fk_appeal_reviewed_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'closed_by', 'app_user', 'fk_appeal_closed_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('appeal', 'created_by', 'app_user', 'fk_appeal_created_by_tenant');
  PERFORM pg_temp.ensure_tenant_fk('score_adjustment', 'appeal_id', 'appeal', 'fk_score_adjustment_appeal_tenant');
  PERFORM pg_temp.ensure_tenant_fk('score_adjustment', 'exam_id', 'exam', 'fk_score_adjustment_exam_tenant');
  PERFORM pg_temp.ensure_tenant_fk('score_adjustment', 'submission_id', 'submission', 'fk_score_adjustment_submission_tenant');
  PERFORM pg_temp.ensure_tenant_fk('score_adjustment', 'submission_grade_id', 'submission_grade', 'fk_score_adjustment_submission_grade_tenant');
  PERFORM pg_temp.ensure_tenant_fk('score_adjustment', 'final_grade_id', 'final_grade', 'fk_score_adjustment_final_grade_tenant');
  PERFORM pg_temp.ensure_tenant_fk('score_adjustment', 'question_id', 'question', 'fk_score_adjustment_question_tenant');
  PERFORM pg_temp.ensure_tenant_fk('score_adjustment', 'adjusted_by', 'app_user', 'fk_score_adjustment_adjusted_by_tenant');
END $$;
