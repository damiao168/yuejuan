import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Progress, Space } from "antd";
import { motion } from "framer-motion";
import { ArrowRight, ClipboardCheck, FileUp, RefreshCw, ScanLine } from "lucide-react";
import { listArbitrationTasks, listReviewTasks, type ReviewTask } from "../api/review";
import { listExams, type Exam } from "../api/exams";
import { listSubmissions, type Submission } from "../api/submissions";
import { getSystemStatus, type SystemStatus } from "../api/system";
import { hasAnyPermission, hasEveryPermission, type SessionUser } from "../auth/session";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import { examStatusLabels, examSubjectLabel } from "../constants/examStatus";
import type { StatusTone } from "../types";

interface HomeData {
  exams: Exam[];
  submissions: Submission[];
  reviewTasks: ReviewTask[];
  arbitrationCount: number;
  systemStatus?: SystemStatus;
  warnings: string[];
}

const dependencyNames: Record<string, string> = {
  postgres: "数据库",
  redis: "缓存队列",
  minio: "文件存储",
  qdrant: "检索服务",
  ai_service: "智能评分服务",
  ocr_worker: "文字识别服务"
};

function dependencyTone(status: string): StatusTone {
  if (status === "ok") return "success";
  if (status === "not_configured") return "warning";
  return "danger";
}

function dependencyText(status: string) {
  if (status === "ok") return "正常";
  if (status === "not_configured") return "待接入";
  return "异常";
}

function progressFor(status: string) {
  return ({ draft: 10, configured: 25, ready: 35, collecting: 45, grading: 68, reviewing: 82, finalized: 94, published: 100, archived: 100 } as Record<string, number>)[status] ?? 0;
}

function roleLabel(user: SessionUser) {
  if (user.roles.includes("platform_admin")) return "系统运维";
  return "管理端";
}

async function fetchHome(user: SessionUser): Promise<HomeData> {
  const canReadExams = hasEveryPermission(user, ["exam:manage"]);
  const canReview = hasAnyPermission(user, ["review:manage", "review:work"]);
  const canArbitrate = hasAnyPermission(user, ["arbitration:manage", "arbitration:work"]);
  const isOperations = user.roles.includes("platform_admin") && hasEveryPermission(user, ["system:read"]);
  const warnings: string[] = [];
  const [examResult, reviewResult, arbitrationResult, systemResult] = await Promise.allSettled([
    canReadExams ? listExams() : Promise.resolve({ exams: [] }),
    canReview ? listReviewTasks() : Promise.resolve({ tasks: [] }),
    canArbitrate ? listArbitrationTasks() : Promise.resolve({ arbitration_tasks: [] }),
    isOperations ? getSystemStatus() : Promise.resolve(undefined)
  ]);

  const exams = examResult.status === "fulfilled" ? examResult.value.exams : [];
  if (examResult.status === "rejected") warnings.push("考试列表暂时不可用");
  if (reviewResult.status === "rejected") warnings.push("阅卷待办暂时不可用");
  if (arbitrationResult.status === "rejected") warnings.push("仲裁待办暂时不可用");
  if (systemResult.status === "rejected") warnings.push("系统健康信息暂时不可用");

  let submissions: Submission[] = [];
  if (hasEveryPermission(user, ["submission:manage"])) {
    const active = exams.filter((exam) => !["published", "archived"].includes(exam.status)).slice(0, 4);
    const results = await Promise.allSettled(active.map((exam) => listSubmissions(exam.id)));
    submissions = results.flatMap((result) => result.status === "fulfilled" ? result.value.submissions : []);
    if (results.some((result) => result.status === "rejected")) warnings.push("部分答卷状态未能加载");
  }

  return {
    exams,
    submissions,
    reviewTasks: reviewResult.status === "fulfilled" ? reviewResult.value.tasks : [],
    arbitrationCount: arbitrationResult.status === "fulfilled" ? arbitrationResult.value.arbitration_tasks.length : 0,
    systemStatus: systemResult.status === "fulfilled" ? systemResult.value : undefined,
    warnings
  };
}

