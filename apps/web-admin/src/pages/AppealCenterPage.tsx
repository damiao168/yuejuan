import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Alert, App, Button, Descriptions, Input, InputNumber, List, Select, Space, Tag, type TableColumnsType } from "antd";
import { CheckCircle2, FileWarning, Gavel, LockKeyhole, RefreshCw, Search, Send, UserRoundCheck, XCircle } from "lucide-react";
import { ApiClientError } from "../api/client";
import { listAuditLogs, type AuditLog } from "../api/audit";
import { listExams, type Exam } from "../api/exams";
import { listClasses, listStudents, type SchoolClass, type Student } from "../api/org";
import {
  assignAppeal,
  closeAppeal,
  getAppeal,
  getAppealStatistics,
  listAppeals,
  reviewAppeal,
  submitAppealRecommendation,
  type Appeal,
  type AppealStatistics,
  type ReviewAppealPayload,
  type SubmitAppealRecommendationPayload
} from "../api/appeals";
import { listManagedUsers, type ManagedUser } from "../api/users";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { examSubjectLabel } from "../constants/examStatus";
import { ResponsiveTable } from "../components/ResponsiveTable";
import { StatusTag } from "../components/StatusTag";
import { QuestionAppealWorkspace } from "../components/QuestionAppealWorkspace";
import type { SessionUser } from "../auth/session";
import type { StatusTone } from "../types";
import type { ProductExperience } from "../router/experience";

interface AppealCenterPageProps {
  mode: ProductExperience;
  canRead: boolean;
  canManage: boolean;
  canWork: boolean;
  canReadAudit: boolean;
  canReadIdentities: boolean;
  canReadExams: boolean;
  currentUser: SessionUser;
  initialExamId?: string;
}

interface IdentityMaps {
  students: Record<string, Student>;
  classes: Record<string, SchoolClass>;
  exams: Record<string, Exam>;
  error?: string;
}

const statusLabels: Record<string, string> = {
  submitted: "已提交",
  under_review: "处理中",
  need_more_info: "需补充材料",
  accepted: "已接受",
  rejected: "已驳回",
  score_adjusted: "已改分",
  closed: "已关闭"
};

const targetLabels: Record<string, string> = {
  exam: "整场考试",
  question: "题目",
  deduction_point: "扣分点"
};

const reviewOptions = [
  {
    label: "流转中",
    options: [
      { value: "under_review", label: "标记处理中" },
      { value: "need_more_info", label: "要求补充材料" }
    ]
  },
  {
    label: "最终结论",
    options: [
      { value: "accepted", label: "接受申诉" },
      { value: "rejected", label: "驳回申诉" },
      { value: "score_adjusted", label: "调整分数" }
    ]
  }
];

const recommendationLabels: Record<string, string> = {
  accept: "建议受理",
  reject: "建议驳回",
  adjust_score: "建议改分",
  need_more_info: "建议补充材料"
};

const recommendationOptions = Object.entries(recommendationLabels).map(([value, label]) => ({ value, label }));

const appealWorkerRoles = new Set(["teacher", "grader", "arbitrator"]);
const workerRoleLabels: Record<string, string> = { teacher: "教师", grader: "阅卷员", arbitrator: "仲裁员" };

const auditActionLabels: Record<string, string> = {
  "appeal.submitted": "申诉提交",
  "appeal.assigned": "申诉分派",
  "appeal.reviewed": "申诉处理",
  "appeal.score_adjusted": "申诉改分",
  "appeal.closed": "申诉关闭",
  "score_adjustment.created": "改分记录生成"
};

const auditTargetLabels: Record<string, string> = {
  appeal: "申诉",
  score_adjustment: "改分记录"
};

const finalGradeSourceLabels: Record<string, string> = {
  ai: "AI 评分",
  human: "人工评分",
  arbitration: "仲裁定分",
  appeal: "申诉改分"
};

const finalGradeStatusLabels: Record<string, string> = {
  finalized: "已定分",
  locked: "已锁定",
  pending: "待定分"
};

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    console.warn("申诉中心请求失败", error.status, error.code, error.message);
    return error.message || "操作失败，请稍后重试";
  }
  if (error instanceof Error) {
    return error.message || "操作失败，请稍后重试";
  }
  return "操作失败，请稍后重试";
}

function formatTime(value?: string) {
  if (!value) {
    return "-";
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString("zh-CN", { hour12: false });
}

function formatScore(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return "-";
  }
  return Number(value.toFixed(2)).toString();
}

function statusTone(status: string): StatusTone {
  if (status === "accepted" || status === "score_adjusted") {
    return "success";
  }
  if (status === "rejected") {
    return "danger";
  }
  if (status === "submitted" || status === "need_more_info") {
    return "warning";
  }
  if (status === "under_review") {
    return "processing";
  }
  return "neutral";
}

function mapNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function finalScore(appeal: Appeal | null, key: string) {
  return appeal?.evidence?.final_grade ? mapNumber(appeal.evidence.final_grade[key]) : undefined;
}

