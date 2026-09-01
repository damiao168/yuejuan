-- Product-facing key progress is intentionally separated from the immutable
-- compliance audit stream. The projection carries trusted school/exam keys so
-- school administrators can read it without crossing organizational bounds.

CREATE TABLE IF NOT EXISTS activity_event (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenant(id),
    school_id uuid NOT NULL,
    exam_id uuid NOT NULL,
    actor_id uuid,
    event_type text NOT NULL,
    severity text NOT NULL DEFAULT 'info' CHECK (severity IN ('info','success','warning')),
    title text NOT NULL,
    summary text NOT NULL DEFAULT '',
    action_path text NOT NULL DEFAULT '',
    target_type text NOT NULL DEFAULT '',
    target_id uuid,
    source_audit_id uuid NOT NULL UNIQUE REFERENCES audit_log(id),
    dedupe_key text NOT NULL DEFAULT '',
    happened_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_activity_event_school_tenant
      FOREIGN KEY (tenant_id, school_id) REFERENCES school(tenant_id, id),
    CONSTRAINT fk_activity_event_exam_tenant
      FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_activity_event_school_time
ON activity_event (tenant_id, school_id, happened_at DESC);

CREATE INDEX IF NOT EXISTS idx_activity_event_exam_time
ON activity_event (tenant_id, exam_id, happened_at DESC);

CREATE OR REPLACE FUNCTION project_key_activity(p_audit_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    a audit_log%ROWTYPE;
    v_exam_id uuid;
    v_school_id uuid;
    v_title text;
    v_summary text;
    v_severity text := 'info';
    v_path text;
BEGIN
    SELECT * INTO a FROM audit_log WHERE id = p_audit_id;
    IF NOT FOUND OR a.action NOT IN (
        'exam.created',
        'exam.readiness_confirmed',
        'exam.collection_started',
        'submission.pages_processing_started',
        'capture.student_match_confirmed',
        'capture.student_marked_unknown',
        'capture.submissions_merged',
        'capture.submission_split',
        'review.tasks_batch_assigned',
        'review.task_returned',
        'score.confirmed',
        'score.finalized',
        'score.published'
    ) THEN
        RETURN;
    END IF;

    CASE a.target_type
      WHEN 'exam' THEN v_exam_id := a.target_id;
      WHEN 'submission' THEN
        SELECT s.exam_id INTO v_exam_id
        FROM submission s WHERE s.tenant_id = a.tenant_id AND s.id = a.target_id;
      WHEN 'review_task' THEN
        SELECT r.exam_id INTO v_exam_id
        FROM review_task r WHERE r.tenant_id = a.tenant_id AND r.id = a.target_id;
      WHEN 'arbitration_task' THEN
        SELECT r.exam_id INTO v_exam_id
        FROM arbitration_task r WHERE r.tenant_id = a.tenant_id AND r.id = a.target_id;
      WHEN 'capture_batch' THEN
        SELECT b.exam_id INTO v_exam_id
        FROM capture_batch b WHERE b.tenant_id = a.tenant_id AND b.id = a.target_id;
      WHEN 'capture_page' THEN
        SELECT b.exam_id INTO v_exam_id
        FROM capture_page p
        JOIN capture_batch b ON b.tenant_id = p.tenant_id AND b.id = p.capture_batch_id
        WHERE p.tenant_id = a.tenant_id AND p.id = a.target_id;
      ELSE
        v_exam_id := NULL;
    END CASE;

    IF v_exam_id IS NULL THEN
        RETURN;
    END IF;
    SELECT e.school_id INTO v_school_id
    FROM exam e WHERE e.tenant_id = a.tenant_id AND e.id = v_exam_id AND e.deleted_at IS NULL;
    IF v_school_id IS NULL THEN
        RETURN;
    END IF;

    v_title := CASE a.action
      WHEN 'exam.created' THEN '已创建考试'
      WHEN 'exam.readiness_confirmed' THEN '考试已准备就绪'
      WHEN 'exam.collection_started' THEN '已开始答卷采集'
      WHEN 'submission.pages_processing_started' THEN '答卷处理已开始'
      WHEN 'capture.student_match_confirmed' THEN '答卷学生身份已确认'
      WHEN 'capture.student_marked_unknown' THEN '答卷学生身份待处理'
      WHEN 'capture.submissions_merged' THEN '答卷已合并'
      WHEN 'capture.submission_split' THEN '答卷已拆分'
      WHEN 'review.tasks_batch_assigned' THEN '阅卷任务已分配'
      WHEN 'review.task_returned' THEN '阅卷任务已退回'
      WHEN 'score.confirmed' THEN '成绩已确认'
      WHEN 'score.finalized' THEN '成绩已定稿'
      WHEN 'score.published' THEN '成绩已发布'
    END;
    v_summary := CASE a.action
      WHEN 'exam.created' THEN '考试已建立，可继续完善试卷和学生范围。'
      WHEN 'exam.readiness_confirmed' THEN '考试配置检查已通过，可进入答卷采集。'
      WHEN 'exam.collection_started' THEN '考试已进入答卷采集阶段。'
      WHEN 'submission.pages_processing_started' THEN '新答卷已进入识别和切题流程。'
      WHEN 'capture.student_match_confirmed' THEN '待确认答卷已关联到学生。'
      WHEN 'capture.student_marked_unknown' THEN '存在无法确认学生身份的答卷。'
      WHEN 'capture.submissions_merged' THEN '重复或拆散的答卷已完成合并。'
      WHEN 'capture.submission_split' THEN '答卷已按实际归属拆分。'
      WHEN 'review.tasks_batch_assigned' THEN '一批阅卷任务已分配给阅卷人员。'
      WHEN 'review.task_returned' THEN '有阅卷任务被退回，需要重新处理。'
      WHEN 'score.confirmed' THEN '考试成绩已经人工确认。'
      WHEN 'score.finalized' THEN '阅卷结果已锁定，等待发布。'
      WHEN 'score.published' THEN '学生现在可以查看已发布成绩。'
    END;
    IF a.action IN ('capture.student_marked_unknown','review.task_returned') THEN
        v_severity := 'warning';
    ELSIF a.action IN ('exam.readiness_confirmed','score.confirmed','score.finalized','score.published') THEN
        v_severity := 'success';
    END IF;
    v_path := CASE
      WHEN a.action LIKE 'capture.%' OR a.action LIKE 'submission.%'
        THEN '/exams/' || v_exam_id::text || '/capture'
      WHEN a.action LIKE 'review.%'
        THEN '/exams/' || v_exam_id::text || '/grading'
      WHEN a.action LIKE 'score.%'
        THEN '/exams/' || v_exam_id::text || '/scores'
      ELSE '/exams/' || v_exam_id::text || '/overview'
    END;

    INSERT INTO activity_event (
        tenant_id, school_id, exam_id, actor_id, event_type, severity,
        title, summary, action_path, target_type, target_id,
        source_audit_id, dedupe_key, happened_at
    ) VALUES (
        a.tenant_id, v_school_id, v_exam_id, a.actor_id, a.action, v_severity,
        v_title, v_summary, v_path, a.target_type, a.target_id,
        a.id, a.action || ':' || v_exam_id::text, a.created_at
    ) ON CONFLICT (source_audit_id) DO NOTHING;
END;
$$;

CREATE OR REPLACE FUNCTION project_key_activity_trigger()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM project_key_activity(NEW.id);
    RETURN NEW;
EXCEPTION WHEN OTHERS THEN
    -- Activity projection is secondary. It must never roll back the immutable
    -- audit record or the business transaction that produced it.
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_audit_log_project_key_activity ON audit_log;
CREATE TRIGGER trg_audit_log_project_key_activity
AFTER INSERT ON audit_log
FOR EACH ROW EXECUTE FUNCTION project_key_activity_trigger();

-- Only the curated whitelist is projected. Login, worker, file, draft-save and
-- other high-volume audit events remain available solely in the audit stream.
SELECT project_key_activity(id)
FROM audit_log
WHERE action IN (
    'exam.created','exam.readiness_confirmed','exam.collection_started',
    'submission.pages_processing_started','capture.student_match_confirmed',
    'capture.student_marked_unknown','capture.submissions_merged','capture.submission_split',
    'review.tasks_batch_assigned','review.task_returned',
    'score.confirmed','score.finalized','score.published'
);
