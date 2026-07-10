import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  App,
  Button,
  Descriptions,
  Drawer,
  Input,
  InputNumber,
  List,
  Progress,
  Select,
  Space,
  Table,
  Tooltip,
  Upload,
  type TableColumnsType,
  type UploadProps
} from "antd";
import {
  CheckCircle2,
  Eye,
  FileSearch,
  FileUp,
  Image as ImageIcon,
  Layers3,
  Play,
  RefreshCw,
  RotateCcw,
  Search,
  ShieldAlert
} from "lucide-react";
import { ApiClientError } from "../api/client";
import { downloadFileBlob, uploadFile, uploadFileWithProgress } from "../api/files";
import { listExams, type Exam } from "../api/exams";
import { listStudents, type Student } from "../api/org";
import {
  addSubmissionPage,
  createOcrTask,
  createSubmission,
  generateAnswerSegments,
  getOcrTask,
  getSubmission,
  listAnswerSegments,
  listOcrTasks,
  listSubmissionPages,
  listSubmissions,
  replaceSubmissionPage,
  runQualityCheck,
  updateSubmissionStatus,
  type AnswerSegment,
  type OcrTask,
  type QualityIssue,
  type Submission,
  type SubmissionPage
} from "../api/submissions";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import type { StatusTone } from "../types";

type UploadRequest = Parameters<NonNullable<UploadProps["customRequest"]>>[0];
type QualityFilter = "all" | "blurry" | "missing_page" | "duplicate_page" | "ocr_failed" | "needs_manual_handling";

interface SubmissionView {
  submission: Submission;
  pages: SubmissionPage[];
  ocrTasks: OcrTask[];
  segments: AnswerSegment[];
  detailError?: string;
}

interface UploadQueueItem {
  id: string;
  fileName: string;
  status: "processing" | "success" | "error";
  phase: string;
  percent: number;
  error?: string;
}

interface PreviewState {
  url: string;
  contentType: string;
  filename?: string;
}

interface Filters {
  search: string;
  quality: QualityFilter;
}

const qualityFilterOptions: { label: string; value: QualityFilter }[] = [
  { label: "全部质量状态", value: "all" },
  { label: "模糊", value: "blurry" },
  { label: "缺页", value: "missing_page" },
  { label: "重复页", value: "duplicate_page" },
  { label: "OCR 失败", value: "ocr_failed" },
  { label: "需要人工处理", value: "needs_manual_handling" }
];

const submissionStatusLabels: Record<string, string> = {
  created: "已创建",
  pages_uploaded: "已上传",
  quality_checked: "质量通过",
  ready_for_ocr: "待 OCR",
  rejected: "已拒绝"
};

const qualityStatusLabels: Record<string, string> = {
  unchecked: "未检查",
  passed: "通过",
  failed: "未通过"
};

const ocrStatusLabels: Record<string, string> = {
  queued: "排队中",
  processing: "处理中",
  completed: "已完成",
  failed: "失败"
};

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    return `${error.status} ${error.code}: ${error.message}`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "未知错误";
}

function formatTime(value?: string) {
  if (!value) {
    return "-";
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString("zh-CN", { hour12: false });
}

function compactFileName(name: string) {
  return name.replace(/\.[^.]+$/, "").trim().slice(0, 64) || name.slice(0, 64);
}

function sourceTypeFor(file: File) {
  const name = file.name.toLowerCase();
  return file.type === "application/pdf" || name.endsWith(".pdf") ? "pdf_upload" : "image_upload";
}

function statusTone(status: string): StatusTone {
  if (status === "ready_for_ocr" || status === "quality_checked") {
    return "success";
  }
  if (status === "rejected") {
    return "danger";
  }
  if (status === "created") {
    return "neutral";
  }
  return "processing";
}

function qualityTone(status: string): StatusTone {
  if (status === "passed") {
    return "success";
  }
  if (status === "failed") {
    return "danger";
  }
  return "neutral";
}

function ocrTone(task?: OcrTask): StatusTone {
  if (!task) {
    return "neutral";
  }
  if (task.status === "completed" && !task.requires_human_review) {
    return "success";
  }
  if (task.status === "failed" || task.requires_human_review) {
    return "warning";
  }
  return "processing";
}

function latestTask(tasks: OcrTask[]) {
  return [...tasks].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0];
}

