import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { Alert, App, Button, Select, Space, Table, Tag, type TableColumnsType } from "antd";
import { Download, FileWarning, RefreshCw, ShieldCheck } from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis
} from "recharts";
import { ApiClientError } from "../api/client";
import { listExams, type Exam } from "../api/exams";
import {
  exportLearningReport,
  getGradingQualityReport,
  getReportOverview,
  listClassReports,
  listQuestionReports,
  type ClassComparison,
  type ClassReport,
  type ErrorClue,
  type GradingQualityReport,
  type KnowledgeMastery,
  type OverviewReport,
  type QuestionAnalysis,
  type ReportMetric
} from "../api/reports";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";

interface LearningReportsPageProps {
  canRead: boolean;
  canExport: boolean;
  initialExamId?: string;
}

interface KnowledgeRow {
  knowledge_point: string;
  mastery_rate: number;
  question_count: number;
  score: number;
  max_score: number;
}

interface ErrorRow {
  key: string;
  question_no: string;
  source: string;
  text: string;
  count: number;
  score_rate?: number;
}

const chartBlue = "#1677ff";
const chartGreen = "#52c41a";
const chartAmber = "#faad14";
const chartRed = "#ff4d4f";
const chartCyan = "#13c2c2";

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    return `${error.status} ${error.code}: ${error.message}`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "未知错误";
}

function formatScore(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return "-";
  }
  return Number(value.toFixed(2)).toString();
}

function formatPercent(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return "-";
  }
  return `${Number((value * 100).toFixed(1)).toString()}%`;
}

function percentValue(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return 0;
  }
  return Number((value * 100).toFixed(1));
}

function formatMetric(metric?: ReportMetric, mode: "percent" | "score" = "percent") {
  if (!metric?.available) {
    return "不可用";
  }
  return mode === "percent" ? formatPercent(metric.value) : formatScore(metric.value);
}

function metricDetail(metric?: ReportMetric) {
  if (!metric?.available) {
    return metric?.reason ? `来源不足：${metric.reason}` : "来源数据不足";
  }
  if (metric.denominator) {
    return `${metric.numerator ?? 0} / ${metric.denominator}`;
  }
  return "真实统计";
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function emptyReason(reason?: string) {
  if (reason === "no_published_grades") {
    return "当前考试尚无已发布且锁定的成绩，报告 API 返回空状态。";
  }
  return reason ? `报告 API 返回空状态：${reason}` : "当前考试暂无可展示报告数据。";
}

function aggregateKnowledge(classes: ClassReport[]): KnowledgeRow[] {
  const grouped = new Map<string, KnowledgeRow>();
  for (const classReport of classes) {
    for (const item of classReport.weak_knowledge_points ?? []) {
      const current = grouped.get(item.knowledge_point) ?? {
        knowledge_point: item.knowledge_point,
        mastery_rate: 0,
        question_count: 0,
        score: 0,
        max_score: 0
      };
      current.score += item.score;
      current.max_score += item.max_score;
      current.question_count += item.question_count;
      current.mastery_rate = current.max_score > 0 ? current.score / current.max_score : item.mastery_rate;
      grouped.set(item.knowledge_point, current);
    }
  }
  return Array.from(grouped.values())
    .sort((a, b) => a.mastery_rate - b.mastery_rate)
    .slice(0, 10);
}

function flattenErrors(questions: QuestionAnalysis[], classes: ClassReport[]): ErrorRow[] {
  const fromQuestions = questions.flatMap((question) =>
    (question.frequent_errors ?? []).map((item: ErrorClue, index) => ({
      key: `${question.question_id}-${item.text}-${index}`,
      question_no: item.question_no || question.question_no,
      source: item.source,
      text: item.text,
      count: item.count ?? 1,
      score_rate: question.score_rate
    }))
  );
  if (fromQuestions.length > 0) {
    return fromQuestions.sort((a, b) => b.count - a.count).slice(0, 8);
  }
  return classes
    .flatMap((classReport) =>
      (classReport.frequent_wrong_questions ?? []).map((item, index) => ({
        key: `${classReport.class_id}-${item.question_id}-${index}`,
        question_no: item.question_no,
        source: classReport.class_name,
        text: `班级高频错题，得分率 ${formatPercent(item.score_rate)}`,
        count: item.wrong_count,
        score_rate: item.score_rate
      }))
    )
    .sort((a, b) => b.count - a.count)
    .slice(0, 8);
}

function ChartPanel({
  title,
  description,
  empty,
  emptyTitle,
  children,
  className = ""
}: {
  title: string;
  description: string;
  empty: boolean;
  emptyTitle: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`reports-panel ${className}`}>
      <div className="panel-head">
        <div>
          <h2>{title}</h2>
          <p>{description}</p>
        </div>
      </div>
      {empty ? <EmptyState title={emptyTitle} description="当前报告 API 没有返回可绘制的数据。" /> : <div className="reports-chart">{children}</div>}
    </section>
  );
}

