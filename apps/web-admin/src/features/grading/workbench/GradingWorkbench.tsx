import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  App,
  Button,
  Checkbox,
  Drawer,
  Empty,
  Input,
  List,
  Modal,
  Popconfirm,
  Progress,
  Segmented,
  Select,
  Space,
  Tabs,
  Tooltip
} from "antd";
import {
  ArrowRight,
  Award,
  BadgeCheck,
  Check,
  CheckCircle2,
  CircleStop,
  Eye,
  Flag,
  LogOut,
  Maximize2,
  Play,
  RefreshCw,
  RotateCcw,
  Search,
  UserRoundCheck,
  ZoomIn,
  ZoomOut
} from "lucide-react";
import { ApiClientError } from "../../../api/client";
import { loadReviewDraftFallback, removeReviewDraftFallback, saveReviewDraftFallback } from "../../../auth/reviewDraftFallback";
import { displayNameOrUsername } from "../../../auth/session";
import { type Question, type RubricPoint } from "../../../api/papers";
import {
  assignReviewTask,
  batchAssignReviewTasks,
  claimNextReviewTask,
  cancelScoringRun,
  getExamAutomationResults,
  downloadReviewWorkspaceImage,
  downloadScoringResultImage,
  getScoringReadiness,
  getScoringSummary,
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
  type AutomationResult,
  type EvidenceJob,
  type ReviewTask,
  type ReviewTaskContext,
  type RubricSelection,
  type ExamAutomationResults,
  type ScoringRunItem,
  type ScoringReadiness,
  type ScoringSummary
} from "../../../api/review";
import { listManagedUsers, type ManagedUser } from "../../../api/users";
import {
  type AnswerSegment,
  type OcrResult,
  type OcrTask,
  type SubmissionPage
} from "../../../api/submissions";
import { EmptyState, ErrorState, LoadingState } from "../../../components/PageState";
import { OcrWorkerAlert } from "../../../components/OcrWorkerAlert";
import { ResponsiveTable } from "../../../components/ResponsiveTable";
import { ScoringPaperMonitor } from "../../../components/ScoringPaperMonitor";
import { StatusTag } from "../../../components/StatusTag";
import type { StatusTone } from "../../../types";
import { appQueryClient } from "../../../query/client";
import { reviewTaskContextKeys, reviewTaskContextQueryOptions } from "../../../query/reviewTaskContext";
import { ReviewAnnotationWorkspace } from "../../../components/features/review-annotation";
import { BackmarkQueue } from "../../../components/BackmarkQueue";
import { RegradeQueue } from "../../../components/RegradeQueue";
import { ReviewContextInspector } from "./ReviewContextInspector";
import { questionFromContext, requiresExplicitSecondOpinion, secondOpinionMetadata } from "./reviewContext";
import { gradingShortcutIntent } from "./keyboardShortcuts";
import { SharedScoreControl } from "../SharedScoreControl";
import { MathEvidenceInspector } from "./MathEvidenceInspector";
import { CalibrationDrawer } from "../../calibration";
import {
  GoldPaperManagerDrawer,
  GoldPaperNominationDrawer,
  type GoldPaperNominationCandidate
} from "../../gold-papers";

type TaskFilter = "active" | "pending" | "assigned" | "in_progress" | "returned" | "submitted";
type ViewerMode = "segment" | "original" | "ocr";
type DraftSaveStatus = "idle" | "saving" | "saved" | "offline" | "conflict" | "error" | "readonly";
type ScoringResultType = "all" | "choice" | "fill";
type ScoringResultState = "all" | "confirmed" | "review" | "failed" | "processing";

interface WorkbenchContext {
  task: ReviewTask;
  reviewContext: ReviewTaskContext;
  segment?: AnswerSegment;
  question?: Question;
  pages: SubmissionPage[];
  page?: SubmissionPage;
  ocrTasks: OcrTask[];
  ocrResults: OcrResult[];
  aiGrades: AiGrade[];
  automationResult?: AutomationResult;
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

interface ScoringImagePreview extends PreviewState {
  title: string;
}

interface PrefetchedTaskBundle {
  task: ReviewTask;
  context: WorkbenchContext;
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

interface ScoreDraftChange {
  score: number | null;
  rubricSelections: Record<string, number>;
}

interface ReviewerProgress {
  id: string;
  name: string;
  total: number;
  completed: number;
  active: number;
  percent: number;
}

interface DraftFallbackSnapshot {
  draft: ScoreDraft;
  viewer: {
    mode: ViewerMode;
    scale: number;
    rotation: number;
    offset: { x: number; y: number };
    fit: boolean;
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
  ai_low_confidence: "AI 把握不足",
  ocr_low_confidence: "识别把握不足",
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

const questionTypeLabels: Record<string, string> = {
  single_choice: "单选题",
  multiple_choice: "多选题",
  true_false: "判断题",
  fill_blank: "填空题",
  numeric: "数值题"
};

const recognitionDecisionLabels: Record<string, string> = {
  selected: "识别成功",
  confirmed: "识别成功",
  blank: "未作答",
  multiple: "多选冲突",
  ambiguous: "结果不明确",
  parse_failed: "解析失败"
};

const recognitionSourceLabels: Record<string, string> = {
  omr: "填涂识别",
  ocr: "文字识别",
  ocr_text: "文字识别",
  imported: "导入",
  imported_answer: "导入",
  manual: "人工录入",
  manual_entry: "人工录入"
};

const riskFlagLabels: Record<string, string> = {
  low_confidence: "置信度低",
  ocr_low_confidence: "识别把握不足",
  ocr_text_empty_review_required: "未识别到作答内容",
  ambiguous_answer: "答案表述不明确",
  insufficient_evidence: "证据不足",
  possible_off_topic: "疑似离题",
  score_needs_review: "得分需人工复核",
  schema_repaired: "结果经系统修正",
  prompt_injection_suspected: "疑似异常输入",
  human_review_required: "需人工复核",
  needs_human_review: "需人工复核",
  format_mismatch: "格式不符",
  partial_match: "部分匹配"
};

const evidenceTypeLabels: Record<string, string> = {
  answer_text: "作答文本",
  answer_segment_answer: "作答内容",
  text_match: "文本匹配",
  rule: "规则命中",
  keyword: "关键词",
  mock: "演示数据"
};

const reasonLabels: Record<string, string> = {
  rule_not_auto_confirmed: "规则未能自动确认",
  rule_review_required: "规则评分需人工确认",
  grading_failure: "评分处理失败",
  scoring_run_cancelled: "本次评分已取消",
  dead_letter: "多次失败",
  retryable_error: "处理失败，稍后会重试",
  terminal_error: "多次失败，已停止重试",
  omr_failed: "填涂识别失败",
  ocr_failed_unhandled: "文字识别失败",
  storage_timeout: "答题图像读取超时",
  source_lease_failed: "答题图像暂不可用",
  lease_expired: "任务超时已放回",
  segment_not_found: "答题图像缺失",
  image_quality_processing_failed: "图像质量检查失败"
};

const commentPresets = ["答案完整，逻辑清晰", "关键步骤缺失", "结论正确但过程不充分", "请补充必要说明"];

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    console.error("接口请求失败", error.status, error.code, error.message);
    return error.message || "操作失败，请稍后重试";
  }
  if (error instanceof Error) {
    return error.message || "操作失败，请稍后重试";
  }
  return "操作失败，请稍后重试";
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

function formatAnswer(value: unknown): string {
  if (value === null || value === undefined || value === "") return "-";
  if (Array.isArray(value)) return value.map(formatAnswer).filter((item) => item !== "-").join("、") || "-";
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    if ("answer" in record) return formatAnswer(record.answer);
    if ("answers" in record) return formatAnswer(record.answers);
    return Object.values(record).map(formatAnswer).filter((item) => item !== "-").join("、") || "-";
  }
  if (typeof value === "boolean") return value ? "正确" : "错误";
  return String(value);
}

function resultState(item: ScoringRunItem): { label: string; tone: StatusTone } {
  if (item.state === "confirmed" && item.grade_source === "rule_confirmed") return { label: "自动确认", tone: "success" };
  if (item.state === "confirmed") return { label: "人工完成", tone: "success" };
  if (item.state === "review") return { label: "待人工复核", tone: "warning" };
  if (item.state === "failed") return { label: "处理失败", tone: "danger" };
  return { label: scoringItemStateLabels[item.state] ?? "处理中", tone: "processing" };
}

function gradingConclusion(item: ScoringRunItem) {
  if (typeof item.score !== "number" || typeof item.max_score !== "number") return "尚未判分";
  if (item.score === item.max_score) return "匹配";
  if (item.score === 0) return "不匹配";
  return "部分得分";
}

function latestGrade(grades: AiGrade[]) {
  return [...grades]
    .filter((grade) => grade.delivery_mode !== "shadow_only")
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0];
}

function pointLabel(point: RubricPoint) {
  return `${point.description || "未命名采分点"} (${point.score} 分)`;
}

function isInputTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName) || target.isContentEditable;
}

function resolveTaskAISuggestion(value: unknown): { grade?: AiGrade; warning?: string } {
  if (!value || typeof value !== "object") {
    return {};
  }
  const candidate = value as Partial<AiGrade>;
  if (candidate.delivery_mode === "shadow_only") {
    return { warning: "本场考试暂未开放 AI 建议，请按评分细则人工评分。" };
  }
  if (candidate.status === "failed") {
    return { warning: "AI 建议生成失败，已切换为人工阅卷；本任务仍可正常提交。" };
  }
  const complete =
    typeof candidate.id === "string" && candidate.id.length > 0 &&
    typeof candidate.suggested_score === "number" &&
    typeof candidate.max_score === "number" &&
    typeof candidate.confidence === "number" &&
    Array.isArray(candidate.matched_points) &&
    Array.isArray(candidate.missing_points) &&
    Array.isArray(candidate.evidence) &&
    Array.isArray(candidate.risk_flags);
  if (!complete) {
    return { warning: "AI 建议数据不完整，已切换为人工阅卷；本任务仍可正常提交。" };
  }
  return { grade: candidate as AiGrade };
}

async function loadTaskContext(taskId: string, allowOriginalImage: boolean): Promise<WorkbenchContext> {
  const reviewContext = await appQueryClient.fetchQuery(reviewTaskContextQueryOptions(taskId));
  const task = reviewContext.task;
  const metadata = secondOpinionMetadata(reviewContext, false);
  const suggestion = metadata ? resolveTaskAISuggestion(metadata) : {};
  const question = questionFromContext(reviewContext);
  const artifact = reviewContext.answer_artifact;
  const segment = { id: task.answer_segment_id, tenant_id: task.tenant_id, submission_id: task.submission_id, submission_page_id: "", question_id: task.question_id, question_no: task.question_no, bbox: [], source: artifact.source || "context", status: artifact.status, created_at: task.created_at, confidence: artifact.confidence } as AnswerSegment;
  const recognizedText = reviewContext.automation_result?.recognized_answer || artifact.ocr_text || artifact.raw_answer || "";
  return {
    task,
    reviewContext,
    segment,
    question,
    pages: [],
    ocrTasks: [],
    ocrResults: [],
    aiGrades: suggestion.grade ? [suggestion.grade] : [],
    automationResult: reviewContext.automation_result,
    warnings: suggestion.warning ? [suggestion.warning] : [],
    ocrText: recognizedText,
    segmentImageUrl: artifact.segment_image_url,
    originalImageUrl: allowOriginalImage ? artifact.original_image_url : undefined
  };
}

