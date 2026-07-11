import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  App,
  Button,
  Checkbox,
  Descriptions,
  Empty,
  Input,
  InputNumber,
  List,
  Progress,
  Segmented,
  Select,
  Space,
  Tabs,
  Tooltip
} from "antd";
import {
  BadgeCheck,
  CheckCircle2,
  Eye,
  FileText,
  Flag,
  Maximize2,
  MessageSquareText,
  Minus,
  Play,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Undo2,
  ZoomIn,
  ZoomOut
} from "lucide-react";
import { ApiClientError } from "../api/client";
import { downloadFileBlob } from "../api/files";
import { listQuestions, type Question, type RubricPoint } from "../api/papers";
import {
  createRuleGrade,
  createSubjectiveAiGrade,
  getReviewTask,
  listAiGrades,
  listReviewTasks,
  recordSegmentAnswer,
  returnReviewTask,
  submitHumanGrade,
  verifyEvidence,
  type AiGrade,
  type EvidenceJob,
  type ReviewTask,
  type RubricSelection
} from "../api/review";
import {
  getOcrTask,
  listAnswerSegments,
  listOcrTasks,
  listSubmissionPages,
  type AnswerSegment,
  type OcrResult,
  type OcrTask,
  type SubmissionPage
} from "../api/submissions";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import type { StatusTone } from "../types";

type TaskFilter = "active" | "pending" | "assigned" | "in_progress" | "returned" | "submitted";
type ViewerMode = "original" | "ocr";

interface WorkbenchContext {
  task: ReviewTask;
  segment?: AnswerSegment;
  question?: Question;
  pages: SubmissionPage[];
  page?: SubmissionPage;
  ocrTasks: OcrTask[];
  ocrResults: OcrResult[];
  aiGrades: AiGrade[];
  evidenceJob?: EvidenceJob;
  warnings: string[];
}

interface PreviewState {
  url: string;
  contentType: string;
  filename?: string;
}

interface ScoreDraft {
  score: number | null;
  comments: string;
  privateNote: string;
  studentFeedback: string;
  reason: string;
  disputeReason: string;
  rubricSelections: Record<string, number>;
  answerText: string;
}

const taskFilterOptions: { label: string; value: TaskFilter }[] = [
  { label: "可处理", value: "active" },
  { label: "待分配", value: "pending" },
  { label: "已分配", value: "assigned" },
  { label: "处理中", value: "in_progress" },
  { label: "退回", value: "returned" },
  { label: "已提交", value: "submitted" }
];

const sourceLabels: Record<string, string> = {
  ai_low_confidence: "AI 低置信",
  ocr_low_confidence: "OCR 低置信",
  subjective_default_review: "主观题复核",
  evidence_verification_failed: "证据校验失败",
  double_mark_required: "双评任务",
  score_anomaly: "分数异常",
  manual_sample: "人工抽检"
};

const taskStatusLabels: Record<string, string> = {
  pending: "待分配",
  assigned: "已分配",
  in_progress: "处理中",
  returned: "退回",
  submitted: "已提交",
  completed: "已完成"
};

const commentPresets = ["答案完整，逻辑清晰", "关键步骤缺失", "结论正确但过程不充分", "请补充必要说明"];

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

function taskTone(status: string): StatusTone {
  if (status === "submitted" || status === "completed") {
    return "success";
  }
  if (status === "returned") {
    return "warning";
  }
  if (status === "pending") {
    return "neutral";
  }
  return "processing";
}

function confidenceTone(value: number): StatusTone {
  if (value >= 0.85) {
    return "success";
  }
  if (value >= 0.65) {
    return "warning";
  }
  return "danger";
}

function latestGrade(grades: AiGrade[]) {
  return [...grades].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0];
}

function pointLabel(point: RubricPoint) {
  return `${point.description || point.id} (${point.score} 分)`;
}

function isInputTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName) || target.isContentEditable;
}

function gradeMockLabel(grade?: AiGrade) {
  if (!grade) {
    return "无 AI 建议";
  }
  if (grade.mock) {
    return "MOCK AI";
  }
  return grade.grader_type === "rule_based_objective" ? "规则判分" : "AI 建议";
}