function ocrLabel(tasks: OcrTask[]) {
  const task = latestTask(tasks);
  if (!task) {
    return "未触发";
  }
  const suffix = task.requires_human_review ? " / 需人工" : "";
  return `${ocrStatusLabels[task.status] ?? task.status}${suffix}`;
}

function segmentTone(segments: AnswerSegment[]): StatusTone {
  if (segments.length === 0) {
    return "neutral";
  }
  if (segments.some((item) => item.status === "rejected" || item.status === "needs_manual_review")) {
    return "warning";
  }
  return "success";
}

function segmentLabel(segments: AnswerSegment[]) {
  if (segments.length === 0) {
    return "未切分";
  }
  const manual = segments.filter((item) => item.status === "needs_manual_review" || item.status === "rejected").length;
  return manual > 0 ? `${segments.length} 段 / ${manual} 需处理` : `${segments.length} 段`;
}

function derivedIssues(row: SubmissionView): QualityIssue[] {
  const issues = [...(row.submission.quality_issues ?? [])];
  for (const page of row.pages) {
    issues.push(...(page.quality_issues ?? []).map((issue) => ({ code: issue.code, message: `第 ${page.page_no} 页：${issue.message}` })));
  }
  if (row.ocrTasks.some((task) => task.status === "failed")) {
    issues.push({ code: "ocr_failed", message: "OCR 任务失败" });
  }
  if (row.ocrTasks.some((task) => task.requires_human_review)) {
    issues.push({ code: "needs_manual_handling", message: "OCR 结果需要人工处理" });
  }
  if (row.segments.some((segment) => segment.status === "needs_manual_review" || segment.status === "rejected")) {
    issues.push({ code: "needs_manual_handling", message: "切分结果需要人工处理" });
  }
  return issues;
}

function matchesQualityFilter(row: SubmissionView, filter: QualityFilter) {
  if (filter === "all") {
    return true;
  }
  const issues = derivedIssues(row);
  if (filter === "missing_page") {
    return issues.some((issue) => issue.code === "missing_page" || issue.code === "page_count_mismatch" || issue.code === "no_pages");
  }
  if (filter === "ocr_failed") {
    return issues.some((issue) => issue.code === "ocr_failed") || row.ocrTasks.some((task) => task.status === "failed");
  }
  if (filter === "needs_manual_handling") {
    return issues.some((issue) => issue.code === "needs_manual_handling");
  }
  return issues.some((issue) => issue.code === filter);
}

function issueTone(code: string): StatusTone {
  if (code === "missing_page" || code === "page_count_mismatch" || code === "no_pages" || code === "ocr_failed") {
    return "danger";
  }
  if (code === "needs_manual_handling" || code === "blurry" || code === "duplicate_page") {
    return "warning";
  }
  return "neutral";
}

async function buildSubmissionView(submission: Submission): Promise<SubmissionView> {
  const [pagesResult, ocrResult, segmentResult] = await Promise.allSettled([
    listSubmissionPages(submission.id),
    listOcrTasks(submission.id),
    listAnswerSegments(submission.id)
  ]);
  const errors = [pagesResult, ocrResult, segmentResult]
    .filter((result): result is PromiseRejectedResult => result.status === "rejected")
    .map((result) => formatError(result.reason));
  return {
    submission,
    pages: pagesResult.status === "fulfilled" ? pagesResult.value.pages : submission.pages ?? [],
    ocrTasks: ocrResult.status === "fulfilled" ? ocrResult.value.tasks : [],
    segments: segmentResult.status === "fulfilled" ? segmentResult.value.segments : [],
    detailError: errors.length > 0 ? errors.join("；") : undefined
  };
}