async function fetchTaskBundle(taskId: string, currentUserId: string, initialExamId: string, allowOriginalImage: boolean): Promise<PrefetchedTaskBundle> {
  const context = await loadTaskContext(taskId, allowOriginalImage);
  if (initialExamId && context.task.exam_id !== initialExamId) {
    throw new Error("该阅卷任务不属于当前考试，已停止加载。");
  }
  if (context.task.assigned_to !== currentUserId) {
    throw new Error("该阅卷任务已不再分配给当前用户。");
  }
  return { task: context.task, context };
}

function createInitialDraft(ctx: WorkbenchContext | null): ScoreDraft {
  const ocrText = ctx?.ocrText || ctx?.ocrResults.map((item) => item.text).filter(Boolean).join("\n") || "";
  const selections: Record<string, number> = {};
  for (const point of ctx?.question?.rubric?.points ?? []) {
    selections[point.id] = 0;
  }
  return {
    score: null,
    comments: "",
    privateNote: "",
    studentFeedback: "",
    reason: "教师复核完成",
    disputeReason: "",
    rubricSelections: selections,
    answerText: ocrText
  };
}

function createDraftSnapshot(draft: ScoreDraft, mode: ViewerMode, scale: number, rotation: number, offset: { x: number; y: number }, fit: boolean): DraftFallbackSnapshot {
  return { draft, viewer: { mode, scale, rotation, offset, fit } };
}

function fallbackSnapshot(value: unknown, initial: ScoreDraft): DraftFallbackSnapshot | null {
  if (!value || typeof value !== "object") return null;
  const root = value as { draft?: unknown; viewer?: unknown };
  if (!root.draft || typeof root.draft !== "object" || !root.viewer || typeof root.viewer !== "object") return null;
  const cachedDraft = root.draft as Partial<ScoreDraft>;
  const cachedViewer = root.viewer as { mode?: unknown; scale?: unknown; rotation?: unknown; offset?: unknown; fit?: unknown };
  const selections = cachedDraft.rubricSelections && typeof cachedDraft.rubricSelections === "object" && !Array.isArray(cachedDraft.rubricSelections)
    ? Object.fromEntries(Object.entries(cachedDraft.rubricSelections).flatMap(([key, value]) => typeof value === "number" && Number.isFinite(value) ? [[key, value]] : []))
    : initial.rubricSelections;
  const rawOffset = cachedViewer.offset && typeof cachedViewer.offset === "object" ? cachedViewer.offset as { x?: unknown; y?: unknown } : {};
  const score = typeof cachedDraft.score === "number" && Number.isFinite(cachedDraft.score) ? cachedDraft.score : null;
  const mode = cachedViewer.mode === "original" || cachedViewer.mode === "ocr" || cachedViewer.mode === "segment" ? cachedViewer.mode : "segment";
  const scale = typeof cachedViewer.scale === "number" && Number.isFinite(cachedViewer.scale) ? Math.min(4, Math.max(0.25, cachedViewer.scale)) : 1;
  const rotation = typeof cachedViewer.rotation === "number" && Number.isFinite(cachedViewer.rotation) ? cachedViewer.rotation : 0;
  const fit = cachedViewer.fit !== false;
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
      fit,
      offset: {
        x: typeof rawOffset.x === "number" && Number.isFinite(rawOffset.x) ? rawOffset.x : 0,
        y: typeof rawOffset.y === "number" && Number.isFinite(rawOffset.y) ? rawOffset.y : 0
      }
    }
  };
}

export interface GradingWorkbenchProps {
  canWork: boolean;
  canManageTasks: boolean;
  canViewOriginalImage: boolean;
  canGrade: boolean;
  canVerifyEvidence: boolean;
  canReturn: boolean;
  currentUserId: string;
  initialExamId?: string;
  personalScope?: boolean;
}

