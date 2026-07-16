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
  Modal,
  Popconfirm,
  Progress,
  Segmented,
  Select,
  Space,
  Table,
  Tabs,
  Tooltip
} from "antd";
import {
  BadgeCheck,
  CheckCircle2,
  CircleStop,
  Eye,
  Flag,
  LogOut,
  Maximize2,
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
import { loadReviewDraftFallback, removeReviewDraftFallback, saveReviewDraftFallback } from "../auth/reviewDraftFallback";
import { downloadFileBlob } from "../api/files";
import { type Question, type RubricPoint } from "../api/papers";
import {
  claimNextReviewTask,
  cancelScoringRun,
  getScoringRun,
  getReviewTask,
  getReviewWorkspace,
  getReviewDraft,
  downloadReviewWorkspaceImage,
  getScoringSummary,
  listAiGrades,
  listReviewTasks,
  returnReviewTask,
  renewReviewTask,
  releaseReviewTask,
  retryFailedScoringRun,
  saveReviewDraft,
  submitHumanGrade,
  startScoringRun,
  verifyEvidence,
  type AiGrade,
  type EvidenceJob,
  type ReviewTask,
  type RubricSelection,
  type ScoringRunDetail,
  type ScoringSummary
} from "../api/review";
import {
  type AnswerSegment,
  type OcrResult,
  type OcrTask,
  type SubmissionPage
} from "../api/submissions";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { ResponsiveTable } from "../components/ResponsiveTable";
import { StatusTag } from "../components/StatusTag";
import type { StatusTone } from "../types";

type TaskFilter = "active" | "pending" | "assigned" | "in_progress" | "returned" | "submitted";
type ViewerMode = "segment" | "original" | "ocr";
type DraftSaveStatus = "idle" | "saving" | "saved" | "offline" | "conflict" | "error";

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
  ocrText: string;
  segmentImageUrl: string;
  originalImageUrl?: string;
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

interface DraftFallbackSnapshot {
  draft: ScoreDraft;
  viewer: {
    mode: ViewerMode;
    scale: number;
    rotation: number;
    offset: { x: number; y: number };
  };
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
  manual_sample: "人工抽检",
  omr_ambiguous: "涂卡结果待确认",
  rule_review_required: "规则评分待确认",
  grading_failure: "评分处理失败"
};

const taskStatusLabels: Record<string, string> = {
  pending: "待分配",
  assigned: "已分配",
  in_progress: "处理中",
  returned: "退回",
  submitted: "已提交",
  completed: "已完成"
};

const scoringRunStatusLabels: Record<string, string> = {
  queued: "等待处理",
  processing: "处理中",
  needs_review: "等待人工",
  failed: "处理失败",
  cancelling: "正在取消",
  cancelled: "已取消",
  completed: "已完成"
};

const scoringItemStateLabels: Record<string, string> = {
  pending: "等待处理",
  processing: "处理中",
  review: "等待人工",
  confirmed: "已确认",
  failed: "失败",
  cancelling: "正在取消",
  cancelled: "已取消"
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
  const [workspaceResult, gradeResult] = await Promise.all([getReviewWorkspace(task.id), listAiGrades(task.answer_segment_id)]);
  const workspace = workspaceResult.workspace;
  const question = { ...workspace.context.question, rubric: workspace.context.rubric ?? workspace.context.question.rubric };
  const segment = { id: task.answer_segment_id, tenant_id: task.tenant_id, submission_id: task.submission_id, submission_page_id: "", question_id: task.question_id, question_no: task.question_no, bbox: [], source: "workspace", status: workspace.segment_status, created_at: task.created_at, confidence: workspace.segment_confidence } as AnswerSegment;
  return { task: workspace.task, segment, question, pages: [], ocrTasks: [], ocrResults: [], aiGrades: gradeResult.grades, warnings: [], ocrText: workspace.context.ocr_text, segmentImageUrl: workspace.segment_image_url, originalImageUrl: workspace.original_image_url };
}

function createInitialDraft(ctx: WorkbenchContext | null): ScoreDraft {
  const grade = latestGrade(ctx?.aiGrades ?? []);
  const ocrText = ctx?.ocrText || ctx?.ocrResults.map((item) => item.text).filter(Boolean).join("\n") || "";
  const selections: Record<string, number> = {};
  for (const point of ctx?.question?.rubric?.points ?? []) {
    selections[point.id] = 0;
  }
  return {
    score: grade && !grade.mock ? grade.suggested_score : null,
    comments: "",
    privateNote: "",
    studentFeedback: grade?.student_feedback ?? "",
    reason: "教师复核完成",
    disputeReason: "",
    rubricSelections: selections,
    answerText: ocrText
  };
}

function createDraftSnapshot(draft: ScoreDraft, mode: ViewerMode, scale: number, rotation: number, offset: { x: number; y: number }): DraftFallbackSnapshot {
  return { draft, viewer: { mode, scale, rotation, offset } };
}

function fallbackSnapshot(value: unknown, initial: ScoreDraft): DraftFallbackSnapshot | null {
  if (!value || typeof value !== "object") return null;
  const root = value as { draft?: unknown; viewer?: unknown };
  if (!root.draft || typeof root.draft !== "object" || !root.viewer || typeof root.viewer !== "object") return null;
  const cachedDraft = root.draft as Partial<ScoreDraft>;
  const cachedViewer = root.viewer as { mode?: unknown; scale?: unknown; rotation?: unknown; offset?: unknown };
  const selections = cachedDraft.rubricSelections && typeof cachedDraft.rubricSelections === "object" && !Array.isArray(cachedDraft.rubricSelections)
    ? Object.fromEntries(Object.entries(cachedDraft.rubricSelections).flatMap(([key, value]) => typeof value === "number" && Number.isFinite(value) ? [[key, value]] : []))
    : initial.rubricSelections;
  const rawOffset = cachedViewer.offset && typeof cachedViewer.offset === "object" ? cachedViewer.offset as { x?: unknown; y?: unknown } : {};
  const score = typeof cachedDraft.score === "number" && Number.isFinite(cachedDraft.score) ? cachedDraft.score : null;
  const mode = cachedViewer.mode === "original" || cachedViewer.mode === "ocr" || cachedViewer.mode === "segment" ? cachedViewer.mode : "segment";
  const scale = typeof cachedViewer.scale === "number" && Number.isFinite(cachedViewer.scale) ? Math.min(4, Math.max(0.25, cachedViewer.scale)) : 1;
  const rotation = typeof cachedViewer.rotation === "number" && Number.isFinite(cachedViewer.rotation) ? cachedViewer.rotation : 0;
  return {
    draft: {
      ...initial,
      score,
      comments: typeof cachedDraft.comments === "string" ? cachedDraft.comments : initial.comments,
      privateNote: typeof cachedDraft.privateNote === "string" ? cachedDraft.privateNote : initial.privateNote,
      studentFeedback: typeof cachedDraft.studentFeedback === "string" ? cachedDraft.studentFeedback : initial.studentFeedback,
      reason: typeof cachedDraft.reason === "string" ? cachedDraft.reason : initial.reason,
      disputeReason: typeof cachedDraft.disputeReason === "string" ? cachedDraft.disputeReason : initial.disputeReason,
      rubricSelections: selections,
      answerText: typeof cachedDraft.answerText === "string" ? cachedDraft.answerText : initial.answerText
    },
    viewer: {
      mode,
      scale,
      rotation,
      offset: {
        x: typeof rawOffset.x === "number" && Number.isFinite(rawOffset.x) ? rawOffset.x : 0,
        y: typeof rawOffset.y === "number" && Number.isFinite(rawOffset.y) ? rawOffset.y : 0
      }
    }
  };
}

export function GradingWorkbenchPage({ canWork, canGrade, canVerifyEvidence, canReturn, currentUserId, initialExamId = "", personalScope = false }: { canWork: boolean; canGrade: boolean; canVerifyEvidence: boolean; canReturn: boolean; currentUserId: string; initialExamId?: string; personalScope?: boolean }) {
  const { message } = App.useApp();
  const hasSession = true;
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
  const [viewerMode, setViewerMode] = useState<ViewerMode>("segment");
  const [scale, setScale] = useState(1);
  const [rotation, setRotation] = useState(0);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  const [actioning, setActioning] = useState<string | null>(null);
  const [scoringSummary, setScoringSummary] = useState<ScoringSummary | null>(null);
  const [scoringLoading, setScoringLoading] = useState(false);
  const [scoringRunDetail, setScoringRunDetail] = useState<ScoringRunDetail | null>(null);
  const [scoringDetailOpen, setScoringDetailOpen] = useState(false);
  const [online, setOnline] = useState(() => typeof navigator === "undefined" || navigator.onLine);
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const [draftRevision, setDraftRevision] = useState(0);
  const [draftSaveStatus, setDraftSaveStatus] = useState<DraftSaveStatus>("idle");
  const [draftHydrated, setDraftHydrated] = useState(false);
  const lastSavedDraft = useRef("");

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

  useEffect(() => {
    if (loadingTasks || filteredTasks.some((task) => task.id === selectedTaskId)) {
      return;
    }
    setSelectedTaskId(filteredTasks[0]?.id ?? "");
  }, [filteredTasks, loadingTasks, selectedTaskId]);

  const selectedIndex = useMemo(() => filteredTasks.findIndex((task) => task.id === selectedTaskId), [filteredTasks, selectedTaskId]);
  const selectedGrade = useMemo(() => latestGrade(ctx?.aiGrades ?? []), [ctx?.aiGrades]);
  const maxScore = ctx?.question?.score ?? selectedGrade?.max_score ?? 0;
  const rubricPoints = ctx?.question?.rubric?.points ?? [];
  const rubricTotal = useMemo(() => Object.values(draft.rubricSelections).reduce((sum, value) => sum + (Number(value) || 0), 0), [draft.rubricSelections]);
  const ownsSelectedTask = Boolean(ctx?.task.assigned_to && ctx.task.assigned_to === currentUserId);
  const canSubmit = canWork && hasSession && ownsSelectedTask && Boolean(ctx && ["assigned", "in_progress", "returned"].includes(ctx.task.status));

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
      const result = await listReviewTasks(personalScope ? { assigned_to: currentUserId } : {});
      setTasks(result.tasks);
      setSelectedTaskId((current) => current || result.tasks.find((task) => ["assigned", "in_progress", "returned"].includes(task.status))?.id || "");
    } catch (currentError) {
      setTaskError(formatError(currentError));
    } finally {
      setLoadingTasks(false);
    }
  }, [currentUserId, hasSession, personalScope]);

  const loadScoringSummary = useCallback(async () => {
    if (!initialExamId || !canGrade) return;
    setScoringLoading(true);
    try {
      const result = await getScoringSummary(initialExamId);
      setScoringSummary(result.scoring_summary);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setScoringLoading(false);
    }
  }, [canGrade, initialExamId, message]);

  const startExamScoring = useCallback(async () => {
    if (!initialExamId || !canGrade) return;
    setActioning("start-scoring");
    try {
      await startScoringRun(initialExamId, `web-${crypto.randomUUID()}`);
      message.success("评分任务已生成");
      await Promise.all([loadScoringSummary(), loadTasks()]);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  }, [canGrade, initialExamId, loadScoringSummary, loadTasks, message]);

  const showScoringRunDetail = useCallback(async () => {
    const runId = scoringSummary?.run?.id;
    if (!runId) return;
    setActioning("scoring-detail");
    try {
      const detail = await getScoringRun(runId);
      setScoringRunDetail(detail);
      setScoringDetailOpen(true);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  }, [message, scoringSummary?.run?.id]);

  const retryFailedScoring = useCallback(async () => {
    const runId = scoringSummary?.run?.id;
    if (!runId) return;
    setActioning("retry-scoring");
    try {
      const result = await retryFailedScoringRun(runId);
      if (result.requeued > 0) {
        message.success(`已重新安排 ${result.requeued} 个失败项`);
      } else {
        message.info("当前没有可重新处理的失败项");
      }
      await Promise.all([loadScoringSummary(), loadTasks()]);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  }, [loadScoringSummary, loadTasks, message, scoringSummary?.run?.id]);

  const cancelCurrentScoringRun = useCallback(async () => {
    const runId = scoringSummary?.run?.id;
    if (!runId) return;
    setActioning("cancel-scoring");
    try {
      await cancelScoringRun(runId);
      message.success("本次评分已取消，未完成任务不会继续写入结果");
      await Promise.all([loadScoringSummary(), loadTasks()]);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  }, [loadScoringSummary, loadTasks, message, scoringSummary?.run?.id]);

  const loadContext = useCallback(async (taskId: string) => {
    if (!taskId || !hasSession) {
      setCtx(null);
      setDraftHydrated(false);
      return;
    }
    setContextLoading(true);
    setContextError(null);
    setDraftHydrated(false);
    setPreview(null);
    setScale(1);
    setRotation(0);
    setOffset({ x: 0, y: 0 });
    try {
      const detail = await getReviewTask(taskId);
      const next = await loadTaskContext(detail.task);
      setCtx(next);
      const initial = createInitialDraft(next);
      const draftResult = await getReviewDraft(taskId);
      let restored = initial;
      let revision = 0;
      let restoredViewer: DraftFallbackSnapshot["viewer"] = { mode: "segment", scale: 1, rotation: 0, offset: { x: 0, y: 0 } };
      let serverUpdatedAt = 0;
      if (draftResult.draft) {
        restored = {
          ...initial,
          score: draftResult.draft.score ?? null,
          comments: draftResult.draft.comments,
          privateNote: draftResult.draft.private_note,
          studentFeedback: draftResult.draft.student_feedback,
          rubricSelections: Object.fromEntries(draftResult.draft.rubric_selections.map((item) => [item.point_id, item.score]))
        };
        revision = draftResult.draft.revision;
        const viewer = draftResult.draft.viewer_state;
        const mode = ["segment", "original", "ocr"].includes(String(viewer.mode)) ? viewer.mode as ViewerMode : "segment";
        const scale = Number(viewer.scale ?? 1);
        const rotation = Number(viewer.rotation ?? 0);
        const savedOffset = viewer.offset as { x?: number; y?: number } | undefined;
        restoredViewer = { mode, scale, rotation, offset: { x: Number(savedOffset?.x ?? 0), y: Number(savedOffset?.y ?? 0) } };
        serverUpdatedAt = Date.parse(draftResult.draft.updated_at) || 0;
      }
      const serverSnapshot = createDraftSnapshot(restored, restoredViewer.mode, restoredViewer.scale, restoredViewer.rotation, restoredViewer.offset);
      const localDraft = loadReviewDraftFallback<unknown>(currentUserId, taskId);
      const localSnapshot = localDraft ? fallbackSnapshot(localDraft.snapshot, initial) : null;
      const useLocalDraft = Boolean(localDraft && localSnapshot && localDraft.updatedAt > serverUpdatedAt);
      if (useLocalDraft && localSnapshot) {
        restored = localSnapshot.draft;
        restoredViewer = localSnapshot.viewer;
        setDraftSaveStatus("offline");
      } else {
        if (localDraft) removeReviewDraftFallback(currentUserId, taskId);
        setDraftSaveStatus(draftResult.draft ? "saved" : "idle");
      }
      setDraft(restored);
      setDraftRevision(revision);
      setViewerMode(restoredViewer.mode);
      setScale(restoredViewer.scale);
      setRotation(restoredViewer.rotation);
      setOffset(restoredViewer.offset);
      lastSavedDraft.current = JSON.stringify(serverSnapshot);
      setDraftHydrated(true);
    } catch (currentError) {
      setContextError(formatError(currentError));
    } finally {
      setContextLoading(false);
    }
  }, [currentUserId, hasSession]);

  useEffect(() => {
    const updateOnlineState = () => setOnline(navigator.onLine);
    window.addEventListener("online", updateOnlineState);
    window.addEventListener("offline", updateOnlineState);
    return () => {
      window.removeEventListener("online", updateOnlineState);
      window.removeEventListener("offline", updateOnlineState);
    };
  }, []);

  useEffect(() => {
    if (!draftHydrated || !ctx || ctx.task.status === "submitted") return;
    const snapshotValue = createDraftSnapshot(draft, viewerMode, scale, rotation, offset);
    const snapshot = JSON.stringify(snapshotValue);
    if (snapshot === lastSavedDraft.current) return;
    saveReviewDraftFallback(currentUserId, ctx.task.id, snapshotValue);
    if (draftSaveStatus === "conflict") return;
    const timer = window.setTimeout(async () => {
      if (!online) {
        setDraftSaveStatus("offline");
        return;
      }
      setDraftSaveStatus("saving");
      try {
        const selections = Object.entries(draft.rubricSelections).filter(([, value]) => Number(value) > 0).map(([point_id, value]) => ({ point_id, score: Number(value) }));
        const result = await saveReviewDraft(ctx.task.id, {
          score: draft.score,
          rubric_selections: selections,
          comments: draft.comments,
          private_note: draft.privateNote,
          student_feedback: draft.studentFeedback,
          viewer_state: { mode: viewerMode, scale, rotation, offset },
          expected_revision: draftRevision
        });
        setDraftRevision(result.draft.revision);
        lastSavedDraft.current = snapshot;
        removeReviewDraftFallback(currentUserId, ctx.task.id);
        setDraftSaveStatus("saved");
      } catch (currentError) {
        setDraftSaveStatus(currentError instanceof ApiClientError && currentError.status === 409 ? "conflict" : (!navigator.onLine ? "offline" : "error"));
      }
    }, 1200);
    return () => window.clearTimeout(timer);
  }, [ctx, currentUserId, draft, draftHydrated, draftRevision, offset, online, rotation, scale, viewerMode]);

  useEffect(() => {
    void loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    void loadScoringSummary();
  }, [loadScoringSummary]);

  useEffect(() => {
    void loadContext(selectedTaskId);
  }, [loadContext, selectedTaskId]);

  useEffect(() => {
    if (!selectedTaskId || !ctx || !["assigned", "in_progress", "returned"].includes(ctx.task.status)) return;
    const renew = () => void renewReviewTask(selectedTaskId).catch(() => setDraftSaveStatus("error"));
    const timer = window.setInterval(renew, 5 * 60 * 1000);
    return () => window.clearInterval(timer);
  }, [ctx, selectedTaskId]);

  useEffect(() => {
    return () => {
      if (preview?.url) {
        URL.revokeObjectURL(preview.url);
      }
    };
  }, [preview?.url]);

  const loadPreview = useCallback(async () => {
    if (!ctx || viewerMode === "ocr" || (viewerMode === "original" && !ctx.originalImageUrl) || (viewerMode === "segment" && !ctx.segmentImageUrl)) {
      return;
    }
    setPreviewLoading(true);
    setPreview((current) => {
      if (current?.url) URL.revokeObjectURL(current.url);
      return null;
    });
    try {
      const file = await downloadReviewWorkspaceImage(viewerMode === "segment" ? ctx.segmentImageUrl : ctx.originalImageUrl!);
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
  }, [ctx, message, viewerMode]);

  useEffect(() => {
    if (ctx && viewerMode !== "ocr") {
      void loadPreview();
    }
  }, [ctx, loadPreview, viewerMode]);

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

  const goNext = async () => {
    const next = filteredTasks.slice(selectedIndex + 1).find((task) => task.status !== "submitted" && task.status !== "completed");
    if (next) {
      setSelectedTaskId(next.id);
      return;
    }
    const fallback = filteredTasks.find((task) => task.status !== "submitted" && task.status !== "completed");
    if (fallback) {
      setSelectedTaskId(fallback.id);
      return;
    }
    try {
      const result = await claimNextReviewTask(initialExamId);
      setTasks((current) => current.some((item) => item.id === result.task.id)
        ? current.map((item) => item.id === result.task.id ? result.task : item)
        : [...current, result.task]);
      setSelectedTaskId(result.task.id);
    } catch (currentError) {
      if (currentError instanceof ApiClientError && currentError.status === 404) {
        message.info("当前没有待领取的阅卷任务");
        setSelectedTaskId("");
        return;
      }
      message.error(formatError(currentError));
    }
  };

  const releaseCurrentTask = async () => {
    if (!selectedTaskId || !ownsSelectedTask) return;
    await runAction("release", async () => {
      const result = await releaseReviewTask(selectedTaskId);
      setTasks((current) => current.map((item) => item.id === result.task.id ? result.task : item));
      setSelectedTaskId("");
    }, "任务已释放，草稿仍会保留");
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
          reason: draft.reason || "教师复核完成"
        });
        removeReviewDraftFallback(currentUserId, ctx.task.id);
        const [refreshed] = await Promise.all([listReviewTasks(), loadScoringSummary()]);
        setTasks(refreshed.tasks);
        const nextTask = refreshed.tasks.find((task) => (!initialExamId || task.exam_id === initialExamId) && ["assigned", "in_progress", "returned"].includes(task.status));
        if (nextTask) {
          setSelectedTaskId(nextTask.id);
        } else if (!refreshed.tasks.some((task) => (!initialExamId || task.exam_id === initialExamId) && task.status === "pending")) {
          setSelectedTaskId("");
        } else {
          try {
            const claimed = await claimNextReviewTask(initialExamId);
            setTasks((current) => current.some((item) => item.id === claimed.task.id)
              ? current.map((item) => item.id === claimed.task.id ? claimed.task : item)
              : [...current, claimed.task]);
            setSelectedTaskId(claimed.task.id);
          } catch (claimError) {
            if (claimError instanceof ApiClientError && claimError.status === 404) setSelectedTaskId("");
            else throw claimError;
          }
        }
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
    if (scale <= 1) {
      return;
    }
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
    const ocrText = ctx?.ocrText || ctx?.ocrResults.map((item) => item.text).filter(Boolean).join("\n") || "当前页面暂无 OCR 结果。";
    return (
      <div
        ref={viewportRef}
        className={dragging ? "answer-viewer dragging" : "answer-viewer"}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
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
      return <EmptyState title="暂无 AI 建议" description="请依据答题区域与评分细则完成人工复核。" />;
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
      {initialExamId && canGrade ? <section className="grading-overview">
        <div className="grading-overview-head">
          <div><h2>评分进度</h2><p>系统只自动确认证据完整且规则明确的答案，其余进入人工队列。</p></div>
          <Space wrap>
            <Button icon={<RefreshCw size={15} />} loading={scoringLoading} onClick={() => void loadScoringSummary()}>刷新</Button>
            {scoringSummary?.run ? <Button icon={<Eye size={15} />} loading={actioning === "scoring-detail"} onClick={() => void showScoringRunDetail()}>查看处理明细</Button> : null}
            {scoringSummary?.run && scoringSummary.run.failed_count > 0 ? <Button icon={<RotateCcw size={15} />} loading={actioning === "retry-scoring"} onClick={() => void retryFailedScoring()}>重新处理失败项</Button> : null}
            {scoringSummary?.run && ["queued", "processing", "needs_review", "failed"].includes(scoringSummary.run.status) ? <Popconfirm title="取消本次评分？" description="未完成的自动处理和人工任务将停止，已保留的历史结果不会删除。" okText="取消评分" cancelText="保留" okButtonProps={{ danger: true }} onConfirm={() => void cancelCurrentScoringRun()}>
              <Button danger icon={<CircleStop size={15} />} loading={actioning === "cancel-scoring"}>取消评分</Button>
            </Popconfirm> : null}
            <Button type="primary" icon={<Play size={15} />} loading={actioning === "start-scoring"} onClick={() => void startExamScoring()}>开始评分</Button>
          </Space>
        </div>
        {scoringSummary?.run ? <div className="grading-run-strip">
          <StatusTag tone={scoringSummary.run.status === "completed" ? "success" : scoringSummary.run.status === "failed" ? "danger" : scoringSummary.run.status === "needs_review" ? "warning" : "processing"}>{scoringRunStatusLabels[scoringSummary.run.status] ?? scoringSummary.run.status}</StatusTag>
          <span>总计 <strong>{scoringSummary.run.total_count}</strong></span>
          <span>处理中 <strong>{scoringSummary.run.queued_count}</strong></span>
          <span>自动确认 <strong>{scoringSummary.run.auto_confirmed_count}</strong></span>
          <span>人工完成 <strong>{scoringSummary.run.human_confirmed_count}</strong></span>
          <span>待人工 <strong>{scoringSummary.run.review_count}</strong></span>
          <span>失败 <strong>{scoringSummary.run.failed_count}</strong></span>
        </div> : <Alert type="info" showIcon message="尚未开始评分" description="确认采集与切题完成后启动；缺少规则或证据的答案会自动进入人工队列。" />}
        <ResponsiveTable size="small" pagination={false} loading={scoringLoading} rowKey="question_id" dataSource={scoringSummary?.questions ?? []} columns={[
          { title: "题号", dataIndex: "question_no", width: 90 },
          { title: "题型", dataIndex: "question_type", width: 140 },
          { title: "答卷", dataIndex: "total", width: 80 },
          { title: "处理中", dataIndex: "queued", width: 90 },
          { title: "已确认", dataIndex: "confirmed", width: 90 },
          { title: "待人工", dataIndex: "review", width: 90 },
          { title: "失败", dataIndex: "failed", width: 80 }
        ]} />
      </section> : null}
      <Modal title="评分处理明细" open={scoringDetailOpen} onCancel={() => setScoringDetailOpen(false)} footer={<Button onClick={() => setScoringDetailOpen(false)}>关闭</Button>} width={900}>
        <ResponsiveTable size="small" pagination={{ pageSize: 12, showSizeChanger: false }} rowKey="answer_segment_id" dataSource={scoringRunDetail?.items ?? []} columns={[
          { title: "题号", dataIndex: "question_no", width: 84 },
          { title: "题型", dataIndex: "question_type", width: 130 },
          { title: "当前状态", dataIndex: "state", width: 112, render: (value: string) => <StatusTag tone={value === "confirmed" ? "success" : value === "failed" ? "danger" : value === "review" ? "warning" : "processing"}>{scoringItemStateLabels[value] ?? value}</StatusTag> },
          { title: "需要处理的原因", render: (_, item: ScoringRunDetail["items"][number]) => item.error_code || item.reason_code || (item.state === "confirmed" ? "已完成" : "-") },
          { title: "处理任务", render: (_, item: ScoringRunDetail["items"][number]) => item.runtime_status || item.review_status || "-", width: 120 }
        ]} />
      </Modal>
      <section className="grading-topbar">
        <div>
          <Space>
            <h1>阅卷工作台</h1>
          </Space>
          <p>查看学生答案，对照评分标准，确认得分后进入下一份。</p>
        </div>
        <Space wrap>
          <span className={`draft-save-status ${draftSaveStatus}`}>{draftSaveStatus === "saving" ? "正在保存" : draftSaveStatus === "saved" ? "草稿已保存" : draftSaveStatus === "offline" ? "离线草稿待同步" : draftSaveStatus === "conflict" ? "草稿冲突" : draftSaveStatus === "error" ? "草稿保存失败" : "尚未修改"}</span>
          <Button icon={<RefreshCw size={16} />} onClick={() => void refreshCurrent()} loading={loadingTasks || contextLoading}>
            刷新
          </Button>
          <Button icon={<Undo2 size={16} />} onClick={() => void goNext()} disabled={!canWork}>
            领取下一份
          </Button>
          <Tooltip title="释放当前任务并保留草稿">
            <Button icon={<LogOut size={16} />} disabled={!ownsSelectedTask} loading={actioning === "release"} onClick={() => void releaseCurrentTask()} aria-label="释放当前任务" />
          </Tooltip>
          <Button type="primary" icon={<Save size={16} />} disabled={!canSubmit || !ctx || ctx.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void submitGrade()}>
            确认并下一份
          </Button>
        </Space>
      </section>

      {draftSaveStatus === "conflict" ? <Alert type="error" showIcon message="草稿已被其他会话更新" description="为防止覆盖他人修改，自动保存已暂停。重新载入任务后再应用本地修改。" action={<Button onClick={() => void loadContext(selectedTaskId)}>重新载入</Button>} /> : draftSaveStatus === "offline" ? <Alert type="warning" showIcon message="当前离线，草稿已保存在本机" description="恢复网络后会按版本号同步；提交或退出后会清理本机草稿。" /> : draftSaveStatus === "error" ? <Alert type="warning" showIcon message="草稿暂未保存到服务端" description="本机保留了短期草稿；检查网络后系统会再次尝试保存。" /> : null}

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
            <LoadingState label="正在读取阅卷任务" />
          ) : taskError ? (
            <ErrorState message={taskError} onRetry={() => void loadTasks()} />
          ) : filteredTasks.length === 0 ? (
            <EmptyState title="暂无阅卷任务" description="当前筛选下没有需要处理的答卷。" />
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
                    {task.priority >= 80 ? <span>优先处理</span> : null}
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
            <EmptyState title="请选择一份答卷" description="从左侧队列选择，或领取下一份待阅答卷。" />
          </main>
        ) : (
          <main className="grading-main">
            {ctx.warnings.length > 0 ? <Alert type="warning" showIcon message="上下文不完整" description={ctx.warnings.join("；")} /> : null}

            <section className="grading-context-row">
              <Descriptions bordered size="small" column={{ xs: 1, sm: 2, lg: 4 }}>
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
                    <h2>学生答案</h2>
                    <p>{preview?.filename ?? (viewerMode === "segment" ? "当前题目裁图" : "原始答卷页")}</p>
                  </div>
                  <Space wrap>
                    <Segmented<ViewerMode> size="small" value={viewerMode} options={[{ label: "答题区域", value: "segment" }, { label: "原图", value: "original" }, { label: "OCR", value: "ocr" }]} onChange={setViewerMode} />
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
                    <h2>识别与建议</h2>
                    <p>{gradeMockLabel(selectedGrade)}</p>
                  </div>
                  <Space wrap>
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
                  readOnly
                  placeholder="当前没有可用的 OCR 文本"
                />
                {renderEvidence()}
                {renderEvidenceJob()}
              </section>

              <aside className="score-panel">
                <div className="panel-head">
                  <div>
                    <h2>评分标准</h2>
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
                      <strong>评分细则</strong>
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
                  <Alert type="info" showIcon message="当前题目没有评分细则" description="仍可根据参考答案直接填写最终得分。" />
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
                    确认并下一份
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
