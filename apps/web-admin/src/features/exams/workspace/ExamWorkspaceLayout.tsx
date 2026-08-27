import { useMemo, type ReactNode } from "react";
import { Button, Space } from "antd";
import { ArrowLeft, ArrowRight, RefreshCw } from "lucide-react";
import {
  WorkspaceLayout,
  WorkspaceMetricStrip,
  ExamStageIndicator,
  QualityIndicator,
  WorkspaceStageRail,
  type WorkspaceMetric
} from "@edugrade/ui";
import { StatusTag } from "../../../components/StatusTag";
import { examStatusLabels, examStatusTone } from "../../../constants/examStatus";
import type { SessionUser } from "../../../auth/session";
import type { ProductExperience } from "../../../router/experience";
import type { ExamWorkspaceProjection } from "../../../api/workspace";
import { currentExamBusinessStage, examBusinessStages, sectionBusinessStage } from "./businessStages";

const sectionNames: Record<string, string> = {
  overview: "考试概览",
  students: "学生范围",
  paper: "试卷",
  questions: "题目与评分标准",
  template: "答题卡模板",
  settings: "开考准备",
  capture: "答卷导入",
  processing: "识别处理",
  grading: "阅卷",
  quality: "复核与异常",
  scores: "成绩与报告",
  appeals: "申诉处理",
  reports: "成绩分析"
};

function workspaceMetrics(data: ExamWorkspaceProjection): WorkspaceMetric[] {
  return [
    { key: "submissions", label: "已导入答卷", value: data.counts.submission_count, suffix: "份", helper: data.counts.quality_issue_submission_count ? `${data.counts.quality_issue_submission_count} 份需关注` : "当前无质量异常" },
    { key: "questions", label: "题目", value: data.counts.question_count, suffix: "题", helper: `${data.counts.paper_count} 个试卷版本` },
    { key: "review", label: "待阅任务", value: data.counts.pending_review_count, suffix: "个", helper: "按阅卷任务口径统计" },
    { key: "arbitration", label: "待人工复核", value: data.counts.pending_arbitration_count, suffix: "个", helper: data.counts.pending_arbitration_count ? "完成后才能确认成绩" : "当前无需复核" }
  ];
}

function Overview({ data, onNavigate }: { data: ExamWorkspaceProjection; onNavigate: (path: string) => void }) {
  const metrics = useMemo(() => workspaceMetrics(data), [data]);
  return (
    <main className="eg-workspace-overview">
      <WorkspaceMetricStrip metrics={metrics} />
      <section className="eg-workspace-panel eg-workspace-next-panel">
        <div className="eg-workspace-panel-heading"><div><span>优先处理</span><h2>当前下一步</h2></div><small>{data.next_actions.length} 项</small></div>
        <div className="eg-next-actions">
          {data.next_actions.map((action) => (
            <button type="button" key={action.code} onClick={() => onNavigate(action.route)}>
              <div><strong>{action.label}</strong><p>{action.description}</p></div><ArrowRight size={16} />
            </button>
          ))}
        </div>
        {data.blockers.length || data.warnings.length ? <div className="eg-workspace-check-summary">
          <span>{data.blockers.length ? `${data.blockers.length} 项开考检查待处理` : `${data.warnings.length} 项需要确认`}</span>
          <Button type="link" onClick={() => onNavigate(`/exams/${encodeURIComponent(data.exam_id)}/settings`)}>查看开考准备</Button>
        </div> : null}
      </section>
    </main>
  );
}

export function ExamWorkspaceLayout({
  data,
  section,
  experience,
  currentUser,
  isFetching,
  moduleContent,
  onNavigate,
  onRefresh
}: {
  data: ExamWorkspaceProjection;
  section: string;
  experience: ProductExperience;
  currentUser: SessionUser;
  isFetching: boolean;
  moduleContent?: ReactNode;
  onNavigate: (path: string) => void;
  onRefresh: () => void;
}) {
  const primaryAction = data.next_actions[0];
  const currentSectionName = sectionNames[section] ?? "考试工作区";
  const businessStages = useMemo(() => examBusinessStages(data), [data]);
  const currentStage = useMemo(() => currentExamBusinessStage(data), [data]);
  const showPreparationReturn = sectionBusinessStage[section] === "preparation" && !["overview", "settings"].includes(section);
  return (
    <WorkspaceLayout
      header={
        <header className="exam-context-header">
          <Button type="text" icon={<ArrowLeft size={17} />} onClick={() => onNavigate("/exams")} aria-label="返回考试列表" />
          <div className="exam-context-title">
            <span>{data.subject_summary.label} · {data.subject_summary.total_score} 分</span>
            <h1>{data.exam_name}</h1>
            <Space size="small">
              <StatusTag tone={examStatusTone(data.exam_status)}>{examStatusLabels[data.exam_status] ?? "未知状态"}</StatusTag>
              <ExamStageIndicator stage={currentStage} />
              <QualityIndicator blockers={data.blockers.length} warnings={data.warnings.length} />
              <span className={`eg-risk-tier is-${data.risk_tier.toLowerCase()}`}>{data.risk_tier === "unknown" ? "风险待确认" : `风险 ${data.risk_tier}`}</span>
              <span>{experience === "admin" ? `当前操作人：${currentUser.name}` : "我的考试"}</span>
            </Space>
          </div>
          <div className="exam-context-progress">
            <span>当前环节</span>
            <strong>{currentStage?.label ?? currentSectionName}</strong>
          </div>
          <Space>
            <Button icon={<RefreshCw size={16} />} loading={isFetching} onClick={onRefresh} aria-label="刷新考试工作区" />
            {primaryAction ? <Button type="primary" onClick={() => onNavigate(primaryAction.route)}>{primaryAction.label}</Button> : null}
          </Space>
        </header>
      }
      stageRail={<WorkspaceStageRail stages={businessStages} onNavigate={onNavigate} />}
    >
      {section === "overview" ? <Overview data={data} onNavigate={onNavigate} /> : moduleContent ? <div className="exam-workspace-module">{showPreparationReturn ? <Button className="exam-module-return" type="link" icon={<ArrowLeft size={15} />} onClick={() => onNavigate(`/exams/${encodeURIComponent(data.exam_id)}/settings`)}>返回考试准备</Button> : null}{moduleContent}</div> : (
        <main className="eg-workspace-panel eg-workspace-section-empty">
          <div><span>考试工作区</span><h2>{currentSectionName}</h2><p>该环节暂时没有独立页面，请从阶段导航选择可用入口。</p></div>
          <Button onClick={() => onNavigate(`/exams/${encodeURIComponent(data.exam_id)}/overview`)}>返回概览</Button>
        </main>
      )}
    </WorkspaceLayout>
  );
}
