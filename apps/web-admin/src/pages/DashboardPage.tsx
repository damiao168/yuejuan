import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Progress, Space } from "antd";
import { motion } from "framer-motion";
import { ArrowRight, BrainCircuit, Building2, RefreshCw, ScrollText, ServerCog } from "lucide-react";
import { listArbitrationTasks, listReviewTasks, type ArbitrationTask, type ReviewTask } from "../api/review";
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
  arbitrationTasks: ArbitrationTask[];
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

function examActionLabel(status: string) {
  if (["draft", "configured"].includes(status)) return "继续配置";
  if (["ready", "collecting"].includes(status)) return "查看采集";
  if (["grading", "reviewing"].includes(status)) return "继续阅卷";
  if (status === "finalized") return "发布成绩";
  return "查看考试";
}

function formatActivityTime(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const now = new Date();
  const time = date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
  const sameDay = date.toDateString() === now.toDateString();
  if (sameDay) return `今天 ${time}`;
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (date.toDateString() === yesterday.toDateString()) return `昨天 ${time}`;
  return `${date.getMonth() + 1}月${date.getDate()}日`;
}

async function fetchHome(user: SessionUser): Promise<HomeData> {
  const isPlatform = user.roles.includes("platform_admin");
  const canReadExams = !isPlatform && hasEveryPermission(user, ["exam:manage"]);
  const canReview = !isPlatform && hasAnyPermission(user, ["review:manage", "review:work"]);
  const canArbitrate = !isPlatform && hasAnyPermission(user, ["arbitration:manage", "arbitration:work"]);
  const isOperations = isPlatform && hasEveryPermission(user, ["system:read"]);
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
  if (!isPlatform && hasEveryPermission(user, ["submission:manage"])) {
    const active = exams.filter((exam) => !["published", "archived"].includes(exam.status)).slice(0, 4);
    const results = await Promise.allSettled(active.map((exam) => listSubmissions(exam.id)));
    submissions = results.flatMap((result) => result.status === "fulfilled" ? result.value.submissions : []);
    if (results.some((result) => result.status === "rejected")) warnings.push("部分答卷状态未能加载");
  }

  return {
    exams,
    submissions,
    reviewTasks: reviewResult.status === "fulfilled" ? reviewResult.value.tasks : [],
    arbitrationTasks: arbitrationResult.status === "fulfilled" ? arbitrationResult.value.arbitration_tasks : [],
    systemStatus: systemResult.status === "fulfilled" ? systemResult.value : undefined,
    warnings
  };
}