async function loadTaskContext(task: ReviewTask): Promise<WorkbenchContext> {
  const [segmentResult, pagesResult, ocrTaskResult, questionResult, gradeResult] = await Promise.allSettled([
    listAnswerSegments(task.submission_id),
    listSubmissionPages(task.submission_id),
    listOcrTasks(task.submission_id),
    listQuestions(task.exam_id),
    listAiGrades(task.answer_segment_id)
  ]);
  const warnings = [segmentResult, pagesResult, ocrTaskResult, questionResult, gradeResult]
    .filter((item): item is PromiseRejectedResult => item.status === "rejected")
    .map((item) => formatError(item.reason));

  const segments = segmentResult.status === "fulfilled" ? segmentResult.value.segments : [];
  const pages = pagesResult.status === "fulfilled" ? pagesResult.value.pages : [];
  const taskList = ocrTaskResult.status === "fulfilled" ? ocrTaskResult.value.tasks : [];
  const ocrTasks = (
    await Promise.allSettled(taskList.map((item) => getOcrTask(item.id).then((result) => result.task)))
  )
    .filter((item): item is PromiseFulfilledResult<OcrTask> => item.status === "fulfilled")
    .map((item) => item.value);
  const questions = questionResult.status === "fulfilled" ? questionResult.value.questions : [];
  const segment = segments.find((item) => item.id === task.answer_segment_id);
  const question = questions.find((item) => item.id === task.question_id);
  const page = segment ? pages.find((item) => item.id === segment.submission_page_id) : undefined;
  const ocrResults = ocrTasks.flatMap((ocrTask) => ocrTask.results ?? []).filter((result) => !segment || result.submission_page_id === segment.submission_page_id);
  const aiGrades = gradeResult.status === "fulfilled" ? gradeResult.value.grades : [];

  if (!segment) {
    warnings.push("未能通过 submission_id 找到当前 answer_segment。");
  }
  if (!question) {
    warnings.push("未能通过 exam_id/question_id 找到题目与 Rubric。");
  }
  if (!page) {
    warnings.push("未能定位当前 answer_segment 对应的页面文件。");
  }

  return { task, segment, question, pages, page, ocrTasks, ocrResults, aiGrades, warnings };
}

function createInitialDraft(ctx: WorkbenchContext | null): ScoreDraft {
  const grade = latestGrade(ctx?.aiGrades ?? []);
  const ocrText = ctx?.ocrResults.map((item) => item.text).filter(Boolean).join("\n") ?? "";
  const selections: Record<string, number> = {};
  for (const point of ctx?.question?.rubric?.points ?? []) {
    selections[point.id] = 0;
  }
  return {
    score: grade && !grade.mock ? grade.suggested_score : null,
    comments: "",
    privateNote: "",
    studentFeedback: grade?.student_feedback ?? "",
    reason: "manual review completed",
    disputeReason: "",
    rubricSelections: selections,
    answerText: ocrText
  };
}

