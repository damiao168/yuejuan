import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Progress, Space } from "antd";
import { motion } from "framer-motion";
import { ArrowRight, ClipboardCheck, FileUp, RefreshCw, ScanLine, ShieldAlert } from "lucide-react";
import { listArbitrationTasks, listReviewTasks, type ReviewTask } from "../api/review";
import { listExams, type Exam } from "../api/exams";
import { listSubmissions, type Submission } from "../api/submissions";
import { getSystemStatus, type SystemStatus } from "../api/system";
import { hasAnyPermission, hasEveryPermission, type SessionUser } from "../auth/session";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import type { StatusTone } from "../types";

interface HomeData {
  exams: Exam[];
  submissions: Submission[];
  reviewTasks: ReviewTask[];
  arbitrationCount: number;
  systemStatus?: SystemStatus;
  warnings: string[];
}

const statusLabels: Record<string, string> = {
  draft: "草稿",
  configured: "配置中",
  ready: "准备完成",
  collecting: "采集中",
  grading: "阅卷中",
  reviewing: "质量检查",
  finalized: "待发布",
  published: "已发布",
  archived: "已归档",
  pending: "等待处理",
  assigned: "待阅卷",
  in_progress: "处理中",
  failed: "处理失败"
};

function tone(status: string): StatusTone {
  if (["published", "completed", "ok"].includes(status)) return "success";
  if (status.includes("failed") || status.includes("error")) return "danger";
  if (["finalized", "reviewing", "pending"].includes(status)) return "warning";
  if (["collecting", "grading", "assigned", "in_progress"].includes(status)) return "processing";
  return "neutral";
}

function progressFor(status: string) {
  return ({ draft: 10, configured: 25, ready: 35, collecting: 45, grading: 68, reviewing: 82, finalized: 94, published: 100, archived: 100 } as Record<string, number>)[status] ?? 0;
}

function roleLabel(user: SessionUser) {
  if (user.roles.includes("grader")) return "阅卷工作台";
  if (user.roles.includes("arbitrator")) return "仲裁工作台";
  if (user.roles.includes("platform_admin")) return "系统运维工作台";
  if (hasEveryPermission(user, ["submission:manage"]) && !hasEveryPermission(user, ["exam:manage"])) return "采集工作台";
  return "考试工作台";
}

