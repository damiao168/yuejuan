import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App, Button, Checkbox, Progress, Space } from "antd";
import { ArrowRight, CheckCircle2, CircleAlert, ClipboardCheck, Play, RefreshCw, Save } from "lucide-react";
import { ApiClientError } from "../api/client";
import { confirmExamReadiness, getExamReadiness, startExamCollection, type ExamReadiness } from "../api/configuration";
import { getExam, updateExam, type Exam } from "../api/exams";
import { listClasses, listGrades, type Grade, type SchoolClass } from "../api/org";
import { ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";

function formatError(error: unknown) {
  if (error instanceof ApiClientError) return error.message;
  return error instanceof Error ? error.message : "操作失败，请重试";
}

export function ExamStudentScopePage({ examId, canManage, onExamChanged }: { examId: string; canManage: boolean; onExamChanged?: () => void }) {
  const { message } = App.useApp();
  const [exam, setExam] = useState<Exam>();
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [grades, setGrades] = useState<Grade[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const [examResponse, classResponse, gradeResponse] = await Promise.all([getExam(examId), listClasses(), listGrades()]);
      setExam(examResponse.exam);
      setClasses(classResponse.classes.filter((item) => item.school_id === examResponse.exam.school_id));
      setGrades(gradeResponse.grades.filter((item) => item.school_id === examResponse.exam.school_id));
      setSelected(examResponse.exam.class_ids);
    } catch (loadError) { setError(formatError(loadError)); } finally { setLoading(false); }
  }, [examId]);

  useEffect(() => { void load(); }, [load]);

  const gradeById = useMemo(() => new Map(grades.map((item) => [item.id, item])), [grades]);
  const changed = exam ? [...selected].sort().join(",") !== [...exam.class_ids].sort().join(",") : false;

  async function save() {
    if (!exam || !selected.length) { message.error("至少选择一个班级"); return; }
    setSaving(true);
    try {
      const response = await updateExam(exam.id, { class_ids: selected });
      setExam(response.exam);
      onExamChanged?.();
      message.success("学生范围已保存，原准备确认已自动失效");
    } catch (saveError) { message.error(formatError(saveError)); } finally { setSaving(false); }
  }

  if (loading) return <LoadingState label="正在加载学生范围" />;
  if (error) return <ErrorState message={error} onRetry={() => void load()} />;

  return (
    <div className="preparation-page">
      <section className="preparation-heading"><div><h2>学生范围</h2><p>选择参加本场考试的班级，学生名单以机构中的在读学生为准。</p></div><Space><Button icon={<RefreshCw size={16} />} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<Save size={16} />} disabled={!canManage || !changed || !selected.length} loading={saving} onClick={() => void save()}>保存范围</Button></Space></section>
      {!classes.length ? <Alert type="warning" showIcon message="当前学校还没有班级" description="请先在组织与用户中创建年级、班级并导入学生。" /> : (
        <div className="class-scope-list">
          {grades.map((grade) => {
            const gradeClasses = classes.filter((item) => item.grade_id === grade.id);
            if (!gradeClasses.length) return null;
            return <section key={grade.id}><div><strong>{grade.name}</strong><span>{grade.academic_year}</span></div><Checkbox.Group value={selected} onChange={(values) => setSelected(values as string[])} disabled={!canManage}>{gradeClasses.map((item) => <Checkbox key={item.id} value={item.id}><span>{item.name}</span><small>{item.code}</small></Checkbox>)}</Checkbox.Group></section>;
          })}
          {classes.filter((item) => !gradeById.has(item.grade_id)).length ? <section><div><strong>未分组班级</strong></div><Checkbox.Group value={selected} onChange={(values) => setSelected(values as string[])}>{classes.filter((item) => !gradeById.has(item.grade_id)).map((item) => <Checkbox key={item.id} value={item.id}>{item.name}</Checkbox>)}</Checkbox.Group></section> : null}
        </div>
      )}
    </div>
  );
}

