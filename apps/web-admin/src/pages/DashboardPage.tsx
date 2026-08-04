import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Progress, Space } from "antd";
import { motion } from "framer-motion";
import { ArrowRight, BrainCircuit, Building2, RefreshCw, ScrollText, ServerCog } from "lucide-react";
import { getDashboardSummary, type DashboardActivity, type DashboardSummary } from "../api/dashboard";
import { getSystemStatus, type SystemStatus } from "../api/system";
import { hasEveryPermission, type SessionUser } from "../auth/session";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import { examStatusLabels, examSubjectLabel } from "../constants/examStatus";
import type { StatusTone } from "../types";

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
  if (status === "mock") return "info";
  if (status === "disabled" || status === "not_configured") return "neutral";
  return "danger";
}

function dependencyText(status: string) {
  if (status === "ok") return "正常";
  if (status === "disabled") return "已关闭";
  if (status === "mock") return "演示模式";
  if (status === "not_configured") return "未启用";
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
  if (date.toDateString() === now.toDateString()) return `今天 ${time}`;
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (date.toDateString() === yesterday.toDateString()) return `昨天 ${time}`;
  return `${date.getMonth() + 1}月${date.getDate()}日`;
}

function activityLabel(activity: DashboardActivity) {
  const labels: Record<string, string> = {
    "exam.created": "创建考试",
    "exam.updated": "更新考试",
    "exam.status_updated": "更新考试状态",
    "submission.created": "导入答题卡",
    "submission.status_updated": "更新答题卡状态",
    "capture.batch_completed": "完成答题卡导入",
    "review.grade_submitted": "完成阅卷",
    "score.exam_finalized": "确认成绩",
    "score.exam_published": "发布成绩"
  };
  return labels[activity.action] ?? activity.reason ?? "完成业务操作";
}

function PlatformDashboard({
  user,
  onNavigate
}: {
  user: SessionUser;
  onNavigate: (path: string) => void;
}) {
  const [status, setStatus] = useState<SystemStatus>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      setStatus(await getSystemStatus());
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "平台状态加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const quickActions = [
    ...(hasEveryPermission(user, ["tenant:manage"]) ? [{ label: "学校管理", path: "/platform/schools", icon: <Building2 size={17} /> }] : []),
    ...(hasEveryPermission(user, ["system:read"]) ? [{ label: "系统状态", path: "/system/status", icon: <ServerCog size={17} /> }] : []),
    ...(hasEveryPermission(user, ["model:read"]) ? [{ label: "模型治理", path: "/system/models", icon: <BrainCircuit size={17} /> }] : []),
    ...(hasEveryPermission(user, ["audit:read"]) ? [{ label: "操作审计", path: "/audit", icon: <ScrollText size={17} /> }] : [])
  ];

  if (!status && loading) return <LoadingState label="正在加载平台状态" />;
  if (!status && error) return <ErrorState message={error} onRetry={() => void load()} />;

  return (
    <div className="page-stack role-dashboard">
      <section className="dashboard-heading">
        <div><span className="dashboard-kicker">平台管理</span><h1>平台状态</h1><p>管理学校并确认平台服务是否可用</p></div>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load()}>刷新</Button>
      </section>
      {error ? <Alert type="error" showIcon message="刷新失败" description={error} /> : null}
      {status ? (
        <section className="operations-strip">
          <strong>服务状态</strong>
          {status.dependencies.map((dependency) => (
            <span key={dependency.name} title={dependency.detail || dependency.name}>
              {dependencyNames[dependency.name] ?? dependency.name}
              <StatusTag tone={dependencyTone(dependency.status)}>{dependencyText(dependency.status)}</StatusTag>
            </span>
          ))}
        </section>
      ) : <EmptyState title="服务状态暂时不可用" description="请刷新页面；若仍失败，请进入系统运维查看原因。" />}
      <section className="quick-actions">
        <Space wrap>{quickActions.map((action) => <Button key={action.label} icon={action.icon} onClick={() => onNavigate(action.path)}>{action.label}</Button>)}</Space>
      </section>
    </div>
  );
}