async function fetchHome(user: SessionUser): Promise<HomeData> {
  const canReadExams = hasEveryPermission(user, ["exam:manage"]);
  const canReview = hasAnyPermission(user, ["review:manage", "review:work"]);
  const canArbitrate = hasAnyPermission(user, ["arbitration:manage", "arbitration:work"]);
  const isOperations = user.roles.includes("platform_admin") && hasEveryPermission(user, ["system:read"]);
  const warnings: string[] = [];
  const [examResult, reviewResult, arbitrationResult, systemResult] = await Promise.allSettled([
    canReadExams ? listExams() : Promise.resolve({ exams: [] }),
    canReview ? listReviewTasks(user.roles.includes("grader") ? { assigned_to: user.id } : {}) : Promise.resolve({ tasks: [] }),
    canArbitrate ? listArbitrationTasks(user.roles.includes("arbitrator") ? { assigned_to: user.id } : {}) : Promise.resolve({ arbitration_tasks: [] }),
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
  const isGrader = user.roles.includes("grader");
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

  const quickActions = [
    ...(hasEveryPermission(user, ["exam:manage"]) ? [{ label: "创建考试", path: "/exams", icon: <ClipboardCheck size={17} /> }] : []),
    ...(hasEveryPermission(user, ["submission:manage"]) ? [{ label: "导入答卷", path: "/capture", icon: <FileUp size={17} /> }] : []),
    ...(hasAnyPermission(user, ["review:manage", "review:work"]) ? [{ label: "继续阅卷", path: "/grading", icon: <ScanLine size={17} /> }] : []),
    ...(hasEveryPermission(user, ["org:manage"]) ? [{ label: "组织启用", path: "/organization/setup", icon: <ArrowRight size={17} /> }] : [])
  ].slice(0, 6);

  if (!data && loading) return <LoadingState label="正在加载工作台" />;
  if (!data && error) return <ErrorState message={error} onRetry={() => setNonce((value) => value + 1)} />;

  return (
    <div className="page-stack role-dashboard">
      <motion.section className="dashboard-heading" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
        <div>
          <span className="dashboard-kicker">{roleLabel(user)}</span>
          <h1>{user.name}，今天需要处理这些事项</h1>
          <p>{user.school} · 数据来自当前账号可访问的真实业务范围</p>
        </div>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => setNonce((value) => value + 1)}>刷新</Button>
      </motion.section>

      {error ? <Alert type="error" showIcon message="刷新失败" description={error} /> : null}
      {data?.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} />)}

      <div className="dashboard-primary-grid">
        <section className="dashboard-pane todo-pane">
          <div className="section-head"><div><h2>我的待办</h2><p>按风险和处理阶段汇总</p></div><strong>{todo.reduce((sum, item) => sum + item.value, 0)}</strong></div>
          {todo.length ? <div className="todo-list">{todo.map((item) => (
            <button key={item.label} className="todo-row" onClick={() => onNavigate(item.path)}>
              <span><StatusTag tone={item.tone}>{item.label}</StatusTag></span><strong>{item.value}</strong><ArrowRight size={16} />
            </button>
          ))}</div> : <EmptyState title="当前没有待办" description="新的采集、阅卷或发布事项出现后会显示在这里。" />}
        </section>

        {isGrader ? (
          <section className="dashboard-pane exam-pane">
            <div className="section-head"><div><h2>最近工作</h2><p>仅显示分配给我的阅卷任务</p></div><strong>{data?.reviewTasks.filter((task) => ["submitted", "finalized"].includes(task.status)).length ?? 0}</strong></div>
            <div className="review-role-summary"><span>当前题目<strong>{pendingTasks[0]?.question_no || "等待分配"}</strong></span><span>待阅<strong>{pendingTasks.length}</strong></span></div>
            {data?.reviewTasks.length ? <div className="active-exam-list">{data.reviewTasks.slice(0, 5).map((task) => (
              <button key={task.id} className="active-exam-row" onClick={() => onNavigate("/grading")}>
                <div><strong>第 {task.question_no} 题</strong><span>{statusLabels[task.status] ?? task.status} · {task.anonymous_code}</span></div>
                <StatusTag tone={tone(task.status)}>{statusLabels[task.status] ?? task.status}</StatusTag>
                <ArrowRight size={16} />
              </button>
            ))}</div> : <EmptyState title="尚未分配阅卷任务" description="任务分配后可从这里继续处理。" />}
          </section>
        ) : (
          <section className="dashboard-pane exam-pane">
            <div className="section-head"><div><h2>正在进行的考试</h2><p>{activeExams.length} 场需要关注</p></div></div>
            {activeExams.length ? <div className="active-exam-list">{activeExams.map((exam) => (
              <button key={exam.id} className="active-exam-row" onClick={() => onNavigate(`/exams/${encodeURIComponent(exam.id)}/overview`)}>
                <div><strong>{exam.name}</strong><span>{exam.subject} · {statusLabels[exam.status] ?? exam.status}</span></div>
                <Progress percent={progressFor(exam.status)} size="small" showInfo={false} />
                <ArrowRight size={16} />
              </button>
            ))}</div> : <EmptyState title="暂无进行中考试" description="创建考试后，可从这里直接进入考试工作区。" />}
          </section>
        )}
      </div>

      {quickActions.length ? <section className="quick-actions"><div className="section-head"><div><h2>常用操作</h2></div></div><Space wrap>{quickActions.map((action) => <Button key={action.label} icon={action.icon} onClick={() => onNavigate(action.path)}>{action.label}</Button>)}</Space></section> : null}

      <section className="attention-band">
        {isGrader ? (
          <><div><ShieldAlert size={19} /><span><strong>质量提醒</strong> 退回、复核和评分冲突会显示在这里。</span></div><span>{data?.reviewTasks.filter((task) => task.status === "returned" || task.status === "conflict").length ?? 0} 项</span></>
        ) : (
          <><div><ShieldAlert size={19} /><span><strong>需要关注</strong> 失败任务、未匹配学生和待发布考试会阻断后续流程。</span></div><span>{failedSubmissions.length + unmatchedSubmissions.length} 项</span></>
        )}
      </section>

      {data?.systemStatus && user.roles.includes("platform_admin") ? <section className="operations-strip"><strong>系统运维</strong>{data.systemStatus.dependencies.map((dependency) => <span key={dependency.name}>{dependency.name}<StatusTag tone={tone(dependency.status)}>{dependency.status === "ok" ? "正常" : "异常"}</StatusTag></span>)}</section> : null}
    </div>
  );
}
