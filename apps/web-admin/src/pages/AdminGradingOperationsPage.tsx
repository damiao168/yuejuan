import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Input, Progress, Space } from "antd";
import { ArrowRight, CircleAlert, RefreshCw, Search, ShieldCheck } from "lucide-react";
import { ApiClientError, getUserErrorMessage } from "../api/client";
import { listExams, type Exam } from "../api/exams";
import { getScoringSummary, type ScoringSummary } from "../api/review";
import { getAIGradingStatus, type AIGradingRuntimeStatus } from "../api/system";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import { examStatusLabels, examStatusTone, examSubjectLabel } from "../constants/examStatus";

interface ExamOperation {
  exam: Exam;
  summary?: ScoringSummary;
}

const runStatusLabels: Record<string, string> = {
  queued: "排队等待",
  processing: "评分进行中",
  running: "评分进行中",
  needs_review: "待人工复核",
  completed: "评分完成",
  failed: "评分有失败项",
  cancelling: "正在取消",
  cancelled: "已取消"
};

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    console.warn("阅卷请求失败", error.status, error.code);
    return getUserErrorMessage(error, "操作失败，请稍后重试");
  }
  return getUserErrorMessage(error, "阅卷数据加载失败");
}

function completion(summary?: ScoringSummary) {
  const run = summary?.run;
  if (!run || run.total_count <= 0) return 0;
  return Math.round(((run.auto_confirmed_count + run.human_confirmed_count) / run.total_count) * 100);
}

export function AdminGradingOperationsPage({ onNavigate }: { onNavigate: (path: string) => void }) {
  const [operations, setOperations] = useState<ExamOperation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [warnings, setWarnings] = useState<string[]>([]);
  const [keyword, setKeyword] = useState("");
  const [aiStatus, setAIStatus] = useState<AIGradingRuntimeStatus>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    setWarnings([]);
    try {
      const examResult = await listExams();
      const aiResult = await getAIGradingStatus().catch(() => undefined);
      setAIStatus(aiResult?.ai_grading);
      const activeExams = examResult.exams.filter((exam) => !["archived", "published"].includes(exam.status));
      const summaries = await Promise.allSettled(activeExams.map((exam) => getScoringSummary(exam.id)));
      setOperations(activeExams.map((exam, index) => ({
        exam,
        summary: summaries[index].status === "fulfilled" ? summaries[index].value.scoring_summary : undefined
      })));
      const unavailable = summaries.filter((result) => result.status === "rejected" && !(result.reason instanceof ApiClientError && result.reason.status === 404)).length;
      if (unavailable > 0) setWarnings([`${unavailable} 场考试的评分摘要暂时不可用`]);
    } catch (loadError) {
      setError(formatError(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const query = keyword.trim().toLowerCase();
    return operations.filter(({ exam }) => !query
      || exam.name.toLowerCase().includes(query)
      || examSubjectLabel(exam.subject).toLowerCase().includes(query));
  }, [keyword, operations]);

  const metrics = useMemo(() => operations.reduce((result, item) => {
    const run = item.summary?.run;
    if (["grading", "reviewing"].includes(item.exam.status)) result.active += 1;
    result.queued += run?.queued_count ?? 0;
    result.review += run?.review_count ?? 0;
    result.failed += run?.failed_count ?? 0;
    return result;
  }, { active: 0, queued: 0, review: 0, failed: 0 }), [operations]);

  if (!operations.length && loading) return <LoadingState label="正在加载阅卷状态" />;
  if (!operations.length && error) return <ErrorState message={error} onRetry={() => void load()} />;

  return (
    <div className="page-stack admin-grading-operations">
      <section className="page-heading">
        <div>
          <h1>阅卷</h1>
          <p>查看正在进行的考试和需要人工处理的评分任务。</p>
        </div>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load()}>刷新</Button>
      </section>

      {error ? <Alert type="error" showIcon message="部分数据刷新失败" description={error} /> : null}
      {warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} />)}
      {aiStatus && (!aiStatus.available || aiStatus.mode === "mock") ? (
        <Alert
          type={aiStatus.mode === "mock" ? "warning" : "info"}
          showIcon
          message={aiStatus.mode === "mock" ? "当前为 Mock 演示模式" : "AI 阅卷未启用"}
          description={aiStatus.mode === "mock"
            ? "结果仅用于验证流程，不能作为正式评分结论；所有主观题仍需人工确认。"
            : "主观题不会生成 AI 建议；客观题规则评分和人工阅卷不受影响。"}
        />
      ) : null}

      <section className="operations-metrics" aria-label="阅卷摘要">
        <div><span>阅卷中考试</span><strong>{metrics.active}</strong></div>
        <div><span>等待处理</span><strong>{metrics.queued}</strong></div>
        <div><span>待人工处理</span><strong>{metrics.review}</strong></div>
        <div className={metrics.failed ? "danger" : ""}><span>处理失败</span><strong>{metrics.failed}</strong></div>
      </section>

      <section className="operations-filter">
        <Input prefix={<Search size={16} />} value={keyword} allowClear placeholder="搜索考试或学科" onChange={(event) => setKeyword(event.target.value)} />
        <span>{filtered.length} 场进行中考试</span>
      </section>

      <section className="operations-list" aria-label="考试阅卷列表">
        {filtered.length ? filtered.map(({ exam, summary }) => {
          const run = summary?.run;
          const progress = completion(summary);
          const needsAttention = (run?.review_count ?? 0) + (run?.failed_count ?? 0);
          return (
            <article className="operations-row" key={exam.id}>
              <div className="operations-exam">
                <Space size="small" wrap>
                  <StatusTag tone={examStatusTone(exam.status)}>{examStatusLabels[exam.status] ?? "进行中"}</StatusTag>
                  <span title={exam.subject}>{examSubjectLabel(exam.subject)}</span>
                </Space>
                <h2>{exam.name}</h2>
                <span title={run?.status}>{exam.class_ids.length} 个班级 · {run ? `自动评分：${runStatusLabels[run.status] ?? "进行中"}` : "尚未开始自动评分"}</span>
              </div>
              <div className="operations-progress">
                <div><span>已确认进度</span><strong>{progress}%</strong></div>
                <Progress percent={progress} showInfo={false} status={(run?.failed_count ?? 0) > 0 ? "exception" : "normal"} />
                <span>{run ? `${run.auto_confirmed_count + run.human_confirmed_count} / ${run.total_count} 项` : "进入考试工作区配置并启动"}</span>
              </div>
              <div className="operations-counts">
                <span><strong>{run?.queued_count ?? 0}</strong>排队</span>
                <span><strong>{run?.review_count ?? 0}</strong>复核</span>
                <span className={(run?.failed_count ?? 0) > 0 ? "danger" : ""}><strong>{run?.failed_count ?? 0}</strong>失败</span>
              </div>
              <div className="operations-actions">
                {needsAttention > 0 ? <span className="operations-warning"><CircleAlert size={15} />{needsAttention} 项需处理</span> : <span className="operations-ok"><ShieldCheck size={15} />无阻断项</span>}
                <Button onClick={() => onNavigate(`/exams/${encodeURIComponent(exam.id)}/grading`)}>{needsAttention > 0 ? "处理异常" : run ? "查看进度" : "进入考试"}<ArrowRight size={16} /></Button>
              </div>
            </article>
          );
        }) : <EmptyState title="没有匹配的考试" description="进行中的考试及其自动评分进度会显示在这里。" />}
      </section>
    </div>
  );
}
