import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert, App, Button, Divider, Empty, Input, List, Modal, Select, Space, type TableColumnsType } from "antd";
import { Calculator, CheckCircle2, ClipboardCheck, Download, FileWarning, LockKeyhole, RefreshCw, Search, Send, ShieldCheck, UserCheck, UserX } from "lucide-react";
import { ApiClientError } from "../api/client";
import { listAuditLogs, type AuditLog } from "../api/audit";
import { examStatusLabels, examSubjectLabel } from "../constants/examStatus";
import { listExams, type Exam } from "../api/exams";
import { listClasses, listStudents, type SchoolClass, type Student } from "../api/org";
import {
  checkExamGradeQuality,
  confirmExamGrades,
  exportExamGrades,
  finalizeExamGrades,
  listExamGrades,
  listExamRoster,
  publishExamGrades,
  setExamAttendance,
  type QualityCheckResult,
  type QualityIssue,
  type RosterEntry,
  type RosterReport,
  type SubmissionGrade
} from "../api/scores";
import { listSubmissions, type Submission } from "../api/submissions";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { ResponsiveTable } from "../components/ResponsiveTable";
import { StatusTag } from "../components/StatusTag";
import type { StatusTone } from "../types";
import { hashQueryParam } from "../router/query";
import type { ProductExperience } from "../router/experience";

interface IdentityMaps {
  students: Record<string, Student>;
  classes: Record<string, SchoolClass>;
  error?: string;
}

interface ScoreSummary {
  expectedStudents: number;
  receivedSubmissions: number;
  completedGrades: number;
  absentStudents: number;
  unresolvedRoster: number;
  unfinishedReviews: number;
  pendingArbitrations: number;
  ocrFailures: number;
  canPublish: boolean;
}

const statusLabels: Record<string, string> = {
  calculating: "计算中",
  pending_confirmation: "待确认",
  confirmed: "已确认",
  pending_publish: "待发布",
  published: "已发布",
  locked: "已锁定"
};

const sourceLabels: Record<string, string> = {
  single_review: "人工单评",
  rule_auto: "规则自动",
  arbitration: "仲裁",
  average: "双评平均",
  first: "第一评",
  second: "第二评",
  higher: "高分优先",
  lower: "低分优先"
};

const qualityLabels: Record<string, string> = {
  unfinished_review_tasks: "还有阅卷任务未完成",
  unfinished_arbitration_tasks: "还有仲裁任务未完成",
  ocr_failed_unhandled: "识别失败（未处理）",
  missing_final_grades: "部分题目还没有最终得分",
  grades_not_confirmed: "成绩未确认",
  no_submission_grades: "无成绩可发布",
  missing_submission_unresolved: "应考学生尚未匹配答卷",
  unidentified_submission: "答卷身份未确认或存在重复",
  missing_pages_unresolved: "答卷缺页或页面质量异常"
};

const auditActionLabels: Record<string, string> = {
  "score.finalized": "成绩汇总",
  "score.confirmed": "成绩确认",
  "score.published": "成绩发布",
  "score.exported": "成绩导出",
  "score.roster_attendance_updated": "名册出勤状态调整"
};

const rosterStatusLabels: Record<string, string> = {
  graded: "已评分",
  absent: "已标记缺考",
  unmatched: "未匹配待处理",
  missing_pages: "缺页待处理"
};

const rosterResolutionLabels: Record<string, string> = {
  graded: "成绩已汇总",
  absent: "已人工确认缺考",
  missing_submission: "未收到答卷（请核查缺考或漏扫）",
  grading_incomplete: "答卷已匹配，评分尚未完成",
  duplicate_submission: "同一学生关联多份答卷",
  student_unidentified: "答卷尚未关联学生",
  student_not_in_roster: "答卷学生不在本场应考名册",
  absent_has_submission: "已标记缺考但存在答卷",
  missing_pages: "实收页数少于应收页数或质量检查失败"
};

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    console.error("请求失败", error.status, error.code, error.message);
    return error.message || "操作失败，请稍后重试";
  }
  if (error instanceof Error) {
    return error.message || "操作失败，请稍后重试";
  }
  return "操作失败，请稍后重试";
}

function formatScore(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return "-";
  }
  return Number(value.toFixed(1)).toString();
}