export function DashboardPage({ user, onNavigate }: { user: SessionUser; onNavigate: (path: string) => void }) {
  const [data, setData] = useState<HomeData>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(undefined);
    fetchHome(user)
      .then((next) => { if (active) setData(next); })
      .catch((loadError) => { if (active) setError(loadError instanceof Error ? loadError.message : "首页加载失败"); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [nonce, user]);

  const activeExams = useMemo(() => (data?.exams ?? []).filter((exam) => !["published", "archived"].includes(exam.status)).slice(0, 5), [data?.exams]);
  const failedSubmissions = (data?.submissions ?? []).filter((submission) => submission.status.includes("failed") || submission.quality_status.includes("failed"));
  const unmatchedSubmissions = (data?.submissions ?? []).filter((submission) => !submission.student_id);
  const pendingTasks = (data?.reviewTasks ?? []).filter((task) => !["submitted", "finalized"].includes(task.status));
  const todo = [
    ...(pendingTasks.length ? [{ label: "待阅答卷", value: pendingTasks.length, path: "/grading", tone: "processing" as StatusTone }] : []),
    ...(data?.arbitrationCount ? [{ label: "待仲裁", value: data.arbitrationCount, path: "/arbitration", tone: "warning" as StatusTone }] : []),
    ...(unmatchedSubmissions.length ? [{ label: "待匹配学生", value: unmatchedSubmissions.length, path: "/capture", tone: "warning" as StatusTone }] : []),
    ...(failedSubmissions.length ? [{ label: "处理失败", value: failedSubmissions.length, path: "/capture", tone: "danger" as StatusTone }] : []),
    ...((data?.exams ?? []).filter((exam) => exam.status === "finalized").length ? [{ label: "待发布考试", value: (data?.exams ?? []).filter((exam) => exam.status === "finalized").length, path: "/scores", tone: "warning" as StatusTone }] : [])
  ];
  const primaryIssue = failedSubmissions.length > 0
    ? { title: `${failedSubmissions.length} 份答卷处理失败`, impact: "会阻断识别、切题和后续阅卷", action: "处理问题答卷", path: "/capture", tone: "danger" as StatusTone }
    : unmatchedSubmissions.length > 0
      ? { title: `${unmatchedSubmissions.length} 名学生尚未匹配`, impact: "未确认归属的答卷不能进入正式阅卷", action: "立即匹配", path: "/capture", tone: "warning" as StatusTone }
      : pendingTasks.length > 0
        ? { title: `${pendingTasks.length} 份答卷等待阅卷`, impact: "完成阅卷后才能汇总并发布成绩", action: "继续阅卷", path: "/grading", tone: "processing" as StatusTone }
        : (data?.exams ?? []).some((exam) => exam.status === "finalized")
          ? { title: "成绩已具备发布条件", impact: "完成发布检查后即可向师生开放成绩", action: "检查并发布", path: "/scores", tone: "success" as StatusTone }
          : null;

  const quickActions = [
    ...(hasEveryPermission(user, ["exam:manage"]) ? [{ label: "创建考试", path: "/exams", icon: <ClipboardCheck size={17} /> }] : []),
    ...(hasEveryPermission(user, ["submission:manage"]) ? [{ label: "导入答卷", path: "/capture", icon: <FileUp size={17} /> }] : []),
    ...(hasAnyPermission(user, ["review:manage", "review:work"]) ? [{ label: "阅卷运营", path: "/grading", icon: <ScanLine size={17} /> }] : []),
    ...(hasEveryPermission(user, ["org:manage"]) ? [{ label: "机构启用", path: "/organization/setup", icon: <ArrowRight size={17} /> }] : [])
  ].slice(0, 6);

  if (!data && loading) return <LoadingState label="正在加载工作台" />;
  if (!data && error) return <ErrorState message={error} onRetry={() => setNonce((value) => value + 1)} />;

  return (
    <div className="page-stack role-dashboard">
      <motion.section className="dashboard-heading" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
        <div>
          <span className="dashboard-kicker">{roleLabel(user)}</span>
          <h1>考试运营工作台</h1>
          <p>{user.school} · 优先处理会阻断考试流程的问题</p>
        </div>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => setNonce((value) => value + 1)}>刷新</Button>
      </motion.section>

      {error ? <Alert type="error" showIcon message="刷新失败" description={error} /> : null}
      {data?.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} />)}

      {primaryIssue ? <motion.section className={`priority-action ${primaryIssue.tone}`} initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.18 }}>
        <div>
          <span>当前最需要处理</span>
          <h2>{primaryIssue.title}</h2>
          <p>{primaryIssue.impact}</p>
        </div>
        <Button type="primary" size="large" onClick={() => onNavigate(primaryIssue.path)}>{primaryIssue.action}<ArrowRight size={17} /></Button>
      </motion.section> : <section className="priority-action success"><div><span>当前状态</span><h2>没有阻断考试流程的问题</h2><p>可以继续查看进行中的考试或创建新考试。</p></div></section>}

      <div className="dashboard-primary-grid">
        <section className="dashboard-pane todo-pane">
          <div className="section-head"><div><h2>我的待办</h2><p>按对考试流程的影响排序</p></div><strong>{todo.reduce((sum, item) => sum + item.value, 0)}</strong></div>
          {todo.length ? <div className="todo-list">{todo.map((item) => (
            <button type="button" key={item.label} className="todo-row" onClick={() => onNavigate(item.path)}>
              <span><StatusTag tone={item.tone}>{item.label}</StatusTag></span><strong>{item.value}</strong><ArrowRight size={16} />
            </button>
          ))}</div> : <EmptyState title="当前没有待办" description="新的采集、阅卷或发布事项出现后会显示在这里。" />}
        </section>

        <section className="dashboard-pane exam-pane">
          <div className="section-head"><div><h2>正在进行的考试</h2><p>{activeExams.length} 场需要关注</p></div></div>
          {activeExams.length ? <div className="active-exam-list">{activeExams.map((exam) => (
      <button type="button" key={exam.id} className="active-exam-row exam-task-row" onClick={() => onNavigate(`/exams/${encodeURIComponent(exam.id)}/overview`)}>
              <div><strong>{exam.name}</strong><span title={exam.status}>{examSubjectLabel(exam.subject)} · 当前阶段：{examStatusLabels[exam.status] ?? "进行中"}</span></div>
              <div className="exam-progress"><Progress percent={progressFor(exam.status)} size="small" showInfo={false} /><span>{progressFor(exam.status)}%</span></div>
              <span className="exam-next-action">进入考试 <ArrowRight size={15} /></span>
            </button>
          ))}</div> : <EmptyState title="暂无进行中考试" description="创建考试后，可从这里直接进入考试工作区。" />}
        </section>
      </div>

      {quickActions.length ? <section className="quick-actions"><div className="section-head"><div><h2>常用操作</h2></div></div><Space wrap>{quickActions.map((action) => <Button key={action.label} icon={action.icon} onClick={() => onNavigate(action.path)}>{action.label}</Button>)}</Space></section> : null}

      {data?.systemStatus && user.roles.includes("platform_admin") ? <section className="operations-strip"><strong>系统运维</strong>{data.systemStatus.dependencies.map((dependency) => <span key={dependency.name} title={dependency.name}>{dependencyNames[dependency.name] ?? dependency.name}<StatusTag tone={dependencyTone(dependency.status)}>{dependencyText(dependency.status)}</StatusTag></span>)}</section> : null}
    </div>
  );
}