export function GradingWorkbench({ canWork, canManageTasks, canViewOriginalImage, canGrade, canVerifyEvidence, canReturn, currentUserId, initialExamId = "", personalScope = false }: GradingWorkbenchProps) {
  const { message } = App.useApp();
  const hasSession = true;
  const [taskFilter, setTaskFilter] = useState<TaskFilter>("active");
  const [keyword, setKeyword] = useState("");
  const [tasks, setTasks] = useState<ReviewTask[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState("");
  const [loadingTasks, setLoadingTasks] = useState(true);
  const [loadingMoreTasks, setLoadingMoreTasks] = useState(false);
  const [nextTaskCursor, setNextTaskCursor] = useState("");
  const [hasMoreTasks, setHasMoreTasks] = useState(false);
  const [taskError, setTaskError] = useState<string | null>(null);
  const [graders, setGraders] = useState<ManagedUser[]>([]);
  const [gradersError, setGradersError] = useState<string | null>(null);
  const [assignmentUserId, setAssignmentUserId] = useState("");
  const [assignmentTaskIds, setAssignmentTaskIds] = useState<string[]>([]);
  const [ctx, setCtx] = useState<WorkbenchContext | null>(null);
  const [contextLoading, setContextLoading] = useState(false);
  const [contextError, setContextError] = useState<string | null>(null);
  const [draft, setDraft] = useState<ScoreDraft>(() => createInitialDraft(null));
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [viewerMode, setViewerMode] = useState<ViewerMode>("segment");
  const [scale, setScale] = useState(1);
  const [fitScale, setFitScale] = useState(1);
  const [autoFit, setAutoFit] = useState(true);
  const [imageSize, setImageSize] = useState<{ width: number; height: number } | null>(null);
  const [rotation, setRotation] = useState(0);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  const [actioning, setActioning] = useState<string | null>(null);
  const [goldPaperManagerOpen, setGoldPaperManagerOpen] = useState(false);
  const [calibrationQuestionId, setCalibrationQuestionId] = useState("");
  const [goldPaperCandidate, setGoldPaperCandidate] = useState<GoldPaperNominationCandidate | null>(null);
  const [scoringSummary, setScoringSummary] = useState<ScoringSummary | null>(null);
  const [scoringReadiness, setScoringReadiness] = useState<ScoringReadiness | null>(null);
  const [scoringLoading, setScoringLoading] = useState(false);
  const [scoringRunDetail, setScoringRunDetail] = useState<ExamAutomationResults | null>(null);
  const [scoringDetailOpen, setScoringDetailOpen] = useState(false);
  const [scoringResultType, setScoringResultType] = useState<ScoringResultType>("all");
  const [scoringResultState, setScoringResultState] = useState<ScoringResultState>("all");
  const [scoringResultKeyword, setScoringResultKeyword] = useState("");
  const [scoringImage, setScoringImage] = useState<ScoringImagePreview | null>(null);
  const [scoringImageLoading, setScoringImageLoading] = useState("");
  const [online, setOnline] = useState(() => typeof navigator === "undefined" || navigator.onLine);
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const taskListRequestRef = useRef(0);
  const contextRequestRef = useRef(0);
  const previewRequestRef = useRef(0);
  const scoringSummaryRequestRef = useRef(0);
  const scoringImageRequestRef = useRef(0);
  const suppressAutoSelectRef = useRef(false);
  const prefetchedTaskRef = useRef(new Map<string, Promise<PrefetchedTaskBundle>>());
  const prefetchedPreviewRef = useRef(new Map<string, Promise<Awaited<ReturnType<typeof downloadReviewWorkspaceImage>> | null>>());
  const [draftRevision, setDraftRevision] = useState(0);
  const [draftSaveStatus, setDraftSaveStatus] = useState<DraftSaveStatus>("idle");
  const [draftHydrated, setDraftHydrated] = useState(false);
  const lastSavedDraft = useRef("");
  const lastScoreDraftChange = useRef<ScoreDraftChange | null>(null);
  const [hasLastScoreDraftChange, setHasLastScoreDraftChange] = useState(false);

  const filteredTasks = useMemo(() => {
    const text = keyword.trim().toLowerCase();
    return tasks.filter((task) => {
      const activeStatuses = canManageTasks ? ["pending", "assigned", "in_progress", "returned"] : ["assigned", "in_progress", "returned"];
      const statusMatched = taskFilter === "active" ? activeStatuses.includes(task.status) : task.status === taskFilter;
      const examMatched = !initialExamId || task.exam_id === initialExamId;
      const keywordMatched =
        !text ||
        task.id.toLowerCase().includes(text) ||
        task.anonymous_code.toLowerCase().includes(text) ||
        task.question_no.toLowerCase().includes(text) ||
        task.source.toLowerCase().includes(text);
      return examMatched && statusMatched && keywordMatched;
    });
  }, [canManageTasks, initialExamId, keyword, taskFilter, tasks]);

  const assignableTasks = useMemo(
    () => filteredTasks.filter((task) => !["submitted", "completed", "in_progress"].includes(task.status)),
    [filteredTasks]
  );
  const graderOptions = useMemo(
    () => graders.map((grader) => {
      const name = displayNameOrUsername(grader.display_name, grader.username);
      const username = grader.username.trim();
      return {
        value: grader.id,
        label: !username || name === username ? name : `${name} · ${username}`
      };
    }),
    [graders]
  );
  const graderNames = useMemo(
    () => Object.fromEntries(graders.map((grader) => [grader.id, displayNameOrUsername(grader.display_name, grader.username)])),
    [graders]
  );

  const reviewerProgress = useMemo<ReviewerProgress[]>(() => {
    const names = new Map<string, string>();
    if (canManageTasks) {
      graders.forEach((grader) => names.set(grader.id, displayNameOrUsername(grader.display_name, grader.username)));
    } else {
      names.set(currentUserId, "我的阅卷");
    }

    const totals = new Map<string, { total: number; completed: number; active: number }>();
    names.forEach((_name, id) => {
      totals.set(id, { total: 0, completed: 0, active: 0 });
    });
    tasks.forEach((task) => {
      const reviewerId = task.assigned_to;
      if (!reviewerId) return;
      if (!names.has(reviewerId)) names.set(reviewerId, "未知阅卷员");
      const current = totals.get(reviewerId) ?? { total: 0, completed: 0, active: 0 };
      current.total += 1;
      if (["submitted", "completed"].includes(task.status)) {
        current.completed += 1;
      } else {
        current.active += 1;
      }
      totals.set(reviewerId, current);
    });

    return Array.from(totals.entries())
      .map(([id, counts]) => ({
        id,
        name: names.get(id) ?? "阅卷员",
        ...counts,
        percent: counts.total > 0 ? Math.round((counts.completed / counts.total) * 100) : 0
      }))
      .sort((left, right) => right.total - left.total || left.name.localeCompare(right.name));
  }, [canManageTasks, currentUserId, graders, tasks]);

  const myProgress = useMemo(
    () => reviewerProgress.find((reviewer) => reviewer.id === currentUserId),
    [currentUserId, reviewerProgress]
  );
  const remainingCount = useMemo(
    () => filteredTasks.filter((task) => !["submitted", "completed"].includes(task.status)).length,
    [filteredTasks]
  );

  useEffect(() => {
    if (loadingTasks || filteredTasks.some((task) => task.id === selectedTaskId)) {
      return;
    }
    if (!selectedTaskId && suppressAutoSelectRef.current) {
      return;
    }
    setSelectedTaskId(filteredTasks[0]?.id ?? "");
  }, [filteredTasks, loadingTasks, selectedTaskId]);

  useEffect(() => {
    const available = new Set(assignableTasks.map((task) => task.id));
    setAssignmentTaskIds((current) => current.filter((taskId) => available.has(taskId)));
  }, [assignableTasks]);

  const selectedIndex = useMemo(() => filteredTasks.findIndex((task) => task.id === selectedTaskId), [filteredTasks, selectedTaskId]);
  const prefetchTasks = useMemo(() => {
    const actionable = (task: ReviewTask) => task.id !== selectedTaskId && !["submitted", "completed"].includes(task.status);
    const ordered = [...filteredTasks.slice(selectedIndex + 1), ...filteredTasks.slice(0, Math.max(selectedIndex, 0))]
      .filter(actionable);
    return ordered.slice(0, 2);
  }, [filteredTasks, selectedIndex, selectedTaskId]);
  const nextTask = useMemo(() => {
    const actionable = (task: ReviewTask) => task.id !== selectedTaskId && !["submitted", "completed"].includes(task.status);
    return filteredTasks.slice(selectedIndex + 1).find(actionable) ?? filteredTasks.find(actionable);
  }, [filteredTasks, selectedIndex, selectedTaskId]);
  const selectedGrade = useMemo(() => latestGrade(ctx?.aiGrades ?? []), [ctx?.aiGrades]);
  const scoringBlockingChecks = useMemo(
    () => scoringReadiness?.checks.filter((check) => check.severity === "blocker" && !check.passed) ?? [],
    [scoringReadiness]
  );
  const scoringWarningChecks = useMemo(
    () => scoringReadiness?.checks.filter((check) => check.severity === "warning" && !check.passed) ?? [],
    [scoringReadiness]
  );
  const hasUnresolvedScoringRun = Boolean(
    scoringSummary?.run && ["queued", "processing", "needs_review", "failed", "cancelling"].includes(scoringSummary.run.status)
  );
  const maxScore = ctx?.question?.score ?? selectedGrade?.max_score ?? 0;
  const rubricPoints = ctx?.question?.rubric?.points ?? [];
  const ownsSelectedTask = Boolean(ctx?.task.assigned_to && ctx.task.assigned_to === currentUserId);
  const canEditDraft = canWork && hasSession && ownsSelectedTask && Boolean(ctx && ["assigned", "in_progress", "returned"].includes(ctx.task.status));
  const canSubmit = canEditDraft;
  const canUndoScoreChange = hasLastScoreDraftChange;
  const filteredScoringItems = useMemo(() => {
    const text = scoringResultKeyword.trim().toLowerCase();
    return (scoringRunDetail?.items ?? []).filter((item) => {
      const typeMatched = scoringResultType === "all" ||
        (scoringResultType === "choice" && ["single_choice", "multiple_choice", "true_false"].includes(item.question_type)) ||
        (scoringResultType === "fill" && ["fill_blank", "numeric"].includes(item.question_type));
      const stateMatched = scoringResultState === "all" ||
        (scoringResultState === "processing" ? ["pending", "processing", "cancelling"].includes(item.state) : item.state === scoringResultState);
      const keywordMatched = !text || [item.anonymous_code, item.question_no, item.recognized_answer, formatAnswer(item.standard_answer)]
        .some((value) => (value ?? "").toLowerCase().includes(text));
      return typeMatched && stateMatched && keywordMatched;
    });
  }, [scoringResultKeyword, scoringResultState, scoringResultType, scoringRunDetail?.items]);

  const scoringResultMetrics = useMemo(() => {
    const items = scoringRunDetail?.items ?? [];
    return {
      total: items.length,
      auto: items.filter((item) => item.state === "confirmed" && item.grade_source === "rule_confirmed").length,
      review: items.filter((item) => item.state === "review").length,
      failed: items.filter((item) => item.state === "failed").length
    };
  }, [scoringRunDetail?.items]);

  const loadTasks = useCallback(async () => {
    const requestId = ++taskListRequestRef.current;
    setLoadingTasks(true);
    setTaskError(null);
    try {
      const result = await listReviewTasks({
        ...(personalScope ? { assigned_to: currentUserId } : {}),
        ...(initialExamId ? { exam_id: initialExamId } : {}),
        limit: 50
      });
      if (requestId !== taskListRequestRef.current) return;
      const scopedTasks = initialExamId ? result.tasks.filter((task) => task.exam_id === initialExamId) : result.tasks;
      setTasks(scopedTasks);
      setNextTaskCursor(result.next_cursor ?? "");
      setHasMoreTasks(Boolean(result.has_more));
      setSelectedTaskId((current) => scopedTasks.some((task) => task.id === current)
        ? current
        : scopedTasks.find((task) => ["assigned", "in_progress", "returned"].includes(task.status))?.id || "");
    } catch (currentError) {
      if (requestId !== taskListRequestRef.current) return;
      setTasks([]);
      setNextTaskCursor("");
      setHasMoreTasks(false);
      setSelectedTaskId("");
      setTaskError(formatError(currentError));
    } finally {
      if (requestId === taskListRequestRef.current) setLoadingTasks(false);
    }
  }, [currentUserId, initialExamId, personalScope]);

  const loadMoreTasks = useCallback(async () => {
    if (!hasMoreTasks || !nextTaskCursor || loadingMoreTasks) return;
    setLoadingMoreTasks(true);
    try {
      const result = await listReviewTasks({
        ...(personalScope ? { assigned_to: currentUserId } : {}),
        ...(initialExamId ? { exam_id: initialExamId } : {}),
        limit: 50,
        cursor: nextTaskCursor
      });
      setTasks((current) => {
        const known = new Set(current.map((task) => task.id));
        return [...current, ...result.tasks.filter((task) => !known.has(task.id))];
      });
      setNextTaskCursor(result.next_cursor ?? "");
      setHasMoreTasks(Boolean(result.has_more));
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setLoadingMoreTasks(false);
    }
  }, [currentUserId, hasMoreTasks, initialExamId, loadingMoreTasks, message, nextTaskCursor, personalScope]);

  const loadGraders = useCallback(async () => {
    if (!canManageTasks) {
      setGraders([]);
      setGradersError(null);
      return;
    }
    try {
      const result = await listManagedUsers({ limit: 200 });
      const available = result.users.filter((user) => user.status === "active" && user.roles.includes("grader"));
      setGraders(available);
      setGradersError(available.length ? null : "当前没有可分配的有效阅卷员账号");
      setAssignmentUserId((current) => available.some((user) => user.id === current) ? current : "");
    } catch (currentError) {
      setGraders([]);
      setAssignmentUserId("");
      setGradersError(formatError(currentError));
    }
  }, [canManageTasks]);

  const loadScoringSummary = useCallback(async () => {
    const requestId = ++scoringSummaryRequestRef.current;
    if (!initialExamId || !canGrade) {
      setScoringSummary(null);
      setScoringReadiness(null);
      setScoringLoading(false);
      return;
    }
    setScoringLoading(true);
    try {
      const [result, readinessResult] = await Promise.all([
        getScoringSummary(initialExamId),
        getScoringReadiness(initialExamId)
      ]);
      if (requestId !== scoringSummaryRequestRef.current) return;
      setScoringSummary(result.scoring_summary);
      setScoringReadiness(readinessResult.scoring_readiness);
    } catch (currentError) {
      if (requestId !== scoringSummaryRequestRef.current) return;
      message.error(formatError(currentError));
    } finally {
      if (requestId === scoringSummaryRequestRef.current) setScoringLoading(false);
    }
  }, [canGrade, initialExamId, message]);

  const startExamScoring = useCallback(async () => {
    if (!initialExamId || !canGrade) return;
    setActioning("start-scoring");
    try {
      const readinessResult = await getScoringReadiness(initialExamId);
      setScoringReadiness(readinessResult.scoring_readiness);
      if (!readinessResult.scoring_readiness.ready) {
        message.warning("评分准备检查未通过，请先处理阻塞项");
        return;
      }
      await startScoringRun(initialExamId, `web-${crypto.randomUUID()}`);
      message.success("评分任务已生成");
      setScoringDetailOpen(true);
      const [summary, detail, nextReadiness] = await Promise.all([
        getScoringSummary(initialExamId),
        getExamAutomationResults(initialExamId),
        getScoringReadiness(initialExamId)
      ]);
      setScoringSummary(summary.scoring_summary);
      setScoringRunDetail(detail);
      setScoringReadiness(nextReadiness.scoring_readiness);
      await loadTasks();
    } catch (currentError) {
      message.error(formatError(currentError));
      void getScoringReadiness(initialExamId)
        .then((result) => setScoringReadiness(result.scoring_readiness))
        .catch(() => undefined);
    } finally {
      setActioning(null);
    }
  }, [canGrade, initialExamId, loadTasks, message]);

  const showScoringRunDetail = useCallback(async () => {
    if (!initialExamId) return;
    setActioning("scoring-detail");
    try {
      const [summary, detail] = await Promise.all([
        getScoringSummary(initialExamId),
        getExamAutomationResults(initialExamId)
      ]);
      setScoringSummary(summary.scoring_summary);
      setScoringRunDetail(detail);
      setScoringDetailOpen(true);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  }, [initialExamId, message]);

  useEffect(() => {
    const status = scoringSummary?.run?.status;
    if (!scoringDetailOpen || !initialExamId || !status || !["queued", "processing"].includes(status)) return;
    let disposed = false;
    const refresh = () => {
      void Promise.all([
        getScoringSummary(initialExamId),
        getExamAutomationResults(initialExamId),
        getScoringReadiness(initialExamId)
      ]).then(([summary, detail, readiness]) => {
        if (disposed) return;
        setScoringSummary(summary.scoring_summary);
        setScoringRunDetail(detail);
        setScoringReadiness(readiness.scoring_readiness);
      }).catch(() => undefined);
    };
    const timer = window.setInterval(refresh, 2500);
    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, [initialExamId, scoringDetailOpen, scoringSummary?.run?.status]);

  const showScoringResultImage = useCallback(async (item: ScoringRunItem) => {
    const requestId = ++scoringImageRequestRef.current;
    setScoringImageLoading(item.answer_segment_id);
    try {
      const file = await downloadScoringResultImage(item.answer_segment_id);
      if (requestId !== scoringImageRequestRef.current) return;
      setScoringImage((current) => {
        if (current?.url) URL.revokeObjectURL(current.url);
        return {
          url: URL.createObjectURL(file.blob),
          contentType: file.contentType,
          filename: file.filename,
          title: `${item.question_no} · ${item.anonymous_code}`
        };
      });
    } catch (currentError) {
      if (requestId === scoringImageRequestRef.current) message.error(formatError(currentError));
    } finally {
      if (requestId === scoringImageRequestRef.current) setScoringImageLoading("");
    }
  }, [message]);

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
    const requestId = ++contextRequestRef.current;
    previewRequestRef.current += 1;
    if (!taskId || !hasSession) {
      setCtx(null);
      setContextError(null);
      setContextLoading(false);
      setPreview(null);
      setImageSize(null);
      setDraft(createInitialDraft(null));
      lastScoreDraftChange.current = null;
      setHasLastScoreDraftChange(false);
      setDraftHydrated(false);
      setDraftSaveStatus("idle");
      return;
    }
    setContextLoading(true);
    setContextError(null);
    setCtx(null);
    setDraftHydrated(false);
    setPreview(null);
    setImageSize(null);
    setAutoFit(true);
    setFitScale(1);
    setScale(1);
    setRotation(0);
    setOffset({ x: 0, y: 0 });
    try {
      const prefetched = prefetchedTaskRef.current.get(taskId);
      prefetchedTaskRef.current.delete(taskId);
      const next = prefetched
        ? (await prefetched).context
        : await loadTaskContext(taskId, canViewOriginalImage);
      if (initialExamId && next.task.exam_id !== initialExamId) {
        setSelectedTaskId("");
        setContextError("该阅卷任务不属于当前考试，已停止加载。");
        return;
      }
      const reviewerOwnsTask = next.task.assigned_to === currentUserId;
      // Blind quality samples use the ordinary workbench but deliberately do
      // not participate in the review task's draft or lease endpoints.
      const mayPersistDraft = reviewerOwnsTask && next.reviewContext.claim.can_renew;
      const draftResult = { draft: reviewerOwnsTask ? next.reviewContext.draft : null };
      if (requestId !== contextRequestRef.current) return;
      const initial = createInitialDraft(next);
      let restored = initial;
      let revision = 0;
      let restoredViewer: DraftFallbackSnapshot["viewer"] = { mode: "segment", scale: 1, rotation: 0, offset: { x: 0, y: 0 }, fit: true };
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
        const savedMode = ["segment", "original", "ocr"].includes(String(viewer.mode)) ? viewer.mode as ViewerMode : "segment";
        const mode = savedMode === "original" && !canViewOriginalImage ? "segment" : savedMode;
        const scale = Number(viewer.scale ?? 1);
        const rotation = Number(viewer.rotation ?? 0);
        const savedOffset = viewer.offset as { x?: number; y?: number } | undefined;
        restoredViewer = { mode, scale, rotation, offset: { x: Number(savedOffset?.x ?? 0), y: Number(savedOffset?.y ?? 0) }, fit: viewer.fit !== false };
        serverUpdatedAt = Date.parse(draftResult.draft.updated_at) || 0;
      }
      const serverSnapshot = createDraftSnapshot(restored, restoredViewer.mode, restoredViewer.scale, restoredViewer.rotation, restoredViewer.offset, restoredViewer.fit);
      const localDraft = mayPersistDraft ? loadReviewDraftFallback<unknown>(currentUserId, taskId) : null;
      const localSnapshot = localDraft ? fallbackSnapshot(localDraft.snapshot, initial) : null;
      const useLocalDraft = Boolean(localDraft && localSnapshot && localDraft.updatedAt > serverUpdatedAt);
      if (useLocalDraft && localSnapshot) {
        restored = localSnapshot.draft;
        restoredViewer = localSnapshot.viewer.mode === "original" && !canViewOriginalImage
          ? { ...localSnapshot.viewer, mode: "segment" }
          : localSnapshot.viewer;
        setDraftSaveStatus("offline");
      } else {
        if (localDraft) removeReviewDraftFallback(currentUserId, taskId);
        setDraftSaveStatus(mayPersistDraft ? (draftResult.draft ? "saved" : "idle") : "readonly");
      }
      setDraft(restored);
      lastScoreDraftChange.current = null;
      setHasLastScoreDraftChange(false);
      setDraftRevision(revision);
      setViewerMode(restoredViewer.mode);
      setScale(restoredViewer.scale);
      setAutoFit(restoredViewer.fit);
      setRotation(restoredViewer.rotation);
      setOffset(restoredViewer.offset);
      setCtx(next);
      lastSavedDraft.current = JSON.stringify(serverSnapshot);
      setDraftHydrated(mayPersistDraft);
    } catch (currentError) {
      if (requestId === contextRequestRef.current) setContextError(formatError(currentError));
    } finally {
      if (requestId === contextRequestRef.current) setContextLoading(false);
    }
  }, [canViewOriginalImage, currentUserId, hasSession, initialExamId]);

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
    if (!draftHydrated || !ctx || ctx.task.assigned_to !== currentUserId || ctx.task.status === "submitted") return;
    const snapshotValue = createDraftSnapshot(draft, viewerMode, scale, rotation, offset, autoFit);
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
          viewer_state: { mode: viewerMode, scale, rotation, offset, fit: autoFit },
          expected_revision: draftRevision,
          client_updated_at: new Date().toISOString()
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
  }, [autoFit, ctx, currentUserId, draft, draftHydrated, draftRevision, offset, online, rotation, scale, viewerMode]);

  useEffect(() => {
    suppressAutoSelectRef.current = false;
    taskListRequestRef.current += 1;
    contextRequestRef.current += 1;
    previewRequestRef.current += 1;
    scoringSummaryRequestRef.current += 1;
    scoringImageRequestRef.current += 1;
    prefetchedTaskRef.current.clear();
    prefetchedPreviewRef.current.clear();
    setTasks([]);
    setNextTaskCursor("");
    setHasMoreTasks(false);
    setSelectedTaskId("");
    setAssignmentTaskIds([]);
    setCtx(null);
    setContextError(null);
    setContextLoading(false);
    setPreview(null);
    setImageSize(null);
    setAutoFit(true);
    setFitScale(1);
    setScale(1);
    setRotation(0);
    setOffset({ x: 0, y: 0 });
    setDraft(createInitialDraft(null));
    setDraftHydrated(false);
    setDraftSaveStatus("idle");
    lastSavedDraft.current = "";
    setScoringSummary(null);
    setScoringReadiness(null);
    setScoringLoading(false);
    setScoringRunDetail(null);
    setScoringDetailOpen(false);
    setScoringImage(null);
    setScoringImageLoading("");
  }, [currentUserId, initialExamId]);

  useEffect(() => {
    void loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    void loadGraders();
  }, [loadGraders]);

  useEffect(() => {
    void loadScoringSummary();
  }, [loadScoringSummary]);

  useEffect(() => {
    void loadContext(selectedTaskId);
  }, [loadContext, selectedTaskId]);

  useEffect(() => {
    if (
      !canWork ||
      prefetchTasks.length === 0
    ) {
      return;
    }
    for (const taskId of prefetchedTaskRef.current.keys()) {
      if (taskId !== selectedTaskId && !prefetchTasks.some((task) => task.id === taskId)) prefetchedTaskRef.current.delete(taskId);
    }
    for (const taskId of prefetchedPreviewRef.current.keys()) {
      if (taskId !== selectedTaskId && !prefetchTasks.some((task) => task.id === taskId)) prefetchedPreviewRef.current.delete(taskId);
    }
    for (const task of prefetchTasks) {
      if (task.assigned_to !== currentUserId || !["assigned", "in_progress", "returned"].includes(task.status) || prefetchedTaskRef.current.has(task.id)) continue;
      const bundlePromise = fetchTaskBundle(task.id, currentUserId, initialExamId, canViewOriginalImage);
      prefetchedTaskRef.current.set(task.id, bundlePromise);
      prefetchedPreviewRef.current.set(
        task.id,
        bundlePromise
          .then((bundle) => bundle.context.segmentImageUrl
            ? downloadReviewWorkspaceImage(bundle.context.segmentImageUrl)
            : null)
          .catch(() => null)
      );
      void bundlePromise.catch(() => {
        prefetchedTaskRef.current.delete(task.id);
        prefetchedPreviewRef.current.delete(task.id);
      });
    }
  }, [canViewOriginalImage, canWork, currentUserId, initialExamId, prefetchTasks, selectedTaskId]);

  useEffect(() => {
    if (
      !selectedTaskId ||
      !ctx ||
      !ctx.reviewContext.claim.can_renew ||
      ctx.task.assigned_to !== currentUserId ||
      !["assigned", "in_progress", "returned"].includes(ctx.task.status)
    ) return;
    const renew = () => void renewReviewTask(selectedTaskId).catch(() => setDraftSaveStatus("error"));
    // Establish the lease as soon as an assigned task is opened. Waiting for
    // the first interval leaves the task without a claim during the initial
    // editing window and makes release/renew state inconsistent.
    renew();
    const timer = window.setInterval(renew, 5 * 60 * 1000);
    return () => window.clearInterval(timer);
  }, [ctx, currentUserId, selectedTaskId]);

  useEffect(() => {
    return () => {
      if (preview?.url) {
        URL.revokeObjectURL(preview.url);
      }
    };
  }, [preview?.url]);

  useEffect(() => {
    return () => {
      if (scoringImage?.url) URL.revokeObjectURL(scoringImage.url);
    };
  }, [scoringImage?.url]);

  const calculateFitScale = useCallback((size: { width: number; height: number }, angle = rotation) => {
    const viewport = viewportRef.current;
    if (!viewport || size.width <= 0 || size.height <= 0) return 1;
    const quarterTurn = Math.abs(angle % 180) === 90;
    const contentWidth = quarterTurn ? size.height : size.width;
    const contentHeight = quarterTurn ? size.width : size.height;
    const availableWidth = Math.max(120, viewport.clientWidth - 32);
    const availableHeight = Math.max(80, viewport.clientHeight - 32);
    return Math.min(4, Math.max(0.1, Math.min(availableWidth / contentWidth, availableHeight / contentHeight)));
  }, [rotation]);

  const applyViewerFit = useCallback((size = imageSize, force = false) => {
    if (!size) return;
    const nextScale = calculateFitScale(size);
    setFitScale(nextScale);
    if (autoFit || force) {
      setAutoFit(true);
      setScale(nextScale);
      setOffset({ x: 0, y: 0 });
    }
  }, [autoFit, calculateFitScale, imageSize]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || !imageSize) return;
    const update = () => applyViewerFit(imageSize);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, [applyViewerFit, imageSize]);

  const loadPreview = useCallback(async () => {
    const requestId = ++previewRequestRef.current;
    if (!ctx || viewerMode === "ocr" || (viewerMode === "original" && (!canViewOriginalImage || !ctx.originalImageUrl)) || (viewerMode === "segment" && !ctx.segmentImageUrl)) {
      setPreview(null);
      setPreviewLoading(false);
      setImageSize(null);
      return;
    }
    setPreviewLoading(true);
    setPreview((current) => {
      if (current?.url) URL.revokeObjectURL(current.url);
      return null;
    });
    setImageSize(null);
    setAutoFit(true);
    try {
      const imagePath = viewerMode === "segment" ? ctx.segmentImageUrl : ctx.originalImageUrl!;
      const cachedPreview = viewerMode === "segment" ? prefetchedPreviewRef.current.get(ctx.task.id) : undefined;
      if (cachedPreview) prefetchedPreviewRef.current.delete(ctx.task.id);
      const file = (cachedPreview ? await cachedPreview : null) ?? await downloadReviewWorkspaceImage(imagePath);
      if (requestId !== previewRequestRef.current) return;
      setPreview((current) => {
        if (current?.url) {
          URL.revokeObjectURL(current.url);
        }
        return { url: URL.createObjectURL(file.blob), contentType: file.contentType, filename: file.filename };
      });
    } catch (currentError) {
      if (requestId === previewRequestRef.current) message.error(formatError(currentError));
    } finally {
      if (requestId === previewRequestRef.current) setPreviewLoading(false);
    }
  }, [canViewOriginalImage, ctx, message, viewerMode]);

  useEffect(() => {
    if (ctx && viewerMode !== "ocr") {
      void loadPreview();
    }
  }, [ctx, loadPreview, viewerMode]);

  const refreshCurrent = async () => {
    if (selectedTaskId) {
      await appQueryClient.invalidateQueries({ queryKey: reviewTaskContextKeys.detail(selectedTaskId) });
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

  const assignSelectedTasks = async () => {
    const available = new Set(assignableTasks.map((task) => task.id));
    const taskIds = assignmentTaskIds.filter((taskId) => available.has(taskId));
    const selectedTasks = assignableTasks.filter((task) => taskIds.includes(task.id));
    if (!assignmentUserId || taskIds.length === 0) {
      message.warning("请选择阅卷员和需要分配的任务");
      return;
    }
    setActioning("assign-tasks");
    try {
      const updated = taskIds.length === 1
        ? [(await assignReviewTask(selectedTasks[0].id, assignmentUserId, selectedTasks[0].revision)).task]
        : (await batchAssignReviewTasks(selectedTasks, assignmentUserId)).tasks;
      const updatedById = new Map(updated.map((task) => [task.id, task]));
      setTasks((current) => current.map((task) => updatedById.get(task.id) ?? task));
      setCtx((current) => current && updatedById.has(current.task.id)
        ? { ...current, task: updatedById.get(current.task.id)! }
        : current);
      setAssignmentTaskIds([]);
      message.success(`已将 ${updated.length} 份任务分配给 ${graderNames[assignmentUserId] ?? "阅卷员"}`);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  };

  const claimTask = async () => {
    const target = ctx?.task ?? filteredTasks.find((task) => ["assigned", "returned"].includes(task.status));
    setActioning("claim-task");
    try {
      const result = await claimNextReviewTask(initialExamId || target?.exam_id || "", target?.question_id || "");
      setTasks((current) => current.some((task) => task.id === result.task.id)
        ? current.map((task) => task.id === result.task.id ? result.task : task)
        : [result.task, ...current]);
      setSelectedTaskId(result.task.id);
    } catch (currentError) {
      if (currentError instanceof ApiClientError && currentError.code === "grader_qualification_required" && target?.question_id) {
        setCalibrationQuestionId(target.question_id);
        message.warning("该题属于高风险阅卷，请先完成本题校准");
      } else {
        message.error(formatError(currentError));
      }
    } finally {
      setActioning(null);
    }
  };

  const goNext = async () => {
    if (nextTask) {
      setSelectedTaskId(nextTask.id);
      return;
    }
    if (!canManageTasks) {
      setSelectedTaskId("");
      message.info("当前没有更多已分配给你的阅卷任务");
      return;
    }
    setSelectedTaskId("");
    setTaskFilter("pending");
    message.info("没有更多已分配任务；请先把待分配任务交给阅卷员");
  };

  const releaseCurrentTask = async () => {
    if (!selectedTaskId || !ownsSelectedTask) return;
    await runAction("release", async () => {
      const result = await releaseReviewTask(selectedTaskId);
      setTasks((current) => current.map((item) => item.id === result.task.id ? result.task : item));
      suppressAutoSelectRef.current = true;
      setSelectedTaskId("");
    }, "已放回队列，草稿已保留");
  };

  const submitGrade = async (nominateAsGold = false) => {
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
        const submittedTaskId = ctx.task.id;
        const nextTaskId = nextTask?.id ?? "";
        const selections: RubricSelection[] = Object.entries(draft.rubricSelections)
          .filter(([, value]) => Number(value) > 0)
          .map(([point_id, value]) => ({ point_id, score: Number(value) }));
        const result = await submitHumanGrade(submittedTaskId, {
          expected_revision: ctx.reviewContext.expected_revision,
          score,
          rubric_selections: selections,
          comments: draft.comments,
          private_note: draft.privateNote,
          student_feedback: draft.studentFeedback,
          reason: draft.reason || "教师复核完成"
        });
        if (nominateAsGold) {
          setGoldPaperCandidate({
            examId: ctx.task.exam_id,
            questionId: ctx.task.question_id,
            questionNo: ctx.task.question_no,
            submissionId: ctx.task.submission_id,
            answerImageUrl: ctx.originalImageUrl ?? ctx.segmentImageUrl,
            referenceScore: score,
            maxScore,
            explanation: draft.comments,
            sourceGradeId: result.human_grade.id,
            rubricPoints: rubricPoints.map((point) => ({ id: point.id, description: point.description, score: point.score, required: point.required })),
            rubricSelections: { ...draft.rubricSelections }
          });
        }
        removeReviewDraftFallback(currentUserId, submittedTaskId);
        appQueryClient.removeQueries({ queryKey: reviewTaskContextKeys.detail(submittedTaskId) });
        contextRequestRef.current += 1;
        previewRequestRef.current += 1;
        setTasks((current) => current.map((task) => task.id === submittedTaskId ? { ...task, status: "submitted" } : task));
        setCtx(null);
        setSelectedTaskId(nextTaskId);
        setPreview(null);
        setImageSize(null);
        setDraft(createInitialDraft(null));
        lastScoreDraftChange.current = null;
        setHasLastScoreDraftChange(false);
        setDraftHydrated(false);
        setDraftSaveStatus("idle");
        lastSavedDraft.current = "";
        await Promise.all([loadTasks(), loadScoringSummary()]);
        if (!nextTaskId && !canManageTasks) {
          message.info("当前没有更多已分配给你的阅卷任务");
        }
      },
      "人工评分已提交"
    );
  };

  const markDispute = async () => {
    if (!ctx) {
      return;
    }
    const reason = draft.disputeReason.trim() || "教师标记争议，需要重新评阅";
    await runAction(
      "return",
      async () => {
        await returnReviewTask(ctx.task.id, reason, ctx.task.revision);
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
    if (selectedGrade.delivery_mode === "shadow_only") {
      message.warning("本场考试未开放 AI 建议，无法采纳");
      return;
    }
    lastScoreDraftChange.current = {
      score: draft.score,
      rubricSelections: { ...draft.rubricSelections }
    };
    setHasLastScoreDraftChange(true);
    setDraft((current) => ({
      ...current,
      score: selectedGrade.suggested_score,
      studentFeedback: current.studentFeedback || selectedGrade.student_feedback || "",
      privateNote: current.privateNote || selectedGrade.teacher_note || ""
    }));
  };

  const setScoreFromShortcut = (score: number) => {
    lastScoreDraftChange.current = {
      score: draft.score,
      rubricSelections: { ...draft.rubricSelections }
    };
    setHasLastScoreDraftChange(true);
    setDraft((current) => ({ ...current, score }));
    message.info(`已打分 ${score} 分`);
  };

  const toggleCriterionFromShortcut = (index: number) => {
    const point = rubricPoints[index];
    if (!point) return;
    lastScoreDraftChange.current = {
      score: draft.score,
      rubricSelections: { ...draft.rubricSelections }
    };
    setHasLastScoreDraftChange(true);
    setDraft((current) => {
      const nextScore = (current.rubricSelections[point.id] ?? 0) > 0 ? 0 : point.score;
      const rubricSelections = { ...current.rubricSelections, [point.id]: nextScore };
      const score = rubricPoints.reduce((total, item) => total + (rubricSelections[item.id] ?? 0), 0);
      return { ...current, rubricSelections, score };
    });
    message.info(`已${(draft.rubricSelections[point.id] ?? 0) > 0 ? "取消" : "计入"}评分点 ${index + 1}`);
  };

  const undoLastScoreDraftChange = () => {
    const previous = lastScoreDraftChange.current;
    if (!previous) return;
    setDraft((current) => ({ ...current, ...previous }));
    lastScoreDraftChange.current = null;
    setHasLastScoreDraftChange(false);
    message.info("已撤销本次本地评分修改");
  };

  const confirmReturnTask = (asException: boolean) => {
    if (!canReturn || !canEditDraft || !ctx) return;
    Modal.confirm({
      title: asException ? "标记异常并退回重评？" : "退回重新评阅？",
      content: asException
        ? "该答卷会进入重评流程，不会被丢弃。"
        : "该答卷会退出你的队列并进入重评流程。",
      okText: "确认退回",
      okButtonProps: { danger: true },
      cancelText: "取消",
      onOk: () => markDispute()
    });
  };

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const intent = gradingShortcutIntent({
        key: event.key,
        code: event.code,
        ctrlKey: event.ctrlKey,
        isInputTarget: isInputTarget(event.target),
        hasTask: Boolean(ctx),
        canEditDraft,
        canSubmit,
        canReturn,
        canUndoScoreChange,
        hasAiSuggestion: Boolean(selectedGrade && selectedGrade.delivery_mode !== "shadow_only"),
        maxScore,
        rubricPointCount: rubricPoints.length
      });
      if (!intent) return;
      event.preventDefault();
      switch (intent.type) {
        case "submit":
          void submitGrade();
          break;
        case "set_score":
          setScoreFromShortcut(intent.score);
          break;
        case "toggle_criterion":
          toggleCriterionFromShortcut(intent.index);
          break;
        case "adopt_ai":
          adoptAiScore();
          break;
        case "flag_exception":
          confirmReturnTask(true);
          break;
        case "return_task":
          confirmReturnTask(false);
          break;
        case "undo_score_change":
          undoLastScoreDraftChange();
          break;
        case "zoom":
          zoomViewer(intent.direction);
          break;
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [canEditDraft, canReturn, canSubmit, canUndoScoreChange, ctx, draft, fitScale, maxScore, rubricPoints, selectedGrade]);

  const changeViewerMode = (mode: ViewerMode) => {
    if (mode === "original" && !canViewOriginalImage) {
      message.warning("完整原图仅限管理员查看");
      return;
    }
    previewRequestRef.current += 1;
    setViewerMode(mode);
    setPreview(null);
    setImageSize(null);
    setAutoFit(true);
    setFitScale(1);
    setScale(1);
    setRotation(0);
    setOffset({ x: 0, y: 0 });
  };

  const zoomViewer = (direction: -1 | 1) => {
    setAutoFit(false);
    const step = Math.max(0.05, fitScale * 0.15);
    setScale((value) => Math.min(4, Math.max(0.1, Number((value + direction * step).toFixed(3)))));
  };

  const rotateViewer = () => {
    setRotation((value) => (value + 90) % 360);
    setOffset({ x: 0, y: 0 });
  };

  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (scale <= fitScale * 1.001) {
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
    const ocrText = ctx?.ocrText || ctx?.ocrResults.map((item) => item.text).filter(Boolean).join("\n") || "当前页面暂无识别文本。";
    if (viewerMode !== "ocr" && !preview) {
      return <EmptyState title="暂无答卷页面" description="当前任务没有可下载的页面文件。" />;
    }
    const isImage = Boolean(preview?.contentType.startsWith("image/"));
    const isPDF = preview?.contentType === "application/pdf";
    const viewerTransform = isImage
      ? `translate(${offset.x}px, ${offset.y}px) rotate(${rotation}deg)`
      : `translate(${offset.x}px, ${offset.y}px) scale(${scale}) rotate(${rotation}deg)`;
    const imageStyle = isImage && imageSize
      ? { width: imageSize.width * scale, height: imageSize.height * scale }
      : undefined;
    return (
      <div
        ref={viewportRef}
        className={dragging ? "answer-viewer dragging" : "answer-viewer"}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
      >
        <div className={isImage ? "answer-viewer-stage image-stage" : "answer-viewer-stage"} style={{ transform: viewerTransform, ...imageStyle }}>
          {viewerMode === "ocr" ? (
            <pre className="ocr-overlay-text">{ocrText}</pre>
          ) : isImage ? (
            <img
              src={preview!.url}
              alt={preview!.filename ?? "答卷页面"}
              onLoad={(event) => {
                const size = { width: event.currentTarget.naturalWidth, height: event.currentTarget.naturalHeight };
                setImageSize(size);
                window.requestAnimationFrame(() => applyViewerFit(size));
              }}
            />
          ) : isPDF ? (
            <iframe title={preview!.filename ?? "答卷 PDF"} src={preview!.url} />
          ) : (
            <Alert
              type="info"
              showIcon
              message="该文件类型不支持内嵌预览"
              description="可下载后用本机程序查看。"
              action={<Button size="small" href={preview!.url} target="_blank" download={preview!.filename ?? "答卷文件"}>下载查看</Button>}
            />
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

  const renderAutomationSummary = () => {
    const result = ctx?.automationResult;
    if (!result) {
      return (
        <div className="ocr-snippet">
          <span>识别文本</span>
          <p>{draft.answerText || "当前没有可用的识别结果"}</p>
        </div>
      );
    }
    const automaticallyConfirmed = result.grade_source === "rule_confirmed" && typeof result.score === "number";
    return (
      <div className="task-automation-result">
        <div className="task-automation-grid">
          <div><span>识别答案</span><strong>{result.recognized_answer || recognitionDecisionLabels[result.decision ?? ""] || "-"}</strong></div>
          <div><span>标准答案</span><strong>{formatAnswer(result.standard_answer)}</strong></div>
          <div><span>置信度</span><strong>{typeof result.confidence === "number" ? `${Math.round(result.confidence * 100)}%` : "-"}</strong></div>
          <div><span>自动判定</span><strong>{automaticallyConfirmed ? `${result.score} / ${result.max_score ?? maxScore}` : "未生效 · 转人工"}</strong></div>
        </div>
        <div className="task-automation-note">
          <StatusTag tone="warning">{sourceLabels[ctx?.task.source ?? ""] ?? "人工复核"}</StatusTag>
          <span title={[result.source, result.decision].filter(Boolean).join(" / ") || undefined}>{recognitionSourceLabels[result.source ?? ""] ?? "识别服务"} · {recognitionDecisionLabels[result.decision ?? ""] ?? "规则未自动确认"}</span>
        </div>
      </div>
    );
  };

  const renderEvidence = () => {
    if (!selectedGrade) {
      return (
        <div className="ai-empty-state">
          <strong>暂无 AI 建议</strong>
          <span>请按评分细则人工判定。</span>
        </div>
      );
    }
    // confidence 恒为 0 表示该建议还没有可靠的置信度，显示 0% 会误导教师，改为显式状态文案。
    const confidenceCalibrated = selectedGrade.confidence > 0;
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
            {confidenceCalibrated ? (
              <strong>{Math.round(selectedGrade.confidence * 100)}%</strong>
            ) : (
              <Tooltip title="该 AI 建议暂无可靠置信度，请以人工判断为准">
                <span><StatusTag tone="neutral">暂无置信度</StatusTag></span>
              </Tooltip>
            )}
          </div>
          <div>
            <span>判定</span>
            <StatusTag tone={selectedGrade.needs_human_review ? "warning" : selectedGrade.mock ? "warning" : confidenceTone(selectedGrade.confidence)}>
              {selectedGrade.needs_human_review ? "需人工复核" : "未标记风险"}
            </StatusTag>
          </div>
        </div>

        <details className="ai-evidence-details">
          <summary>
            <span>查看 AI 依据</span>
            <small>{selectedGrade.matched_points.length} 个采分点 · {selectedGrade.evidence.length} 条证据 · {selectedGrade.risk_flags.length} 个风险</small>
          </summary>
          <Tabs size="small" items={[
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
                        <span>{point.label || "未命名采分点"}</span>
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
                        <span>{point.label || "未命名采分点"}</span>
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
                        <strong title={item.type || undefined}>{evidenceTypeLabels[item.type] ?? "证据"}</strong>
                        <span>{item.answer_text || item.rule || item.standard_answer || "无文本证据"}</span>
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
                  {selectedGrade.risk_flags.length > 0 ? selectedGrade.risk_flags.map((flag) => <span key={flag} title={flag}><StatusTag tone="warning">{riskFlagLabels[flag] ?? "其他风险"}</StatusTag></span>) : <StatusTag tone="success">无风险标记</StatusTag>}
                </Space>
              )
            }
          ]} />
        </details>
      </div>
    );
  };

  const renderEvidenceJob = () => {
    if (!ctx?.evidenceJob) {
      return null;
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
              <span title={item.code}>{item.message || "校验未通过"}</span>
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
          <div><h2>自动阅卷</h2><p>查看整张答题卡、识别结果和题目得分；异常题目进入人工复核。</p></div>
          <Space wrap>
            <Button icon={<RefreshCw size={15} />} loading={scoringLoading} onClick={() => void loadScoringSummary()}>刷新</Button>
            <Button type={scoringSummary?.run ? "primary" : "default"} icon={<Eye size={15} />} loading={actioning === "scoring-detail"} onClick={() => void showScoringRunDetail()}>逐卷查看</Button>
            {canManageTasks ? <Button icon={<Award size={15} />} onClick={() => setGoldPaperManagerOpen(true)}>标准卷</Button> : null}
            {canManageTasks && (ctx?.task.question_id || scoringSummary?.questions[0]?.question_id) ? <Button icon={<BadgeCheck size={15} />} onClick={() => setCalibrationQuestionId(ctx?.task.question_id || scoringSummary?.questions[0]?.question_id || "")}>阅卷校准</Button> : null}
            {scoringSummary?.run && scoringSummary.run.failed_count > 0 ? <Button icon={<RotateCcw size={15} />} loading={actioning === "retry-scoring"} onClick={() => void retryFailedScoring()}>重新处理失败项</Button> : null}
            {scoringSummary?.run && ["queued", "processing", "needs_review", "failed"].includes(scoringSummary.run.status) ? <Popconfirm title="取消本次评分？" description="未完成的自动处理和人工任务将停止，已保留的历史结果不会删除。" okText="取消评分" cancelText="保留" okButtonProps={{ danger: true }} onConfirm={() => void cancelCurrentScoringRun()}>
              <Button danger icon={<CircleStop size={15} />} loading={actioning === "cancel-scoring"}>取消评分</Button>
            </Popconfirm> : null}
            {!hasUnresolvedScoringRun && scoringSummary?.run ? (
              <Popconfirm title="重新开始自动评分？" description="已确认的结果会保留，未完成项将重新处理" okText="重新评分" cancelText="暂不" onConfirm={() => void startExamScoring()}>
                <Button icon={<Play size={15} />} disabled={!scoringReadiness?.ready} loading={actioning === "start-scoring"}>重新评分</Button>
              </Popconfirm>
            ) : !hasUnresolvedScoringRun ? <Button type="primary" icon={<Play size={15} />} disabled={!scoringReadiness?.ready} loading={actioning === "start-scoring"} onClick={() => void startExamScoring()}>开始评分</Button> : null}
          </Space>
        </div>
        {scoringReadiness ? (
          <Alert
            className="grading-readiness-alert"
            type={hasUnresolvedScoringRun ? "info" : scoringReadiness.ready ? "success" : "error"}
            showIcon
            message={hasUnresolvedScoringRun
              ? "已有评分任务，不能重复启动"
              : scoringReadiness.ready
                ? "评分启动条件已满足"
                : `${scoringBlockingChecks.length} 项条件阻止启动评分`}
            description={(
              <div className="grading-readiness-content">
                <div className="grading-readiness-counts">
                  <span>题目 <strong>{scoringReadiness.total_questions}</strong></span>
                  <span>题块 <strong>{scoringReadiness.ready_segments}/{scoringReadiness.total_segments}</strong></span>
                  <span>预计自动 <strong>{scoringReadiness.automatic_candidates}</strong></span>
                  <span>预计人工 <strong>{scoringReadiness.manual_review_candidates}</strong></span>
                </div>
                {(hasUnresolvedScoringRun ? scoringBlockingChecks.filter((check) => check.code === "active_run_clear") : [...scoringBlockingChecks, ...scoringWarningChecks]).length ? (
                  <ul className="grading-readiness-issues">
                    {(hasUnresolvedScoringRun ? scoringBlockingChecks.filter((check) => check.code === "active_run_clear") : [...scoringBlockingChecks, ...scoringWarningChecks]).map((check) => (
                      <li key={check.code} className={check.severity}>
                        <strong>{check.label}</strong>
                        <span>{check.message}</span>
                      </li>
                    ))}
                  </ul>
                ) : <span className="grading-readiness-ok">全部题块均可进入自动处理或人工复核。</span>}
              </div>
            )}
          />
        ) : null}
        {scoringSummary?.run ? <div className="grading-run-strip">
          <StatusTag tone={scoringSummary.run.status === "completed" ? "success" : scoringSummary.run.status === "failed" ? "danger" : scoringSummary.run.status === "needs_review" ? "warning" : "processing"}>{scoringRunStatusLabels[scoringSummary.run.status] ?? "处理中"}</StatusTag>
          <span>总计 <strong>{scoringSummary.run.total_count}</strong></span>
          <span>处理中 <strong>{scoringSummary.run.queued_count}</strong></span>
          <span>自动确认 <strong>{scoringSummary.run.auto_confirmed_count}</strong></span>
          <span>人工完成 <strong>{scoringSummary.run.human_confirmed_count}</strong></span>
          <span>待人工 <strong>{scoringSummary.run.review_count}</strong></span>
          <span>失败 <strong>{scoringSummary.run.failed_count}</strong></span>
        </div> : <Alert type="info" showIcon message="尚未开始评分" description="请先完成答题卡上传与题目切分，再点击右上角“开始评分”；无法自动判分的答案会转入人工阅卷。" />}
        <ResponsiveTable size="small" pagination={false} loading={scoringLoading} rowKey="question_id" dataSource={scoringSummary?.questions ?? []} columns={[
          { title: "题号", dataIndex: "question_no", width: 90 },
          { title: "题型", dataIndex: "question_type", width: 140, render: (value: string) => <span title={value}>{questionTypeLabels[value] ?? "其他题型"}</span> },
          { title: "答卷", dataIndex: "total", width: 80 },
          { title: "处理中", dataIndex: "queued", width: 90 },
          { title: "已确认", dataIndex: "confirmed", width: 90 },
          { title: "待人工", dataIndex: "review", width: 90 },
          { title: "失败", dataIndex: "failed", width: 80 }
        ]} />
      </section> : null}
      <OcrWorkerAlert enabled={canWork} />
      <Drawer
        title="自动阅卷 · 逐卷查看"
        width="min(1680px, 98vw)"
        open={scoringDetailOpen}
        onClose={() => setScoringDetailOpen(false)}
        destroyOnClose={false}
      >
        <div className="automation-results">
          <ScoringPaperMonitor
            run={scoringSummary?.run}
            items={scoringRunDetail?.items ?? []}
            loading={actioning === "scoring-detail"}
            onRefresh={() => void showScoringRunDetail()}
          />
          <div className="automation-results-head">
            <div>
              <strong>全部判分结果</strong>
              <span>逐条核对每道题的识别结果与自动判分，可按题型和状态筛选。</span>
            </div>
            <div className="automation-result-metrics">
              <span>总计 <strong>{scoringResultMetrics.total}</strong></span>
              <span>自动确认 <strong>{scoringResultMetrics.auto}</strong></span>
              <span>待人工 <strong>{scoringResultMetrics.review}</strong></span>
              <span className={scoringResultMetrics.failed ? "danger" : ""}>失败 <strong>{scoringResultMetrics.failed}</strong></span>
            </div>
          </div>
          <div className="automation-result-filters">
            <Segmented<ScoringResultType>
              value={scoringResultType}
              options={[
                { label: "全部题型", value: "all" },
                { label: "选择题", value: "choice" },
                { label: "填空与数值", value: "fill" }
              ]}
              onChange={setScoringResultType}
            />
            <Select<ScoringResultState>
              value={scoringResultState}
              options={[
                { label: "全部状态", value: "all" },
                { label: "自动/人工已确认", value: "confirmed" },
                { label: "待人工复核", value: "review" },
                { label: "处理失败", value: "failed" },
                { label: "处理中", value: "processing" }
              ]}
              onChange={setScoringResultState}
            />
            <Input
              allowClear
              prefix={<Search size={15} />}
              value={scoringResultKeyword}
              placeholder="搜索匿名码、题号或答案"
              onChange={(event) => setScoringResultKeyword(event.target.value)}
            />
            <span className="muted">{filteredScoringItems.length} 条</span>
          </div>
          <ResponsiveTable<ScoringRunItem>
            className="automation-result-table"
            size="small"
            rowKey="answer_segment_id"
            dataSource={filteredScoringItems}
            pagination={{ pageSize: 15, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
            mobilePrimaryCount={4}
            columns={[
              { title: "匿名码", dataIndex: "anonymous_code", width: 138, ellipsis: true },
              { title: "题目", width: 100, render: (_, item) => <div className="automation-result-cell"><strong>{item.question_no}</strong><span title={item.question_type}>{questionTypeLabels[item.question_type] ?? "其他题型"}</span></div> },
              { title: "识别答案", width: 150, render: (_, item) => <div className="automation-result-cell"><strong>{item.recognized_answer || recognitionDecisionLabels[item.recognition_decision ?? ""] || "-"}</strong><span title={[item.recognition_source, item.recognition_decision].filter(Boolean).join(" / ") || undefined}>{recognitionSourceLabels[item.recognition_source ?? ""] ?? "未识别"} · {recognitionDecisionLabels[item.recognition_decision ?? ""] ?? "待处理"}</span></div> },
              { title: "标准答案", width: 135, render: (_, item) => <span className="automation-standard-answer">{formatAnswer(item.standard_answer)}</span> },
              { title: "置信度", width: 92, render: (_, item) => typeof item.recognition_confidence === "number" ? <StatusTag tone={confidenceTone(item.recognition_confidence)}>{`${Math.round(item.recognition_confidence * 100)}%`}</StatusTag> : "-" },
              { title: "判分", width: 120, render: (_, item) => <div className="automation-result-cell"><strong>{typeof item.score === "number" ? `${item.score} / ${item.max_score ?? "-"}` : "-"}</strong><span>{gradingConclusion(item)}</span></div> },
              { title: "处理结果", width: 118, render: (_, item) => { const state = resultState(item); return <StatusTag tone={state.tone}>{state.label}</StatusTag>; } },
              { title: "说明", width: 180, ellipsis: true, render: (_, item) => {
                const rawCode = item.error_code || item.reason_code;
                if (rawCode) {
                  return <Tooltip title={rawCode}><span>{reasonLabels[rawCode] ?? "处理异常"}</span></Tooltip>;
                }
                if (item.grade_source === "rule_confirmed") return "规则与证据通过";
                if (item.state === "confirmed") return "人工复核完成";
                const status = item.runtime_status || item.review_status;
                if (!status) return "-";
                return <span title={status}>{scoringItemStateLabels[status] ?? taskStatusLabels[status] ?? reasonLabels[status] ?? "处理中"}</span>;
              } },
              { title: "操作", width: 88, fixed: "right", render: (_, item) => <Button type="link" size="small" icon={<Eye size={14} />} loading={scoringImageLoading === item.answer_segment_id} onClick={() => void showScoringResultImage(item)}>查看答题图</Button> }
            ]}
          />
        </div>
      </Drawer>
      <Modal title={scoringImage?.title ?? "答题图"} open={Boolean(scoringImage)} footer={null} width={920} onCancel={() => {
        scoringImageRequestRef.current += 1;
        setScoringImageLoading("");
        setScoringImage(null);
      }} destroyOnClose>
        {scoringImage?.contentType.startsWith("image/") ? <div className="automation-image-preview"><img src={scoringImage.url} alt={scoringImage.title} /></div> : <Alert type="warning" showIcon message="该答题片段不是可直接预览的图片格式" />}
      </Modal>
      <section className="grading-topbar">
        <div className="grading-work-title">
          <h1>阅卷</h1>
          <span>
            {!canManageTasks
              ? `已完成 ${myProgress?.completed ?? 0} / 共 ${myProgress?.total ?? 0}`
              : ctx
                ? `剩余 ${remainingCount} 份`
                : "请选择任务"}
          </span>
        </div>
        <Space wrap>
          <span className={`draft-save-status ${draftSaveStatus}`}>{draftSaveStatus === "saving" ? "正在保存" : draftSaveStatus === "saved" ? "草稿已保存" : draftSaveStatus === "offline" ? "离线草稿待同步" : draftSaveStatus === "conflict" ? "草稿冲突" : draftSaveStatus === "error" ? "草稿保存失败" : draftSaveStatus === "readonly" ? "管理员只读" : "尚未修改"}</span>
          <Button icon={<RefreshCw size={16} />} onClick={() => void refreshCurrent()} loading={loadingTasks || contextLoading}>
            刷新
          </Button>
          <Button icon={<ArrowRight size={16} />} onClick={() => void goNext()} disabled={!canWork}>
            下一份
          </Button>
          {!canManageTasks ? <BackmarkQueue canWork={canWork} /> : null}
          {!canManageTasks ? <RegradeQueue canWork={canWork} /> : null}
          {!canManageTasks ? <Button type="primary" icon={<BadgeCheck size={16} />} loading={actioning === "claim-task"} disabled={!canWork} onClick={() => void claimTask()}>领取任务</Button> : null}
          <Tooltip title="把这份答卷放回队列，稍后可继续，草稿会保留">
            <Button icon={<LogOut size={16} />} disabled={!ownsSelectedTask} loading={actioning === "release"} onClick={() => void releaseCurrentTask()}>
              暂放
            </Button>
          </Tooltip>
        </Space>
      </section>

      {draftSaveStatus === "conflict" ? <Alert type="error" showIcon message="草稿已被其他会话更新" description="为防止覆盖他人修改，自动保存已暂停。重新载入任务后再应用本地修改。" action={<Button onClick={() => void loadContext(selectedTaskId)}>重新载入</Button>} /> : draftSaveStatus === "offline" ? <Alert type="warning" showIcon message="当前离线，草稿已保存在本机" description="恢复网络后会按版本号同步；提交或退出后会清理本机草稿。" /> : draftSaveStatus === "error" ? <Alert type="warning" showIcon message="草稿暂未保存到服务端" description="本机保留了短期草稿；检查网络后系统会再次尝试保存。" /> : draftSaveStatus === "readonly" ? <Alert type="info" showIcon message="管理员只读检查" description="管理员可查看材料、分配和管理任务；评分草稿与最终提交只能由被分配的阅卷员完成。" /> : null}

      {canManageTasks ? (
        <section className="reviewer-progress-panel" aria-label="阅卷员进度">
          <div className="reviewer-progress-head">
            <div>
              <h2>阅卷员进度</h2>
            </div>
            <span className="muted">{`${reviewerProgress.length} 名阅卷员`}</span>
          </div>
          <div className="reviewer-progress-list">
            {reviewerProgress.length ? reviewerProgress.map((reviewer) => (
              <div className="reviewer-progress-row" key={reviewer.id}>
                <div className="reviewer-progress-label">
                  <strong>{reviewer.name}</strong>
                  <span>{reviewer.completed} / {reviewer.total} 题</span>
                </div>
                <Progress percent={reviewer.percent} showInfo={false} status={reviewer.percent === 100 ? "success" : "active"} />
                <div className="reviewer-progress-meta">
                  <span>进行中 {reviewer.active}</span>
                  <span>待完成 {reviewer.total - reviewer.completed}</span>
                  <strong>{reviewer.percent}%</strong>
                </div>
              </div>
            )) : <span className="muted">暂无已分配的阅卷任务</span>}
          </div>
        </section>
      ) : null}

      <section className="grading-workspace">
        <aside className="grading-task-rail">
          <section className="grading-taskbar">
            <Select className="toolbar-select" value={taskFilter} options={taskFilterOptions} onChange={setTaskFilter} />
            <Input prefix={<Search size={16} />} placeholder="搜索任务" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
            <span className="muted">{filteredTasks.length} / {tasks.length}</span>
          </section>
          {canManageTasks ? (
            <section className="grading-assignment-bar" aria-label="分配阅卷任务">
              <div className="grading-assignment-select-all">
                <Checkbox
                  checked={assignableTasks.length > 0 && assignableTasks.every((task) => assignmentTaskIds.includes(task.id))}
                  indeterminate={assignmentTaskIds.length > 0 && !assignableTasks.every((task) => assignmentTaskIds.includes(task.id))}
                  disabled={assignableTasks.length === 0}
                  onChange={(event) => setAssignmentTaskIds(event.target.checked ? assignableTasks.map((task) => task.id) : [])}
                >
                  {assignmentTaskIds.length ? `已选 ${assignmentTaskIds.length}` : "选择任务"}
                </Checkbox>
              </div>
              <Select
                value={assignmentUserId || undefined}
                options={graderOptions}
                placeholder="选择阅卷员"
                showSearch
                optionFilterProp="label"
                status={gradersError ? "warning" : undefined}
                onChange={setAssignmentUserId}
              />
              <Tooltip title={gradersError ?? "分配选中的阅卷任务"}>
                <Button
                  type="primary"
                  icon={<UserRoundCheck size={15} />}
                  disabled={!assignmentUserId || assignmentTaskIds.length === 0 || Boolean(gradersError)}
                  loading={actioning === "assign-tasks"}
                  onClick={() => void assignSelectedTasks()}
                >
                  分配
                </Button>
              </Tooltip>
            </section>
          ) : null}
          <div className="grading-task-list">
            {loadingTasks ? (
              <LoadingState label="正在读取阅卷任务" />
            ) : taskError ? (
              <ErrorState message={taskError} onRetry={() => void loadTasks()} />
            ) : filteredTasks.length === 0 ? (
              <EmptyState title="暂无阅卷任务" description="当前筛选下没有需要处理的答卷。" />
            ) : (
              <List
                dataSource={filteredTasks}
                loadMore={hasMoreTasks ? (
                  <div className="grading-task-load-more">
                    <Button loading={loadingMoreTasks} onClick={() => void loadMoreTasks()}>加载更多</Button>
                  </div>
                ) : null}
                renderItem={(task) => (
                  <List.Item
                    className={task.id === selectedTaskId ? "grading-task-item active" : "grading-task-item"}
                    onClick={() => {
                      suppressAutoSelectRef.current = false;
                      setSelectedTaskId(task.id);
                    }}
                  >
                    {canManageTasks ? (
                      <Checkbox
                        checked={assignmentTaskIds.includes(task.id)}
                        disabled={["submitted", "completed", "in_progress"].includes(task.status)}
                        aria-label={`选择 ${task.question_no} ${task.anonymous_code}`}
                        onClick={(event) => event.stopPropagation()}
                        onChange={(event) => setAssignmentTaskIds((current) => event.target.checked
                          ? [...new Set([...current, task.id])]
                          : current.filter((taskId) => taskId !== task.id))}
                      />
                    ) : null}
                    <div className="grading-task-primary">
                      <strong>{task.anonymous_code || "暂无匿名码"}</strong>
                      <span title={sourceLabels[task.source] ? undefined : task.source}>{task.question_no} · {sourceLabels[task.source] ?? "其他来源"}{canManageTasks && task.assigned_to ? ` · ${graderNames[task.assigned_to] ?? "已分配"}` : ""}</span>
                    </div>
                    <div className="grading-task-status">
                      <StatusTag tone={taskTone(task.status)}>{taskStatusLabels[task.status] ?? "未知状态"}</StatusTag>
                      {task.priority >= 80 ? <StatusTag tone="warning">优先</StatusTag> : null}
                    </div>
                  </List.Item>
                )}
              />
            )}
          </div>
        </aside>

        {contextLoading ? (
          <main className="grading-main-empty">
            <LoadingState label="正在加载答卷" />
          </main>
        ) : contextError ? (
          <main className="grading-main-empty">
            <ErrorState message={contextError} onRetry={() => void loadContext(selectedTaskId)} />
          </main>
        ) : !ctx ? (
          <main className="grading-main-empty">
            <EmptyState
              title="请选择一份答卷"
              description={tasks.length > 0
                ? canManageTasks ? "从左侧队列选择需要检查的答卷。" : "从左侧任务队列选择一份答卷开始阅卷。"
                : canManageTasks ? "当前没有待处理任务。" : "当前没有已分配任务，请等待管理员分配。"}
            />
          </main>
        ) : (
          <main className="grading-main">
            {ctx.warnings.length > 0 ? <Alert type="warning" showIcon message="AI 辅助不可用" description={ctx.warnings.join("；")} /> : null}

            <section className="grading-panels">
              <section className="answer-panel">
                <div className="panel-head">
                  <div>
                    <h2>{ctx.task.question_no} · 学生答案</h2>
                    <p title={[sourceLabels[ctx.task.source] ? "" : ctx.task.source, taskStatusLabels[ctx.task.status] ? "" : ctx.task.status].filter(Boolean).join(" / ") || undefined}>{ctx.task.anonymous_code || "暂无匿名码"} · {sourceLabels[ctx.task.source] ?? "其他来源"} · {taskStatusLabels[ctx.task.status] ?? "未知状态"}</p>
                  </div>
                  <Space wrap>
                    <Segmented<ViewerMode>
                      size="small"
                      value={viewerMode}
                      options={canViewOriginalImage
                        ? [{ label: "答题区域", value: "segment" }, { label: "原图", value: "original" }, { label: "识别文本", value: "ocr" }]
                        : [{ label: "答题区域", value: "segment" }, { label: "识别文本", value: "ocr" }]}
                      onChange={changeViewerMode}
                    />
                    <Tooltip title="缩小">
                      <Button icon={<ZoomOut size={14} />} onClick={() => zoomViewer(-1)} aria-label="缩小答题图" />
                    </Tooltip>
                    <span className="viewer-scale">{Math.round(scale * 100)}%</span>
                    <Tooltip title="放大">
                      <Button icon={<ZoomIn size={14} />} onClick={() => zoomViewer(1)} aria-label="放大答题图" />
                    </Tooltip>
                    <Tooltip title="顺时针旋转">
                      <Button icon={<RotateCcw size={14} />} onClick={rotateViewer} aria-label="旋转答题图" />
                    </Tooltip>
                    <Tooltip title="适配窗口">
                      <Button type={autoFit ? "primary" : "default"} icon={<Maximize2 size={14} />} onClick={() => applyViewerFit(imageSize, true)} aria-label="适配窗口" />
                    </Tooltip>
                  </Space>
                </div>
                {renderViewerContent()}
                {ctx.segmentImageUrl ? (
                  <details className="grading-more-fields">
                    <summary>答题区域批注与常用评语</summary>
                    <div className="grading-more-fields-body">
                      <ReviewAnnotationWorkspace
                        key={ctx.task.id}
                        reviewTaskId={ctx.task.id}
                        imageUrl={ctx.segmentImageUrl}
                        imageAlt={`${ctx.task.question_no} 答题区域`}
                        disabled={!canEditDraft}
                        rubricCriteria={rubricPoints.map((point) => ({
                          id: point.id,
                          label: pointLabel(point)
                        }))}
                      />
                    </div>
                  </details>
                ) : null}
              </section>

              <aside className="grading-inspector">
                <ReviewContextInspector key={ctx.task.id} context={ctx.reviewContext} />
                <MathEvidenceInspector
                  key={`math-${ctx.task.id}`}
                  segmentId={ctx.task.answer_segment_id}
                  subjectCode={ctx.reviewContext.subject_tool_hints.subject_code}
                  disabled={!canEditDraft}
                />
                {!requiresExplicitSecondOpinion(ctx.reviewContext) ? renderAutomationSummary() : null}
                {selectedGrade ? renderEvidence() : null}
                {renderEvidenceJob()}
                {canVerifyEvidence && selectedGrade ? (
                  <Button
                    icon={<BadgeCheck size={14} />}
                    loading={actioning === "evidence"}
                    onClick={() => void runAction(
                      "evidence",
                      async () => {
                        const result = await verifyEvidence(selectedGrade.id);
                        setCtx((current) => (current ? { ...current, evidenceJob: result.job } : current));
                      },
                      "证据校验完成"
                    )}
                  >
                    校验 AI 证据
                  </Button>
                ) : null}

                <section className="score-panel">
                <div className="panel-head">
                  <div>
                    <h2>最终评分</h2>
                    <p>满分 {maxScore || "—"}</p>
                  </div>
                  {selectedGrade && !requiresExplicitSecondOpinion(ctx.reviewContext) ? (
                    <Button icon={<Check size={14} />} disabled={!canEditDraft} onClick={adoptAiScore}>
                      采纳 AI 建议
                    </Button>
                  ) : null}
                </div>

                <SharedScoreControl
                  disabled={!canEditDraft}
                  maxScore={maxScore}
                  rubricPoints={rubricPoints}
                  value={{ score: draft.score, rubricSelections: draft.rubricSelections }}
                  onChange={(value) => setDraft((current) => ({ ...current, ...value }))}
                />

                <details className="grading-more-fields">
                  <summary>评语与备注（可选）</summary>
                  <div className="grading-more-fields-body">
                    <div className="grading-comment-presets">
                      <span className="muted">常用评语（点击追加）</span>
                      <Space wrap size={4}>
                        {commentPresets.map((item) => (
                          <Button
                            key={item}
                            size="small"
                            disabled={!canEditDraft}
                            onClick={() => setDraft((current) => ({ ...current, comments: current.comments ? `${current.comments}；${item}` : item }))}
                          >
                            {item}
                          </Button>
                        ))}
                      </Space>
                    </div>
                    <div className="grading-field">
                      <label className="grading-field-label muted" htmlFor="grading-comments-input">教师评语（阅卷记录）</label>
                      <Input.TextArea id="grading-comments-input" disabled={!canEditDraft} rows={2} placeholder="记录评分依据（可选）" value={draft.comments} onChange={(event) => setDraft((current) => ({ ...current, comments: event.target.value }))} />
                    </div>
                    <div className="grading-field">
                      <div className="grading-field-label">
                        <label className="muted" htmlFor="grading-student-feedback-input">学生可见反馈（将展示给学生）</label>
                        <StatusTag tone="warning">学生可见</StatusTag>
                      </div>
                      <Input.TextArea id="grading-student-feedback-input" disabled={!canEditDraft} rows={2} placeholder="这段文字会展示给学生（可选）" value={draft.studentFeedback} onChange={(event) => setDraft((current) => ({ ...current, studentFeedback: event.target.value }))} />
                    </div>
                    <div className="grading-field">
                      <label className="grading-field-label muted" htmlFor="grading-private-note-input">私密备注（仅教师可见）</label>
                      <Input.TextArea id="grading-private-note-input" disabled={!canEditDraft} rows={2} placeholder="仅教师与管理端可见（可选）" value={draft.privateNote} onChange={(event) => setDraft((current) => ({ ...current, privateNote: event.target.value }))} />
                    </div>
                    {canReturn ? <Input disabled={!canEditDraft} placeholder="争议原因（可选，退回重评时会一并记录）" prefix={<Flag size={14} />} value={draft.disputeReason} onChange={(event) => setDraft((current) => ({ ...current, disputeReason: event.target.value }))} /> : null}
                  </div>
                </details>

                <div className="grading-submit-note">提交后进入质检，不会直接发布成绩</div>
                <div className="muted">快捷键：有评分点时 1–9 勾选评分点并重算总分；否则 1–9 直接打分 · A 采纳建议 · Enter 提交 · F 异常 · R 退回 · Z 撤销 · +/- 缩放</div>

                <Space wrap className="grading-submit-bar">
                  {canReturn ? (
                    <Popconfirm
                      title="标记争议并退回重评？"
                      description="该答卷将退出你的队列，进入重评流程"
                      okText="确认退回"
                      cancelText="再想想"
                      okButtonProps={{ danger: true }}
                      onConfirm={() => void markDispute()}
                    >
                      <Button danger icon={<Flag size={16} />} disabled={!ctx} loading={actioning === "return"}>
                        标记争议
                      </Button>
                    </Popconfirm>
                  ) : null}
                  {canManageTasks ? (
                    <Button icon={<Award size={16} />} disabled={!canSubmit || !ctx || ctx.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void submitGrade(true)}>
                      提交并提名标准卷
                    </Button>
                  ) : null}
                  <Tooltip title="Ctrl+Enter">
                    <Button type="primary" icon={<CheckCircle2 size={16} />} disabled={!canSubmit || !ctx || ctx.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void submitGrade()}>
                      提交并下一份
                    </Button>
                  </Tooltip>
                </Space>
                </section>
              </aside>
            </section>

          </main>
        )}
      </section>
      <GoldPaperManagerDrawer open={goldPaperManagerOpen} examId={initialExamId || undefined} onClose={() => setGoldPaperManagerOpen(false)} />
      <CalibrationDrawer
        open={Boolean(calibrationQuestionId)}
        examId={initialExamId || ctx?.task.exam_id || ""}
        questionId={calibrationQuestionId}
        graderId={currentUserId}
        canManagePolicy={canManageTasks}
        onClose={() => setCalibrationQuestionId("")}
        onQualified={() => { message.success("校准已通过，可以重新领取本题任务"); void loadTasks(); }}
      />
      <GoldPaperNominationDrawer candidate={goldPaperCandidate} onClose={() => setGoldPaperCandidate(null)} onCreated={() => setGoldPaperManagerOpen(true)} />
    </div>
  );
}
