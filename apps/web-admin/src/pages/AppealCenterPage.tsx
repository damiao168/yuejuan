import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App, Button, Descriptions, Input, InputNumber, List, Select, Space, Table, Tabs, Tag, type TableColumnsType } from "antd";
import { CheckCircle2, FileWarning, Gavel, LockKeyhole, RefreshCw, Search, ShieldCheck, XCircle } from "lucide-react";
import { ApiClientError } from "../api/client";
import { listAuditLogs, type AuditLog } from "../api/audit";
import { listExams, type Exam } from "../api/exams";
import { listClasses, listStudents, type SchoolClass, type Student } from "../api/org";
import {
  closeAppeal,
  getAppeal,
  getAppealStatistics,
  listAppeals,
  reviewAppeal,
  type Appeal,
  type AppealStatistics,
  type ReviewAppealPayload
} from "../api/appeals";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import type { SessionUser } from "../auth/session";
import type { StatusTone } from "../types";

interface AppealCenterPageProps {
  canRead: boolean;
  canManage: boolean;
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
  need_more_info: "需补充",
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
  { value: "under_review", label: "标记处理中" },
  { value: "need_more_info", label: "要求补充" },
  { value: "accepted", label: "接受申诉" },
  { value: "rejected", label: "驳回申诉" },
  { value: "score_adjusted", label: "调整分数" }
];

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

function formatScore(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return "-";
  }
  return Number(value.toFixed(2)).toString();
}