export function LearningReportsPage({ canRead, canExport, initialExamId = "" }: LearningReportsPageProps) {
  const { message, modal } = App.useApp();
  const hasSession = true;
  const [exams, setExams] = useState<Exam[]>([]);
  const [selectedExamId, setSelectedExamId] = useState(initialExamId);
  const [overview, setOverview] = useState<OverviewReport | null>(null);
  const [classReports, setClassReports] = useState<ClassReport[]>([]);
  const [questions, setQuestions] = useState<QuestionAnalysis[]>([]);
  const [quality, setQuality] = useState<GradingQualityReport | null>(null);
  const [loadingExams, setLoadingExams] = useState(true);
  const [loadingReports, setLoadingReports] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  const [lastExport, setLastExport] = useState<{ filename?: string; watermark?: string } | null>(null);

  const selectedExam = useMemo(() => exams.find((exam) => exam.id === selectedExamId), [exams, selectedExamId]);
  const isEmptyReport = Boolean(overview?.empty?.empty);
  const canExportReport = canRead && canExport && hasSession && Boolean(selectedExamId) && Boolean(overview) && !isEmptyReport;

  const classComparisonData = useMemo(() => {
    const source: ClassComparison[] =
      overview?.class_comparisons?.length
        ? overview.class_comparisons
        : classReports.map((item) => ({
            class_id: item.class_id,
            class_name: item.class_name,
            student_count: item.student_count,
            average: item.stats.average,
            median: item.stats.median,
            pass_rate: item.stats.pass_rate,
            excellent_rate: item.stats.excellent_rate
          }));
    return source.map((item) => ({
      name: item.class_name || item.class_id,
      average: item.average,
      median: item.median,
      passRate: percentValue(item.pass_rate),
      excellentRate: percentValue(item.excellent_rate),
      studentCount: item.student_count
    }));
  }, [classReports, overview]);

  const distributionData = useMemo(
    () =>
      (overview?.stats.distribution ?? []).map((item) => ({
        name: item.label,
        count: item.count,
        range: `${item.min}-${item.max}`
      })),
    [overview]
  );

  const questionScoreData = useMemo(
    () =>
      questions.map((item) => ({
        name: item.question_no,
        scoreRate: percentValue(item.score_rate),
        correctRate: percentValue(item.correct_rate),
        difficulty: percentValue(item.difficulty),
        discrimination: percentValue(item.discrimination)
      })),
    [questions]
  );

  const knowledgeData = useMemo(() => aggregateKnowledge(classReports), [classReports]);
  const errorRows = useMemo(() => flattenErrors(questions, classReports), [classReports, questions]);
  const objectiveQuestions = useMemo(() => questions.filter((item) => (item.option_distribution?.length ?? 0) > 0).slice(0, 4), [questions]);

  const qualityData = useMemo(() => {
    if (!quality) {
      return [];
    }
    return [
      { name: "AI采纳", value: quality.ai_adoption_rate.available ? percentValue(quality.ai_adoption_rate.value) : 0, available: quality.ai_adoption_rate.available },
      {
        name: "人工改分",
        value: quality.human_modification_rate.available ? percentValue(quality.human_modification_rate.value) : 0,
        available: quality.human_modification_rate.available
      },
      { name: "OCR失败", value: quality.ocr_failure_rate.available ? percentValue(quality.ocr_failure_rate.value) : 0, available: quality.ocr_failure_rate.available }
    ].filter((item) => item.available);
  }, [quality]);

  const questionColumns = useMemo<TableColumnsType<QuestionAnalysis>>(
    () => [
      { title: "题号", dataIndex: "question_no", width: 80 },
      { title: "题型", dataIndex: "question_type", width: 110 },
      { title: "得分率", dataIndex: "score_rate", width: 90, render: (value: number) => formatPercent(value) },
      { title: "难度", dataIndex: "difficulty", width: 90, render: (value: number) => formatPercent(value) },
      { title: "区分度", dataIndex: "discrimination", width: 90, render: (value: number) => formatPercent(value) },
      {
        title: "知识点",
        dataIndex: "knowledge_points",
        render: (value: string[]) => (value?.length ? value.map((item) => <Tag key={item}>{item}</Tag>) : <span className="muted">未返回</span>)
      }
    ],
    []
  );

  const classColumns = useMemo<TableColumnsType<ClassReport>>(
    () => [
      { title: "班级", dataIndex: "class_name" },
      { title: "人数", dataIndex: "student_count", width: 80 },
      { title: "平均分", dataIndex: ["stats", "average"], width: 90, render: (value: number) => formatScore(value) },
      { title: "中位数", dataIndex: ["stats", "median"], width: 90, render: (value: number) => formatScore(value) },
      { title: "及格率", dataIndex: ["stats", "pass_rate"], width: 90, render: (value: number) => formatPercent(value) },
      { title: "优秀率", dataIndex: ["stats", "excellent_rate"], width: 90, render: (value: number) => formatPercent(value) }
    ],
    []
  );

  const errorColumns = useMemo<TableColumnsType<ErrorRow>>(
    () => [
      { title: "题号", dataIndex: "question_no", width: 80 },
      { title: "来源", dataIndex: "source", width: 110 },
      { title: "错误线索", dataIndex: "text" },
      { title: "次数", dataIndex: "count", width: 80 },
      { title: "得分率", dataIndex: "score_rate", width: 90, render: (value?: number) => formatPercent(value) }
    ],
    []
  );

  const loadExamList = useCallback(async () => {
    setLoadingExams(true);
    setError(null);
    if (!hasSession) {
      setExams([]);
      setSelectedExamId("");
      setError("当前没有有效登录会话，无法调用真实后端 API。");
      setLoadingExams(false);
      return;
    }
    try {
      const result = await listExams();
      setExams(result.exams);
      setSelectedExamId((current) => current || result.exams[0]?.id || "");
    } catch (currentError) {
      setError(formatError(currentError));
    } finally {
      setLoadingExams(false);
    }
  }, [hasSession]);

  const loadReports = useCallback(
    async (examId: string) => {
      if (!examId || !hasSession) {
        setOverview(null);
        setClassReports([]);
        setQuestions([]);
        setQuality(null);
        return;
      }
      setLoadingReports(true);
      setError(null);
      try {
        const [overviewResult, classResult, questionResult, qualityResult] = await Promise.all([
          getReportOverview(examId),
          listClassReports(examId),
          listQuestionReports(examId),
          getGradingQualityReport(examId)
        ]);
        setOverview(overviewResult.overview);
        setClassReports(classResult.classes);
        setQuestions(questionResult.questions);
        setQuality(qualityResult.grading_quality);
      } catch (currentError) {
        setError(formatError(currentError));
        setOverview(null);
        setClassReports([]);
        setQuestions([]);
        setQuality(null);
      } finally {
        setLoadingReports(false);
      }
    },
    [hasSession]
  );

  useEffect(() => {
    void loadExamList();
  }, [loadExamList]);

  useEffect(() => {
    void loadReports(selectedExamId);
  }, [loadReports, selectedExamId]);

  const refresh = async () => {
    await loadExamList();
    if (selectedExamId) {
      await loadReports(selectedExamId);
    }
  };

  const exportReport = () => {
    if (!selectedExamId) {
      message.error("请先选择考试");
      return;
    }
    modal.confirm({
      title: "导出学情报告",
      content: "导出会写 report.exported 审计，并在 CSV 与响应头中包含水印。请确认当前报告数据可发布给授权人员。",
      okText: "确认导出",
      cancelText: "取消",
      onOk: async () => {
        setExporting(true);
        try {
          const result = await exportLearningReport(selectedExamId);
          const filename = result.filename ?? `exam-${selectedExamId}-report.csv`;
          setLastExport({ filename, watermark: result.watermark });
          saveBlob(result.blob, filename);
          message.success("学情报告 CSV 已导出");
        } catch (currentError) {
          message.error(formatError(currentError));
        } finally {
          setExporting(false);
        }
      }
    });
  };

  const stats = overview?.stats;
  const kpis = [
    { label: "平均分", value: formatScore(stats?.average) },
    { label: "中位数", value: formatScore(stats?.median) },
    { label: "最高分", value: formatScore(stats?.highest) },
    { label: "最低分", value: formatScore(stats?.lowest) },
    { label: "标准差", value: formatScore(stats?.stddev) },
    { label: "及格率", value: formatPercent(stats?.pass_rate) },
    { label: "优秀率", value: formatPercent(stats?.excellent_rate) }
  ];

  return (
    <div className="reports-shell">
      <section className="reports-topbar">
        <div>
          <Space align="center" wrap>
            <h1>学情报告</h1>
          </Space>
          <p>查看已发布成绩形成的考试质量、班级、题目和阅卷质量分析。</p>
        </div>
        <Space wrap>
          <Select
            className="reports-exam-select"
            placeholder="选择考试"
            value={selectedExamId || undefined}
            options={exams.map((exam) => ({ value: exam.id, label: `${exam.name} · ${exam.subject}` }))}
            loading={loadingExams}
            onChange={setSelectedExamId}
          />
          <Button icon={<RefreshCw size={16} />} onClick={refresh} loading={loadingExams || loadingReports}>
            刷新
          </Button>
          <Button type="primary" icon={<Download size={16} />} disabled={!canExportReport} loading={exporting} onClick={exportReport}>
            导出 CSV
          </Button>
        </Space>
      </section>

      {!hasSession ? (
        <Alert
          type="warning"
          showIcon
          message="未检测到真实后端访问令牌"
            description="基于已发布成绩查看考试、班级和题目分析。"
        />
      ) : null}

      {!canRead ? <Alert type="error" showIcon message="无报告读取权限" description="当前账号缺少 report:read，不能读取学情报告。" /> : null}
      {error ? <ErrorState message={error} onRetry={refresh} /> : null}
      {overview?.empty?.empty ? <Alert type="info" showIcon message="报告为空" description={emptyReason(overview.empty.reason)} /> : null}

      <section className="reports-kpi-strip">
        {kpis.map((item) => (
          <div key={item.label}>
            <span>{item.label}</span>
            <strong>{item.value}</strong>
          </div>
        ))}
      </section>

      {loadingReports ? (
        <LoadingState label="正在读取学情报告" />
      ) : (
        <section className="reports-grid">
          <ChartPanel className="reports-wide" title="分数分布" description={selectedExam ? selectedExam.name : "未选择考试"} empty={distributionData.length === 0} emptyTitle="暂无分数分布">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={distributionData}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="name" />
                <YAxis allowDecimals={false} />
                <ChartTooltip />
                <Bar dataKey="count" name="人数" fill={chartBlue} radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </ChartPanel>

          <ChartPanel title="班级对比" description="平均分、及格率与优秀率" empty={classComparisonData.length === 0} emptyTitle="暂无班级对比">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={classComparisonData}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="name" />
                <YAxis yAxisId="score" />
                <YAxis yAxisId="rate" orientation="right" />
                <ChartTooltip />
                <Bar yAxisId="score" dataKey="average" name="平均分" fill={chartBlue} radius={[4, 4, 0, 0]} />
                <Bar yAxisId="rate" dataKey="passRate" name="及格率%" fill={chartGreen} radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </ChartPanel>

          <ChartPanel title="题目得分率" description="按题号展示得分率与正确率" empty={questionScoreData.length === 0} emptyTitle="暂无题目得分率">
            <ResponsiveContainer width="100%" height={260}>
              <LineChart data={questionScoreData}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="name" />
                <YAxis domain={[0, 100]} />
                <ChartTooltip />
                <Line type="monotone" dataKey="scoreRate" name="得分率%" stroke={chartBlue} strokeWidth={2} dot={{ r: 3 }} />
                <Line type="monotone" dataKey="correctRate" name="正确率%" stroke={chartGreen} strokeWidth={2} dot={{ r: 3 }} />
              </LineChart>
            </ResponsiveContainer>
          </ChartPanel>

          <ChartPanel title="知识点掌握率" description="来自班级报告的薄弱知识点真实聚合" empty={knowledgeData.length === 0} emptyTitle="暂无知识点数据">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={knowledgeData.map((item) => ({ name: item.knowledge_point, mastery: percentValue(item.mastery_rate), questions: item.question_count }))}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="name" interval={0} tick={{ fontSize: 11 }} />
                <YAxis domain={[0, 100]} />
                <ChartTooltip />
                <Bar dataKey="mastery" name="掌握率%" fill={chartCyan} radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </ChartPanel>

          <section className="reports-panel reports-wide">
            <div className="panel-head">
              <div>
                <h2>阅卷质量分析</h2>
                <p>只展示后端标记为 available 的真实质量指标。</p>
              </div>
            </div>
            {!quality ? (
              <EmptyState title="暂无阅卷质量" description="当前报告 API 没有返回阅卷质量数据。" />
            ) : (
              <>
                <div className="reports-quality-strip">
                  <div>
                    <span>AI 采纳率</span>
                    <strong>{formatMetric(quality.ai_adoption_rate)}</strong>
                    <small>{metricDetail(quality.ai_adoption_rate)}</small>
                  </div>
                  <div>
                    <span>人工改分率</span>
                    <strong>{formatMetric(quality.human_modification_rate)}</strong>
                    <small>{metricDetail(quality.human_modification_rate)}</small>
                  </div>
                  <div>
                    <span>平均双评分差</span>
                    <strong>{formatMetric(quality.average_double_mark_diff, "score")}</strong>
                    <small>{metricDetail(quality.average_double_mark_diff)}</small>
                  </div>
                  <div>
                    <span>仲裁数量</span>
                    <strong>{quality.arbitration_count}</strong>
                    <small>{quality.double_mark_session_count} 个双评会话</small>
                  </div>
                  <div>
                    <span>OCR 失败率</span>
                    <strong>{formatMetric(quality.ocr_failure_rate)}</strong>
                    <small>{metricDetail(quality.ocr_failure_rate)}</small>
                  </div>
                </div>
                {qualityData.length === 0 ? (
                  <EmptyState title="暂无可绘制质量指标" description="质量指标存在但来源不足，后端返回 available=false。" />
                ) : (
                  <div className="reports-chart compact">
                    <ResponsiveContainer width="100%" height={180}>
                      <BarChart data={qualityData}>
                        <CartesianGrid strokeDasharray="3 3" vertical={false} />
                        <XAxis dataKey="name" />
                        <YAxis domain={[0, 100]} />
                        <ChartTooltip />
                        <Bar dataKey="value" name="比例%" fill={chartAmber} radius={[4, 4, 0, 0]} />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </>
            )}
          </section>

          <ChartPanel title="题目质量分析" description="难度与区分度来自真实题目分析 API" empty={questionScoreData.length === 0} emptyTitle="暂无题目质量">
            <ResponsiveContainer width="100%" height={260}>
              <LineChart data={questionScoreData}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="name" />
                <YAxis domain={[0, 100]} />
                <ChartTooltip />
                <Line type="monotone" dataKey="difficulty" name="难度%" stroke={chartAmber} strokeWidth={2} dot={{ r: 3 }} />
                <Line type="monotone" dataKey="discrimination" name="区分度%" stroke={chartRed} strokeWidth={2} dot={{ r: 3 }} />
              </LineChart>
            </ResponsiveContainer>
          </ChartPanel>

          <section className="reports-panel">
            <div className="panel-head">
              <div>
                <h2>客观题选项分布</h2>
                <p>只展示真实 answer payload 中能解析出的选项。</p>
              </div>
            </div>
            {objectiveQuestions.length === 0 ? (
              <EmptyState title="暂无选项分布" description="报告 API 未返回客观题选项分布时不生成示例图。" />
            ) : (
              <div className="reports-option-list">
                {objectiveQuestions.map((question) => {
                  const total = (question.option_distribution ?? []).reduce((sum, item) => sum + item.count, 0);
                  return (
                    <div className="reports-option-block" key={question.question_id}>
                      <div>
                        <strong>{question.question_no}</strong>
                        <span>{question.question_type}</span>
                      </div>
                      {(question.option_distribution ?? []).map((item) => (
                        <div className="reports-option-row" key={`${question.question_id}-${item.option}`}>
                          <span>{item.option}</span>
                          <div>
                            <i style={{ width: `${total > 0 ? (item.count / total) * 100 : 0}%` }} />
                          </div>
                          <em>{item.count}</em>
                        </div>
                      ))}
                    </div>
                  );
                })}
              </div>
            )}
          </section>

          <section className="reports-panel reports-wide">
            <div className="panel-head">
              <div>
                <h2>题目明细</h2>
                <p>{questions.length} 道题目分析</p>
              </div>
            </div>
            <Table
              rowKey="question_id"
              size="small"
              columns={questionColumns}
              dataSource={questions}
              pagination={{ pageSize: 6 }}
              scroll={{ x: 760 }}
              locale={{ emptyText: <EmptyState title="暂无题目明细" description="当前报告 API 没有返回题目分析。" /> }}
            />
          </section>

          <section className="reports-panel">
            <div className="panel-head">
              <div>
                <h2>班级明细</h2>
                <p>{classReports.length} 个班级报告</p>
              </div>
            </div>
            <Table
              rowKey="class_id"
              size="small"
              columns={classColumns}
              dataSource={classReports}
              pagination={false}
              scroll={{ x: 620 }}
              locale={{ emptyText: <EmptyState title="暂无班级明细" description="当前报告 API 没有返回班级报告。" /> }}
            />
          </section>

          <section className="reports-panel">
            <div className="panel-head">
              <div>
                <h2>高频错误</h2>
                <p>来自题目错误线索或班级高频错题。</p>
              </div>
            </div>
            <Table
              rowKey="key"
              size="small"
              columns={errorColumns}
              dataSource={errorRows}
              pagination={false}
              scroll={{ x: 640 }}
              locale={{ emptyText: <EmptyState title="暂无高频错误" description="当前报告 API 没有返回错误线索或高频错题。" /> }}
            />
          </section>

          <section className="reports-panel reports-export-panel">
            <div className="panel-head">
              <div>
                <h2>导出审计</h2>
                <p>报告导出会写入审计并返回水印。</p>
              </div>
              <ShieldCheck size={20} />
            </div>
            <Alert
              type="info"
              showIcon
              icon={<FileWarning size={18} />}
              message="导出报告"
              description={
                lastExport?.watermark
                  ? `最近导出：${lastExport.filename ?? "report.csv"}；水印：${lastExport.watermark}`
                  : "导出使用真实 report export API，响应头返回 X-EduGrade-Watermark。"
              }
            />
          </section>
        </section>
      )}
    </div>
  );
}