export function GradingWorkbenchPage({ canWork, canGrade, canVerifyEvidence, canReturn, initialExamId = "" }: { canWork: boolean; canGrade: boolean; canVerifyEvidence: boolean; canReturn: boolean; initialExamId?: string }) {
  const { message } = App.useApp();
  const hasSession = true;
  const canSubmit = canWork && hasSession;
  const [taskFilter, setTaskFilter] = useState<TaskFilter>("active");
  const [keyword, setKeyword] = useState("");
  const [tasks, setTasks] = useState<ReviewTask[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState("");
  const [loadingTasks, setLoadingTasks] = useState(true);
  const [taskError, setTaskError] = useState<string | null>(null);
  const [ctx, setCtx] = useState<WorkbenchContext | null>(null);
  const [contextLoading, setContextLoading] = useState(false);
  const [contextError, setContextError] = useState<string | null>(null);
  const [draft, setDraft] = useState<ScoreDraft>(() => createInitialDraft(null));
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [viewerMode, setViewerMode] = useState<ViewerMode>("original");
  const [scale, setScale] = useState(1);
  const [rotation, setRotation] = useState(0);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  const [actioning, setActioning] = useState<string | null>(null);
  const viewportRef = useRef<HTMLDivElement | null>(null);

  const filteredTasks = useMemo(() => {
    const text = keyword.trim().toLowerCase();
    return tasks.filter((task) => {
      const statusMatched = taskFilter === "active" ? ["assigned", "in_progress", "returned"].includes(task.status) : task.status === taskFilter;
      const examMatched = !initialExamId || task.exam_id === initialExamId;
      const keywordMatched =
        !text ||
        task.id.toLowerCase().includes(text) ||
        task.anonymous_code.toLowerCase().includes(text) ||
        task.question_no.toLowerCase().includes(text) ||
        task.source.toLowerCase().includes(text);
      return examMatched && statusMatched && keywordMatched;
    });
  }, [initialExamId, keyword, taskFilter, tasks]);

  const selectedIndex = useMemo(() => filteredTasks.findIndex((task) => task.id === selectedTaskId), [filteredTasks, selectedTaskId]);
  const selectedGrade = useMemo(() => latestGrade(ctx?.aiGrades ?? []), [ctx?.aiGrades]);
  const maxScore = ctx?.question?.score ?? selectedGrade?.max_score ?? 0;
  const rubricPoints = ctx?.question?.rubric?.points ?? [];
  const rubricTotal = useMemo(() => Object.values(draft.rubricSelections).reduce((sum, value) => sum + (Number(value) || 0), 0), [draft.rubricSelections]);

  const loadTasks = useCallback(async () => {
    setLoadingTasks(true);
    setTaskError(null);
    if (!hasSession) {
      setTasks([]);
      setSelectedTaskId("");
      setTaskError("当前没有有效登录会话，无法调用真实后端 API。");
      setLoadingTasks(false);
      return;
    }
    try {
      const result = await listReviewTasks();
      setTasks(result.tasks);
      setSelectedTaskId((current) => current || result.tasks[0]?.id || "");
    } catch (currentError) {
      setTaskError(formatError(currentError));
    } finally {
      setLoadingTasks(false);
    }
  }, [hasSession]);

  const loadContext = useCallback(async (taskId: string) => {
    if (!taskId || !hasSession) {
      setCtx(null);
      return;
    }
    setContextLoading(true);
    setContextError(null);
    setPreview(null);
    setScale(1);
    setRotation(0);
    setOffset({ x: 0, y: 0 });
    try {
      const detail = await getReviewTask(taskId);
      const next = await loadTaskContext(detail.task);
      setCtx(next);
      setDraft(createInitialDraft(next));
    } catch (currentError) {
      setContextError(formatError(currentError));
    } finally {
      setContextLoading(false);
    }
  }, [hasSession]);

  useEffect(() => {
    void loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    void loadContext(selectedTaskId);
  }, [loadContext, selectedTaskId]);

  useEffect(() => {
    return () => {
      if (preview?.url) {
        URL.revokeObjectURL(preview.url);
      }
    };
  }, [preview?.url]);

  const loadPreview = useCallback(async () => {
    if (!ctx?.page) {
      return;
    }
    setPreviewLoading(true);
    try {
      const file = await downloadFileBlob(ctx.page.file_asset_id);
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
  }, [ctx?.page, message]);

  useEffect(() => {
    if (ctx?.page) {
      void loadPreview();
    }
  }, [ctx?.page, loadPreview]);

  const refreshCurrent = async () => {
    if (selectedTaskId) {
      await loadContext(selectedTaskId);
      await loadTasks();
    }
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

  const goNext = () => {
    const next = filteredTasks.slice(selectedIndex + 1).find((task) => task.status !== "submitted" && task.status !== "completed");
    if (next) {
      setSelectedTaskId(next.id);
      return;
    }
    const fallback = filteredTasks.find((task) => task.status !== "submitted" && task.status !== "completed");
    setSelectedTaskId(fallback?.id ?? "");
  };

  const submitGrade = async () => {
    if (!ctx) {
      message.error("请先选择任务");
      return;
    }
    const score = Number(draft.score);
    if (!Number.isFinite(score) || score < 0 || score > maxScore) {
      message.error("最终分必须在 0 到题目满分之间");
      return;
    }
    await runAction(
      "submit",
      async () => {
        const selections: RubricSelection[] = Object.entries(draft.rubricSelections)
          .filter(([, value]) => Number(value) > 0)
          .map(([point_id, value]) => ({ point_id, score: Number(value) }));
        await submitHumanGrade(ctx.task.id, {
          score,
          rubric_selections: selections,
          comments: draft.comments,
          private_note: draft.privateNote,
          student_feedback: draft.studentFeedback,
          reason: draft.reason || "manual review completed"
        });
        await loadTasks();
        goNext();
      },
      "人工评分已提交"
    );
  };

  const markDispute = async () => {
    if (!ctx) {
      return;
    }
    const reason = draft.disputeReason.trim() || "marked for second look";
    await runAction(
      "return",
      async () => {
        await returnReviewTask(ctx.task.id, reason);
        await refreshCurrent();
      },
      "已标记争议并退回重评"
    );
  };

  const adoptAiScore = () => {
    if (!selectedGrade) {
      message.warning("当前任务没有 AI 建议分");
      return;
    }
    setDraft((current) => ({
      ...current,
      score: selectedGrade.suggested_score,
      studentFeedback: current.studentFeedback || selectedGrade.student_feedback || "",
      privateNote: current.privateNote || selectedGrade.teacher_note || ""
    }));
  };

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (isInputTarget(event.target)) {
        return;
      }
      if (event.ctrlKey && event.key === "Enter") {
        event.preventDefault();
        if (canSubmit) {
          void submitGrade();
        }
        return;
      }
      const key = event.key.toLowerCase();
      if (/^[1-9]$/.test(key)) {
        const value = Number(key);
        if (value <= maxScore) {
          setDraft((current) => ({ ...current, score: value }));
        }
        return;
      }
      if (key === "a") {
        adoptAiScore();
        return;
      }
      if (key === "r" && canReturn) {
        setDraft((current) => ({ ...current, disputeReason: current.disputeReason || "needs second look" }));
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  });

  const viewerTransform = `translate(${offset.x}px, ${offset.y}px) scale(${scale}) rotate(${rotation}deg)`;

  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    setDragging(true);
    setDragStart({ x: event.clientX - offset.x, y: event.clientY - offset.y });
    viewportRef.current?.setPointerCapture(event.pointerId);
  };

  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!dragging) {
      return;
    }
    setOffset({ x: event.clientX - dragStart.x, y: event.clientY - dragStart.y });
  };

  const onPointerUp = (event: React.PointerEvent<HTMLDivElement>) => {
    setDragging(false);
    viewportRef.current?.releasePointerCapture(event.pointerId);
  };

  const renderViewerContent = () => {
    if (previewLoading) {
      return <LoadingState label="正在读取答卷页面" />;
    }
    if (!preview) {
      return <EmptyState title="暂无答卷页面" description="当前任务没有可下载的页面文件。" />;
    }
    const isImage = preview.contentType.startsWith("image/");
    const isPDF = preview.contentType === "application/pdf";
    const ocrText = ctx?.ocrResults.map((item) => item.text).filter(Boolean).join("\n") || "当前页面暂无 OCR 结果。";
    return (
      <div
        ref={viewportRef}
        className={dragging ? "answer-viewer dragging" : "answer-viewer"}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
      >
        <div className="answer-viewer-stage" style={{ transform: viewerTransform }}>
          {viewerMode === "ocr" ? (
            <pre className="ocr-overlay-text">{ocrText}</pre>
          ) : isImage ? (
            <img src={preview.url} alt={preview.filename ?? "答卷页面"} />
          ) : isPDF ? (
            <iframe title={preview.filename ?? "答卷 PDF"} src={preview.url} />
          ) : (
            <Alert type="info" showIcon message="该文件类型不支持内嵌预览" description="可在新窗口打开或下载后查看。" />
          )}
          {viewerMode === "ocr"
            ? ctx?.ocrResults.slice(0, 12).map((result) =>
                result.bbox?.length === 4 ? (
                  <span
                    className="ocr-bbox"
                    key={result.id}
                    style={{ left: result.bbox[0], top: result.bbox[1], width: Math.max(12, result.bbox[2] - result.bbox[0]), height: Math.max(12, result.bbox[3] - result.bbox[1]) }}
                    title={result.text}
                  />
                ) : null
              )
            : null}
        </div>
      </div>
    );
  };

  const renderEvidence = () => {
    if (!selectedGrade) {
      return <EmptyState title="暂无 AI 建议" description="可先记录答案文本，再执行规则判分或主观题 AI 接口。" />;
    }
    return (
      <div className="evidence-stack">
        <div className="ai-score-strip">
          <div>
            <span>建议分</span>
            <strong>
              {selectedGrade.suggested_score} / {selectedGrade.max_score}
            </strong>
          </div>
          <div>
            <span>置信度</span>
            <Progress percent={Math.round(selectedGrade.confidence * 100)} size="small" status={selectedGrade.confidence >= 0.65 ? "normal" : "exception"} />
          </div>
          <StatusTag tone={selectedGrade.mock ? "warning" : confidenceTone(selectedGrade.confidence)}>{gradeMockLabel(selectedGrade)}</StatusTag>
        </div>

        <Tabs
          size="small"
          items={[
            {
              key: "points",
              label: "采分点",
              children: (
                <div className="point-result-list">
                  <List
                    size="small"
                    dataSource={selectedGrade.matched_points}
                    locale={{ emptyText: <Empty description="暂无命中采分点" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
                    renderItem={(point) => (
                      <List.Item>
                        <StatusTag tone="success">{`${point.score} 分`}</StatusTag>
                        <span>{point.label || point.code}</span>
                      </List.Item>
                    )}
                  />
                  <List
                    size="small"
                    dataSource={selectedGrade.missing_points}
                    locale={{ emptyText: <Empty description="暂无缺失采分点" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
                    renderItem={(point) => (
                      <List.Item>
                        <StatusTag tone="danger">{`${point.score} 分`}</StatusTag>
                        <span>{point.label || point.code}</span>
                      </List.Item>
                    )}
                  />
                </div>
              )
            },
            {
              key: "evidence",
              label: "证据",
              children: (
                <List
                  size="small"
                  dataSource={selectedGrade.evidence}
                  locale={{ emptyText: <Empty description="暂无证据" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
                  renderItem={(item) => (
                    <List.Item>
                      <div className="evidence-item">
                        <strong>{item.type || "evidence"}</strong>
                        <span>{item.answer_text || item.rule || item.standard_answer || "无文本证据"}</span>
                        {item.bbox ? <code>[{item.bbox.join(", ")}]</code> : null}
                      </div>
                    </List.Item>
                  )}
                />
              )
            },
            {
              key: "risk",
              label: "风险",
              children: (
                <Space wrap>
                  {selectedGrade.risk_flags.length > 0 ? selectedGrade.risk_flags.map((flag) => <StatusTag key={flag} tone="warning">{flag}</StatusTag>) : <StatusTag tone="success">无风险标记</StatusTag>}
                  {selectedGrade.needs_human_review ? <StatusTag tone="danger">需要人工复核</StatusTag> : <StatusTag tone="success">可自动通过</StatusTag>}
                </Space>
              )
            }
          ]}
        />
      </div>
    );
  };

  const renderEvidenceJob = () => {
    if (!ctx?.evidenceJob) {
      return <EmptyState title="暂无证据校验结果" description="点击证据校验后会显示真实校验任务结果。" />;
    }
    const job = ctx.evidenceJob;
    return (
      <div className="evidence-job">
        <Space>
          <StatusTag tone={job.result.passed ? "success" : "danger"}>{job.result.passed ? "通过" : "未通过"}</StatusTag>
          {job.needs_human_review ? <StatusTag tone="warning">需人工复核</StatusTag> : null}
        </Space>
        <List
          size="small"
          dataSource={[...job.result.failed, ...job.result.warnings]}
          locale={{ emptyText: <Empty description="无失败项或警告" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
          renderItem={(item) => (
            <List.Item>
              <span>{item.code}</span>
              <span className="muted">{item.message}</span>
            </List.Item>
          )}
        />
      </div>
    );
  };

  return (
    <div className="grading-shell">
      <section className="grading-topbar">
        <div>
          <Space>
            <h1>阅卷工作台</h1>
          </Space>
          <p>处理 review_task，核对原图、OCR、AI 建议和 Rubric 后提交人工分。</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} onClick={() => void refreshCurrent()} loading={loadingTasks || contextLoading}>
            刷新
          </Button>
          <Button icon={<Undo2 size={16} />} onClick={goNext} disabled={filteredTasks.length === 0}>
            下一份
          </Button>
          <Button type="primary" icon={<Save size={16} />} disabled={!canSubmit || !ctx || ctx.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void submitGrade()}>
            提交评分
          </Button>
        </Space>
      </section>

      {!hasSession ? (
        <Alert
          type="warning"
          showIcon
          message="未检测到真实后端访问令牌"
            description="核对答题材料、评分细则和已有建议后提交人工评分。"
        />
      ) : null}

      <section className="grading-taskbar">
        <Select className="toolbar-select" value={taskFilter} options={taskFilterOptions} onChange={setTaskFilter} />
        <Input prefix={<Search size={16} />} placeholder="搜索任务、匿名码、题号" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <span className="muted">{filteredTasks.length} / {tasks.length} 个任务</span>
      </section>

      <section className="grading-workspace">
        <aside className="grading-task-list">
          {loadingTasks ? (
            <LoadingState label="正在读取 review_task" />
          ) : taskError ? (
            <ErrorState message={taskError} onRetry={() => void loadTasks()} />
          ) : filteredTasks.length === 0 ? (
            <EmptyState title="暂无阅卷任务" description="当前筛选下没有后端返回的 review_task。" />
          ) : (
            <List
              dataSource={filteredTasks}
              renderItem={(task) => (
                <List.Item className={task.id === selectedTaskId ? "grading-task-item active" : "grading-task-item"} onClick={() => setSelectedTaskId(task.id)}>
                  <div>
                    <strong>{task.anonymous_code || "匿名码未返回"}</strong>
                    <span>{task.question_no} · {sourceLabels[task.source] ?? task.source}</span>
                  </div>
                  <div>
                    <StatusTag tone={taskTone(task.status)}>{taskStatusLabels[task.status] ?? task.status}</StatusTag>
                    <span>P{task.priority}</span>
                  </div>
                </List.Item>
              )}
            />
          )}
        </aside>

        {contextLoading ? (
          <main className="grading-main-empty">
            <LoadingState label="正在拼装阅卷上下文" />
          </main>
        ) : contextError ? (
          <main className="grading-main-empty">
            <ErrorState message={contextError} onRetry={() => void loadContext(selectedTaskId)} />
          </main>
        ) : !ctx ? (
          <main className="grading-main-empty">
            <EmptyState title="请选择任务" description="选择左侧 review_task 后进入阅卷工作台。" />
          </main>
        ) : (
          <main className="grading-main">
            {ctx.warnings.length > 0 ? <Alert type="warning" showIcon message="上下文不完整" description={ctx.warnings.join("；")} /> : null}

            <section className="grading-context-row">
              <Descriptions bordered size="small" column={4}>
                <Descriptions.Item label="匿名码">{ctx.task.anonymous_code}</Descriptions.Item>
                <Descriptions.Item label="题号">{ctx.task.question_no}</Descriptions.Item>
                <Descriptions.Item label="来源">{sourceLabels[ctx.task.source] ?? ctx.task.source}</Descriptions.Item>
                <Descriptions.Item label="状态">{taskStatusLabels[ctx.task.status] ?? ctx.task.status}</Descriptions.Item>
              </Descriptions>
            </section>

            <section className="grading-panels">
              <section className="answer-panel">
                <div className="panel-head">
                  <div>
                    <h2>原始答卷</h2>
                    <p>{preview?.filename ?? ctx.page?.file_asset_id ?? "未定位页面文件"}</p>
                  </div>
                  <Space wrap>
                    <Segmented<ViewerMode> size="small" value={viewerMode} options={[{ label: "原图", value: "original" }, { label: "OCR", value: "ocr" }]} onChange={setViewerMode} />
                    <Tooltip title="缩小">
                      <Button icon={<ZoomOut size={14} />} onClick={() => setScale((value) => Math.max(0.4, Number((value - 0.1).toFixed(2))))} />
                    </Tooltip>
                    <span className="viewer-scale">{Math.round(scale * 100)}%</span>
                    <Tooltip title="放大">
                      <Button icon={<ZoomIn size={14} />} onClick={() => setScale((value) => Math.min(3, Number((value + 0.1).toFixed(2))))} />
                    </Tooltip>
                    <Button icon={<RotateCcw size={14} />} onClick={() => setRotation((value) => (value + 90) % 360)} />
                    <Button icon={<Maximize2 size={14} />} onClick={() => { setScale(1); setRotation(0); setOffset({ x: 0, y: 0 }); }} />
                  </Space>
                </div>
                {renderViewerContent()}
              </section>

              <section className="evidence-panel">
                <div className="panel-head">
                  <div>
                    <h2>AI 证据</h2>
                    <p>{gradeMockLabel(selectedGrade)}</p>
                  </div>
                  <Space wrap>
                    <Button
                      icon={<FileText size={14} />}
                  disabled={!canGrade || !ctx.segment}
                      loading={actioning === "record-answer"}
                      onClick={() =>
                        void runAction(
                          "record-answer",
                          async () => {
                            if (!ctx.segment) {
                              throw new Error("缺少 answer_segment");
                            }
                            await recordSegmentAnswer(ctx.segment.id, {
                              answer_text: draft.answerText,
                              answer_payload: { text: draft.answerText },
                              source: "ocr_text",
                              confidence: ctx.ocrResults[0]?.confidence
                            });
                          },
                          "答案文本已记录"
                        )
                      }
                    >
                      记录答案
                    </Button>
                    <Button
                      icon={<Play size={14} />}
                  disabled={!canGrade || !ctx.segment}
                      loading={actioning === "rule"}
                      onClick={() =>
                        void runAction(
                          "rule",
                          async () => {
                            if (!ctx.segment) {
                              throw new Error("缺少 answer_segment");
                            }
                            await createRuleGrade(ctx.segment.id);
                            await loadContext(ctx.task.id);
                          },
                          "规则判分已生成"
                        )
                      }
                    >
                      规则判分
                    </Button>
                    <Button
                      icon={<MessageSquareText size={14} />}
                  disabled={!canGrade || !ctx.segment}
                      loading={actioning === "subjective"}
                      onClick={() =>
                        void runAction(
                          "subjective",
                          async () => {
                            if (!ctx.segment) {
                              throw new Error("缺少 answer_segment");
                            }
                            await createSubjectiveAiGrade(ctx.segment.id);
                            await loadContext(ctx.task.id);
                          },
                          "主观题 AI 接口已调用"
                        )
                      }
                    >
                      主观题 AI
                    </Button>
                    <Button
                      icon={<BadgeCheck size={14} />}
                  disabled={!canVerifyEvidence || !selectedGrade}
                      loading={actioning === "evidence"}
                      onClick={() =>
                        void runAction(
                          "evidence",
                          async () => {
                            if (!selectedGrade) {
                              throw new Error("缺少 AI grade");
                            }
                            const result = await verifyEvidence(selectedGrade.id);
                            setCtx((current) => (current ? { ...current, evidenceJob: result.job } : current));
                          },
                          "证据校验完成"
                        )
                      }
                    >
                      证据校验
                    </Button>
                  </Space>
                </div>
                <Input.TextArea
                  value={draft.answerText}
                  rows={4}
                  placeholder="OCR 文本或人工录入答案"
                  onChange={(event) => setDraft((current) => ({ ...current, answerText: event.target.value }))}
                />
                {renderEvidence()}
                {renderEvidenceJob()}
              </section>

              <aside className="score-panel">
                <div className="panel-head">
                  <div>
                    <h2>人工评分</h2>
                    <p>满分 {maxScore || "未返回"}</p>
                  </div>
                  {selectedGrade ? (
                    <Button icon={<Eye size={14} />} onClick={adoptAiScore}>
                      采纳 AI
                    </Button>
                  ) : null}
                </div>

                <div className="score-input-row">
                  <InputNumber
                    min={0}
                    max={maxScore || undefined}
                    precision={1}
                    value={draft.score}
                    placeholder="最终分"
                    onChange={(value) => setDraft((current) => ({ ...current, score: value === null ? null : Number(value) }))}
                  />
                  <span>/ {maxScore || "-"}</span>
                </div>

                {rubricPoints.length > 0 ? (
                  <div className="rubric-score-list">
                    <div className="rubric-score-head">
                      <strong>Rubric</strong>
                      <span>{rubricTotal} 分</span>
                    </div>
                    {rubricPoints.map((point) => (
                      <label className="rubric-score-item" key={point.id}>
                        <Checkbox
                          checked={(draft.rubricSelections[point.id] ?? 0) > 0}
                          onChange={(event) =>
                            setDraft((current) => ({
                              ...current,
                              rubricSelections: { ...current.rubricSelections, [point.id]: event.target.checked ? point.score : 0 }
                            }))
                          }
                        />
                        <span>{pointLabel(point)}</span>
                        <InputNumber
                          min={0}
                          max={point.score}
                          precision={1}
                          value={draft.rubricSelections[point.id] ?? 0}
                          onChange={(value) =>
                            setDraft((current) => ({
                              ...current,
                              rubricSelections: { ...current.rubricSelections, [point.id]: Number(value ?? 0) }
                            }))
                          }
                        />
                      </label>
                    ))}
                  </div>
                ) : (
                  <Alert type="info" showIcon message="当前题目没有 Rubric" description="可提交总分，但不会提交 rubric_selections。" />
                )}

                <Select
                  mode="tags"
                  placeholder="常用评语"
                  options={commentPresets.map((item) => ({ label: item, value: item }))}
                  onChange={(values) => setDraft((current) => ({ ...current, comments: values.join("；") }))}
                />
                <Input.TextArea rows={3} placeholder="教师评语" value={draft.comments} onChange={(event) => setDraft((current) => ({ ...current, comments: event.target.value }))} />
                <Input.TextArea rows={3} placeholder="学生可见反馈" value={draft.studentFeedback} onChange={(event) => setDraft((current) => ({ ...current, studentFeedback: event.target.value }))} />
                <Input.TextArea rows={2} placeholder="教师私密备注" value={draft.privateNote} onChange={(event) => setDraft((current) => ({ ...current, privateNote: event.target.value }))} />
                <Input placeholder="提交原因" value={draft.reason} onChange={(event) => setDraft((current) => ({ ...current, reason: event.target.value }))} />
                <Input placeholder="争议原因" prefix={<Flag size={14} />} value={draft.disputeReason} onChange={(event) => setDraft((current) => ({ ...current, disputeReason: event.target.value }))} />

                <Space wrap>
                  <Button danger icon={<Flag size={16} />} disabled={!canReturn || !ctx} loading={actioning === "return"} onClick={() => void markDispute()}>
                    标记争议
                  </Button>
                  <Button type="primary" icon={<CheckCircle2 size={16} />} disabled={!canSubmit || !ctx || ctx.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void submitGrade()}>
                    提交
                  </Button>
                </Space>
              </aside>
            </section>

          </main>
        )}
      </section>
    </div>
  );
}