function statusTone(status: string): StatusTone {
  if (status === "accepted" || status === "score_adjusted" || status === "closed") {
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

function formatJSON(value?: unknown) {
  if (value === undefined || value === null) {
    return "";
  }
  return JSON.stringify(value, null, 2);
}

function mapNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function finalScore(appeal: Appeal | null, key: string) {
  return appeal?.evidence?.final_grade ? mapNumber(appeal.evidence.final_grade[key]) : undefined;
}

async function loadIdentities(canReadIdentities: boolean, canReadExams: boolean): Promise<IdentityMaps> {
  const output: IdentityMaps = { students: {}, classes: {}, exams: {} };
  const [studentsResult, classesResult, examsResult] = await Promise.allSettled([
    canReadIdentities ? listStudents() : Promise.resolve({ students: [] }),
    canReadIdentities ? listClasses() : Promise.resolve({ classes: [] }),
    canReadExams ? listExams() : Promise.resolve({ exams: [] })
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

function JsonBlock({ title, value, empty }: { title: string; value?: unknown; empty: string }) {
  const text = typeof value === "string" ? value : formatJSON(value);
  return (
    <div className="appeal-evidence-block">
      <strong>{title}</strong>
      {text ? <pre>{text}</pre> : <span>{empty}</span>}
    </div>
  );
}

export function AppealCenterPage({ canRead, canManage, canReadAudit, canReadIdentities, canReadExams, currentUser, initialExamId = "" }: AppealCenterPageProps) {
  const { message, modal } = App.useApp();
  const hasSession = true;
  const [appeals, setAppeals] = useState<Appeal[]>([]);
  const [selectedAppealId, setSelectedAppealId] = useState("");
  const [selectedAppeal, setSelectedAppeal] = useState<Appeal | null>(null);
  const [statistics, setStatistics] = useState<AppealStatistics | null>(null);
  const [identities, setIdentities] = useState<IdentityMaps>({ students: {}, classes: {}, exams: {} });
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [examFilter, setExamFilter] = useState<string>(initialExamId || "all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [keyword, setKeyword] = useState("");
  const [reviewStatus, setReviewStatus] = useState("under_review");
  const [reviewReason, setReviewReason] = useState("");
  const [adjustedScore, setAdjustedScore] = useState<number | null>(null);
  const [loadingList, setLoadingList] = useState(true);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [actioning, setActioning] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const examOptions = useMemo(
    () => Object.values(identities.exams).map((exam) => ({ value: exam.id, label: `${exam.name} · ${exam.subject}` })),
    [identities.exams]
  );

  const selectedStudent = selectedAppeal ? identities.students[selectedAppeal.student_id] : undefined;
  const selectedClass = selectedStudent ? identities.classes[selectedStudent.class_id] : undefined;
  const currentScore = finalScore(selectedAppeal, "score");
  const maxScore = finalScore(selectedAppeal, "max_score");
  const isTerminalAppeal = selectedAppeal ? ["accepted", "rejected", "score_adjusted", "closed"].includes(selectedAppeal.status) : false;
  const canAct = canManage && hasSession && Boolean(selectedAppeal);
  const canReview = canAct && !isTerminalAppeal;
  const canClose = canAct && selectedAppeal?.status !== "closed";

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
    () => [
      {
        title: "学生",
        dataIndex: "student_id",
        render: (value: string) => identities.students[value]?.name ?? <span className="muted">{canReadIdentities ? value : "权限受限"}</span>
      },
      {
        title: "班级",
        dataIndex: "student_id",
        width: 110,
        render: (value: string) => {
          const student = identities.students[value];
          return student ? identities.classes[student.class_id]?.name ?? "-" : <span className="muted">权限受限</span>;
        }
      },
      { title: "考试", dataIndex: "exam_id", render: (value: string) => identities.exams[value]?.name ?? <span className="muted">{value}</span> },
      { title: "题号", dataIndex: "question_no", width: 80, render: (value?: string) => value || "整卷" },
      { title: "申诉原因", dataIndex: "reason", ellipsis: true },
      { title: "状态", dataIndex: "status", width: 100, render: (value: string) => <StatusTag tone={statusTone(value)}>{statusLabels[value] ?? value}</StatusTag> },
      { title: "提交时间", dataIndex: "created_at", width: 160, render: (value: string) => formatTime(value) }
    ],
    [canReadIdentities, identities.classes, identities.exams, identities.students]
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
    setLoadingList(true);
    setError(null);
    if (!hasSession) {
      setAppeals([]);
      setSelectedAppealId("");
      setStatistics(null);
      setError("当前没有有效登录会话，无法调用真实后端 API。");
      setLoadingList(false);
      return;
    }
    try {
      const [identityResult, appealResult, statsResult] = await Promise.allSettled([
        loadIdentities(canReadIdentities, canReadExams),
        listAppeals({
          exam_id: examFilter === "all" ? undefined : examFilter,
          status: statusFilter === "all" ? undefined : statusFilter
        }),
        canManage ? getAppealStatistics(examFilter === "all" ? undefined : examFilter) : Promise.resolve({ statistics: null as unknown as AppealStatistics })
      ]);
      if (identityResult.status === "fulfilled") {
        setIdentities(identityResult.value);
      }
      if (appealResult.status === "fulfilled") {
        setAppeals(appealResult.value.appeals);
        setSelectedAppealId((current) => (appealResult.value.appeals.some((item) => item.id === current) ? current : appealResult.value.appeals[0]?.id ?? ""));
      } else {
        throw appealResult.reason;
      }
      if (statsResult.status === "fulfilled") {
        setStatistics(statsResult.value.statistics);
      }
      if (identityResult.status === "fulfilled" && identityResult.value.error) {
        message.warning(`身份映射读取不完整：${identityResult.value.error}`);
      }
    } catch (currentError) {
      setAppeals([]);
      setSelectedAppealId("");
      setStatistics(null);
      setError(formatError(currentError));
    } finally {
      setLoadingList(false);
    }
  }, [canManage, canReadExams, canReadIdentities, examFilter, hasSession, message, statusFilter]);

  const loadAuditForAppeal = useCallback(
    async (appeal: Appeal) => {
      if (!canReadAudit) {
        setAuditLogs([]);
        return;
      }
      const auditRequests = [
        listAuditLogs({ target_type: "appeal", target_id: appeal.id, limit: 10 }),
        ...(appeal.adjustments ?? []).map((item) => listAuditLogs({ target_type: "score_adjustment", target_id: item.id, limit: 5 }))
      ];
      const results = await Promise.allSettled(auditRequests);
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
      if (!id || !hasSession) {
        setSelectedAppeal(null);
        setAuditLogs([]);
        return;
      }
      setLoadingDetail(true);
      setError(null);
      try {
        const result = await getAppeal(id);
        setSelectedAppeal(result.appeal);
        setReviewStatus(result.appeal.status === "submitted" ? "under_review" : "accepted");
        setReviewReason("");
        setAdjustedScore(finalScore(result.appeal, "score") ?? null);
        await loadAuditForAppeal(result.appeal);
      } catch (currentError) {
        setSelectedAppeal(null);
        setAuditLogs([]);
        setError(formatError(currentError));
      } finally {
        setLoadingDetail(false);
      }
    },
    [hasSession, loadAuditForAppeal]
  );

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
      assigned_to: currentUser.id,
      final_grade_id: selectedAppeal.final_grade_id
    };
    if (reviewStatus === "score_adjusted") {
      if (adjustedScore === null || adjustedScore === undefined) {
        message.error("请填写调整后分数");
        return;
      }
      payload.adjusted_score = adjustedScore;
      modal.confirm({
        title: "确认调整分数",
        content: `当前最终分 ${formatScore(currentScore)}，将调整为 ${formatScore(adjustedScore)}。确认后后端会写 score_adjustment 和 audit_log。`,
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
      content: "关闭后该申诉进入 closed 状态，结果仍对学生可见。",
      okText: "确认关闭",
      cancelText: "取消",
      onOk: async () => {
        setActioning("close");
        try {
          const result = await closeAppeal(selectedAppeal.id, reviewReason.trim());
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
      return <EmptyState title="无审计读取权限" description="申诉处理和改分仍由后端写审计；当前用户不能读取审计日志。" />;
    }
    if (auditLogs.length === 0) {
      return <EmptyState title="暂无审计记录" description="当前申诉尚未返回匹配的 appeal 或 score_adjustment 审计。" />;
    }
    return (
      <List
        size="small"
        dataSource={auditLogs.slice(0, 10)}
        renderItem={(item) => (
          <List.Item>
            <div className="appeal-audit-row">
              <strong>{item.action}</strong>
              <span>{item.reason || item.target_type}</span>
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
      return <EmptyState title="请选择申诉" description="从左侧列表选择一条真实申诉后查看证据与处理记录。" />;
    }
    return (
      <div className="appeal-detail-stack">
        <section className="appeal-panel">
          <div className="panel-head">
            <div>
              <h2>{targetLabels[selectedAppeal.target_type] ?? selectedAppeal.target_type}</h2>
              <p>
                {identities.exams[selectedAppeal.exam_id]?.name ?? selectedAppeal.exam_id} · {selectedAppeal.question_no || "整卷"}
              </p>
            </div>
            <StatusTag tone={statusTone(selectedAppeal.status)}>{statusLabels[selectedAppeal.status] ?? selectedAppeal.status}</StatusTag>
          </div>
          <Descriptions size="small" column={2} className="appeal-descriptions">
            <Descriptions.Item label="学生">{selectedStudent?.name ?? (canReadIdentities ? selectedAppeal.student_id : "权限受限")}</Descriptions.Item>
            <Descriptions.Item label="班级">{selectedClass?.name ?? (selectedStudent ? selectedStudent.class_id : "权限受限")}</Descriptions.Item>
            <Descriptions.Item label="提交时间">{formatTime(selectedAppeal.created_at)}</Descriptions.Item>
            <Descriptions.Item label="创建人">{selectedAppeal.created_by}</Descriptions.Item>
            <Descriptions.Item label="申诉原因" span={2}>
              {selectedAppeal.reason}
            </Descriptions.Item>
            <Descriptions.Item label="附件" span={2}>
              {selectedAppeal.attachment ? <pre className="appeal-inline-json">{formatJSON(selectedAppeal.attachment)}</pre> : <span className="muted">无附件元数据</span>}
            </Descriptions.Item>
          </Descriptions>
        </section>

        <section className="appeal-panel">
          <div className="panel-head">
            <div>
              <h2>证据快照</h2>
              <p>来自申诉 API 的真实 evidence 字段。</p>
            </div>
          </div>
          <div className="appeal-evidence-grid">
            <JsonBlock title="原始答卷" value={selectedAppeal.evidence?.raw_answer} empty="当前 evidence 未返回原始答卷文本。" />
            <JsonBlock title="OCR" value={selectedAppeal.evidence?.ocr_text} empty="当前 evidence 未返回 OCR 文本。" />
            <JsonBlock title="AI 评分" value={selectedAppeal.evidence?.ai_grades} empty="当前 evidence 未返回 AI 评分。" />
            <JsonBlock title="人工评分" value={selectedAppeal.evidence?.human_grades} empty="当前 evidence 未返回人工评分。" />
            <JsonBlock title="最终分" value={selectedAppeal.evidence?.final_grade} empty="当前 evidence 未返回最终分。" />
            <JsonBlock title="Rubric" value={selectedAppeal.evidence?.rubric} empty="当前 evidence 未返回 Rubric。" />
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
              <strong>{statusLabels[selectedAppeal.status] ?? selectedAppeal.status}</strong>
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
              <p>{selectedAppeal.adjustments?.length ?? 0} 条 score_adjustment</p>
            </div>
          </div>
          <Table
            rowKey="id"
            size="small"
            columns={adjustmentColumns}
            dataSource={selectedAppeal.adjustments ?? []}
            pagination={false}
            scroll={{ x: 720 }}
            locale={{ emptyText: <EmptyState title="暂无改分记录" description="当前申诉尚未产生 score_adjustment。" /> }}
          />
        </section>
      </div>
    );
  };

  return (
    <div className={appeals.length === 0 && !loadingList ? "appeal-shell empty" : "appeal-shell"}>
      <section className="appeal-topbar">
        <div>
          <Space align="center" wrap>
            <h1>申诉中心</h1>
          </Space>
          <p>处理已发布成绩的学生申诉，复核证据、调整分数并保留审计链路。</p>
        </div>
        <Space wrap>
          <Select
            className="appeal-filter-select"
            value={examFilter}
            options={[{ value: "all", label: "全部考试" }, ...examOptions]}
            onChange={setExamFilter}
            disabled={!canReadExams || !hasSession}
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

      {!hasSession ? (
        <Alert
          type="warning"
          showIcon
          message="未检测到真实后端访问令牌"
            description="集中查看申诉材料、处理进度和成绩调整记录。"
        />
      ) : null}
      {!canRead ? <Alert type="error" showIcon message="无申诉读取权限" description="当前账号缺少 appeal:read，不能读取申诉列表和详情。" /> : null}
      {error ? <ErrorState message={error} onRetry={refresh} /> : null}

      <section className="appeal-summary-strip">
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
          <span>平均处理小时</span>
          <strong>{statistics ? formatScore(statistics.average_handle_hours) : "-"}</strong>
        </div>
      </section>

      <section className="appeal-workspace">
        <aside className="appeal-list-panel">
          <div className="panel-head">
            <div>
              <h2>申诉列表</h2>
              <p>{filteredAppeals.length} / {appeals.length} 条申诉</p>
            </div>
          </div>
          <Input prefix={<Search size={16} />} placeholder="搜索学生、班级、考试、题号、原因" value={keyword} onChange={(event) => setKeyword(event.target.value)} allowClear />
          {loadingList ? (
            <LoadingState label="正在读取申诉列表" />
          ) : (
            <Table
              rowKey="id"
              size="small"
              columns={columns}
              dataSource={filteredAppeals}
              pagination={{ pageSize: 8 }}
              scroll={{ x: 980 }}
              onRow={(record) => ({ onClick: () => setSelectedAppealId(record.id) })}
              rowClassName={(record) => (record.id === selectedAppealId ? "selected-table-row" : "")}
              locale={{ emptyText: <EmptyState title="暂无申诉" description="当前筛选条件下没有真实申诉记录。" /> }}
            />
          )}
        </aside>

        <main className="appeal-main">
          {renderDetail()}
        </main>

        <aside className="appeal-action-panel">
          <div className="panel-head">
            <div>
              <h2>处理申诉</h2>
              <p>处理说明会进入学生可见结果和审计链路。</p>
            </div>
            <Gavel size={20} />
          </div>
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
          <Button danger icon={<XCircle size={16} />} disabled={!canClose} loading={actioning === "close"} onClick={closeSelectedAppeal}>
            关闭申诉
          </Button>
          <Alert
            type="info"
            showIcon
            icon={<FileWarning size={18} />}
            message="改分审计"
            description="调整分数必须二次确认；后端会创建 score_adjustment，并写 appeal.reviewed 与 appeal.score_adjusted 审计。"
          />
          <Tabs
            size="small"
            items={[
              { key: "audit", label: "审计", children: renderAudit() },
              {
                key: "scope",
                label: "权限",
                children: (
                  <div className="appeal-scope-note">
                    <LockKeyhole size={16} />
                    <span>学生只能读取自己的申诉由后端 data_scope 强制；本页按 appeal:manage 控制处理动作。</span>
                  </div>
                )
              },
              {
                key: "watermark",
                label: "留痕",
                children: (
                  <div className="appeal-scope-note">
                    <ShieldCheck size={16} />
                    <span>前端不直接写分数或审计，只提交真实 review/close API。</span>
                  </div>
                )
              }
            ]}
          />
        </aside>
      </section>
    </div>
  );
}