export function ExamReadinessPage({ examId, canManage, onNavigate, onExamChanged }: { examId: string; canManage: boolean; onNavigate: (path: string) => void; onExamChanged?: () => void }) {
  const { message, modal } = App.useApp();
  const [exam, setExam] = useState<Exam>();
  const [readiness, setReadiness] = useState<ExamReadiness>();
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const [examResponse, readinessResponse] = await Promise.all([getExam(examId), getExamReadiness(examId)]);
      setExam(examResponse.exam);
      setReadiness(readinessResponse.readiness);
    } catch (loadError) { setError(formatError(loadError)); } finally { setLoading(false); }
  }, [examId]);

  useEffect(() => { void load(); }, [load]);

  const passed = readiness?.checks.filter((item) => item.passed).length ?? 0;
  const total = readiness?.checks.length ?? 0;
  const percent = total ? Math.round((passed / total) * 100) : 0;

  async function confirm() {
    setWorking(true);
    try {
      const response = await confirmExamReadiness(examId);
      setReadiness(response.readiness);
      await load();
      onExamChanged?.();
      message.success("开考准备已确认");
    } catch (confirmError) { message.error(formatError(confirmError)); } finally { setWorking(false); }
  }

  function confirmStart() {
    modal.confirm({ title: "开始采集答卷", content: "开始后考试配置将进入采集阶段。确认试卷、答案和模板已完成最终核对。", okText: "开始采集", cancelText: "取消", onOk: async () => {
      setWorking(true);
      try {
        await startExamCollection(examId);
        message.success("考试已进入采集阶段");
        await load();
        onExamChanged?.();
      } catch (startError) { message.error(formatError(startError)); } finally { setWorking(false); }
    }});
  }

  if (loading && !readiness) return <LoadingState label="正在执行开考准备检查" />;
  if (error && !readiness) return <ErrorState message={error} onRetry={() => void load()} />;
  if (!readiness || !exam) return null;

  return (
    <div className="preparation-page readiness-page">
      <section className="preparation-heading"><div><h2>开考准备</h2><p>检查由服务端执行，未通过的项目会阻止考试进入采集。</p></div><Space><Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load()}>重新检查</Button>{readiness.ready && !readiness.confirmed && exam.status !== "collecting" ? <Button type="primary" icon={<ClipboardCheck size={16} />} disabled={!canManage} loading={working} onClick={() => void confirm()}>确认准备完成</Button> : null}{readiness.confirmed && exam.status === "ready" ? <Button type="primary" icon={<Play size={16} />} disabled={!canManage} loading={working} onClick={confirmStart}>开始采集</Button> : null}</Space></section>

      <section className="readiness-summary"><div><span>准备进度</span><strong>{passed}/{total}</strong></div><Progress percent={percent} status={readiness.ready ? "success" : "active"} /><div className="readiness-summary-state"><StatusTag tone={exam.status === "collecting" ? "processing" : readiness.confirmed ? "success" : readiness.ready ? "processing" : "warning"}>{exam.status === "collecting" ? "采集中" : readiness.confirmed ? "准备完成" : readiness.ready ? "等待确认" : "存在阻断项"}</StatusTag>{readiness.confirmed_at ? <span>确认时间：{new Date(readiness.confirmed_at).toLocaleString("zh-CN")}</span> : null}</div></section>

      <div className="readiness-check-list">
        {readiness.checks.map((check) => <button key={check.code} className={check.passed ? "passed" : "failed"} onClick={() => !check.passed && onNavigate(`/exams/${encodeURIComponent(examId)}/${check.section}`)}><span className="readiness-icon">{check.passed ? <CheckCircle2 size={20} /> : <CircleAlert size={20} />}</span><span><strong>{check.label}</strong><small>{check.message}</small></span>{!check.passed ? <><span>去处理</span><ArrowRight size={17} /></> : null}</button>)}
      </div>

      {readiness.ready && !readiness.confirmed ? <Alert type="info" showIcon message="所有检查已通过" description="请由考试负责人确认准备完成。确认后若修改题目、答案、模板或学生范围，系统会自动撤销本次确认。" /> : null}
      {exam.status === "collecting" ? <Alert type="success" showIcon message="考试已进入采集阶段" description="后续答卷导入与页面处理将在采集工作区完成。" /> : null}
    </div>
  );
}