async function loadIdentities(canReadIdentities: boolean, canReadExams: boolean, studentIDs: string[] = []): Promise<IdentityMaps> {
  const output: IdentityMaps = { students: {}, classes: {}, exams: {} };
  const [studentsResult, classesResult, examsResult] = await Promise.allSettled([
    canReadIdentities && studentIDs.length > 0 ? listStudents({ ids: studentIDs, limit: 200 }) : Promise.resolve({ students: [] }),
    canReadIdentities ? listClasses() : Promise.resolve({ classes: [] }),
    canReadExams ? listExams({ limit: 200 }) : Promise.resolve({ exams: [] })
  ]);
  if (studentsResult.status === "fulfilled") {
    output.students = Object.fromEntries(studentsResult.value.students.map((item) => [item.id, item]));
  } else {
    output.error = formatError(studentsResult.reason);
  }
  if (classesResult.status === "fulfilled") {
    output.classes = Object.fromEntries(classesResult.value.classes.map((item) => [item.id, item]));
  } else {
    output.error = formatError(classesResult.reason);
  }
  if (examsResult.status === "fulfilled") {
    output.exams = Object.fromEntries(examsResult.value.exams.map((item) => [item.id, item]));
  } else {
    output.error = formatError(examsResult.reason);
  }
  return output;
}

function EvidenceBlock({ title, children, empty }: { title: string; children?: ReactNode; empty: string }) {
  return (
    <div className="appeal-evidence-block">
      <strong>{title}</strong>
      {children || <span>{empty}</span>}
    </div>
  );
}

function textValue(value: unknown) {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function recordList(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value) ? value.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object" && !Array.isArray(item)) : [];
}