export function DashboardPage({ user, onNavigate }: { user: SessionUser; onNavigate: (path: string) => void }) {
  const isPlatform = user.roles.includes("platform_admin");
  const [data, setData] = useState<DashboardSummary>();
  const [loading, setLoading] = useState(!isPlatform);
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    if (isPlatform) return;
    setLoading(true);
    setError(undefined);
    try {
      setData(await getDashboardSummary());
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "工作台加载失败");
    } finally {
      setLoading(false);
    }
  }, [isPlatform]);

  useEffect(() => { void load(); }, [load]);

  const stats = data?.statistics;
  const todo = useMemo(() => stats ? [
    ...(stats.pending_review_question_count ? [{ label: "待阅主观题", detail: `来自 ${stats.pending_review_submission_count} 份答题卡`, value: stats.pending_review_question_count, unit: "题", action: "继续阅卷", path: "/grading?status=pending", tone: "processing" as StatusTone }] : []),
    ...(stats.pending_arbitration_submission_count ? [{ label: "待人工复核", detail: `${stats.pending_arbitration_count} 项评分差异`, value: stats.pending_arbitration_submission_count, unit: "份", action: "开始复核", path: "/arbitration?status=pending", tone: "danger" as StatusTone }] : []),
    ...(stats.failed_submission_count ? [{ label: "处理失败答题卡", detail: "会阻断后续阅卷", value: stats.failed_submission_count, unit: "份", action: "查看异常", path: "/capture?issue=failed", tone: "danger" as StatusTone }] : []),
    ...(stats.unmatched_submission_count ? [{ label: "待匹配学生", detail: "尚未确认答题卡归属", value: stats.unmatched_submission_count, unit: "份", action: "确认身份", path: "/capture?issue=unmatched", tone: "warning" as StatusTone }] : []),
    ...(stats.finalized_exam_count ? [{ label: "待发布成绩", detail: "完成检查后即可发布", value: stats.finalized_exam_count, unit: "场", action: "去发布", path: "/scores?status=finalized", tone: "warning" as StatusTone }] : [])
  ] : [], [stats]);

  if (isPlatform) return <PlatformDashboard user={user} onNavigate={onNavigate} />;
  if (!data && loading) return <LoadingState label="正在加载工作台" />;
  if (!data && error) return <ErrorState message={error} onRetry={() => void load()} />;
  if (!data || !stats) return <EmptyState title="工作台暂时不可用" description="请刷新页面；考试数据不会受影响。" />;

  return (
    <div className="page-stack role-dashboard school-dashboard">
      <motion.section className="dashboard-heading" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.16 }}>
        <div><h1>考试工作台</h1><p>{user.school}</p></div>
        <div className="dashboard-update">
          <span>最后更新 {new Date(data.updated_at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false })}</span>
          <Button type="text" size="small" aria-label="刷新工作台" icon={<RefreshCw size={15} />} loading={loading} onClick={() => void load()} />
        </div>
      </motion.section>

      {error ? <Alert type="error" showIcon message="刷新失败" description={`${error}；页面继续显示上次成功数据。`} /> : null}
      {data.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} description="其他统计仍可使用，请稍后刷新。" />)}

      <section className="dashboard-summary" aria-label="关键状态">
        <button type="button" onClick={() => onNavigate("/grading?status=pending")}>
          <span>待阅答题卡</span><strong>{stats.pending_review_submission_count}<em>份</em></strong><small>{stats.pending_review_question_count ? `包含 ${stats.pending_review_question_count} 道待阅题目` : "当前无待阅题目"}</small>
        </button>
        <button type="button" onClick={() => onNavigate("/arbitration?status=pending")}>
          <span>待人工复核</span><strong>{stats.pending_arbitration_submission_count}<em>份</em></strong><small>{stats.pending_arbitration_count ? `包含 ${stats.pending_arbitration_count} 项评分差异` : "当前无需复核"}</small>
        </button>
        <button type="button" onClick={() => onNavigate("/exams?status=active")}>
          <span>进行中的考试</span><strong>{stats.active_exam_count}<em>场</em></strong><small>{stats.collecting_exam_count} 场正在采集</small>
        </button>
        <button type="button" onClick={() => onNavigate("/scores?status=finalized")}>
          <span>待发布成绩</span><strong>{stats.finalized_exam_count}<em>场</em></strong><small>{stats.finalized_exam_count ? "完成检查后即可发布" : "当前无需发布"}</small>
        </button>
      </section>

      <div className="dashboard-primary-grid">
        <section className="dashboard-pane todo-pane">
          <div className="section-head"><div><h2>我的待办</h2><p>只显示当前需要处理的事项</p></div></div>
          {todo.length ? <div className="todo-list">{todo.map((item) => (
            <button type="button" key={item.label} className="todo-row" onClick={() => onNavigate(item.path)}>
              <span className="todo-label"><StatusTag tone={item.tone}>{item.label}</StatusTag><small>{item.detail}</small></span>
              <strong>{item.value}<small>{item.unit}</small></strong><span className="todo-action">{item.action}</span><ArrowRight size={16} />
            </button>
          ))}</div> : <EmptyState title="当前没有待办" description="新导入、阅卷或发布事项出现后会显示在这里。" />}
        </section>

        <section className="dashboard-pane exam-pane">
          <div className="section-head"><div><h2>进行中的考试</h2><p>{stats.active_exam_count} 场</p></div></div>
          {data.active_exams.length ? <div className="active-exam-list">{data.active_exams.slice(0, 5).map((exam) => (
            <button type="button" key={exam.id} className="active-exam-row" onClick={() => onNavigate(`/exams/${encodeURIComponent(exam.id)}/overview`)}>
              <div className="active-exam-copy"><strong>{exam.name}</strong><span>{examSubjectLabel(exam.subject)} · 已导入 {exam.submission_count} 份答题卡</span></div>
              <div className="active-exam-status">
                <span>流程进度：{examStatusLabels[exam.status] ?? "进行中"}</span>
                <div className="exam-progress"><Progress percent={progressFor(exam.status)} size="small" showInfo={false} /><strong>{progressFor(exam.status)}%</strong></div>
                {exam.failed_count + exam.quality_issue_count + exam.unmatched_count > 0 ? <small>存在 {exam.failed_count + exam.quality_issue_count + exam.unmatched_count} 项待处理问题</small> : null}
              </div>
              <span className="exam-next-action">{examActionLabel(exam.status)} <ArrowRight size={15} /></span>
            </button>
          ))}</div> : <EmptyState title="暂无进行中考试" description="创建考试后，可从这里直接进入考试工作区。" />}
        </section>
      </div>

      <div className={`dashboard-secondary-grid ${data.blocking_issues.length ? "" : "without-attention"}`}>
        {data.blocking_issues.length ? <section className="dashboard-pane attention-pane">
          <div className="section-head"><div><h2>需要关注</h2><p>可能影响考试流程的异常</p></div></div>
          <div className="attention-list">{data.blocking_issues.map((item) => (
            <button type="button" key={item.code} onClick={() => onNavigate(item.drilldown_path)}>
              <StatusTag tone={item.code === "failed_submissions" || item.code === "pending_arbitration" ? "danger" : "warning"}>{`${item.count} ${item.unit}${item.label}`}</StatusTag>
              <span>{item.impact}；{item.action}</span><ArrowRight size={15} />
            </button>
          ))}</div>
        </section> : null}
        <section className="dashboard-pane activity-pane">
          <div className="section-head"><div><h2>最近动态</h2><p>来自操作审计的最近业务动作</p></div></div>
          {data.recent_activities.length ? <div className="activity-list">{data.recent_activities.map((item) => (
            <button type="button" key={item.id} disabled={!item.drilldown_path} onClick={() => item.drilldown_path && onNavigate(item.drilldown_path)}>
              <span>{activityLabel(item)}</span><time>{formatActivityTime(item.created_at)}</time>
            </button>
          ))}</div> : <EmptyState title="暂无动态" description="创建考试或导入答题卡后会显示在这里。" />}
        </section>
      </div>
    </div>
  );
}