export function DashboardPage({ user, onNavigate }: { user: SessionUser; onNavigate: (path: string) => void }) {
  const [data, setData] = useState<HomeData>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [nonce, setNonce] = useState(0);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<Date>();
  const isPlatform = user.roles.includes("platform_admin");

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(undefined);
    fetchHome(user)
      .then((next) => {
        if (active) {
          setData(next);
          setLastUpdatedAt(new Date());
        }
      })
      .catch((loadError) => { if (active) setError(loadError instanceof Error ? loadError.message : "首页加载失败"); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [nonce, user]);

  const activeExams = useMemo(() => (data?.exams ?? []).filter((exam) => !["published", "archived"].includes(exam.status)).slice(0, 5), [data?.exams]);
  const failedSubmissions = (data?.submissions ?? []).filter((submission) => submission.status.includes("failed") || submission.quality_status.includes("failed"));
  const unmatchedSubmissions = (data?.submissions ?? []).filter((submission) => !submission.student_id);
  const pendingTasks = (data?.reviewTasks ?? []).filter((task) => !["submitted", "finalized"].includes(task.status));
  const pendingSubmissionCount = new Set(pendingTasks.map((task) => task.submission_id)).size;
  const pendingReviewSubmissionCount = new Set((data?.arbitrationTasks ?? []).map((task) => task.submission_id)).size;
  const finalizedExamCount = (data?.exams ?? []).filter((exam) => exam.status === "finalized").length;
  const qualityIssueSubmissions = (data?.submissions ?? []).filter((submission) => submission.quality_issues.length > 0);
  const todo = [
    ...(pendingTasks.length ? [{ label: "待阅主观题", detail: `来自 ${pendingSubmissionCount} 份答卷`, value: pendingTasks.length, unit: "题", action: "继续阅卷", path: "/grading", tone: "processing" as StatusTone }] : []),
    ...(pendingReviewSubmissionCount ? [{ label: "待人工复核", detail: `${data?.arbitrationTasks.length ?? 0} 项评分差异`, value: pendingReviewSubmissionCount, unit: "份", action: "开始复核", path: "/arbitration", tone: "danger" as StatusTone }] : []),
    ...(failedSubmissions.length ? [{ label: "处理失败答卷", detail: "会阻断后续阅卷", value: failedSubmissions.length, unit: "份", action: "查看异常", path: "/capture", tone: "danger" as StatusTone }] : []),
    ...(unmatchedSubmissions.length ? [{ label: "待匹配学生", detail: "尚未确认答卷归属", value: unmatchedSubmissions.length, unit: "份", action: "确认身份", path: "/capture", tone: "warning" as StatusTone }] : []),
    ...(finalizedExamCount ? [{ label: "待发布成绩", detail: "完成检查后即可发布", value: finalizedExamCount, unit: "场", action: "去发布", path: "/scores", tone: "warning" as StatusTone }] : [])
  ];
  const attention = [
    ...(failedSubmissions.length ? [{ label: `${failedSubmissions.length} 份答卷处理失败`, detail: "会阻断后续阅卷", path: "/capture", tone: "danger" as StatusTone }] : []),
    ...(qualityIssueSubmissions.length ? [{ label: `${qualityIssueSubmissions.length} 份答卷存在图像质量问题`, detail: "需要确认清晰度与完整性", path: "/capture", tone: "warning" as StatusTone }] : []),
    ...(unmatchedSubmissions.length ? [{ label: `${unmatchedSubmissions.length} 份答卷未匹配学生`, detail: "确认身份后才能计入成绩", path: "/capture", tone: "warning" as StatusTone }] : []),
    ...(pendingReviewSubmissionCount ? [{ label: `${pendingReviewSubmissionCount} 份答卷待人工复核`, detail: "需要确认最终得分", path: "/arbitration", tone: "danger" as StatusTone }] : [])
  ];
  const recentActivityCandidates = [
    ...(data?.exams ?? []).map((exam) => ({
      id: `exam-${exam.id}`,
      label: `创建考试：${exam.name}`,
      at: exam.created_at,
      path: `/exams/${encodeURIComponent(exam.id)}/overview`
    })),
    ...(data?.submissions ?? []).map((submission) => ({
      id: `submission-${submission.id}`,
      label: `导入答卷：${submission.candidate_no || "待确认考生"}`,
      at: submission.created_at,
      path: `/exams/${encodeURIComponent(submission.exam_id)}/capture`
    })),
    ...(data?.reviewTasks ?? []).filter((task) => ["submitted", "finalized"].includes(task.status)).map((task) => ({
      id: `review-${task.id}`,
      label: `完成阅卷：${task.question_no} · ${task.anonymous_code}`,
      at: task.updated_at,
      path: "/grading"
    }))
  ]
    .filter((item) => item.at)
    .sort((left, right) => new Date(right.at ?? 0).getTime() - new Date(left.at ?? 0).getTime());
  const seenActivityLabels = new Set<string>();
  const recentActivity = recentActivityCandidates
    .filter((item) => {
      if (seenActivityLabels.has(item.label)) return false;
      seenActivityLabels.add(item.label);
      return true;
    })
    .slice(0, 5);

  const quickActions = [
    ...(hasEveryPermission(user, ["tenant:manage"]) ? [{ label: "学校管理", path: "/platform/schools", icon: <Building2 size={17} /> }] : []),
    ...(hasEveryPermission(user, ["system:read"]) ? [{ label: "系统状态", path: "/system/status", icon: <ServerCog size={17} /> }] : []),
    ...(hasEveryPermission(user, ["model:read"]) ? [{ label: "模型治理", path: "/system/models", icon: <BrainCircuit size={17} /> }] : []),
    ...(hasEveryPermission(user, ["audit:read"]) ? [{ label: "操作审计", path: "/audit", icon: <ScrollText size={17} /> }] : [])
  ];

  if (!data && loading) return <LoadingState label="正在加载工作台" />;
  if (!data && error) return <ErrorState message={error} onRetry={() => setNonce((value) => value + 1)} />;

  if (isPlatform) {
    return (
      <div className="page-stack role-dashboard">
        <section className="dashboard-heading">
          <div>
            <span className="dashboard-kicker">平台管理</span>
            <h1>平台状态</h1>
            <p>查看服务状态和模型配置</p>
          </div>
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => setNonce((value) => value + 1)}>刷新</Button>
        </section>

        {error ? <Alert type="error" showIcon message="刷新失败" description={error} /> : null}
        {data?.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} />)}

        {data?.systemStatus ? (
          <section className="operations-strip">
            <strong>服务状态</strong>
            {data.systemStatus.dependencies.map((dependency) => (
              <span key={dependency.name} title={dependency.name}>
                {dependencyNames[dependency.name] ?? dependency.name}
                <StatusTag tone={dependencyTone(dependency.status)}>{dependencyText(dependency.status)}</StatusTag>
              </span>
            ))}
          </section>
        ) : <EmptyState title="暂无服务状态" description="请刷新后重试。" />}

        {quickActions.length ? (
          <section className="quick-actions">
            <Space wrap>{quickActions.map((action) => <Button key={action.label} icon={action.icon} onClick={() => onNavigate(action.path)}>{action.label}</Button>)}</Space>
          </section>
        ) : null}
      </div>
    );
  }

  return (
    <div className="page-stack role-dashboard school-dashboard">
      <motion.section className="dashboard-heading" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.16 }}>
        <div>
          <h1>考试工作台</h1>
          <p>{user.school}</p>
        </div>
        <div className="dashboard-update">
          <span>{lastUpdatedAt ? `最后更新 ${lastUpdatedAt.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false })}` : "正在更新"}</span>
          <Button type="text" size="small" aria-label="刷新工作台" icon={<RefreshCw size={15} />} loading={loading} onClick={() => setNonce((value) => value + 1)} />
        </div>
      </motion.section>

      {error ? <Alert type="error" showIcon message="刷新失败" description={error} /> : null}
      {data?.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} />)}

      <section className="dashboard-summary" aria-label="关键状态">
        <button type="button" onClick={() => onNavigate("/grading")}>
          <span>待阅答卷</span><strong>{pendingSubmissionCount}<em>份</em></strong><small>{pendingTasks.length ? `包含 ${pendingTasks.length} 道待阅题目` : "当前无待阅题目"}</small>
        </button>
        <button type="button" onClick={() => onNavigate("/arbitration")}>
          <span>待人工复核</span><strong>{pendingReviewSubmissionCount}<em>份</em></strong><small>{pendingReviewSubmissionCount ? "需要确认最终得分" : "当前无需复核"}</small>
        </button>
        <button type="button" onClick={() => onNavigate("/exams")}>
          <span>进行中考试</span><strong>{activeExams.length}<em>场</em></strong><small>{activeExams.filter((exam) => exam.status === "collecting").length} 场正在采集</small>
        </button>
        <button type="button" onClick={() => onNavigate("/scores")}>
          <span>待发布成绩</span><strong>{finalizedExamCount}<em>场</em></strong><small>{finalizedExamCount ? "完成检查后即可发布" : "当前无需发布"}</small>
        </button>
      </section>

      <div className="dashboard-primary-grid">
        <section className="dashboard-pane todo-pane">
          <div className="section-head"><div><h2>我的待办</h2><p>只显示当前需要处理的事项</p></div></div>
          {todo.length ? <div className="todo-list">{todo.map((item) => (
            <button type="button" key={item.label} className="todo-row" onClick={() => onNavigate(item.path)}>
              <span className="todo-label"><StatusTag tone={item.tone}>{item.label}</StatusTag><small>{item.detail}</small></span>
              <strong>{item.value}<small>{item.unit}</small></strong>
              <span className="todo-action">{item.action}</span>
              <ArrowRight size={16} />
            </button>
          ))}</div> : <EmptyState title="当前没有待办" description="新的采集、阅卷或发布事项出现后会显示在这里。" />}
        </section>

        <section className="dashboard-pane exam-pane">
          <div className="section-head"><div><h2>进行中的考试</h2><p>{activeExams.length} 场</p></div></div>
          {activeExams.length ? <div className="active-exam-list">{activeExams.map((exam) => (
            <button type="button" key={exam.id} className="active-exam-row" onClick={() => onNavigate(`/exams/${encodeURIComponent(exam.id)}/overview`)}>
              <div className="active-exam-copy">
                <strong>{exam.name}</strong>
                <span>{examSubjectLabel(exam.subject)} · 已导入 {(data?.submissions ?? []).filter((submission) => submission.exam_id === exam.id).length} 份答卷</span>
              </div>
              <div className="active-exam-status">
                <span>流程进度：{examStatusLabels[exam.status] ?? "进行中"}</span>
                <div className="exam-progress"><Progress percent={progressFor(exam.status)} size="small" showInfo={false} /><strong>{progressFor(exam.status)}%</strong></div>
                {(data?.submissions ?? []).some((submission) => submission.exam_id === exam.id && (submission.quality_issues.length > 0 || submission.status.includes("failed"))) ? <small>存在异常答卷</small> : null}
              </div>
              <span className="exam-next-action">{examActionLabel(exam.status)} <ArrowRight size={15} /></span>
            </button>
          ))}</div> : <EmptyState title="暂无进行中考试" description="创建考试后，可从这里直接进入考试工作区。" />}
        </section>
      </div>

      <div className={`dashboard-secondary-grid ${attention.length ? "" : "without-attention"}`}>
        {attention.length ? <section className="dashboard-pane attention-pane">
          <div className="section-head"><div><h2>需要关注</h2><p>可能影响考试流程的异常</p></div></div>
          <div className="attention-list">{attention.map((item) => (
            <button type="button" key={item.label} onClick={() => onNavigate(item.path)}>
              <StatusTag tone={item.tone}>{item.label}</StatusTag><span>{item.detail}</span><ArrowRight size={15} />
            </button>
          ))}</div>
        </section> : null}
        <section className="dashboard-pane activity-pane">
          <div className="section-head"><div><h2>最近动态</h2><p>最近的考试与答卷操作</p></div></div>
          {recentActivity.length ? <div className="activity-list">{recentActivity.map((item) => (
            <button type="button" key={item.id} onClick={() => onNavigate(item.path)}>
              <span>{item.label}</span><time>{formatActivityTime(item.at)}</time>
            </button>
          ))}</div> : <EmptyState title="暂无动态" description="创建考试或导入答卷后会显示在这里。" />}
        </section>
      </div>
    </div>
  );
}