function formatFileSize(size: number) {
  if (size >= 1024 * 1024) {
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  }
  if (size >= 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${size} B`;
}

function AttachmentInfo({ value }: { value: Record<string, unknown> }) {
  const name = textValue(value.file_name) ?? textValue(value.filename) ?? textValue(value.name);
  const size = mapNumber(value.size) ?? mapNumber(value.file_size);
  const uploadedAt = textValue(value.uploaded_at) ?? textValue(value.created_at);
  const description = textValue(value.description) ?? textValue(value.remark);
  const lines: string[] = [];
  if (name) {
    lines.push(`文件名：${name}`);
  }
  if (size !== undefined) {
    lines.push(`大小：${formatFileSize(size)}`);
  }
  if (uploadedAt) {
    lines.push(`上传时间：${formatTime(uploadedAt)}`);
  }
  if (description) {
    lines.push(`说明：${description}`);
  }
  if (lines.length === 0) {
    return <span className="muted">附件信息格式异常</span>;
  }
  return (
    <div className="appeal-attachment-info">
      {lines.map((line) => <span key={line}>{line}</span>)}
    </div>
  );
}

function ScoreEvidence({ items, ai = false }: { items: Record<string, unknown>[]; ai?: boolean }) {
  return (
    <div className="appeal-evidence-list">
      {items.map((item, index) => {
        const score = mapNumber(item[ai ? "suggested_score" : "score"]);
        const maximum = mapNumber(item.max_score);
        const round = textValue(item.grade_round);
        const comments = textValue(item.comments);
        const confidence = mapNumber(item.confidence);
        return (
          <div className="appeal-evidence-line" key={`${score ?? "score"}-${index}`}>
            <strong>{formatScore(score)}{maximum === undefined ? "" : ` / ${formatScore(maximum)}`}</strong>
            <span>{ai ? (confidence === undefined ? "AI 建议" : `置信度 ${Math.round(confidence * 100)}%`) : (round ? `第 ${round} 轮` : "人工评分")}</span>
            {comments ? <small>{comments}</small> : null}
          </div>
        );
      })}
    </div>
  );
}

function FinalGradeEvidence({ value }: { value: Record<string, unknown> }) {
  const score = mapNumber(value.score);
  const maximum = mapNumber(value.max_score);
  const source = textValue(value.source);
  const status = textValue(value.status);
  return (
    <div className="appeal-final-grade">
      <strong>{formatScore(score)}{maximum === undefined ? "" : ` / ${formatScore(maximum)}`}</strong>
      <span>{(source ? finalGradeSourceLabels[source] : undefined) ?? "最终评分"} · {(status ? finalGradeStatusLabels[status] : undefined) ?? "状态未知"}</span>
    </div>
  );
}

function RubricEvidence({ value }: { value: Record<string, unknown> }) {
  const points = recordList(value.points);
  return (
    <div className="appeal-rubric-list">
      {points.map((point, index) => (
        <div key={`${textValue(point.id) ?? "point"}-${index}`}>
          <span>{textValue(point.description) ?? `评分点 ${index + 1}`}</span>
          <strong>{formatScore(mapNumber(point.score))} 分</strong>
        </div>
      ))}
    </div>
  );
}

export function AppealCenterPage({ mode, canRead, canManage, canWork, canReadAudit, canReadIdentities, canReadExams, currentUser, initialExamId = "" }: AppealCenterPageProps) {
  const { message, modal } = App.useApp();
  const [appeals, setAppeals] = useState<Appeal[]>([]);
  const [selectedAppealId, setSelectedAppealId] = useState("");
  const [selectedAppeal, setSelectedAppeal] = useState<Appeal | null>(null);
  const [statistics, setStatistics] = useState<AppealStatistics | null>(null);
  const [identities, setIdentities] = useState<IdentityMaps>({ students: {}, classes: {}, exams: {} });
  const [appealWorkers, setAppealWorkers] = useState<ManagedUser[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [examFilter, setExamFilter] = useState<string>(initialExamId || "all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [keyword, setKeyword] = useState("");
  const [reviewStatus, setReviewStatus] = useState("under_review");
  const [reviewReason, setReviewReason] = useState("");
  const [adjustedScore, setAdjustedScore] = useState<number | null>(null);
  const [assignedTo, setAssignedTo] = useState("");
  const [recommendation, setRecommendation] = useState<SubmitAppealRecommendationPayload["recommendation"]>("accept");
  const [recommendedScore, setRecommendedScore] = useState<number | null>(null);
  const [loadingList, setLoadingList] = useState(true);
  const [loadingMoreAppeals, setLoadingMoreAppeals] = useState(false);
  const [nextAppealCursor, setNextAppealCursor] = useState("");
  const [hasMoreAppeals, setHasMoreAppeals] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [actioning, setActioning] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const listRequestRef = useRef(0);
  const detailRequestRef = useRef(0);
  const auditRequestRef = useRef(0);

  const examOptions = useMemo(
    () => {
      const options = new Map(Object.values(identities.exams).map((exam) => [exam.id, `${exam.name} · ${examSubjectLabel(exam.subject)}`]));
      appeals.forEach((appeal) => {
        if (!options.has(appeal.exam_id)) {
          options.set(appeal.exam_id, `${appeal.exam_name ?? "未命名考试"}${appeal.subject ? ` · ${examSubjectLabel(appeal.subject)}` : ""}`);
        }
      });
      return Array.from(options, ([value, label]) => ({ value, label }));
    },
    [appeals, identities.exams]
  );

  const selectedStudent = selectedAppeal ? identities.students[selectedAppeal.student_id] : undefined;
  const selectedClass = selectedStudent ? identities.classes[selectedStudent.class_id] : undefined;
  const currentScore = finalScore(selectedAppeal, "score");
  const maxScore = finalScore(selectedAppeal, "max_score");
  const isTerminalAppeal = selectedAppeal ? ["accepted", "rejected", "score_adjusted", "closed"].includes(selectedAppeal.status) : false;
  const canAct = canManage && Boolean(selectedAppeal);
  const canReview = canAct && !isTerminalAppeal;
  const canClose = canAct && selectedAppeal?.status !== "closed";
  const canAssign = canAct && !isTerminalAppeal && Boolean(assignedTo);
  const canSubmitRecommendation = canWork
    && Boolean(selectedAppeal)
    && selectedAppeal?.assigned_to === currentUser.id
    && !isTerminalAppeal;

  const workerNames = useMemo(
    () => Object.fromEntries(appealWorkers.map((worker) => [worker.id, worker.display_name])),
    [appealWorkers]
  );

  const workerOptions = useMemo(
    () => appealWorkers.map((worker) => ({
      value: worker.id,
      label: `${worker.display_name} · ${worker.roles.map((role) => workerRoleLabels[role] ?? "其他角色").join("/")}`
    })),
    [appealWorkers]
  );

  const filteredAppeals = useMemo(() => {
    const text = keyword.trim().toLowerCase();
    return appeals.filter((appeal) => {
      const student = identities.students[appeal.student_id];
      const studentName = student?.name ?? appeal.student_id;
      const className = student ? identities.classes[student.class_id]?.name ?? "" : "";
      const examName = identities.exams[appeal.exam_id]?.name ?? appeal.exam_id;
      return (
        !text ||
        studentName.toLowerCase().includes(text) ||
        className.toLowerCase().includes(text) ||
        examName.toLowerCase().includes(text) ||
        appeal.reason.toLowerCase().includes(text) ||
        (appeal.question_no ?? "").toLowerCase().includes(text)
      );
    });
  }, [appeals, identities.classes, identities.exams, identities.students, keyword]);

  const columns = useMemo<TableColumnsType<Appeal>>(
    () => mode === "teacher" ? [
      {
        title: "匿名码",
        dataIndex: "anonymous_code",
        width: 130,
        render: (value: string | undefined) => value || "未生成匿名码"
      },
      { title: "考试", dataIndex: "exam_name", render: (value: string | undefined, record: Appeal) => value || <span className="muted" title={record.exam_id}>未匹配到考试</span> },
      { title: "题号", dataIndex: "question_no", width: 80, render: (value?: string) => value || "整卷" },
      { title: "申诉原因", dataIndex: "reason", ellipsis: true },
      { title: "复核建议", dataIndex: "teacher_recommendation", width: 110, render: (value?: string) => value ? <Tag color="blue" title={value}>{recommendationLabels[value] ?? "其他建议"}</Tag> : <span className="muted">待提交</span> },
      { title: "状态", dataIndex: "status", width: 100, render: (value: string) => <StatusTag tone={statusTone(value)}>{statusLabels[value] ?? "未知状态"}</StatusTag> }
    ] : [
      {
        key: "student",
        title: "学生",
        dataIndex: "student_id",
        render: (value: string) => identities.students[value]?.name ?? <span className="muted" title={value}>{canReadIdentities ? "未匹配到学生" : "权限受限"}</span>
      },
      {
        key: "class",
        title: "班级",
        dataIndex: "student_id",
        width: 110,
        render: (value: string) => {
          const student = identities.students[value];
          return student ? identities.classes[student.class_id]?.name ?? "-" : <span className="muted">权限受限</span>;
        }
      },
      { title: "考试", dataIndex: "exam_id", render: (value: string, record: Appeal) => identities.exams[value]?.name ?? record.exam_name ?? <span className="muted" title={value}>未匹配到考试</span> },
      { title: "题号", dataIndex: "question_no", width: 80, render: (value?: string) => value || "整卷" },
      { title: "申诉原因", dataIndex: "reason", ellipsis: true },
      { title: "状态", dataIndex: "status", width: 100, render: (value: string) => <StatusTag tone={statusTone(value)}>{statusLabels[value] ?? "未知状态"}</StatusTag> },
      { title: "提交时间", dataIndex: "created_at", width: 160, render: (value: string) => formatTime(value) }
    ],
    [canReadIdentities, identities.classes, identities.exams, identities.students, mode]
  );

  const adjustmentColumns = useMemo<TableColumnsType<NonNullable<Appeal["adjustments"]>[number]>>(
    () => [
      { title: "题号", dataIndex: "question_no", width: 90 },
      { title: "原分", dataIndex: "previous_score", width: 80, render: (value: number) => formatScore(value) },
      { title: "调整后", dataIndex: "adjusted_score", width: 90, render: (value: number) => formatScore(value) },
      { title: "差值", dataIndex: "delta", width: 80, render: (value: number) => formatScore(value) },
      { title: "原因", dataIndex: "reason" },
      { title: "时间", dataIndex: "created_at", width: 160, render: (value: string) => formatTime(value) }
    ],
    []
  );

  const loadList = useCallback(async () => {
    const requestId = ++listRequestRef.current;
    setLoadingList(true);
    setError(null);
    try {
      const appealResponse = await listAppeals({
        exam_id: examFilter === "all" ? undefined : examFilter,
        status: statusFilter === "all" ? undefined : statusFilter,
        limit: 50
      });
      const studentIDs = Array.from(new Set(appealResponse.appeals.map((item) => item.student_id).filter(Boolean)));
      const [identityResult, statsResult, workersResult] = await Promise.allSettled([
        loadIdentities(canReadIdentities, canReadExams, studentIDs),
        canManage ? getAppealStatistics(examFilter === "all" ? undefined : examFilter) : Promise.resolve({ statistics: null as unknown as AppealStatistics }),
        canManage ? listManagedUsers({ limit: 200 }) : Promise.resolve({ users: [] as ManagedUser[] })
      ]);
      if (requestId !== listRequestRef.current) return;
      if (identityResult.status === "fulfilled") {
        setIdentities(identityResult.value);
      }
      setAppeals(appealResponse.appeals);
      setNextAppealCursor(appealResponse.next_cursor ?? "");
      setHasMoreAppeals(Boolean(appealResponse.has_more));
      setSelectedAppealId((current) => (appealResponse.appeals.some((item) => item.id === current) ? current : appealResponse.appeals[0]?.id ?? ""));
      if (statsResult.status === "fulfilled") {
        setStatistics(statsResult.value.statistics);
      }
      if (workersResult.status === "fulfilled") {
        setAppealWorkers(workersResult.value.users.filter((worker) => worker.status === "active" && worker.roles.some((role) => appealWorkerRoles.has(role))));
      } else if (canManage) {
        message.warning(`申诉处理人读取失败：${formatError(workersResult.reason)}`);
      }
      if (identityResult.status === "fulfilled" && identityResult.value.error) {
        message.warning(`身份映射读取不完整：${identityResult.value.error}`);
      }
    } catch (currentError) {
      if (requestId !== listRequestRef.current) return;
      setAppeals([]);
      setNextAppealCursor("");
      setHasMoreAppeals(false);
      setSelectedAppealId("");
      setStatistics(null);
      setError(formatError(currentError));
    } finally {
      if (requestId === listRequestRef.current) setLoadingList(false);
    }
  }, [canManage, canReadExams, canReadIdentities, examFilter, message, statusFilter]);

  const loadMoreAppeals = useCallback(async () => {
    if (!hasMoreAppeals || !nextAppealCursor || loadingMoreAppeals) return;
    const requestId = ++listRequestRef.current;
    setLoadingMoreAppeals(true);
    try {
      const result = await listAppeals({
        exam_id: examFilter === "all" ? undefined : examFilter,
        status: statusFilter === "all" ? undefined : statusFilter,
        limit: 50,
        cursor: nextAppealCursor
      });
      if (requestId !== listRequestRef.current) return;
      if (canReadIdentities) {
        const studentIDs = Array.from(new Set(result.appeals.map((item) => item.student_id).filter(Boolean)));
        if (studentIDs.length > 0) {
          const studentResult = await listStudents({ ids: studentIDs, limit: 200 });
          if (requestId !== listRequestRef.current) return;
          setIdentities((current) => ({
            ...current,
            students: { ...current.students, ...Object.fromEntries(studentResult.students.map((item) => [item.id, item])) }
          }));
        }
      }
      setAppeals((current) => {
        const byID = new Map(current.map((item) => [item.id, item]));
        result.appeals.forEach((item) => byID.set(item.id, item));
        return Array.from(byID.values());
      });
      setNextAppealCursor(result.next_cursor ?? "");
      setHasMoreAppeals(Boolean(result.has_more));
    } catch (currentError) {
      if (requestId === listRequestRef.current) message.error(formatError(currentError));
    } finally {
      if (requestId === listRequestRef.current) setLoadingMoreAppeals(false);
    }
  }, [canReadIdentities, examFilter, hasMoreAppeals, loadingMoreAppeals, message, nextAppealCursor, statusFilter]);

  const loadAuditForAppeal = useCallback(
    async (appeal: Appeal) => {
      const requestId = ++auditRequestRef.current;
      if (!canReadAudit) {
        setAuditLogs([]);
        return;
      }
      const auditRequests = [
        listAuditLogs({ target_type: "appeal", target_id: appeal.id, limit: 10 }),
        ...(appeal.adjustments ?? []).map((item) => listAuditLogs({ target_type: "score_adjustment", target_id: item.id, limit: 5 }))
      ];
      const results = await Promise.allSettled(auditRequests);
      if (requestId !== auditRequestRef.current) return;
      setAuditLogs(
        results
          .flatMap((result) => (result.status === "fulfilled" ? result.value.audit_logs : []))
          .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      );
    },
    [canReadAudit]
  );

  const loadDetail = useCallback(
    async (id: string) => {
      const requestId = ++detailRequestRef.current;
      if (!id) {
        setSelectedAppeal(null);
        setAuditLogs([]);
        setLoadingDetail(false);
        return;
      }
      setLoadingDetail(true);
      setError(null);
      try {
        const result = await getAppeal(id);
        if (requestId !== detailRequestRef.current) return;
        setSelectedAppeal(result.appeal);
        setReviewStatus(result.appeal.status === "submitted" ? "under_review" : "accepted");
        setReviewReason("");
        setAdjustedScore(finalScore(result.appeal, "score") ?? null);
        setAssignedTo(result.appeal.assigned_to ?? "");
        setRecommendation((result.appeal.teacher_recommendation as SubmitAppealRecommendationPayload["recommendation"]) || "accept");
        setRecommendedScore(result.appeal.teacher_recommended_score ?? finalScore(result.appeal, "score") ?? null);
        await loadAuditForAppeal(result.appeal);
      } catch (currentError) {
        if (requestId !== detailRequestRef.current) return;
        setSelectedAppeal(null);
        setAuditLogs([]);
        setError(formatError(currentError));
      } finally {
        if (requestId === detailRequestRef.current) setLoadingDetail(false);
      }
    },
    [loadAuditForAppeal]
  );

  useEffect(() => {
    setExamFilter(initialExamId || "all");
  }, [initialExamId]);

  useEffect(() => {
    void loadList();
  }, [loadList]);

  useEffect(() => {
    void loadDetail(selectedAppealId);
  }, [loadDetail, selectedAppealId]);

  const refresh = async () => {
    await loadList();
    if (selectedAppealId) {
      await loadDetail(selectedAppealId);
    }
  };

  const runReview = async (payload: ReviewAppealPayload) => {
    if (!selectedAppeal) {
      return;
    }
    setActioning("review");
    try {
      const result = await reviewAppeal(selectedAppeal.id, payload);
      setSelectedAppeal(result.appeal);
      message.success("申诉处理已提交");
      await loadList();
      await loadDetail(result.appeal.id);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  };

  const assignSelectedAppeal = async () => {
    if (!selectedAppeal || !assignedTo) {
      message.error("请选择处理教师");
      return;
    }
    setActioning("assign");
    try {
      const result = await assignAppeal(selectedAppeal.id, assignedTo, selectedAppeal.revision);
      setSelectedAppeal(result.appeal);
      message.success(`已分派给 ${workerNames[assignedTo] ?? "处理教师"}`);
      await loadList();
      await loadDetail(result.appeal.id);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  };

  const submitRecommendation = async () => {
    if (!selectedAppeal) {
      message.error("请先选择申诉");
      return;
    }
    if (!reviewReason.trim()) {
      message.error("请填写复核依据");
      return;
    }
    const payload: SubmitAppealRecommendationPayload = {
      recommendation,
      reason: reviewReason.trim(),
      expected_revision: selectedAppeal.revision
    };
    if (recommendation === "adjust_score") {
      if (recommendedScore === null || recommendedScore === undefined) {
        message.error("请填写建议分数");
        return;
      }
      payload.recommended_score = recommendedScore;
    }
    setActioning("recommendation");
    try {
      const result = await submitAppealRecommendation(selectedAppeal.id, payload);
      setSelectedAppeal(result.appeal);
      message.success("复核建议已提交给管理员");
      await loadList();
      await loadDetail(result.appeal.id);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  };

  const submitReview = () => {
    if (!selectedAppeal) {
      message.error("请先选择申诉");
      return;
    }
    if (!reviewReason.trim()) {
      message.error("请填写处理说明");
      return;
    }
    const payload: ReviewAppealPayload = {
      status: reviewStatus,
      reason: reviewReason.trim(),
      assigned_to: selectedAppeal.assigned_to,
      final_grade_id: selectedAppeal.final_grade_id,
      expected_revision: selectedAppeal.revision
    };
    if (reviewStatus === "score_adjusted") {
      if (adjustedScore === null || adjustedScore === undefined) {
        message.error("请填写调整后分数");
        return;
      }
      payload.adjusted_score = adjustedScore;
      modal.confirm({
        title: "确认调整分数",
        content: `当前最终分 ${formatScore(currentScore)}，将调整为 ${formatScore(adjustedScore)}。确认后系统将生成改分记录并留存操作痕迹，可在审计中追溯。`,
        okText: "确认改分",
        cancelText: "取消",
        onOk: () => runReview(payload)
      });
      return;
    }
    void runReview(payload);
  };

  const closeSelectedAppeal = () => {
    if (!selectedAppeal) {
      return;
    }
    if (!reviewReason.trim()) {
      message.error("请填写关闭说明");
      return;
    }
    modal.confirm({
      title: "关闭申诉",
      content: "关闭后申诉将标记为“已关闭”，处理结果仍对学生可见。",
      okText: "确认关闭",
      cancelText: "取消",
      onOk: async () => {
        setActioning("close");
        try {
          const result = await closeAppeal(selectedAppeal.id, reviewReason.trim(), selectedAppeal.revision);
          setSelectedAppeal(result.appeal);
          message.success("申诉已关闭");
          await loadList();
          await loadDetail(result.appeal.id);
        } catch (currentError) {
          message.error(formatError(currentError));
        } finally {
          setActioning(null);
        }
      }
    });
  };

  const renderAudit = () => {
    if (!canReadAudit) {
      return <EmptyState title="无法查看操作记录" description="当前账号没有查看审计日志的权限，如需查看请联系系统管理员。" />;
    }
    if (auditLogs.length === 0) {
      return <EmptyState title="暂无操作记录" description="该申诉还没有处理或改分的留痕记录。" />;
    }
    return (
      <List
        size="small"
        dataSource={auditLogs.slice(0, 10)}
        renderItem={(item) => (
          <List.Item>
            <div className="appeal-audit-row">
              <strong title={item.action}>{auditActionLabels[item.action] ?? "其他操作"}</strong>
              <span>{item.reason || (auditTargetLabels[item.target_type] ?? "操作留痕")}</span>
              <small>{formatTime(item.created_at)}</small>
            </div>
          </List.Item>
        )}
      />
    );
  };

  const renderDetail = () => {
    if (loadingDetail) {
      return <LoadingState label="正在读取申诉详情" />;
    }
    if (!selectedAppeal) {
      return <EmptyState title="请选择申诉" description="从左侧列表选择一条申诉，查看证据与处理记录。" />;
    }
    return (
      <div className="appeal-detail-stack">
        <section className="appeal-panel">
          <div className="panel-head">
            <div>
              <h2 title={selectedAppeal.target_type}>{targetLabels[selectedAppeal.target_type] ?? "申诉对象"}</h2>
              <p title={selectedAppeal.exam_id}>
                {identities.exams[selectedAppeal.exam_id]?.name ?? selectedAppeal.exam_name ?? "未匹配到考试"} · {selectedAppeal.subject ? examSubjectLabel(selectedAppeal.subject) : "未标注学科"} · {selectedAppeal.question_no || "整卷"}
              </p>
            </div>
            <StatusTag tone={statusTone(selectedAppeal.status)}>{statusLabels[selectedAppeal.status] ?? "未知状态"}</StatusTag>
          </div>
          <Descriptions size="small" column={2} className="appeal-descriptions">
            <Descriptions.Item label={mode === "teacher" ? "匿名码" : "学生"}>
              {mode === "teacher" ? selectedAppeal.anonymous_code || "未生成匿名码" : selectedStudent?.name ?? <span className="muted" title={selectedAppeal.student_id}>{canReadIdentities ? "未匹配到学生" : "权限受限"}</span>}
            </Descriptions.Item>
            {mode === "admin" ? <Descriptions.Item label="班级">{selectedClass?.name ?? <span className="muted" title={selectedStudent?.class_id}>{selectedStudent ? "未匹配到班级" : "权限受限"}</span>}</Descriptions.Item> : null}
            <Descriptions.Item label="提交时间">{formatTime(selectedAppeal.created_at)}</Descriptions.Item>
            <Descriptions.Item label="处理教师">{selectedAppeal.assigned_to ? workerNames[selectedAppeal.assigned_to] ?? (selectedAppeal.assigned_to === currentUser.id ? currentUser.name : <span title={selectedAppeal.assigned_to}>已分派</span>) : "尚未分派"}</Descriptions.Item>
            <Descriptions.Item label="申诉原因" span={mode === "teacher" ? 1 : 2}>
              {selectedAppeal.reason}
            </Descriptions.Item>
            <Descriptions.Item label="附件" span={2}>
              {selectedAppeal.attachment ? <AttachmentInfo value={selectedAppeal.attachment} /> : <span className="muted">无附件</span>}
            </Descriptions.Item>
          </Descriptions>
        </section>

        <section className="appeal-panel">
          <div className="panel-head">
            <div>
              <h2>教师复核建议</h2>
              <p>{selectedAppeal.teacher_recommendation_at ? formatTime(selectedAppeal.teacher_recommendation_at) : "等待处理教师提交"}</p>
            </div>
            {selectedAppeal.teacher_recommendation ? <Tag color="blue" title={selectedAppeal.teacher_recommendation}>{recommendationLabels[selectedAppeal.teacher_recommendation] ?? "其他建议"}</Tag> : null}
          </div>
          {selectedAppeal.teacher_recommendation ? (
            <div className="appeal-result-strip teacher-recommendation-result">
              <div>
                <span>复核建议</span>
                <strong title={selectedAppeal.teacher_recommendation}>{recommendationLabels[selectedAppeal.teacher_recommendation] ?? "其他建议"}</strong>
              </div>
              <div>
                <span>建议分数</span>
                <strong>{selectedAppeal.teacher_recommended_score === undefined ? "不涉及" : formatScore(selectedAppeal.teacher_recommended_score)}</strong>
              </div>
              <div>
                <span>复核依据</span>
                <strong>{selectedAppeal.teacher_recommendation_reason || "未填写"}</strong>
              </div>
            </div>
          ) : <EmptyState title="尚未提交复核建议" description="分派后的处理教师会在核对答卷与评分证据后提交建议。" />}
        </section>

        <section className="appeal-panel">
          <div className="panel-head">
            <div>
              <h2>证据快照</h2>
              <p>核对原始作答、历史评分和当前评分标准。</p>
            </div>
          </div>
          <div className="appeal-evidence-grid">
            <EvidenceBlock title="原始答卷" empty="暂无原始答卷文本">
              {selectedAppeal.evidence?.raw_answer ? <p className="appeal-evidence-text">{selectedAppeal.evidence.raw_answer}</p> : null}
            </EvidenceBlock>
            <EvidenceBlock title="识别文本" empty="暂无识别文本">
              {selectedAppeal.evidence?.ocr_text ? <p className="appeal-evidence-text">{selectedAppeal.evidence.ocr_text}</p> : null}
            </EvidenceBlock>
            <EvidenceBlock title="AI 评分" empty="暂无 AI 评分">
              {recordList(selectedAppeal.evidence?.ai_grades).length ? <ScoreEvidence items={recordList(selectedAppeal.evidence?.ai_grades)} ai /> : null}
            </EvidenceBlock>
            <EvidenceBlock title="人工评分" empty="暂无人工评分">
              {recordList(selectedAppeal.evidence?.human_grades).length ? <ScoreEvidence items={recordList(selectedAppeal.evidence?.human_grades)} /> : null}
            </EvidenceBlock>
            <EvidenceBlock title="最终分" empty="暂无最终分">
              {selectedAppeal.evidence?.final_grade ? <FinalGradeEvidence value={selectedAppeal.evidence.final_grade} /> : null}
            </EvidenceBlock>
            <EvidenceBlock title="评分标准" empty="暂无评分标准">
              {selectedAppeal.evidence?.rubric && recordList(selectedAppeal.evidence.rubric.points).length ? <RubricEvidence value={selectedAppeal.evidence.rubric} /> : null}
            </EvidenceBlock>
          </div>
        </section>

        <section className="appeal-panel">
          <div className="panel-head">
            <div>
              <h2>学生可见处理结果</h2>
              <p>学生读取自己的申诉时可见状态、处理说明和时间。</p>
            </div>
          </div>
          <div className="appeal-result-strip">
            <div>
              <span>当前状态</span>
              <strong title={selectedAppeal.status}>{statusLabels[selectedAppeal.status] ?? "未知状态"}</strong>
            </div>
            <div>
              <span>处理说明</span>
              <strong>{selectedAppeal.result_reason || "暂无"}</strong>
            </div>
            <div>
              <span>处理时间</span>
              <strong>{formatTime(selectedAppeal.reviewed_at || selectedAppeal.closed_at)}</strong>
            </div>
          </div>
        </section>

        <section className="appeal-panel">
          <div className="panel-head">
            <div>
              <h2>历史修改记录</h2>
              <p>{selectedAppeal.adjustments?.length ?? 0} 条改分记录</p>
            </div>
          </div>
          <ResponsiveTable
            rowKey="id"
            size="small"
            columns={adjustmentColumns}
            dataSource={selectedAppeal.adjustments ?? []}
            pagination={false}
            locale={{ emptyText: <EmptyState title="暂无改分记录" description="该申诉尚未产生改分记录。" /> }}
          />
        </section>
      </div>
    );
  };

  return (
    <div className={`${appeals.length === 0 && !loadingList ? "appeal-shell empty" : "appeal-shell"}${mode === "teacher" ? " teacher" : ""}`}>
      <section className="appeal-topbar">
        <div>
          <Space align="center" wrap>
            <h1>{mode === "teacher" ? "申诉处理" : "申诉管理"}</h1>
          </Space>
          <p>{mode === "teacher" ? "核对分配给你的申诉证据，向管理员提交独立复核建议。" : "分派学生申诉、复核教师建议并完成最终处理。"}</p>
        </div>
        <Space wrap>
          <Select
            className="appeal-filter-select"
            value={examFilter}
            options={[{ value: "all", label: "全部考试" }, ...examOptions]}
            onChange={setExamFilter}
            disabled={examOptions.length === 0}
          />
          <Select
            className="appeal-filter-select narrow"
            value={statusFilter}
            options={[{ value: "all", label: "全部状态" }, ...Object.entries(statusLabels).map(([value, label]) => ({ value, label }))]}
            onChange={setStatusFilter}
          />
          <Button icon={<RefreshCw size={16} />} onClick={refresh} loading={loadingList || loadingDetail}>
            刷新
          </Button>
        </Space>
      </section>

      {!canRead ? <Alert type="error" showIcon message="无申诉查看权限" description="当前账号没有申诉查看权限，无法读取申诉列表和详情，请联系管理员开通。" /> : null}
      {error ? <ErrorState message={error} onRetry={refresh} /> : null}

      {mode === "admin" ? <section className="appeal-summary-strip">
        <div>
          <span>申诉总数</span>
          <strong>{statistics ? statistics.total : "-"}</strong>
        </div>
        <div>
          <span>待处理</span>
          <strong>{statistics ? (statistics.by_status.submitted ?? 0) + (statistics.by_status.under_review ?? 0) + (statistics.by_status.need_more_info ?? 0) : "-"}</strong>
        </div>
        <div>
          <span>已改分</span>
          <strong>{statistics ? statistics.score_adjusted_count : "-"}</strong>
        </div>
        <div>
          <span>平均处理时长</span>
          <strong>{statistics && Number.isFinite(statistics.average_handle_hours) ? `${formatScore(statistics.average_handle_hours)} 小时` : "-"}</strong>
        </div>
      </section> : <section className="appeal-summary-strip teacher-summary">
        <div>
          <span>分配给我</span>
          <strong>{appeals.length}</strong>
        </div>
        <div>
          <span>待提交建议</span>
          <strong>{appeals.filter((appeal) => !appeal.teacher_recommendation).length}</strong>
        </div>
        <div>
          <span>已提交建议</span>
          <strong>{appeals.filter((appeal) => Boolean(appeal.teacher_recommendation)).length}</strong>
        </div>
      </section>}

      <section className="appeal-workspace">
        <aside className="appeal-list-panel">
          <div className="panel-head">
            <div>
              <h2>{mode === "teacher" ? "我的申诉任务" : "申诉列表"}</h2>
              <p>{filteredAppeals.length} / {appeals.length} 条申诉</p>
            </div>
          </div>
          <Input prefix={<Search size={16} />} placeholder={mode === "teacher" ? "搜索考试、题号或申诉原因" : "搜索学生、班级、考试、题号、原因"} value={keyword} onChange={(event) => setKeyword(event.target.value)} allowClear />
          {loadingList ? (
            <LoadingState label="正在读取申诉列表" />
          ) : (
            <>
              <ResponsiveTable
                rowKey="id"
                size="small"
                columns={columns}
                dataSource={filteredAppeals}
                pagination={{ pageSize: 8 }}
                onRow={(record) => ({ onClick: () => setSelectedAppealId(record.id) })}
                rowClassName={(record) => (record.id === selectedAppealId ? "selected-table-row" : "")}
                locale={{ emptyText: <EmptyState title={mode === "teacher" ? "暂无分配给你的申诉" : "暂无申诉"} description={mode === "teacher" ? "管理员分派新的申诉后会显示在这里。" : "当前筛选条件下没有申诉记录。"} /> }}
              />
              {hasMoreAppeals ? (
                <Button block loading={loadingMoreAppeals} onClick={() => void loadMoreAppeals()}>
                  加载更多申诉
                </Button>
              ) : null}
            </>
          )}
        </aside>

        <main className="appeal-main">
          {renderDetail()}
        </main>

        {mode === "admin" ? <aside className="appeal-action-panel">
          <div className="panel-head">
            <div>
              <h2>处理申诉</h2>
              <p>处理说明将展示给学生，并计入操作留痕。</p>
            </div>
            <Gavel size={20} />
          </div>
          <div className="appeal-action-section">
            <strong>分派复核</strong>
            <Select
              value={assignedTo || undefined}
              options={workerOptions}
              onChange={setAssignedTo}
              placeholder="选择教师、阅卷员或仲裁员"
              showSearch
              optionFilterProp="label"
              disabled={!canAct || isTerminalAppeal}
            />
            <Button icon={<UserRoundCheck size={16} />} disabled={!canAssign} loading={actioning === "assign"} onClick={assignSelectedAppeal}>
              {selectedAppeal?.assigned_to ? "重新分派" : "分派复核"}
            </Button>
          </div>
          <div className="appeal-action-divider" />
          <strong>最终处理</strong>
          <Select value={reviewStatus} options={reviewOptions} onChange={setReviewStatus} disabled={!canReview} />
          {reviewStatus === "score_adjusted" ? (
            <InputNumber
              min={0}
              max={maxScore}
              precision={2}
              value={adjustedScore}
              onChange={(value) => setAdjustedScore(value)}
              placeholder="调整后分数"
              disabled={!canReview}
            />
          ) : null}
          <Input.TextArea rows={4} value={reviewReason} onChange={(event) => setReviewReason(event.target.value)} placeholder="处理说明" disabled={!canAct} />
          <Button type="primary" icon={<CheckCircle2 size={16} />} disabled={!canReview} loading={actioning === "review"} onClick={submitReview}>
            提交处理
          </Button>
          <span className="muted">结论确定后可关闭申诉归档</span>
          <Button danger icon={<XCircle size={16} />} disabled={!canClose} loading={actioning === "close"} onClick={closeSelectedAppeal}>
            关闭申诉
          </Button>
          <Alert
            type="info"
            showIcon
            icon={<FileWarning size={18} />}
            message="改分须知"
            description="调整分数需二次确认，所有改分操作都会自动留痕，可在操作记录中追溯。"
          />
          <div className="appeal-action-divider" />
          <strong>操作记录</strong>
          {renderAudit()}
        </aside> : <aside className="appeal-action-panel teacher-action-panel">
          <div className="panel-head">
            <div>
              <h2>提交复核建议</h2>
              <p>{selectedAppeal ? selectedAppeal.anonymous_code || "未生成匿名码" : "选择一条申诉任务"}</p>
            </div>
            <Send size={20} />
          </div>
          <Select value={recommendation} options={recommendationOptions} onChange={setRecommendation} disabled={!canSubmitRecommendation} />
          {recommendation === "adjust_score" ? (
            <InputNumber
              min={0}
              max={maxScore}
              precision={2}
              value={recommendedScore}
              onChange={setRecommendedScore}
              placeholder="建议分数"
              disabled={!canSubmitRecommendation}
            />
          ) : null}
          <Input.TextArea
            rows={5}
            value={reviewReason}
            onChange={(event) => setReviewReason(event.target.value)}
            placeholder="填写基于答卷、评分标准和历史评分的复核依据"
            disabled={!canSubmitRecommendation}
          />
          <Button type="primary" icon={<Send size={16} />} disabled={!canSubmitRecommendation} loading={actioning === "recommendation"} onClick={submitRecommendation}>
            提交给管理员
          </Button>
          <Alert
            type="info"
            showIcon
            icon={<LockKeyhole size={18} />}
            message="独立复核"
            description="建议不会直接修改成绩或关闭申诉，最终决定由管理员完成。"
          />
        </aside>}
      </section>

      <QuestionAppealWorkspace
        examId={examFilter === "all" ? "" : examFilter}
        canManage={canManage}
        canWork={canWork}
        onChanged={() => void loadList()}
      />
    </div>
  );
}
