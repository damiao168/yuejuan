import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Space } from "antd";
import { motion } from "framer-motion";
import { AlertTriangle, ArrowRight, BrainCircuit, Building2, Check, ClipboardCheck, GraduationCap, Plus, RefreshCw, ScrollText, ServerCog, UserCog, UsersRound } from "lucide-react";
import { getDashboardSummary, type DashboardActiveExam, type DashboardSummary } from "../api/dashboard";
import { getSystemStatus, type SystemStatus } from "../api/system";
import { hasEveryPermission, type SessionUser } from "../auth/session";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import { examSubjectLabel } from "../constants/examStatus";
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

const examStages = ["考试准备", "答卷导入", "阅卷", "成绩"] as const;

function examStageIndex(status: string) {
  if (["draft", "configured", "ready"].includes(status)) return 0;
  if (["collecting", "processing"].includes(status)) return 1;
  if (["grading", "reviewing"].includes(status)) return 2;
  return 3;
}

function examAction(exam: DashboardActiveExam) {
  const encodedId = encodeURIComponent(exam.id);
  const issueCount = exam.failed_count + exam.quality_issue_count + exam.unmatched_count;
  if (["draft", "configured", "ready"].includes(exam.status)) {
    return { label: "继续准备", path: `/exams/${encodedId}/settings`, next: exam.status === "ready" ? "开始导入答卷" : "完善试卷与评分标准" };
  }
  if (["collecting", "processing"].includes(exam.status)) {
    return { label: issueCount ? "处理答卷" : "继续导入", path: `/exams/${encodedId}/capture`, next: issueCount ? "处理答卷异常" : "继续导入答卷" };
  }
  if (["grading", "reviewing"].includes(exam.status)) {
    return { label: "继续阅卷", path: `/exams/${encodedId}/grading`, next: "处理需要人工确认的评分" };
  }
  if (exam.status === "finalized") return { label: "发布成绩", path: `/exams/${encodedId}/scores`, next: "检查并发布成绩" };
  if (exam.status === "published") return { label: "查看成绩", path: `/exams/${encodedId}/scores`, next: "查看成绩与报告" };
  return { label: "查看考试", path: `/exams/${encodedId}/overview`, next: "查看考试详情" };
}

