import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Progress, Space, TableColumnsType } from "antd";
import { motion } from "framer-motion";
import { ArrowRight, RefreshCw } from "lucide-react";
import { listAuditLogs, type AuditLog } from "../api/audit";
import { listExams, type Exam } from "../api/exams";
import { listSubmissions, type Submission } from "../api/submissions";
import { getSystemStatus, type SystemStatus } from "../api/system";
import { DataTable } from "../components/DataTable";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import type { StatusTone } from "../types";

interface DashboardData {
  exams: Exam[];
  submissions: Array<{ submission: Submission; exam: Exam }>;
  submissionExamIds: string[];
  auditLogs: AuditLog[];
  systemStatus?: SystemStatus;
  warnings: string[];
}

interface MetricTile {
  label: string;
  value: string;
  trend: string;
  status: StatusTone;
}

interface ExamRow {
  id: string;
  name: string;
  subject: string;
  status: string;
  submissions: string;
  progress: number;
  mode: string;
}

interface SubmissionRow {
  id: string;
  exam: string;
  candidate: string;
  status: string;
  pages: string;
  quality: string;
  createdAt: string;
}

interface AuditRow {
  id: string;
  time: string;
  actor: string;
  action: string;
  target: string;
  result: string;
}

const activeExamStatuses = new Set(["configured", "collecting", "grading", "reviewing"]);

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "请求失败";
}

