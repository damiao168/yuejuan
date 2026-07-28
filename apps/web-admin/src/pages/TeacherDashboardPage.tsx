import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Progress, Space } from "antd";
import { ArrowRight, BookOpenCheck, ClipboardList, FileText, Gavel, RefreshCw } from "lucide-react";
import { listExams, type Exam } from "../api/exams";
import { listArbitrationTasks, listReviewTasks, type ArbitrationTask, type ReviewTask } from "../api/review";
import { hasAnyPermission, hasEveryPermission, type SessionUser } from "../auth/session";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import { examStatusLabels, examStatusTone, examSubjectLabel } from "../constants/examStatus";
import type { StatusTone } from "../types";

interface TeacherHomeData {
  exams: Exam[];
  reviewTasks: ReviewTask[];
  arbitrationTasks: ArbitrationTask[];
  warnings: string[];
}

interface PersonalTask {
  id: string;
  kind: "review" | "arbitration";
  title: string;
  detail: string;
  status: string;
  path: string;
}

const activeReviewStatuses = ["assigned", "in_progress", "returned"];
const activeArbitrationStatuses = ["assigned", "in_progress", "pending"];

const taskStatusLabels: Record<string, string> = {
  assigned: "待阅卷",
  in_progress: "处理中",
  returned: "已退回",
  pending: "待领取",
  submitted: "已提交",
  completed: "已完成"
};

const sourceLabels: Record<string, string> = {
  ai_low_confidence: "智能评分待人工确认",
  double_mark_required: "双评任务",
  evidence_verification_failed: "证据核验未通过",
  manual_sample: "人工抽检",
  returned: "退回重评"
};

function taskTone(status: string): StatusTone {
  if (["submitted", "completed"].includes(status)) return "success";
  if (["returned", "pending"].includes(status)) return "warning";
  if (["assigned", "in_progress"].includes(status)) return "processing";
  return "neutral";
}

async function fetchTeacherHome(user: SessionUser): Promise<TeacherHomeData> {
  const canReadExams = hasEveryPermission(user, ["exam:manage"]);
  const canReview = hasAnyPermission(user, ["review:manage", "review:work"]);
  const canArbitrate = hasAnyPermission(user, ["arbitration:manage", "arbitration:work"]);
  const results = await Promise.allSettled([
    canReadExams ? listExams() : Promise.resolve({ exams: [] }),
    canReview ? listReviewTasks({ assigned_to: user.id }) : Promise.resolve({ tasks: [] }),
    canArbitrate ? listArbitrationTasks({ assigned_to: user.id }) : Promise.resolve({ arbitration_tasks: [] })
  ]);
  const warnings: string[] = [];
  if (results[0].status === "rejected") warnings.push("我的考试暂时不可用");
  if (results[1].status === "rejected") warnings.push("我的阅卷任务暂时不可用");
  if (results[2].status === "rejected") warnings.push("我的仲裁任务暂时不可用");
  return {
    exams: results[0].status === "fulfilled" ? results[0].value.exams : [],
    reviewTasks: results[1].status === "fulfilled" ? results[1].value.tasks : [],
    arbitrationTasks: results[2].status === "fulfilled" ? results[2].value.arbitration_tasks : [],
    warnings
  };
}