function ExamStageRail({ status }: { status: string }) {
  const current = examStageIndex(status);
  return (
    <div className="exam-stage-rail" aria-label={`当前阶段：${examStages[current]}`}>
      {examStages.map((stage, index) => (
        <span key={stage} className={index < current ? "complete" : index === current ? "current" : "future"}>
          <i>{index < current ? <Check size={12} strokeWidth={2.5} /> : index + 1}</i>
          <small>{stage}</small>
        </span>
      ))}
    </div>
  );
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
  const canCreateExam = hasEveryPermission(user, ["exam:manage"]);
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
  const todo = useMemo(() => {
    if (!stats || !data) return [];
    const blockerByCode = new Map(data.blocking_issues.map((issue) => [issue.code, issue]));
    const withBlocker = (code: string, fallback: { detail: string; action: string; path: string }) => {
      const issue = blockerByCode.get(code);
      return issue ? { detail: issue.impact, action: issue.action, path: issue.drilldown_path } : fallback;
    };
    return [
      ...(stats.failed_submission_count ? [{ label: "答卷处理失败", value: stats.failed_submission_count, unit: "份", tone: "danger" as StatusTone, ...withBlocker("failed_submissions", { detail: "会阻断后续阅卷", action: "查看异常", path: "/capture?issue=failed" }) }] : []),
      ...(stats.unmatched_submission_count ? [{ label: "学生身份待确认", value: stats.unmatched_submission_count, unit: "份", tone: "warning" as StatusTone, ...withBlocker("unmatched_submissions", { detail: "答卷尚未匹配学生", action: "立即确认", path: "/capture?issue=unmatched" }) }] : []),
      ...(stats.pending_arbitration_submission_count ? [{ label: "待人工复核", value: stats.pending_arbitration_count, unit: "项", tone: "danger" as StatusTone, ...withBlocker("pending_arbitration", { detail: `来自 ${stats.pending_arbitration_submission_count} 份答卷`, action: "开始复核", path: "/arbitration?status=pending" }) }] : []),
      ...(stats.pending_review_question_count ? [{ label: "主观题等待确认", detail: `来自 ${stats.pending_review_submission_count} 份答题卡`, value: stats.pending_review_question_count, unit: "题", action: "继续阅卷", path: "/grading?status=pending", tone: "processing" as StatusTone }] : []),
      ...(stats.finalized_exam_count ? [{ label: "成绩等待发布", detail: "阅卷已完成，检查后即可发布", value: stats.finalized_exam_count, unit: "场", action: "去发布", path: "/scores?status=finalized", tone: "warning" as StatusTone }] : [])
    ];
  }, [data, stats]);

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
          {canCreateExam ? <Button type="primary" icon={<Plus size={16} />} onClick={() => onNavigate("/exams/new")}>新建考试</Button> : null}
        </div>
      </motion.section>

      {error ? <Alert type="error" showIcon message="刷新失败" description={`${error}；页面继续显示上次成功数据。`} /> : null}
      {data.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} description="其他统计仍可使用，请稍后刷新。" />)}

      <div className="dashboard-brief" aria-label="工作台摘要">
        <span><strong>{stats.pending_review_question_count}</strong> 题待阅</span>
        <span><strong>{stats.active_exam_count}</strong> 场进行中</span>
        <span><strong>{data.blocking_issues.reduce((total, issue) => total + issue.count, 0)}</strong> 项阻断</span>
      </div>

      <div className="dashboard-workbench-grid">
        <main className="dashboard-main-column">
          <section className="dashboard-pane todo-pane">
            <div className="section-head"><div><h2>需要你处理</h2><p>阻断流程的事项优先显示</p></div><strong>{todo.length}</strong></div>
            {todo.length ? <div className="todo-list">{todo.map((item) => (
              <button type="button" key={item.label} className={`todo-row ${item.tone}`} onClick={() => onNavigate(item.path)}>
                <span className="todo-priority" aria-hidden="true">{item.tone === "danger" ? <AlertTriangle size={17} /> : <i />}</span>
                <span className="todo-label"><strong>{item.label}</strong><small>{item.detail}</small></span>
                <span className="todo-value"><strong>{item.value}</strong><small>{item.unit}</small></span>
                <span className="todo-action">{item.action}<ArrowRight size={15} /></span>
              </button>
            ))}</div> : <div className="dashboard-compact-empty"><Check size={17} /><span><strong>当前没有需要你处理的事项</strong><small>新的异常、阅卷或发布任务会显示在这里。</small></span></div>}
          </section>

          <section className="dashboard-pane exam-pane">
            <div className="section-head"><div><h2>进行中的考试</h2><p>按考试推进准备、导入、阅卷与成绩</p></div><Button type="link" onClick={() => onNavigate("/exams")}>全部考试 <ArrowRight size={14} /></Button></div>
            {data.active_exams.length ? <div className="active-exam-list">{data.active_exams.slice(0, 5).map((exam) => {
              const action = examAction(exam);
              const issueCount = exam.failed_count + exam.quality_issue_count + exam.unmatched_count;
              return (
                <article key={exam.id} className="active-exam-row">
                  <header className="active-exam-copy"><strong>{exam.name}</strong><span>{examSubjectLabel(exam.subject)} · {exam.submission_count ? `${exam.submission_count} 份答卷已导入` : "尚未导入答卷"}</span></header>
                  <ExamStageRail status={exam.status} />
                  <div className="active-exam-next">
                    <span><small>下一步</small><strong>{action.next}</strong>{issueCount ? <em>{issueCount} 项需要处理</em> : null}</span>
                    <Button type="primary" ghost onClick={() => onNavigate(action.path)}>{action.label} <ArrowRight size={15} /></Button>
                  </div>
                </article>
              );
            })}</div> : <EmptyState title="暂无进行中考试" description="创建考试后，可从这里直接进入考试工作区。" />}
          </section>
        </main>

        <aside className="dashboard-side-column">
          <section className="dashboard-side-section">
            <div className="section-head"><div><h2>成员管理</h2><p>维护考试所需的基础数据</p></div></div>
            <div className="dashboard-link-list">
              <button type="button" onClick={() => onNavigate("/members/students")}><UsersRound size={17} /><span><strong>学生管理</strong><small>维护学生名册与状态</small></span><ArrowRight size={15} /></button>
              <button type="button" onClick={() => onNavigate("/members/classes")}><GraduationCap size={17} /><span><strong>年级与班级</strong><small>设置考试学生范围</small></span><ArrowRight size={15} /></button>
              <button type="button" onClick={() => onNavigate("/members/teachers")}><UserCog size={17} /><span><strong>教师与阅卷人员</strong><small>维护人员账号</small></span><ArrowRight size={15} /></button>
            </div>
          </section>

          <section className="dashboard-side-section grading-entry">
            <ClipboardCheck size={19} />
            <div><h2>阅卷中心</h2><p>{stats.pending_review_question_count ? <><strong>{stats.pending_review_question_count}</strong> 题等待人工处理</> : "当前没有待人工处理的题目"}</p></div>
            <Button onClick={() => onNavigate("/grading?status=pending")}>进入阅卷中心 <ArrowRight size={14} /></Button>
          </section>
        </aside>
      </div>
    </div>
  );
}
