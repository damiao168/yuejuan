import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Alert, Button, Progress, Segmented, Space, Statistic } from "antd";
import { ArrowLeft, ArrowRight, CheckCircle2, CircleAlert, RefreshCw } from "lucide-react";
import { getExam, type Exam } from "../api/exams";
import { getExamReadiness, type ExamReadiness } from "../api/configuration";
import { listClasses, listGrades } from "../api/org";
import { listPapers, listQuestions } from "../api/papers";
import { listSubmissions } from "../api/submissions";
import { ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import type { SessionUser } from "../auth/session";

interface WorkspaceData {
  exam: Exam;
  paperCount: number;
  questionCount: number;
  submissionCount: number;
  reviewRequiredCount: number;
  gradeLabel: string;
  readiness?: ExamReadiness;
  warnings: string[];
}

const sections = [
  ["overview", "概览"], ["students", "学生"], ["paper", "试卷"], ["questions", "题目与 Rubric"],
  ["template", "答卷模板"], ["capture", "采集"], ["processing", "处理"], ["grading", "阅卷"],
  ["quality", "质量"], ["scores", "成绩"], ["appeals", "申诉"], ["reports", "报告"], ["settings", "设置"]
] as const;

const statusLabels: Record<string, string> = {
  draft: "草稿",
  configured: "配置中",
  ready: "准备完成",
  collecting: "采集中",
  grading: "阅卷中",
  reviewing: "质量检查",
  finalized: "待发布",
  published: "已发布",
  archived: "已归档"
};

const progressByStatus: Record<string, number> = { draft: 10, configured: 25, ready: 35, collecting: 45, grading: 68, reviewing: 82, finalized: 94, published: 100, archived: 100 };

const sectionDestinations: Record<string, { path: string; action: string; description: string }> = {
  students: { path: "/organization/setup", action: "管理学生范围", description: "选择本场考试班级，并核对在读学生范围。" },
  paper: { path: "/papers", action: "配置试卷", description: "上传并登记本场考试使用的试卷版本。" },
  questions: { path: "/papers", action: "配置题目与 Rubric", description: "建立题目、答案和主观题评分规则。" },
  template: { path: "/papers", action: "配置答卷模板", description: "在试卷底图上框选每道题的答题区域并锁定版本。" },
  capture: { path: "/capture", action: "进入答卷采集", description: "导入答卷并执行页面质量检查。" },
  processing: { path: "/capture", action: "查看处理进度", description: "查看质量检测、OCR 和答题区域处理状态。" },
  grading: { path: "/grading", action: "进入阅卷", description: "领取或继续处理本场考试的阅卷任务。" },
  quality: { path: "/arbitration", action: "进入质量处理", description: "查看双评差异和待仲裁事项。" },
  scores: { path: "/scores", action: "进入成绩发布", description: "汇总、确认并发布本场考试成绩。" },
  appeals: { path: "/appeals", action: "查看申诉", description: "处理与本场考试成绩相关的学生申诉。" },
  reports: { path: "/reports", action: "查看报告", description: "查看本场考试的真实学情分析。" },
  settings: { path: "/exams", action: "开考准备检查", description: "查看服务端阻断项，确认准备完成并开始采集。" }
};

async function fetchWorkspace(examId: string): Promise<WorkspaceData> {
  const [examResult, papersResult, questionsResult, submissionsResult, classesResult, gradesResult, readinessResult] = await Promise.allSettled([
    getExam(examId), listPapers(examId), listQuestions(examId), listSubmissions(examId), listClasses(), listGrades(), getExamReadiness(examId)
  ]);
  if (examResult.status === "rejected") throw examResult.reason;
  const warnings: string[] = [];
  if (papersResult.status === "rejected") warnings.push("试卷状态暂时不可用");
  if (questionsResult.status === "rejected") warnings.push("题目状态暂时不可用");
  if (submissionsResult.status === "rejected") warnings.push("答卷状态暂时不可用");
  if (classesResult.status === "rejected" || gradesResult.status === "rejected") warnings.push("年级信息暂时不可用");
  if (readinessResult.status === "rejected") warnings.push("开考准备状态暂时不可用");
  const submissions = submissionsResult.status === "fulfilled" ? submissionsResult.value.submissions : [];
  const classIds = new Set(examResult.value.exam.class_ids);
  const gradeIds = new Set(
    classesResult.status === "fulfilled"
      ? classesResult.value.classes.filter((item) => classIds.has(item.id)).map((item) => item.grade_id)
      : []
  );
  const gradeNames = gradesResult.status === "fulfilled"
    ? gradesResult.value.grades.filter((item) => gradeIds.has(item.id)).map((item) => item.name)
    : [];
  return {
    exam: examResult.value.exam,
    paperCount: papersResult.status === "fulfilled" ? papersResult.value.papers.length : 0,
    questionCount: questionsResult.status === "fulfilled" ? questionsResult.value.questions.length : 0,
    submissionCount: submissions.length,
    reviewRequiredCount: submissions.filter((item) => item.quality_status.includes("review") || !item.student_id).length,
    gradeLabel: gradeNames.length > 1 ? "多个年级" : gradeNames[0] ?? "年级待确认",
    readiness: readinessResult.status === "fulfilled" ? readinessResult.value.readiness : undefined,
    warnings
  };
}

export function ExamWorkspacePage({ examId, section, currentUser, moduleContent, refreshKey = 0, onNavigate }: { examId: string; section: string; currentUser: SessionUser; moduleContent?: ReactNode; refreshKey?: number; onNavigate: (path: string) => void }) {
  const [data, setData] = useState<WorkspaceData>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(undefined);
    fetchWorkspace(examId)
      .then((next) => { if (active) setData(next); })
      .catch((loadError) => { if (active) setError(loadError instanceof Error ? loadError.message : "考试工作区加载失败"); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [examId, nonce, refreshKey]);

  const nextAction = useMemo(() => {
    const status = data?.exam.status;
    if (status === "draft" || status === "configured") return { label: "继续开考准备", section: "settings" };
    if (status === "ready") return { label: "开始采集", section: "settings" };
    if (status === "collecting") return { label: "继续采集", section: "capture" };
    if (status === "grading" || status === "reviewing") return { label: "继续阅卷", section: "grading" };
    if (status === "finalized") return { label: "发布检查", section: "scores" };
    return { label: "查看概览", section: "overview" };
  }, [data?.exam.status]);

  if (!data && loading) return <LoadingState label="正在加载考试工作区" />;
  if (!data && error) return <ErrorState message={error} onRetry={() => setNonce((value) => value + 1)} />;
  if (!data) return null;

  const destination = sectionDestinations[section];
  const readyIssues = data.readiness?.checks.filter((item) => !item.passed) ?? [];

  return (
    <div className="exam-workspace-page">
      <header className="exam-context-header">
        <Button type="text" icon={<ArrowLeft size={17} />} onClick={() => onNavigate("/exams")} aria-label="返回考试列表" />
        <div className="exam-context-title">
          <span>{data.exam.subject} · {data.gradeLabel}</span>
          <h1>{data.exam.name}</h1>
          <Space size="small"><StatusTag tone={data.exam.status === "published" ? "success" : "processing"}>{statusLabels[data.exam.status] ?? data.exam.status}</StatusTag><span>负责人：{data.exam.created_by === currentUser.id ? currentUser.name : "已授权人员"}</span></Space>
        </div>
        <div className="exam-context-progress"><span>总体进度</span><Progress percent={progressByStatus[data.exam.status] ?? 0} size="small" /></div>
        <Space><Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => setNonce((value) => value + 1)} aria-label="刷新考试工作区" /><Button type="primary" onClick={() => onNavigate(`/exams/${encodeURIComponent(examId)}/${nextAction.section}`)}>{nextAction.label}</Button></Space>
      </header>

      <nav className="exam-section-nav" aria-label="考试工作区导航">
        <Segmented value={sections.some(([key]) => key === section) ? section : "overview"} options={sections.map(([value, label]) => ({ value, label }))} onChange={(value) => onNavigate(`/exams/${encodeURIComponent(examId)}/${value}`)} />
      </nav>

      {data.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} />)}

      {section === "overview" ? (
        <main className="exam-overview">
          <section className="exam-facts">
            <Statistic title="学生范围" value={data.exam.class_ids.length} suffix="个班级" />
            <Statistic title="试卷版本" value={data.paperCount} />
            <Statistic title="题目" value={data.questionCount} />
            <Statistic title="已采集答卷" value={data.submissionCount} />
          </section>
          <div className="exam-overview-grid">
            <section className="workspace-section readiness-section">
              <div className="section-head"><div><h2>当前下一步</h2><p>按考试状态和准备情况生成</p></div></div>
              {readyIssues.length ? <div className="readiness-list">{readyIssues.map((issue) => <button key={issue.code} onClick={() => onNavigate(`/exams/${encodeURIComponent(examId)}/${issue.section}`)}><CircleAlert size={17} /><span>{issue.label}：{issue.message}</span><ArrowRight size={16} /></button>)}</div> : <div className="ready-line"><CheckCircle2 size={18} /><span>{data.readiness?.confirmed ? "开考准备已经确认" : "基础配置已通过，可以进行开考确认"}</span></div>}
            </section>
            <section className="workspace-section risk-section">
              <div className="section-head"><div><h2>需要关注</h2><p>需要人工确认的业务事项</p></div><strong>{data.reviewRequiredCount}</strong></div>
              {data.reviewRequiredCount ? <Alert type="warning" showIcon message={`${data.reviewRequiredCount} 份答卷需要确认学生或质量状态`} action={<Button size="small" onClick={() => onNavigate(`/exams/${encodeURIComponent(examId)}/processing`)}>处理</Button>} /> : <div className="ready-line"><CheckCircle2 size={18} /><span>当前没有待人工处理的答卷风险</span></div>}
            </section>
          </div>
        </main>
      ) : moduleContent ? (
        <main className="exam-embedded-module">{moduleContent}</main>
      ) : (
        <main className="workspace-section workspace-section-landing">
          <div><span className="dashboard-kicker">{sections.find(([key]) => key === section)?.[1] ?? "考试工作区"}</span><h2>{destination?.action ?? "返回考试概览"}</h2><p>{destination?.description ?? "当前环节将在后续 Story 完成纵向业务整合。"}</p></div>
          {destination ? <Button type="primary" icon={<ArrowRight size={16} />} onClick={() => onNavigate(destination.path)}>{destination.action}</Button> : <Button onClick={() => onNavigate(`/exams/${encodeURIComponent(examId)}/overview`)}>返回概览</Button>}
        </main>
      )}
    </div>
  );
}