export function TeacherDashboardPage({ user, onNavigate }: { user: SessionUser; onNavigate: (path: string) => void }) {
  const [data, setData] = useState<TeacherHomeData>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      setData(await fetchTeacherHome(user));
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "教师工作台加载失败");
    } finally {
      setLoading(false);
    }
  }, [user]);

  useEffect(() => {
    void load();
  }, [load]);

  const activeReviews = useMemo(() => (data?.reviewTasks ?? []).filter((task) => activeReviewStatuses.includes(task.status)), [data?.reviewTasks]);
  const activeArbitrations = useMemo(() => (data?.arbitrationTasks ?? []).filter((task) => activeArbitrationStatuses.includes(task.status)), [data?.arbitrationTasks]);
  const activeExams = useMemo(() => (data?.exams ?? []).filter((exam) => !["archived", "published"].includes(exam.status)), [data?.exams]);
  const completedReviews = (data?.reviewTasks ?? []).filter((task) => ["submitted", "completed"].includes(task.status)).length;
  const reviewTotal = data?.reviewTasks.length ?? 0;
  const reviewProgress = reviewTotal > 0 ? Math.round((completedReviews / reviewTotal) * 100) : 0;

  const tasks = useMemo<PersonalTask[]>(() => [
    ...activeReviews.map((task) => ({
      id: task.id,
      kind: "review" as const,
      title: `第 ${task.question_no} 题阅卷`,
      detail: sourceLabels[task.source] ? `密号 ${task.anonymous_code} · ${sourceLabels[task.source]}` : `密号 ${task.anonymous_code}`,
      status: task.status,
      path: "/grading"
    })),
    ...activeArbitrations.map((task) => ({
      id: task.id,
      kind: "arbitration" as const,
      title: `第 ${task.question_no} 题仲裁`,
      detail: `密号 ${task.anonymous_code} · 分差 ${task.score_difference}`,
      status: task.status,
      path: "/arbitration"
    }))
  ], [activeArbitrations, activeReviews]);

  const primary = tasks[0];
  const quickLinks = [
    ...(hasEveryPermission(user, ["exam:manage"]) ? [{ label: "我的考试", path: "/exams", icon: <ClipboardList size={16} /> }] : []),
    ...(hasEveryPermission(user, ["exam:manage", "file:manage"]) ? [{ label: "试卷与评分标准", path: "/papers", icon: <FileText size={16} /> }] : []),
    ...(hasAnyPermission(user, ["review:manage", "review:work"]) ? [{ label: "我的阅卷", path: "/grading", icon: <BookOpenCheck size={16} /> }] : []),
    ...(hasAnyPermission(user, ["arbitration:manage", "arbitration:work"]) ? [{ label: "我的仲裁", path: "/arbitration", icon: <Gavel size={16} /> }] : [])
  ];

  if (!data && loading) return <LoadingState label="正在加载我的工作" />;
  if (!data && error) return <ErrorState message={error} onRetry={() => void load()} />;

  return (
    <div className="page-stack teacher-dashboard">
      <section className="teacher-heading">
        <div>
          <span className="dashboard-kicker">教师端</span>
          <h1>{user.name}，这是你的工作</h1>
          <p>{user.school} · 这里只显示已授权考试和分配给你的任务</p>
        </div>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load()}>刷新</Button>
      </section>

      {error ? <Alert type="error" showIcon message="刷新失败" description={error} /> : null}
      {data?.warnings.map((warning) => <Alert type="warning" showIcon key={warning} message={warning} />)}

      <section className="teacher-summary" aria-label="我的工作摘要">
        <div><span>待阅卷</span><strong>{activeReviews.length}</strong></div>
        <div><span>待仲裁</span><strong>{activeArbitrations.length}</strong></div>
        <div><span>已提交阅卷</span><strong>{completedReviews}</strong></div>
        <div><span>进行中考试</span><strong>{activeExams.length}</strong></div>
      </section>

      <section className="reviewer-progress-panel" aria-label="我的阅卷进度">
        <div className="reviewer-progress-head">
          <div>
            <h2>我的阅卷进度</h2>
          </div>
          <strong>已提交 {completedReviews} / 共 {reviewTotal} 题</strong>
        </div>
        <Progress percent={reviewProgress} status={reviewProgress === 100 ? "success" : "active"} />
      </section>

      {primary ? (
        <section className="teacher-next-action">
          <div>
            <span>下一项工作</span>
            <h2>{primary.title}</h2>
            <p>{primary.detail}</p>
          </div>
          <Button type="primary" size="large" onClick={() => onNavigate(primary.path)}>开始处理<ArrowRight size={17} /></Button>
        </section>
      ) : null}

      <div className="teacher-dashboard-grid">
        <section className="teacher-task-pane">
          <div className="section-head"><div><h2>我的任务</h2><p>分配给你的阅卷与仲裁任务</p></div><strong>{tasks.length}</strong></div>
          {tasks.length ? <div className="teacher-task-list">{tasks.slice(0, 8).map((task) => (
            <button type="button" key={`${task.kind}-${task.id}`} onClick={() => onNavigate(task.path)}>
              <span className="teacher-task-icon">{task.kind === "review" ? <BookOpenCheck size={17} /> : <Gavel size={17} />}</span>
              <span><strong>{task.title}</strong><small>{task.detail}</small></span>
              <StatusTag tone={taskTone(task.status)}>{taskStatusLabels[task.status] ?? "处理中"}</StatusTag>
              <ArrowRight size={16} />
            </button>
          ))}</div> : <EmptyState title="当前没有分配任务" description="新的阅卷或仲裁任务分配后会显示在这里。" />}
        </section>

        <section className="teacher-exam-pane">
          <div className="section-head"><div><h2>我的考试</h2><p>仅显示当前账号可访问的考试</p></div></div>
          {activeExams.length ? <div className="teacher-exam-list">{activeExams.slice(0, 6).map((exam) => (
            <button type="button" key={exam.id} onClick={() => onNavigate(`/exams/${encodeURIComponent(exam.id)}/overview`)}>
              <span><strong>{exam.name}</strong><small>{examSubjectLabel(exam.subject)} · {exam.class_ids.length} 个班级</small></span>
              <StatusTag tone={examStatusTone(exam.status)}>{examStatusLabels[exam.status] ?? "进行中"}</StatusTag>
              <ArrowRight size={16} />
            </button>
          ))}</div> : <EmptyState title="暂无授权考试" description="获得考试或班级授权后会显示在这里。" />}
        </section>
      </div>

      {quickLinks.length ? <section className="teacher-quick-links"><h2>常用入口</h2><Space wrap>{quickLinks.map((link) => <Button key={link.path} icon={link.icon} onClick={() => onNavigate(link.path)}>{link.label}</Button>)}</Space></section> : null}
    </div>
  );
}