function formatTime(value?: string) {
  if (!value) {
    return "-";
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString("zh-CN", { hour12: false });
}

function statusTone(status: string): StatusTone {
  if (status === "published" || status === "locked") {
    return "success";
  }
  if (status === "confirmed" || status === "pending_publish") {
    return "processing";
  }
  if (status === "pending_confirmation") {
    return "warning";
  }
  return "neutral";
}

function issueCount(quality: QualityCheckResult | null, code: string) {
  return quality?.quality.issues.find((issue) => issue.code === code)?.count ?? 0;
}

function createSummary(submissions: Submission[], completedGradeCount: number, quality: QualityCheckResult | null, roster: RosterReport | null): ScoreSummary {
  return {
    expectedStudents: roster?.summary.expected ?? submissions.length,
    receivedSubmissions: roster?.summary.received ?? submissions.length,
    completedGrades: completedGradeCount,
    absentStudents: roster?.summary.absent ?? 0,
    unresolvedRoster: roster?.summary.unresolved ?? 0,
    unfinishedReviews: issueCount(quality, "unfinished_review_tasks"),
    pendingArbitrations: issueCount(quality, "unfinished_arbitration_tasks"),
    ocrFailures: issueCount(quality, "ocr_failed_unhandled"),
    canPublish: Boolean(quality?.can_publish)
  };
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

async function loadIdentities(canReadStudentNames: boolean, studentIds: string[]): Promise<IdentityMaps> {
  const ids = [...new Set(studentIds.filter(Boolean))];
  if (!canReadStudentNames || ids.length === 0) {
    return { students: {}, classes: {} };
  }
  const [studentsResult, classesResult] = await Promise.allSettled([listStudents({ ids, limit: 200 }), listClasses()]);
  const maps: IdentityMaps = { students: {}, classes: {} };
  if (studentsResult.status === "fulfilled") {
    for (const student of studentsResult.value.students) {
      maps.students[student.id] = student;
    }
  } else {
    maps.error = formatError(studentsResult.reason);
  }
  if (classesResult.status === "fulfilled") {
    for (const item of classesResult.value.classes) {
      maps.classes[item.id] = item;
    }
  } else {
    maps.error = [maps.error, formatError(classesResult.reason)].filter(Boolean).join("；");
  }
  return maps;
}

export function ScoreManagementPage({
  mode,
  canManage,
  canReadStudentNames,
  canReadAudit,
  initialExamId = ""
}: {
  mode: ProductExperience;
  canManage: boolean;
  canReadStudentNames: boolean;
  canReadAudit: boolean;
  initialExamId?: string;
}) {
  const { message, modal } = App.useApp();
  const canWrite = canManage;
  const [exams, setExams] = useState<Exam[]>([]);
  const [selectedExamId, setSelectedExamId] = useState(initialExamId);
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [grades, setGrades] = useState<SubmissionGrade[]>([]);
  const [gradeTotal, setGradeTotal] = useState(0);
  const [filteredGradeTotal, setFilteredGradeTotal] = useState(0);
  const [allGradesLocked, setAllGradesLocked] = useState(false);
  const [gradeNextCursor, setGradeNextCursor] = useState("");
  const [gradesHaveMore, setGradesHaveMore] = useState(false);
  const [quality, setQuality] = useState<QualityCheckResult | null>(null);
  const [roster, setRoster] = useState<RosterReport | null>(null);
  const [identities, setIdentities] = useState<IdentityMaps>({ students: {}, classes: {} });
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [lastWatermark, setLastWatermark] = useState("");
  const [keyword, setKeyword] = useState("");
  const [appliedKeyword, setAppliedKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [confirmReason, setConfirmReason] = useState("");
  const [publishReason, setPublishReason] = useState("");
  const [loadingExams, setLoadingExams] = useState(true);
  const [loadingScores, setLoadingScores] = useState(false);
  const [loadingMoreGrades, setLoadingMoreGrades] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actioning, setActioning] = useState<string | null>(null);
  const [attendanceEditor, setAttendanceEditor] = useState<{ entry: RosterEntry; status: "expected" | "absent" } | null>(null);
  const [attendanceReason, setAttendanceReason] = useState("");
  const scoreRequestRef = useRef(0);

  const selectedExam = useMemo(() => exams.find((exam) => exam.id === selectedExamId), [exams, selectedExamId]);
  const summary = useMemo(() => createSummary(submissions, gradeTotal, quality, roster), [gradeTotal, quality, roster, submissions]);
  const scoreAuditLogs = useMemo(() => auditLogs.filter((item) => item.action.startsWith("score.")), [auditLogs]);
  const publishedOrLocked = selectedExam?.status === "published" || allGradesLocked;
  const filteredGrades = grades;

  const loadExamList = useCallback(async () => {
    setLoadingExams(true);
    setError(null);
    try {
      const result = await listExams();
      setExams(result.exams);
        setSelectedExamId((current) => {
          if (initialExamId && result.exams.some((exam) => exam.id === initialExamId)) return initialExamId;
          const requestedStatus = hashQueryParam("status");
          const requestedExam = requestedStatus ? result.exams.find((exam) => exam.status === requestedStatus) : undefined;
          if (requestedExam) return requestedExam.id;
          return result.exams.some((exam) => exam.id === current) ? current : result.exams[0]?.id || "";
        });
    } catch (currentError) {
      setError(formatError(currentError));
    } finally {
      setLoadingExams(false);
    }
  }, [initialExamId]);

  const loadScores = useCallback(
    async (examId: string) => {
      const requestId = ++scoreRequestRef.current;
      if (!examId) {
        setSubmissions([]);
        setGrades([]);
        setGradeTotal(0);
        setFilteredGradeTotal(0);
        setAllGradesLocked(false);
        setGradeNextCursor("");
        setGradesHaveMore(false);
        setQuality(null);
        setRoster(null);
        setAuditLogs([]);
        setLoadingScores(false);
        return;
      }
      setLoadingScores(true);
      setError(null);
      try {
        const [submissionResult, gradeResult, qualityResult, rosterResult, auditResult] = await Promise.allSettled([
          listSubmissions(examId, { limit: 50 }),
          listExamGrades(examId, {
            status: statusFilter === "all" ? undefined : statusFilter,
            q: appliedKeyword || undefined,
            limit: 50
          }),
          checkExamGradeQuality(examId, "publish"),
          canManage ? listExamRoster(examId) : Promise.resolve({ roster: null }),
          canReadAudit ? listAuditLogs({ target_type: "exam", target_id: examId, limit: 20 }) : Promise.resolve({ audit_logs: [] })
        ]);
        if (requestId !== scoreRequestRef.current) return;
        if (submissionResult.status === "fulfilled") {
          setSubmissions(submissionResult.value.submissions);
        } else {
          throw submissionResult.reason;
        }
        if (gradeResult.status === "fulfilled") {
          setGrades(gradeResult.value.grades);
          setGradeTotal(gradeResult.value.total);
          setFilteredGradeTotal(gradeResult.value.filtered_total);
          setAllGradesLocked(gradeResult.value.all_locked);
          setGradeNextCursor(gradeResult.value.next_cursor);
          setGradesHaveMore(gradeResult.value.has_more);
          const identityResult = await loadIdentities(
            canReadStudentNames,
            gradeResult.value.grades.flatMap((grade) => (grade.student_id ? [grade.student_id] : []))
          );
          if (requestId !== scoreRequestRef.current) return;
          setIdentities(identityResult);
        } else {
          throw gradeResult.reason;
        }
        if (qualityResult.status === "fulfilled") {
          setQuality(qualityResult.value);
        } else {
          throw qualityResult.reason;
        }
        if (rosterResult.status === "fulfilled") {
          setRoster(rosterResult.value.roster);
        } else {
          throw rosterResult.reason;
        }
        if (auditResult.status === "fulfilled") {
          setAuditLogs(auditResult.value.audit_logs);
        }
      } catch (currentError) {
        if (requestId !== scoreRequestRef.current) return;
        setError(formatError(currentError));
      } finally {
        if (requestId === scoreRequestRef.current) setLoadingScores(false);
      }
    },
    [appliedKeyword, canManage, canReadAudit, canReadStudentNames, statusFilter]
  );

  useEffect(() => {
    void loadExamList();
  }, [loadExamList]);

  useEffect(() => {
    if (initialExamId) setSelectedExamId(initialExamId);
  }, [initialExamId]);

  useEffect(() => {
    void loadScores(selectedExamId);
  }, [loadScores, selectedExamId]);

  const refresh = async () => {
    await loadExamList();
    if (selectedExamId) {
      await loadScores(selectedExamId);
    }
  };

  const loadMoreGrades = async () => {
    if (!selectedExamId || !gradesHaveMore || !gradeNextCursor || loadingMoreGrades) return;
    const requestId = scoreRequestRef.current;
    setLoadingMoreGrades(true);
    try {
      const result = await listExamGrades(selectedExamId, {
        status: statusFilter === "all" ? undefined : statusFilter,
        q: appliedKeyword || undefined,
        limit: 50,
        cursor: gradeNextCursor
      });
      const identityResult = await loadIdentities(
        canReadStudentNames,
        result.grades.flatMap((grade) => (grade.student_id ? [grade.student_id] : []))
      );
      if (requestId !== scoreRequestRef.current) return;
      setGrades((current) => {
        const byId = new Map(current.map((grade) => [grade.id, grade]));
        for (const grade of result.grades) byId.set(grade.id, grade);
        return [...byId.values()];
      });
      setIdentities((current) => ({
        students: { ...current.students, ...identityResult.students },
        classes: { ...current.classes, ...identityResult.classes },
        error: [current.error, identityResult.error].filter(Boolean).join("；") || undefined
      }));
      setGradeTotal(result.total);
      setFilteredGradeTotal(result.filtered_total);
      setAllGradesLocked(result.all_locked);
      setGradeNextCursor(result.next_cursor);
      setGradesHaveMore(result.has_more);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setLoadingMoreGrades(false);
    }
  };

  const runAction = async (key: string, action: () => Promise<void>, successText: string) => {
    setActioning(key);
    try {
      await action();
      message.success(successText);
      if (selectedExamId) {
        await loadScores(selectedExamId);
      }
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(null);
    }
  };

  const finalize = () =>
    runAction(
      "finalize",
      async () => {
        if (!selectedExamId) {
          throw new Error("请先选择考试");
        }
        await finalizeExamGrades(selectedExamId);
      },
      "最终成绩已汇总"
    );

  const confirmGrades = () =>
    runAction(
      "confirm",
      async () => {
        if (!selectedExamId) {
          throw new Error("请先选择考试");
        }
        if (!confirmReason.trim()) {
          throw new Error("请填写确认原因");
        }
        await confirmExamGrades(selectedExamId, confirmReason.trim());
      },
      "成绩已确认"
    );

  const publish = () => {
    if (!selectedExamId) {
      message.error("请先选择考试");
      return;
    }
    if (!quality?.can_publish) {
      message.error("发布前质量检查未通过");
      return;
    }
    if (!publishReason.trim()) {
      message.error("请填写发布原因");
      return;
    }
    modal.confirm({
      title: "确认发布成绩",
      content: `将发布《${selectedExam?.name ?? "未选择考试"}》共 ${gradeTotal} 份成绩，发布后学生成绩即被锁定，如需修改须走成绩申诉流程。确定发布吗？`,
      okText: "发布",
      cancelText: "取消",
      onOk: () =>
        runAction(
          "publish",
          async () => {
            await publishExamGrades(selectedExamId, publishReason.trim());
            await loadExamList();
          },
          "成绩已发布"
        )
    });
  };

  const exportGrades = () => {
    if (!selectedExamId) {
      message.error("请先选择考试");
      return;
    }
    modal.confirm({
      title: "导出成绩",
      content: "将导出该考试的全部成绩（CSV 表格文件）。导出文件带追溯水印，导出操作会被系统记录。确定导出吗？",
      okText: "导出",
      cancelText: "取消",
      onOk: () =>
        runAction(
          "export",
          async () => {
            const result = await exportExamGrades(selectedExamId);
            setLastWatermark(result.watermark ?? "");
            saveBlob(result.blob, result.filename ?? `exam-${selectedExamId}-grades.csv`);
          },
          "成绩已导出"
        )
    });
  };

  const saveAttendance = async () => {
    if (!selectedExamId || !attendanceEditor?.entry.student_id || !attendanceReason.trim()) {
      message.error("请填写本次名册调整原因");
      return;
    }
    await runAction(
      "attendance",
      async () => {
        const result = await setExamAttendance(selectedExamId, attendanceEditor.entry.student_id!, attendanceEditor.status, attendanceReason.trim());
        setRoster(result.roster);
        setAttendanceEditor(null);
        setAttendanceReason("");
      },
      attendanceEditor.status === "absent" ? "已标记缺考" : "已恢复为应考"
    );
  };

  const rosterColumns: TableColumnsType<RosterEntry> = [
    {
      title: "学生",
      key: "student",
      width: 180,
      render: (_value, record) => record.student_name ? (
        <div className="score-roster-student"><strong>{record.student_name}</strong><span>{record.student_no || "无学号"}</span></div>
      ) : <span className="muted">身份待确认</span>
    },
    { title: "班级", dataIndex: "class_name", width: 130, render: (value?: string) => value || "-" },
    { title: "答卷号", dataIndex: "candidate_no", width: 150, render: (value?: string) => value || "-" },
    {
      title: "对账状态",
      dataIndex: "status",
      width: 150,
      render: (value: string) => <StatusTag tone={value === "graded" ? "success" : value === "absent" ? "neutral" : "danger"}>{rosterStatusLabels[value] ?? value}</StatusTag>
    },
    {
      title: "核对说明",
      dataIndex: "resolution_code",
      render: (value: string, record) => (
        <div className="score-roster-resolution">
          <span>{rosterResolutionLabels[value] ?? value}</span>
          {record.expected_page_count > 0 ? <small>{`页数 ${record.actual_page_count}/${record.expected_page_count}`}</small> : null}
          {record.attendance_reason ? <small>{`处置原因：${record.attendance_reason}`}</small> : null}
        </div>
      )
    },
    {
      title: "操作",
      key: "actions",
      width: 130,
      render: (_value, record) => {
        if (!canWrite || publishedOrLocked || !record.student_id || record.key.startsWith("submission:")) return "-";
        if (record.status === "absent") {
          return <Button size="small" icon={<UserCheck size={14} />} onClick={() => { setAttendanceEditor({ entry: record, status: "expected" }); setAttendanceReason(""); }}>恢复应考</Button>;
        }
        if (record.resolution_code === "missing_submission") {
          return <Button size="small" danger icon={<UserX size={14} />} onClick={() => { setAttendanceEditor({ entry: record, status: "absent" }); setAttendanceReason(""); }}>标记缺考</Button>;
        }
        return <span className="muted">请先处理答卷</span>;
      }
    }
  ];

  const columns: TableColumnsType<SubmissionGrade> = [
    { title: "匿名码", dataIndex: "anonymous_code", width: 150 },
    {
      title: "学生姓名",
      dataIndex: "student_id",
      width: 140,
      render: (value?: string) => {
        if (!canReadStudentNames) {
          return <span className="muted">权限受限</span>;
        }
        return value ? identities.students[value]?.name ?? "未匹配" : "未关联";
      }
    },
    {
      title: "班级",
      dataIndex: "student_id",
      width: 140,
      render: (value?: string) => {
        if (!canReadStudentNames) {
          return <span className="muted">权限受限</span>;
        }
        const student = value ? identities.students[value] : undefined;
        return student ? identities.classes[student.class_id]?.name ?? "未匹配" : "未关联";
      }
    },
    {
      title: "总分",
      dataIndex: "total_score",
      width: 120,
      render: (_value: number, record) => (
        <strong>
          {formatScore(record.total_score)} / {formatScore(record.max_score)}
        </strong>
      )
    },
    {
      title: "各题分",
      dataIndex: "items",
      render: (_value: unknown, record) => (
        <div className="score-item-strip">
          {(record.items ?? []).length > 0 ? (
            (record.items ?? []).map((item) => (
              <span key={item.id} title={`${sourceLabels[item.source] ?? item.source} · ${statusLabels[item.status] ?? item.status}`}>
                {item.question_no}: {formatScore(item.score)}/{formatScore(item.max_score)}
              </span>
            ))
          ) : (
            <span>无题目分</span>
          )}
        </div>
      )
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 120,
      render: (value: string) => (
        <span title={value}>
          <StatusTag tone={statusTone(value)}>{statusLabels[value] ?? "未知状态"}</StatusTag>
        </span>
      )
    },
    {
      title: "锁定",
      dataIndex: "locked",
      width: 88,
      render: (value: boolean) => <StatusTag tone={value ? "success" : "neutral"}>{value ? "已锁定" : "未锁定"}</StatusTag>
    }
  ];

  const statusOptions = useMemo(() => {
    const statuses = Array.from(new Set(grades.map((grade) => grade.status)));
    return [{ label: "全部状态", value: "all" }, ...statuses.map((status) => ({ label: statusLabels[status] ?? "未知状态", value: status }))];
  }, [grades]);

  const renderQuality = () => {
    if (!quality) {
      return <EmptyState title="暂无质量检查" description="选择考试后，这里会列出发布前需要处理的问题。" />;
    }
    if (quality.quality.passed) {
      return (
        <div className="score-quality-pass">
          <ShieldCheck size={22} />
          <div>
            <strong>{publishedOrLocked ? "成绩已发布，质量校验通过" : "发布前质量检查通过"}</strong>
            <span>{publishedOrLocked ? "成绩已发布并锁定，所有检查项均已处理。" : "所有检查项均已通过，可以发布成绩。"}</span>
          </div>
        </div>
      );
    }
    return (
      <List
        size="small"
        dataSource={quality.quality.issues}
        locale={{ emptyText: <Empty description="暂无质量问题" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
        renderItem={(issue: QualityIssue) => (
          <List.Item>
            <div className="score-quality-issue" title={issue.message}>
              <StatusTag tone={issue.blocking ? "danger" : "warning"}>{issue.blocking ? "须处理后才能发布" : "提醒"}</StatusTag>
              <strong>{qualityLabels[issue.code] ?? issue.code}</strong>
              <span>{issue.count} 项</span>
            </div>
          </List.Item>
        )}
      />
    );
  };

  const renderAudit = () => {
    if (!canReadAudit) {
      return <EmptyState title="无权查看操作记录" description="您的账号没有查看操作记录的权限。导出、确认、发布等操作仍会被系统记录。" />;
    }
    if (scoreAuditLogs.length === 0) {
      return <EmptyState title="暂无操作记录" description="该考试还没有成绩相关的操作记录。" />;
    }
    return (
      <List
        size="small"
        dataSource={scoreAuditLogs.slice(0, 8)}
        renderItem={(item) => (
          <List.Item>
            <div className="score-audit-row">
              <strong title={item.action}>{auditActionLabels[item.action] ?? "成绩操作"}</strong>
              <span>{item.reason || "未填写原因"}</span>
              <small>{formatTime(item.created_at)}</small>
            </div>
          </List.Item>
        )}
      />
    );
  };

  return (
    <div className={mode === "teacher" ? "score-shell read-only" : "score-shell"}>
      <section className="score-topbar">
        <div>
          <Space>
            <h1>{mode === "teacher" ? "班级成绩" : "成绩发布"}</h1>
          </Space>
          <p>{mode === "teacher" ? "查看当前授权考试的班级成绩与阅卷完成情况。" : "先处理发布前检查发现的问题，检查无误后确认并发布成绩。"}</p>
        </div>
        <Space wrap>
          <Select
            className="score-exam-select"
            loading={loadingExams}
            value={selectedExamId || undefined}
            placeholder="选择考试"
            options={exams.map((exam) => ({ label: `${exam.name} · ${examSubjectLabel(exam.subject)}`, value: exam.id }))}
            onChange={setSelectedExamId}
          />
          <Button icon={<RefreshCw size={16} />} onClick={() => void refresh()} loading={loadingExams || loadingScores}>
            刷新
          </Button>
        </Space>
      </section>

      {identities.error ? <Alert type="warning" showIcon message="学生身份信息读取不完整" description={identities.error} /> : null}

      {error ? <ErrorState message={error} onRetry={() => void refresh()} /> : null}

      <section className="score-summary-strip">
        <div>
          <span>应考人数</span>
          <strong>{summary.expectedStudents}</strong>
        </div>
        <div>
          <span>实收答卷</span>
          <strong>{summary.receivedSubmissions}</strong>
        </div>
        <div>
          <span>已生成成绩</span>
          <strong>{summary.completedGrades}</strong>
        </div>
        <div>
          <span>已确认缺考</span>
          <strong>{summary.absentStudents}</strong>
        </div>
        <div>
          <span>名册未解决</span>
          <strong>{summary.unresolvedRoster}</strong>
        </div>
        <div>
          <span>流程待办</span>
          <strong>{summary.unfinishedReviews + summary.pendingArbitrations + summary.ocrFailures}</strong>
        </div>
        <div>
          <span>{mode === "teacher" ? "当前状态" : publishedOrLocked ? "发布状态" : "是否可发布"}</span>
          <StatusTag tone={mode === "teacher" ? "neutral" : publishedOrLocked || summary.canPublish ? "success" : "danger"}>{mode === "teacher" ? (selectedExam ? examStatusLabels[selectedExam.status] ?? "未知状态" : "未选择") : publishedOrLocked ? "已发布" : summary.canPublish ? "可发布" : "不可发布"}</StatusTag>
        </div>
      </section>

      <section className={mode === "teacher" ? "score-workspace read-only" : "score-workspace"}>
        <main className="score-main">
          <section className="score-table-panel score-roster-panel">
            <div className="panel-head">
              <div>
                <h2>名册对账</h2>
                <p>以本场考试关联班级为应考名单；未交卷、身份冲突和缺页未解决时不能发布成绩。</p>
              </div>
              <StatusTag tone={(roster?.summary.unresolved ?? 0) === 0 ? "success" : "danger"}>
                {(roster?.summary.unresolved ?? 0) === 0 ? "对账完成" : `${roster?.summary.unresolved ?? 0} 项待处理`}
              </StatusTag>
            </div>
            {loadingScores ? <LoadingState label="正在核对考试名册" /> : (
              <ResponsiveTable
                rowKey="key"
                size="small"
                columns={rosterColumns}
                dataSource={roster?.entries ?? []}
                pagination={{ pageSize: 8, showSizeChanger: false }}
                locale={{ emptyText: <Empty description="本场考试尚未关联有效班级名册。" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
              />
            )}
          </section>

          <section className="score-quality-panel">
            <div className="panel-head">
              <div>
                <h2>发布前质量检查</h2>
                <p>{selectedExam ? selectedExam.name : "未选择考试"}</p>
              </div>
              {quality ? (
                quality.quality.passed ? (
                  <StatusTag tone="success">检查通过</StatusTag>
                ) : (
                  <StatusTag tone="danger">{`${quality.quality.issues.filter((issue) => issue.blocking).length} 项须处理`}</StatusTag>
                )
              ) : null}
            </div>
            {loadingScores ? <LoadingState label="正在读取质量检查" /> : renderQuality()}
          </section>

          <section className="score-table-panel">
            <div className="panel-head">
              <div>
                <h2>成绩列表</h2>
                <p>
                  已加载 {filteredGrades.length} / {filteredGradeTotal} 条
                  {filteredGradeTotal !== gradeTotal ? `（全场 ${gradeTotal} 条）` : ""}
                </p>
              </div>
              <Space wrap>
                <Input.Search
                  prefix={<Search size={16} />}
                  placeholder="搜索匿名码、学生、班级"
                  value={keyword}
                  allowClear
                  enterButton="搜索"
                  onChange={(event) => {
                    setKeyword(event.target.value);
                    if (!event.target.value) setAppliedKeyword("");
                  }}
                  onSearch={(value) => setAppliedKeyword(value.trim())}
                />
                <Select className="toolbar-select" value={statusFilter} options={statusOptions} onChange={setStatusFilter} />
              </Space>
            </div>
            {loadingScores ? (
              <LoadingState label="正在读取成绩" />
            ) : (
              <ResponsiveTable
                rowKey="id"
                size="small"
                columns={columns}
                dataSource={filteredGrades}
                pagination={{ pageSize: 8, showSizeChanger: false }}
                locale={{ emptyText: <Empty description={mode === "teacher" ? "该考试暂无成绩，请等待阅卷完成。" : "该考试暂无成绩。请先完成阅卷，再点击『汇总最终成绩』生成成绩。"} image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
              />
            )}
            {!loadingScores && gradesHaveMore ? (
              <div className="load-more-row">
                <Button loading={loadingMoreGrades} onClick={() => void loadMoreGrades()}>加载更多成绩</Button>
              </div>
            ) : null}
          </section>
        </main>

        {mode === "admin" ? <aside className="score-actions-panel">
          <div className="panel-head">
            <div>
              <h2>发布步骤</h2>
              <p>{publishedOrLocked ? "成绩已锁定" : "按顺序完成汇总、确认、发布"}</p>
            </div>
            {publishedOrLocked ? <LockKeyhole size={18} /> : <Calculator size={18} />}
          </div>

          {publishedOrLocked ? (
            <Alert type="success" showIcon message="成绩已发布并锁定" description="成绩已发布并锁定，不能直接修改。如需调整，请通过成绩申诉流程处理。" />
          ) : null}

          <div className="score-step-group">
            <span className="score-step-title">第 1 步 · 汇总最终成绩</span>
            <Button block icon={<Calculator size={16} />} disabled={!canWrite || !selectedExamId || publishedOrLocked} loading={actioning === "finalize"} onClick={() => void finalize()}>
              汇总最终成绩
            </Button>
          </div>

          <div className="score-step-group">
            <span className="score-step-title">第 2 步 · 确认成绩</span>
            <label className="score-step-label" htmlFor="score-confirm-reason">确认原因（必填）</label>
            <Input.TextArea id="score-confirm-reason" rows={3} value={confirmReason} placeholder="例如：已由学科组长复核，成绩无误" onChange={(event) => setConfirmReason(event.target.value)} />
            <Button block icon={<CheckCircle2 size={16} />} disabled={!canWrite || gradeTotal === 0 || publishedOrLocked} loading={actioning === "confirm"} onClick={() => void confirmGrades()}>
              确认成绩
            </Button>
          </div>

          <div className="score-step-group">
            <span className="score-step-title">第 3 步 · 发布成绩</span>
            <label className="score-step-label" htmlFor="score-publish-reason">发布原因（必填）</label>
            <Input.TextArea id="score-publish-reason" rows={3} value={publishReason} placeholder="例如：经教务处审批，同意发布" onChange={(event) => setPublishReason(event.target.value)} />
            <Button block type="primary" icon={<Send size={16} />} disabled={!canWrite || !quality?.can_publish || publishedOrLocked} loading={actioning === "publish"} onClick={publish}>
              发布成绩
            </Button>
          </div>

          <Divider className="score-step-divider" />

          <div className="score-step-group">
            <span className="score-step-title">导出</span>
            <Button block icon={<Download size={16} />} disabled={!canWrite || gradeTotal === 0} loading={actioning === "export"} onClick={exportGrades}>
              导出成绩
            </Button>
          </div>

          <details className="score-advanced-details">
            <summary>操作记录与导出水印</summary>
          <Alert
            type="info"
            showIcon
            icon={<FileWarning size={18} />}
            message="导出记录与水印"
            description={lastWatermark ? `最近一次导出的水印编号：${lastWatermark}` : "每次导出都会记录操作人和时间，导出文件自带可追溯水印。"}
          />

          {renderAudit()}
          </details>
        </aside> : null}
      </section>

      <Modal
        open={Boolean(attendanceEditor)}
        title={attendanceEditor?.status === "absent" ? "确认标记缺考" : "确认恢复应考"}
        okText="确认"
        cancelText="取消"
        confirmLoading={actioning === "attendance"}
        onOk={() => void saveAttendance()}
        onCancel={() => { setAttendanceEditor(null); setAttendanceReason(""); }}
      >
        <Alert
          type={attendanceEditor?.status === "absent" ? "warning" : "info"}
          showIcon
          icon={<ClipboardCheck size={18} />}
          message={attendanceEditor?.entry.student_name ?? "学生"}
          description={attendanceEditor?.status === "absent" ? "缺考学生不计入班级均分；若后续发现答卷，请先恢复应考再完成身份匹配。" : "恢复后该生必须匹配一份完整答卷，才能通过发布检查。"}
        />
        <label className="score-attendance-label" htmlFor="score-attendance-reason">调整原因（必填）</label>
        <Input.TextArea
          id="score-attendance-reason"
          rows={3}
          maxLength={300}
          showCount
          value={attendanceReason}
          placeholder={attendanceEditor?.status === "absent" ? "例如：经监考记录与班主任确认，学生因病缺考" : "例如：已找到并确认该生答卷，恢复为应考"}
          onChange={(event) => setAttendanceReason(event.target.value)}
        />
      </Modal>
    </div>
  );
}