export function SubmissionCapturePage({
  canManage,
  canReadStudentNames
}: {
  canManage: boolean;
  canReadStudentNames: boolean;
}) {
  const { message } = App.useApp();
  const hasSession = true;
  const canWrite = canManage && hasSession;
  const [filters, setFilters] = useState<Filters>({ search: "", quality: "all" });
  const [expectedPages, setExpectedPages] = useState(1);
  const [exams, setExams] = useState<Exam[]>([]);
  const [selectedExamId, setSelectedExamId] = useState("");
  const [rows, setRows] = useState<SubmissionView[]>([]);
  const [students, setStudents] = useState<Student[]>([]);
  const [studentLookupError, setStudentLookupError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [captureLoading, setCaptureLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [captureError, setCaptureError] = useState<string | null>(null);
  const [uploadQueue, setUploadQueue] = useState<UploadQueueItem[]>([]);
  const [actioning, setActioning] = useState<string | null>(null);
  const [pageDrawer, setPageDrawer] = useState<SubmissionView | null>(null);
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [ocrDrawer, setOcrDrawer] = useState<{ row: SubmissionView; tasks: OcrTask[]; loading: boolean; error?: string } | null>(null);

  const selectedExam = useMemo(() => exams.find((exam) => exam.id === selectedExamId), [exams, selectedExamId]);
  const studentById = useMemo(() => new Map(students.map((student) => [student.id, student])), [students]);

  const loadExams = useCallback(async () => {
    setLoading(true);
    setError(null);
    if (!hasSession) {
      setExams([]);
      setSelectedExamId("");
      setError("当前没有有效登录会话，无法调用真实后端 API。");
      setLoading(false);
      return;
    }
    try {
      const result = await listExams();
      setExams(result.exams);
      setSelectedExamId((current) => current || result.exams[0]?.id || "");
    } catch (currentError) {
      setError(formatError(currentError));
    } finally {
      setLoading(false);
    }
  }, [hasSession]);

  const loadCaptureData = useCallback(
    async (examId: string) => {
      if (!examId || !hasSession) {
        setRows([]);
        return;
      }
      setCaptureLoading(true);
      setCaptureError(null);
      setStudentLookupError(null);
      try {
        const [submissionResult, studentResult] = await Promise.allSettled([
          listSubmissions(examId),
          canReadStudentNames ? listStudents() : Promise.resolve({ students: [] })
        ]);
        if (submissionResult.status === "rejected") {
          throw submissionResult.reason;
        }
        if (studentResult.status === "fulfilled") {
          setStudents(studentResult.value.students);
        } else {
          setStudents([]);
          setStudentLookupError(formatError(studentResult.reason));
        }
        const views = await Promise.all(submissionResult.value.submissions.map((submission) => buildSubmissionView(submission)));
        setRows(views);
      } catch (currentError) {
        setCaptureError(formatError(currentError));
      } finally {
        setCaptureLoading(false);
      }
    },
    [canReadStudentNames, hasSession]
  );

  useEffect(() => {
    void loadExams();
  }, [loadExams]);

  useEffect(() => {
    void loadCaptureData(selectedExamId);
  }, [loadCaptureData, selectedExamId]);

  useEffect(() => {
    return () => {
      if (preview?.url) {
        URL.revokeObjectURL(preview.url);
      }
    };
  }, [preview?.url]);

  const refreshSingle = async (submissionId: string) => {
    const detail = await getSubmission(submissionId);
    const view = await buildSubmissionView(detail.submission);
    setRows((current) => current.map((item) => (item.submission.id === submissionId ? view : item)));
    setPageDrawer((current) => (current?.submission.id === submissionId ? view : current));
    setOcrDrawer((current) => (current?.row.submission.id === submissionId ? { ...current, row: view } : current));
  };

  const patchUpload = (id: string, patch: Partial<UploadQueueItem>) => {
    setUploadQueue((current) => current.map((item) => (item.id === id ? { ...item, ...patch } : item)));
  };

  const uploadAnswerFile = async (request: UploadRequest) => {
    const file = request.file as File & { uid?: string };
    const uploadId = file.uid ?? `${Date.now()}-${file.name}`;
    setUploadQueue((current) => [
      { id: uploadId, fileName: file.name, status: "processing", phase: "创建答卷记录", percent: 2 },
      ...current
    ]);
    if (!selectedExam) {
      const errorMessage = "请先选择考试";
      patchUpload(uploadId, { status: "error", phase: "失败", error: errorMessage, percent: 0 });
      request.onError?.(new Error(errorMessage));
      return;
    }
    try {
      const submissionResult = await createSubmission(selectedExam.id, {
        candidate_no: compactFileName(file.name),
        source_type: sourceTypeFor(file),
        expected_page_count: expectedPages
      });
      patchUpload(uploadId, { phase: "上传文件", percent: 8 });
      const uploadResult = await uploadFileWithProgress(
        file,
        {
          owner_type: "submission",
          owner_id: submissionResult.submission.id,
          school_id: selectedExam.school_id,
          exam_id: selectedExam.id,
          submission_id: submissionResult.submission.id
        },
        (percent) => patchUpload(uploadId, { percent: Math.min(92, Math.max(8, percent)) })
      );
      patchUpload(uploadId, { phase: "关联第 1 页", percent: 94 });
      await addSubmissionPage(submissionResult.submission.id, uploadResult.file.id, 1);
      patchUpload(uploadId, { status: "success", phase: "完成", percent: 100 });
      request.onSuccess?.({ ok: true });
      await loadCaptureData(selectedExam.id);
    } catch (currentError) {
      const errorMessage = formatError(currentError);
      patchUpload(uploadId, { status: "error", phase: "失败", error: errorMessage });
      request.onError?.(currentError instanceof Error ? currentError : new Error(errorMessage));
    }
  };

  const uploadProps: UploadProps = {
    multiple: true,
    showUploadList: false,
    customRequest: (request) => {
      void uploadAnswerFile(request);
    }
  };

  const filteredRows = useMemo(() => {
    const keyword = filters.search.trim().toLowerCase();
    return rows.filter((row) => {
      const student = row.submission.student_id ? studentById.get(row.submission.student_id) : undefined;
      const keywordMatched =
        !keyword ||
        row.submission.id.toLowerCase().includes(keyword) ||
        (row.submission.candidate_no ?? "").toLowerCase().includes(keyword) ||
        (student?.name ?? "").toLowerCase().includes(keyword) ||
        (student?.student_no ?? "").toLowerCase().includes(keyword);
      return keywordMatched && matchesQualityFilter(row, filters.quality);
    });
  }, [filters.quality, filters.search, rows, studentById]);

  const uploadPercent = useMemo(() => {
    if (uploadQueue.length === 0) {
      return 0;
    }
    return Math.round(uploadQueue.reduce((sum, item) => sum + item.percent, 0) / uploadQueue.length);
  }, [uploadQueue]);

  const summary = useMemo(() => {
    const issueCount = rows.filter((row) => derivedIssues(row).length > 0).length;
    const readyCount = rows.filter((row) => row.submission.status === "ready_for_ocr").length;
    const ocrDone = rows.filter((row) => row.ocrTasks.some((task) => task.status === "completed")).length;
    const segmented = rows.filter((row) => row.segments.length > 0).length;
    return { total: rows.length, issueCount, readyCount, ocrDone, segmented };
  }, [rows]);

  const studentName = (submission: Submission) => {
    if (!submission.student_id) {
      return "未关联学生";
    }
    if (!canReadStudentNames) {
      return "无姓名权限";
    }
    return studentById.get(submission.student_id)?.name ?? "后端未返回";
  };

  const runAction = async (key: string, action: () => Promise<void>, successText: string) => {
    setActioning(key);
    try {
      await action();
      message.success(successText);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  };

  const openPages = (row: SubmissionView) => {
    setPageDrawer(row);
    setPreview(null);
  };

  const previewPage = async (page: SubmissionPage) => {
    setPreviewLoading(true);
    try {
      const file = await downloadFileBlob(page.file_asset_id);
      setPreview((current) => {
        if (current?.url) {
          URL.revokeObjectURL(current.url);
        }
        return { url: URL.createObjectURL(file.blob), contentType: file.contentType, filename: file.filename };
      });
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setPreviewLoading(false);
    }
  };

  const replacePage = async (page: SubmissionPage, file: File) => {
    if (!pageDrawer || !selectedExam) {
      message.error("请先选择答卷");
      return;
    }
    await runAction(
      `replace-${page.id}`,
      async () => {
        const upload = await uploadFile(file, {
          owner_type: "submission",
          owner_id: pageDrawer.submission.id,
          school_id: selectedExam.school_id,
          exam_id: selectedExam.id,
          submission_id: pageDrawer.submission.id
        });
        await replaceSubmissionPage(pageDrawer.submission.id, page.page_no, upload.file.id);
        await refreshSingle(pageDrawer.submission.id);
      },
      "页面文件已替换，质量门禁已重置"
    );
  };

  const openOcrDrawer = async (row: SubmissionView) => {
    setOcrDrawer({ row, tasks: [], loading: true });
    try {
      const tasks = await Promise.all(row.ocrTasks.map((task) => getOcrTask(task.id).then((result) => result.task)));
      setOcrDrawer({ row, tasks, loading: false });
    } catch (currentError) {
      setOcrDrawer({ row, tasks: [], loading: false, error: formatError(currentError) });
    }
  };

  const columns: TableColumnsType<SubmissionView> = [
    {
      title: "匿名码 / 准考证",
      fixed: "left",
      width: 210,
      render: (_, row) => (
        <div className="capture-identity-cell">
          <strong>{row.submission.candidate_no || "后端未返回"}</strong>
          <span>anonymous_code 独立字段未返回</span>
        </div>
      )
    },
    { title: "学生姓名", width: 130, render: (_, row) => studentName(row.submission) },
    {
      title: "页数",
      width: 92,
      render: (_, row) => `${row.pages.length || row.submission.actual_page_count}/${row.submission.expected_page_count || "未设"}`
    },
    {
      title: "采集状态",
      width: 112,
      render: (_, row) => <StatusTag tone={statusTone(row.submission.status)}>{submissionStatusLabels[row.submission.status] ?? row.submission.status}</StatusTag>
    },
    {
      title: "质量",
      width: 108,
      render: (_, row) => <StatusTag tone={qualityTone(row.submission.quality_status)}>{qualityStatusLabels[row.submission.quality_status] ?? row.submission.quality_status}</StatusTag>
    },
    {
      title: "OCR 状态",
      width: 128,
      render: (_, row) => <StatusTag tone={ocrTone(latestTask(row.ocrTasks))}>{ocrLabel(row.ocrTasks)}</StatusTag>
    },
    {
      title: "切分状态",
      width: 128,
      render: (_, row) => <StatusTag tone={segmentTone(row.segments)}>{segmentLabel(row.segments)}</StatusTag>
    },
    {
      title: "阅卷状态",
      width: 132,
      render: () => <StatusTag tone="neutral">待后续 API 支持</StatusTag>
    },
    {
      title: "质量问题",
      width: 260,
      render: (_, row) => {
        const issues = derivedIssues(row);
        return issues.length > 0 ? (
          <Space wrap size={[0, 4]}>
            {issues.slice(0, 3).map((issue, index) => (
              <StatusTag key={`${issue.code}-${index}`} tone={issueTone(issue.code)}>
                {issue.message || issue.code}
              </StatusTag>
            ))}
            {issues.length > 3 ? <StatusTag tone="neutral">{`+${issues.length - 3}`}</StatusTag> : null}
          </Space>
        ) : (
          <StatusTag tone="success">无</StatusTag>
        );
      }
    },
    {
      title: "操作",
      fixed: "right",
      width: 430,
      render: (_, row) => {
        const id = row.submission.id;
        const readyDisabled = row.submission.quality_status !== "passed" || row.submission.status === "ready_for_ocr";
        const ocrDisabled = row.submission.status !== "ready_for_ocr";
        return (
          <Space className="table-actions" wrap>
            <Button size="small" icon={<ImageIcon size={14} />} onClick={() => openPages(row)}>
              页面
            </Button>
            <Button
              size="small"
              icon={<CheckCircle2 size={14} />}
              disabled={!canWrite}
              loading={actioning === `quality-${id}`}
              onClick={() =>
                void runAction(
                  `quality-${id}`,
                  async () => {
                    await runQualityCheck(id);
                    await refreshSingle(id);
                  },
                  "质量检查完成"
                )
              }
            >
              质量检查
            </Button>
            <Tooltip title={readyDisabled ? "需要质量检查通过后才能进入 OCR 准备状态" : ""}>
              <Button
                size="small"
                icon={<ShieldAlert size={14} />}
                disabled={!canWrite || readyDisabled}
                loading={actioning === `ready-${id}`}
                onClick={() =>
                  void runAction(
                    `ready-${id}`,
                    async () => {
                      await updateSubmissionStatus(id, "ready_for_ocr");
                      await refreshSingle(id);
                    },
                    "已标记为待 OCR"
                  )
                }
              >
                标记 OCR
              </Button>
            </Tooltip>
            <Tooltip title={ocrDisabled ? "submission 必须处于 ready_for_ocr" : "创建 OCR 任务，等待真实 worker 回写结果"}>
              <Button
                size="small"
                icon={<Play size={14} />}
                disabled={!canWrite || ocrDisabled}
                loading={actioning === `ocr-${id}`}
                onClick={() =>
                  void runAction(
                    `ocr-${id}`,
                    async () => {
                      await createOcrTask(id);
                      await refreshSingle(id);
                    },
                    "OCR 任务已创建"
                  )
                }
              >
                触发 OCR
              </Button>
            </Tooltip>
            <Button
              size="small"
              icon={<Layers3 size={14} />}
              disabled={!canWrite || ocrDisabled}
              loading={actioning === `segment-${id}`}
              onClick={() =>
                void runAction(
                  `segment-${id}`,
                  async () => {
                    await generateAnswerSegments(id);
                    await refreshSingle(id);
                  },
                  "切分任务已执行"
                )
              }
            >
              触发切分
            </Button>
            <Button size="small" icon={<FileSearch size={14} />} onClick={() => void openOcrDrawer(row)}>
              OCR 结果
            </Button>
          </Space>
        );
      }
    }
  ];

  const pageColumns: TableColumnsType<SubmissionPage> = [
    { title: "页码", dataIndex: "page_no", width: 72 },
    { title: "文件", dataIndex: "file_asset_id", render: (value: string) => <span className="mono-cell">{value}</span> },
    { title: "状态", dataIndex: "status", width: 110, render: (value: string) => <StatusTag tone="processing">{value}</StatusTag> },
    {
      title: "质量问题",
      width: 180,
      render: (_, page) =>
        page.quality_issues.length > 0 ? (
          <Space wrap>
            {page.quality_issues.map((issue, index) => (
              <StatusTag key={`${issue.code}-${index}`} tone={issueTone(issue.code)}>
                {issue.message}
              </StatusTag>
            ))}
          </Space>
        ) : (
          <StatusTag tone="success">无</StatusTag>
        )
    },
    {
      title: "操作",
      width: 190,
      render: (_, page) => (
        <Space>
          <Button size="small" icon={<Eye size={14} />} loading={previewLoading} onClick={() => void previewPage(page)}>
            查看
          </Button>
          <Upload
            showUploadList={false}
            beforeUpload={(file) => {
              void replacePage(page, file);
              return false;
            }}
          >
            <Button size="small" icon={<RotateCcw size={14} />} disabled={!canWrite || Boolean(actioning)}>
              重传
            </Button>
          </Upload>
        </Space>
      )
    }
  ];

  const ocrTaskColumns: TableColumnsType<OcrTask> = [
    { title: "任务", dataIndex: "id", width: 160, render: (value: string) => <span className="mono-cell">{value}</span> },
    { title: "状态", width: 110, render: (_, task) => <StatusTag tone={ocrTone(task)}>{ocrLabel([task])}</StatusTag> },
    { title: "引擎", width: 180, render: (_, task) => `${task.engine} / ${task.engine_version}` },
    { title: "结果数", dataIndex: "result_count", width: 90 },
    { title: "创建时间", dataIndex: "created_at", width: 170, render: formatTime }
  ];

  return (
    <div className="page-stack">
      <section className="page-heading">
        <div>
          <Space>
            <h1>答卷采集</h1>
            <StatusTag tone="success">真实 API</StatusTag>
          </Space>
          <p>按考试管理答卷上传、质量门禁、OCR 任务和答案切分入口。</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} onClick={() => void loadCaptureData(selectedExamId)} loading={captureLoading}>
            刷新
          </Button>
          <Upload {...uploadProps}>
            <Button type="primary" icon={<FileUp size={16} />} disabled={!canWrite || !selectedExam}>
              批量上传
            </Button>
          </Upload>
        </Space>
      </section>

      {!hasSession ? (
        <Alert
          type="warning"
          showIcon
          message="未检测到真实后端访问令牌"
            description="上传答卷并跟踪页面质量、文字识别和答案切分进度。"
        />
      ) : null}

      {studentLookupError ? (
        <Alert type="info" showIcon message="学生姓名未完全加载" description={`答卷采集仍可继续，姓名列将按权限或关联状态显示：${studentLookupError}`} />
      ) : null}

      <section className="workspace-section filter-panel">
        <div className="capture-toolbar">
          <Select
            className="exam-picker"
            placeholder="选择考试"
            value={selectedExamId || undefined}
            options={exams.map((exam) => ({ label: exam.name, value: exam.id }))}
            onChange={setSelectedExamId}
            loading={loading}
          />
          <Input
            className="toolbar-input"
            prefix={<Search size={16} />}
            placeholder="搜索匿名码、学生、答卷 ID"
            value={filters.search}
            onChange={(event) => setFilters((current) => ({ ...current, search: event.target.value }))}
          />
          <Select
            className="toolbar-select"
            value={filters.quality}
            options={qualityFilterOptions}
            onChange={(value) => setFilters((current) => ({ ...current, quality: value }))}
          />
          <label className="inline-number-field">
            <span>默认页数</span>
            <InputNumber min={1} max={200} precision={0} value={expectedPages} onChange={(value) => setExpectedPages(Number(value ?? 1))} />
          </label>
          {selectedExam ? <span className="muted">批量上传按一个文件创建一份答卷，文件先关联为第 1 页。</span> : null}
        </div>
      </section>

      {uploadQueue.length > 0 ? (
        <section className="workspace-section upload-progress-panel">
          <div className="section-head">
            <div>
              <h2>上传队列</h2>
              <p>{uploadQueue.filter((item) => item.status === "error").length} 个失败</p>
            </div>
            <Progress className="batch-progress" percent={uploadPercent} size="small" />
          </div>
          <List
            className="upload-queue-list"
            dataSource={uploadQueue.slice(0, 8)}
            renderItem={(item) => (
              <List.Item className="upload-queue-item">
                <div>
                  <strong>{item.fileName}</strong>
                  <span>{item.error || item.phase}</span>
                </div>
                <Progress percent={item.percent} size="small" status={item.status === "error" ? "exception" : item.status === "success" ? "success" : "active"} />
              </List.Item>
            )}
          />
        </section>
      ) : null}

      <section className="capture-summary-grid">
        <div className="metric-tile">
          <span>答卷数</span>
          <strong>{summary.total}</strong>
          <small>真实 submission</small>
        </div>
        <div className="metric-tile">
          <span>质量问题</span>
          <strong>{summary.issueCount}</strong>
          <small>后端问题 + OCR/切分风险</small>
        </div>
        <div className="metric-tile">
          <span>待 OCR</span>
          <strong>{summary.readyCount}</strong>
          <small>ready_for_ocr</small>
        </div>
        <div className="metric-tile">
          <span>已切分</span>
          <strong>{summary.segmented}</strong>
          <small>answer_segment 记录</small>
        </div>
      </section>

      {loading ? (
        <section className="workspace-section">
          <LoadingState label="正在读取考试列表" />
        </section>
      ) : error ? (
        <ErrorState message={error} onRetry={() => void loadExams()} />
      ) : !selectedExam ? (
        <section className="workspace-section">
          <EmptyState title="暂无考试" description="后端没有返回可采集答卷的考试。" />
        </section>
      ) : captureLoading ? (
        <section className="workspace-section">
          <LoadingState label="正在读取答卷采集记录" />
        </section>
      ) : captureError ? (
        <ErrorState message={captureError} onRetry={() => void loadCaptureData(selectedExam.id)} />
      ) : (
        <section className="workspace-section">
          <div className="section-head">
            <div>
              <h2>学生答卷列表</h2>
              <p>
                {filteredRows.length} / {rows.length} 条记录，OCR 完成 {summary.ocrDone} 份
              </p>
            </div>
          </div>
          <Table<SubmissionView>
            rowKey={(row) => row.submission.id}
            dataSource={filteredRows}
            columns={columns}
            scroll={{ x: "max-content" }}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            size="small"
            rowClassName={(row) => (row.detailError ? "row-with-warning" : "")}
            locale={{ emptyText: <EmptyState title="暂无答卷" description="当前考试还没有真实 submission 记录。" /> }}
          />
        </section>
      )}

      <Drawer title="答卷页面" open={Boolean(pageDrawer)} width={860} onClose={() => setPageDrawer(null)}>
        {pageDrawer ? (
          <div className="detail-stack">
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="匿名码/准考证">{pageDrawer.submission.candidate_no || "后端未返回"}</Descriptions.Item>
              <Descriptions.Item label="学生姓名">{studentName(pageDrawer.submission)}</Descriptions.Item>
              <Descriptions.Item label="采集状态">{submissionStatusLabels[pageDrawer.submission.status] ?? pageDrawer.submission.status}</Descriptions.Item>
              <Descriptions.Item label="质量状态">{qualityStatusLabels[pageDrawer.submission.quality_status] ?? pageDrawer.submission.quality_status}</Descriptions.Item>
            </Descriptions>
            <Table<SubmissionPage>
              rowKey="id"
              dataSource={pageDrawer.pages}
              columns={pageColumns}
              pagination={false}
              size="small"
              scroll={{ x: "max-content" }}
              locale={{ emptyText: <EmptyState title="暂无页面" description="该答卷还没有关联页面文件。" /> }}
            />
            {preview ? (
              <div className="page-preview">
                <div className="section-head">
                  <div>
                    <h2>{preview.filename ?? "页面预览"}</h2>
                    <p>{preview.contentType}</p>
                  </div>
                  <Button icon={<Eye size={16} />} onClick={() => window.open(preview.url, "_blank", "noopener,noreferrer")}>
                    新窗口打开
                  </Button>
                </div>
                {preview.contentType.startsWith("image/") ? (
                  <img src={preview.url} alt={preview.filename ?? "答卷页面"} />
                ) : preview.contentType === "application/pdf" ? (
                  <iframe title={preview.filename ?? "答卷 PDF"} src={preview.url} />
                ) : (
                  <Alert type="info" showIcon message="该文件类型不支持内嵌预览" description="可使用新窗口打开或浏览器下载查看。" />
                )}
              </div>
            ) : null}
          </div>
        ) : null}
      </Drawer>

      <Drawer title="OCR 任务与结果" open={Boolean(ocrDrawer)} width={920} onClose={() => setOcrDrawer(null)}>
        {ocrDrawer?.loading ? (
          <LoadingState label="正在读取 OCR 任务详情" />
        ) : ocrDrawer?.error ? (
          <ErrorState message={ocrDrawer.error} />
        ) : ocrDrawer ? (
          <div className="detail-stack">
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="匿名码/准考证">{ocrDrawer.row.submission.candidate_no || "后端未返回"}</Descriptions.Item>
              <Descriptions.Item label="OCR 状态">{ocrLabel(ocrDrawer.row.ocrTasks)}</Descriptions.Item>
            </Descriptions>
            <Table<OcrTask>
              rowKey="id"
              dataSource={ocrDrawer.tasks}
              columns={ocrTaskColumns}
              pagination={false}
              size="small"
              scroll={{ x: "max-content" }}
              locale={{ emptyText: <EmptyState title="暂无 OCR 任务" description="还没有通过真实接口创建 OCR 任务。" /> }}
            />
            {ocrDrawer.tasks.flatMap((task) => task.results ?? []).length > 0 ? (
              <div className="ocr-result-list">
                {ocrDrawer.tasks.map((task) => (
                  <section className="ocr-result-block" key={task.id}>
                    <div className="section-head">
                      <div>
                        <h2>{task.id}</h2>
                        <p>{task.results?.length ?? 0} 条 OCR 结果</p>
                      </div>
                    </div>
                    <Table
                      rowKey="id"
                      dataSource={task.results ?? []}
                      pagination={false}
                      size="small"
                      columns={[
                        { title: "页面", dataIndex: "submission_page_id", width: 150 },
                        { title: "文本", dataIndex: "text" },
                        { title: "置信度", dataIndex: "confidence", width: 100, render: (value: number) => value.toFixed(2) },
                        { title: "BBox", dataIndex: "bbox", width: 180, render: (value: number[]) => `[${value.join(", ")}]` }
                      ]}
                      scroll={{ x: "max-content" }}
                    />
                  </section>
                ))}
              </div>
            ) : (
              <EmptyState title="暂无 OCR 结果" description="任务创建后需由真实 OCR worker 启动并回写结果。" />
            )}
          </div>
        ) : null}
      </Drawer>
    </div>
  );
}