function formatDate(value?: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function createdAtTime(exam: Exam): number {
  return exam.created_at ? new Date(exam.created_at).getTime() || 0 : 0;
}

function statusTone(status: string): StatusTone {
  const normalized = status.toLowerCase();
  if (["published", "finalized", "ok", "completed", "ready"].includes(normalized)) {
    return "success";
  }
  if (normalized.includes("failed") || normalized.includes("error") || normalized.includes("rejected")) {
    return "danger";
  }
  if (normalized.includes("reviewing") || normalized.includes("pending") || normalized.includes("not_configured")) {
    return "warning";
  }
  if (normalized.includes("collecting") || normalized.includes("grading") || normalized.includes("running") || normalized.includes("processing")) {
    return "processing";
  }
  return "info";
}

function examProgress(status: string): number {
  const progressByStatus: Record<string, number> = {
    draft: 10,
    configured: 25,
    collecting: 45,
    grading: 65,
    reviewing: 82,
    finalized: 92,
    published: 100,
    archived: 100
  };
  return progressByStatus[status] ?? 0;
}

async function fetchDashboardData(): Promise<DashboardData> {
  const [examResult, auditResult, statusResult] = await Promise.allSettled([
    listExams(),
    listAuditLogs({ limit: 5 }),
    getSystemStatus()
  ]);

  if (examResult.status === "rejected") {
    throw new Error(`考试列表加载失败：${errorMessage(examResult.reason)}`);
  }

  const warnings: string[] = [];
  const exams = examResult.value.exams;
  const recentExams = [...exams].sort((left, right) => createdAtTime(right) - createdAtTime(left)).slice(0, 5);
  const auditLogs = auditResult.status === "fulfilled" ? auditResult.value.audit_logs : [];
  const systemStatus = statusResult.status === "fulfilled" ? statusResult.value : undefined;

  if (auditResult.status === "rejected") {
    warnings.push(`审计日志不可用：${errorMessage(auditResult.reason)}`);
  }
  if (statusResult.status === "rejected") {
    warnings.push(`系统状态不可用：${errorMessage(statusResult.reason)}`);
  }

  const submissionResults = await Promise.allSettled(
    recentExams.map(async (exam) => ({
      exam,
      submissions: (await listSubmissions(exam.id)).submissions
    }))
  );

  const submissions = submissionResults.flatMap((result) => {
    if (result.status === "fulfilled") {
      return result.value.submissions.map((submission) => ({ submission, exam: result.value.exam }));
    }
    warnings.push(`最近答卷不可用：${errorMessage(result.reason)}`);
    return [];
  });

  return {
    exams,
    submissions,
    submissionExamIds: recentExams.map((exam) => exam.id),
    auditLogs,
    systemStatus,
    warnings
  };
}

export function DashboardPage() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState<string>();
  const [refreshNonce, setRefreshNonce] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(undefined);
    fetchDashboardData()
      .then((nextData) => {
        if (active) {
          setData(nextData);
        }
      })
      .catch((loadError) => {
        if (active) {
          setError(errorMessage(loadError));
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [refreshNonce]);

  const metrics = useMemo<MetricTile[]>(() => {
    const exams = data?.exams ?? [];
    const submissions = data?.submissions ?? [];
    const dependencyErrors = data?.systemStatus?.dependencies.filter((dependency) => dependency.status !== "ok").length ?? 0;
    const failedSubmissions = submissions.filter(({ submission }) => submission.status.includes("failed") || submission.quality_status.includes("failed")).length;

    return [
      {
        label: "考试总数",
        value: String(exams.length),
        trend: `${exams.filter((exam) => activeExamStatuses.has(exam.status)).length} 场进行中`,
        status: "processing"
      },
      {
        label: "已发布考试",
        value: String(exams.filter((exam) => exam.status === "published").length),
        trend: `${exams.filter((exam) => exam.status === "finalized").length} 场待发布`,
        status: "success"
      },
      {
        label: "最近答卷",
        value: String(submissions.length),
        trend: failedSubmissions > 0 ? `${failedSubmissions} 份异常` : "无异常记录",
        status: failedSubmissions > 0 ? "warning" : "info"
      },
      {
        label: "依赖异常",
        value: String(dependencyErrors),
        trend: data?.systemStatus ? data.systemStatus.status : "未获取",
        status: dependencyErrors > 0 ? "danger" : "success"
      }
    ];
  }, [data]);

  const examSubmissionCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const { submission } of data?.submissions ?? []) {
      counts.set(submission.exam_id, (counts.get(submission.exam_id) ?? 0) + 1);
    }
    return counts;
  }, [data?.submissions]);

  const loadedSubmissionExamIds = useMemo(() => new Set(data?.submissionExamIds ?? []), [data?.submissionExamIds]);

  const examRows = useMemo<ExamRow[]>(
    () =>
      (data?.exams ?? []).map((exam) => ({
        id: exam.id,
        name: exam.name,
        subject: exam.subject,
        status: exam.status,
        submissions: loadedSubmissionExamIds.has(exam.id) ? String(examSubmissionCounts.get(exam.id) ?? 0) : "未加载",
        progress: examProgress(exam.status),
        mode: exam.grading_mode
      })),
    [data?.exams, examSubmissionCounts, loadedSubmissionExamIds]
  );

  const submissionRows = useMemo<SubmissionRow[]>(
    () =>
      (data?.submissions ?? []).map(({ submission, exam }) => ({
        id: submission.id,
        exam: exam.name,
        candidate: submission.candidate_no || submission.student_id || "-",
        status: submission.status,
        pages: `${submission.actual_page_count}/${submission.expected_page_count}`,
        quality: submission.quality_status,
        createdAt: formatDate(submission.created_at)
      })),
    [data?.submissions]
  );

  const auditRows = useMemo<AuditRow[]>(
    () =>
      (data?.auditLogs ?? []).map((log) => ({
        id: log.id,
        time: formatDate(log.created_at),
        actor: log.actor_id || "-",
        action: log.action,
        target: `${log.target_type}${log.target_id ? `:${log.target_id}` : ""}`,
        result: log.reason || "已记录"
      })),
    [data?.auditLogs]
  );

  const examColumns: TableColumnsType<ExamRow> = [
    { title: "考试", dataIndex: "name" },
    { title: "学科", dataIndex: "subject", width: 96 },
    { title: "状态", dataIndex: "status", width: 132, render: (value: string) => <StatusTag tone={statusTone(value)}>{value}</StatusTag> },
    { title: "答卷", dataIndex: "submissions", width: 120 },
    { title: "进度", dataIndex: "progress", width: 150, render: (value: number) => <Progress percent={value} size="small" /> },
    { title: "模式", dataIndex: "mode" }
  ];

  const submissionColumns: TableColumnsType<SubmissionRow> = [
    { title: "答卷", dataIndex: "id" },
    { title: "考试", dataIndex: "exam" },
    { title: "考号/学生", dataIndex: "candidate", width: 132 },
    { title: "状态", dataIndex: "status", width: 132, render: (value: string) => <StatusTag tone={statusTone(value)}>{value}</StatusTag> },
    { title: "页数", dataIndex: "pages", width: 96 },
    { title: "质量", dataIndex: "quality", width: 132, render: (value: string) => <StatusTag tone={statusTone(value)}>{value}</StatusTag> },
    { title: "创建时间", dataIndex: "createdAt", width: 180 }
  ];

  const auditColumns: TableColumnsType<AuditRow> = [
    { title: "时间", dataIndex: "time", width: 180 },
    { title: "操作人", dataIndex: "actor", width: 160 },
    { title: "动作", dataIndex: "action" },
    { title: "对象", dataIndex: "target" },
    { title: "结果", dataIndex: "result" }
  ];

  if (!data && loading) {
    return <LoadingState label="正在加载首页数据" />;
  }

  if (!data && error) {
    return <ErrorState message={error} onRetry={() => setRefreshNonce((value) => value + 1)} />;
  }

  return (
    <div className="page-stack">
      <motion.section className="page-heading" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25 }}>
        <div>
          <h1>首页</h1>
          <p>汇总当前考试、答卷处理、系统状态和近期操作。</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => setRefreshNonce((value) => value + 1)}>
            刷新
          </Button>
        </Space>
      </motion.section>

      {error ? <Alert type="error" showIcon message="刷新失败" description={error} /> : null}
      {data?.warnings.map((warning) => <Alert key={warning} type="warning" showIcon message={warning} />)}

      <section className="metric-grid">
        {metrics.map((metric) => (
          <motion.div className="metric-tile" key={metric.label} whileHover={{ y: -2 }} transition={{ duration: 0.16 }}>
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
            <StatusTag tone={metric.status}>{metric.trend}</StatusTag>
          </motion.div>
        ))}
      </section>

      <section className="dashboard-grid">
        <div className="workspace-section chart-section">
          <div className="section-head">
            <div>
              <h2>系统依赖</h2>
              <p>{data?.systemStatus ? `${data.systemStatus.service} / ${data.systemStatus.environment}` : "系统状态接口未返回"}</p>
            </div>
          </div>
          {data?.systemStatus ? (
            <div className="agent-list">
              {data.systemStatus.dependencies.map((dependency) => (
                <div className="agent-row" key={dependency.name}>
                  <div>
                    <strong>{dependency.name}</strong>
                    <span>{dependency.detail || dependency.error || `${dependency.duration_ms} ms`}</span>
                  </div>
                  <StatusTag tone={statusTone(dependency.status)}>{dependency.status}</StatusTag>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="暂无系统状态" description="系统状态接口不可用或当前账号无权限。" />
          )}
        </div>

        <div className="workspace-section chart-section">
          <div className="section-head">
            <div>
              <h2>运行概览</h2>
              <p>{data?.auditLogs.length ?? 0} 条近期审计事件</p>
            </div>
          </div>
          <div className="pipeline-list">
            <div className="pipeline-row">
              <span>考试</span>
              <Progress percent={Math.min(100, (data?.exams.length ?? 0) * 10)} size="small" />
              <strong>{data?.exams.length ?? 0}</strong>
            </div>
            <div className="pipeline-row">
              <span>答卷</span>
              <Progress percent={Math.min(100, (data?.submissions.length ?? 0) * 5)} size="small" />
              <strong>{data?.submissions.length ?? 0}</strong>
            </div>
            <div className="pipeline-row">
              <span>依赖</span>
              <Progress percent={data?.systemStatus ? 100 : 0} size="small" status={data?.systemStatus?.status === "degraded" ? "exception" : "normal"} />
              <strong>{data?.systemStatus?.dependencies.length ?? 0}</strong>
            </div>
          </div>
        </div>
      </section>

      <DataTable title="考试队列" rows={examRows} columns={examColumns} searchPlaceholder="搜索考试" filterLabel="状态" rowKey="id" />
      <DataTable title="最近答卷" rows={submissionRows} columns={submissionColumns} searchPlaceholder="搜索答卷" filterLabel="质量" rowKey="id" />
      <DataTable title="审计动态" rows={auditRows} columns={auditColumns} searchPlaceholder="搜索动作" rowKey="id" />

      <Button type="link" className="next-link" onClick={() => { window.location.hash = "/exams"; }}>
        进入考试管理 <ArrowRight size={16} />
      </Button>
    </div>
  );
}
