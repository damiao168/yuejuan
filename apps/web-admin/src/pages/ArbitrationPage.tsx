import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App, Button, Descriptions, Empty, Input, InputNumber, List, Select, Space, Table, Tabs, Tooltip, type TableColumnsType } from "antd";
import { CheckCircle2, ClipboardCheck, Gavel, RefreshCw, Save, ScrollText, Search, ShieldCheck, UserCheck } from "lucide-react";
import { ApiClientError } from "../api/client";
import { getCurrentUser } from "../api/auth";
import { listAuditLogs, type AuditLog } from "../api/audit";
import { getExam } from "../api/exams";
import { listQuestions, type Question, type RubricPoint } from "../api/papers";
import {
  assignArbitrationTask,
  getArbitrationTask,
  listArbitrationTasks,
  submitArbitration,
  type ArbitrationTask,
  type FinalGrade
} from "../api/review";
import type { SessionUser } from "../auth/session";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import type { StatusTone } from "../types";

type ScopeFilter = "mine" | "all";
type StatusFilter = "active" | "pending" | "assigned" | "submitted";

interface ArbitrationDetailState {
  task: ArbitrationTask;
  question?: Question;
  warnings: string[];
}

interface DecisionDraft {
  finalScore: number | null;
  reason: string;
  studentFeedback: string;
}

const statusOptions: { label: string; value: StatusFilter }[] = [
  { label: "待处理", value: "active" },
  { label: "待分配", value: "pending" },
  { label: "已分配", value: "assigned" },
  { label: "已提交", value: "submitted" }
];

const scopeOptions: { label: string; value: ScopeFilter }[] = [
  { label: "我的任务", value: "mine" },
  { label: "全部任务", value: "all" }
];

const statusLabels: Record<string, string> = {
  pending: "待分配",
  assigned: "已分配",
  submitted: "已提交"
};

const auditActionLabels: Record<string, string> = {
  "arbitration.task_created": "仲裁任务创建",
  "arbitration.task_assigned": "仲裁任务分配",
  "arbitration.submitted": "仲裁结果提交",
  "final_grade.created": "最终分写入",
  "review.double_mark_auto_finalized": "双评自动定分"
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

function formatScore(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return "-";
  }
  return Number(value.toFixed(1)).toString();
}

function taskTone(status: string): StatusTone {
  if (status === "submitted") {
    return "success";
  }
  if (status === "pending") {
    return "warning";
  }
  if (status === "assigned") {
    return "processing";
  }
  return "neutral";
}

function pointLabel(point: RubricPoint) {
  return `${point.description || point.id} (${formatScore(point.score)} 分)`;
}

function createInitialDraft(task?: ArbitrationTask): DecisionDraft {
  return {
    finalScore: task?.final_score ?? null,
    reason: task?.reason ?? "",
    studentFeedback: task?.student_feedback ?? ""
  };
}

function suggestionText(task?: ArbitrationTask) {
  const suggestion = task?.context?.ai_suggestion;
  if (!suggestion || Object.keys(suggestion).length === 0) {
    return "";
  }
  return JSON.stringify(suggestion, null, 2);
}

function activeStatusMatched(task: ArbitrationTask) {
  return task.status === "pending" || task.status === "assigned";
}

async function loadQuestion(task: ArbitrationTask) {
  const result = await listQuestions(task.exam_id);
  return result.questions.find((item) => item.id === task.question_id);
}

export function ArbitrationPage({ canAssign, canWork, canReadAudit, canReadExams, currentUser }: { canAssign: boolean; canWork: boolean; canReadAudit: boolean; canReadExams: boolean; currentUser: SessionUser }) {
  const { message } = App.useApp();
  const hasSession = true;
  const canSubmit = canWork && hasSession;
  const [actorId, setActorId] = useState(currentUser.id);
  const [actorError, setActorError] = useState<string | null>(null);
  const [scope, setScope] = useState<ScopeFilter>("mine");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("active");
  const [keyword, setKeyword] = useState("");
  const [tasks, setTasks] = useState<ArbitrationTask[]>([]);
  const [examNames, setExamNames] = useState<Record<string, string>>({});
  const [selectedTaskId, setSelectedTaskId] = useState("");
  const [loadingTasks, setLoadingTasks] = useState(true);
  const [taskError, setTaskError] = useState<string | null>(null);
  const [detail, setDetail] = useState<ArbitrationDetailState | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [draft, setDraft] = useState<DecisionDraft>(() => createInitialDraft());
  const [finalGrade, setFinalGrade] = useState<FinalGrade | null>(null);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditError, setAuditError] = useState<string | null>(null);
  const [actioning, setActioning] = useState<string | null>(null);

  useEffect(() => {
    if (!hasSession) {
      return;
    }
    let cancelled = false;
    void getCurrentUser()
      .then((result) => {
        if (!cancelled) {
          setActorId(result.user.id);
          setActorError(null);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setActorError(formatError(error));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [hasSession]);

  const filteredTasks = useMemo(() => {
    const text = keyword.trim().toLowerCase();
    return tasks.filter((task) => {
      const statusMatched = statusFilter === "active" ? activeStatusMatched(task) : task.status === statusFilter;
      const examLabel = examNames[task.exam_id] ?? task.exam_id;
      const keywordMatched =
        !text ||
        task.id.toLowerCase().includes(text) ||
        task.anonymous_code.toLowerCase().includes(text) ||
        task.question_no.toLowerCase().includes(text) ||
        task.exam_id.toLowerCase().includes(text) ||
        examLabel.toLowerCase().includes(text);
      return statusMatched && keywordMatched;
    });
  }, [examNames, keyword, statusFilter, tasks]);

  const selectedTask = useMemo(() => tasks.find((task) => task.id === selectedTaskId), [selectedTaskId, tasks]);
  const maxScore = detail?.question?.score ?? detail?.question?.rubric?.max_score ?? 0;
  const rubricPoints = detail?.question?.rubric?.points ?? [];
  const aiText = suggestionText(detail?.task);

  const loadAudits = useCallback(
    async (task: ArbitrationTask, grade?: FinalGrade) => {
      if (!hasSession || !canReadAudit) {
        setAuditLogs([]);
        return;
      }
      setAuditLoading(true);
      setAuditError(null);
      try {
        const targets = [
          listAuditLogs({ target_type: "arbitration_task", target_id: task.id, limit: 20 }),
          grade?.id ? listAuditLogs({ target_type: "final_grade", target_id: grade.id, limit: 20 }) : undefined
        ].filter((item): item is Promise<{ audit_logs: AuditLog[] }> => Boolean(item));
        const results = await Promise.all(targets);
        const merged = results
          .flatMap((result) => result.audit_logs)
          .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
        setAuditLogs(merged);
      } catch (error) {
        setAuditError(formatError(error));
        setAuditLogs([]);
      } finally {
        setAuditLoading(false);
      }
    },
    [canReadAudit, hasSession]
  );

  const loadExamNames = useCallback(async (nextTasks: ArbitrationTask[]) => {
    if (!canReadExams) {
      setExamNames({});
      return;
    }
    const ids = Array.from(new Set(nextTasks.map((task) => task.exam_id).filter(Boolean)));
    if (ids.length === 0) {
      setExamNames({});
      return;
    }
    const results = await Promise.allSettled(ids.map((id) => getExam(id).then((result) => [id, result.exam.name] as const)));
    const next: Record<string, string> = {};
    for (const result of results) {
      if (result.status === "fulfilled") {
        next[result.value[0]] = result.value[1];
      }
    }
    setExamNames(next);
  }, [canReadExams]);

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
      const result = await listArbitrationTasks({
        status: statusFilter === "active" ? undefined : statusFilter,
        assigned_to: scope === "mine" ? actorId : undefined
      });
      setTasks(result.arbitration_tasks);
      setSelectedTaskId((current) => (result.arbitration_tasks.some((task) => task.id === current) ? current : result.arbitration_tasks[0]?.id ?? ""));
      void loadExamNames(result.arbitration_tasks);
    } catch (error) {
      setTaskError(formatError(error));
      setTasks([]);
    } finally {
      setLoadingTasks(false);
    }
  }, [actorId, hasSession, loadExamNames, scope, statusFilter]);

  const loadDetail = useCallback(
    async (taskId: string) => {
      if (!taskId || !hasSession) {
        setDetail(null);
        setFinalGrade(null);
        setAuditLogs([]);
        return;
      }
      setDetailLoading(true);
      setDetailError(null);
      setFinalGrade(null);
      try {
        const result = await getArbitrationTask(taskId);
        const warnings: string[] = [];
        let question: Question | undefined;
        try {
          question = canReadExams ? await loadQuestion(result.arbitration_task) : undefined;
          if (!question) {
            warnings.push("未能通过 exam_id/question_id 找到题目与 Rubric。");
          }
        } catch (error) {
          warnings.push(formatError(error));
        }
        if (!result.arbitration_task.context?.raw_answer) {
          warnings.push("仲裁上下文未返回原始答案。");
        }
        if (!result.arbitration_task.context?.ocr_text) {
          warnings.push("仲裁上下文未返回 OCR 文本。");
        }
        setDetail({ task: result.arbitration_task, question, warnings });
        setDraft(createInitialDraft(result.arbitration_task));
        await loadAudits(result.arbitration_task);
      } catch (error) {
        setDetailError(formatError(error));
        setDetail(null);
      } finally {
        setDetailLoading(false);
      }
    },
    [canReadExams, hasSession, loadAudits]
  );

  useEffect(() => {
    void loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    void loadDetail(selectedTaskId);
  }, [loadDetail, selectedTaskId]);

  const refreshCurrent = async () => {
    await loadTasks();
    if (selectedTaskId) {
      await loadDetail(selectedTaskId);
    }
  };

  const assignToMe = async () => {
    if (!canAssign) {
      message.error("当前账号不能分配仲裁任务");
      return;
    }
    if (!detail?.task) {
      message.error("请先选择仲裁任务");
      return;
    }
    if (!actorId) {
      message.error("无法识别当前后端用户 ID");
      return;
    }
    setActioning("assign");
    try {
      const result = await assignArbitrationTask(detail.task.id, { assigned_to: actorId });
      setDetail((current) => (current ? { ...current, task: result.arbitration_task } : current));
      setDraft(createInitialDraft(result.arbitration_task));
      await loadAudits(result.arbitration_task);
      await loadTasks();
      message.success("仲裁任务已分配");
    } catch (error) {
      message.error(formatError(error));
    } finally {
      setActioning(null);
    }
  };

  const submitDecision = async () => {
    if (!detail?.task) {
      message.error("请先选择仲裁任务");
      return;
    }
    const score = Number(draft.finalScore);
    if (!Number.isFinite(score)) {
      message.error("请输入仲裁最终分");
      return;
    }
    if (!maxScore || score < 0 || score > maxScore) {
      message.error("仲裁最终分必须在 0 到题目满分之间");
      return;
    }
    if (!draft.reason.trim()) {
      message.error("请填写仲裁说明");
      return;
    }
    setActioning("submit");
    try {
      const result = await submitArbitration(detail.task.id, {
        final_score: score,
        reason: draft.reason.trim(),
        student_feedback: draft.studentFeedback.trim()
      });
      setDetail((current) => (current ? { ...current, task: result.arbitration_task } : current));
      setDraft(createInitialDraft(result.arbitration_task));
      setFinalGrade(result.final_grade);
      await loadTasks();
      await loadAudits(result.arbitration_task, result.final_grade);
      message.success("仲裁结果已提交");
    } catch (error) {
      message.error(formatError(error));
    } finally {
      setActioning(null);
    }
  };

  const columns: TableColumnsType<ArbitrationTask> = [
    {
      title: "考试",
      dataIndex: "exam_id",
      width: 220,
      render: (value: string) => (
        <span className="arbitration-exam-cell" title={value}>
          {examNames[value] ?? value}
        </span>
      )
    },
    { title: "题号", dataIndex: "question_no", width: 88 },
    { title: "匿名码", dataIndex: "anonymous_code", width: 150 },
    { title: "A 分", dataIndex: "first_score", width: 86, render: (value: number) => formatScore(value) },
    { title: "B 分", dataIndex: "second_score", width: 86, render: (value: number) => formatScore(value) },
    {
      title: "分差",
      dataIndex: "score_difference",
      width: 92,
      render: (value: number) => <StatusTag tone={value > 0 ? "warning" : "neutral"}>{formatScore(value)}</StatusTag>
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 104,
      render: (value: string) => <StatusTag tone={taskTone(value)}>{statusLabels[value] ?? value}</StatusTag>
    }
  ];

  const renderContextTab = () => {
    if (!detail) {
      return <EmptyState title="暂无详情" description="选择仲裁任务后展示答案上下文。" />;
    }
    return (
      <div className="arbitration-text-grid">
        <div>
          <strong>原始答案</strong>
          <pre>{detail.task.context?.raw_answer || "当前仲裁上下文未返回原始答案。"}</pre>
        </div>
        <div>
          <strong>OCR 文本</strong>
          <pre>{detail.task.context?.ocr_text || "当前仲裁上下文未返回 OCR 文本。"}</pre>
        </div>
      </div>
    );
  };

  const renderAiTab = () => {
    if (!aiText) {
      return <EmptyState title="暂无 AI 建议" description="当前仲裁上下文没有 ai_suggestion 字段。" />;
    }
    return <pre className="arbitration-json-view">{aiText}</pre>;
  };

  const renderRubricTab = () => {
    if (!detail?.question) {
      return <EmptyState title="未读取到 Rubric" description="题目与 Rubric 需要通过真实题目 API 返回。" />;
    }
    if (rubricPoints.length === 0) {
      return <EmptyState title="当前题目没有 Rubric points" description="可按题目满分提交仲裁最终分。" />;
    }
    return (
      <List
        size="small"
        dataSource={rubricPoints}
        locale={{ emptyText: <Empty description="暂无 Rubric" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
        renderItem={(point) => (
          <List.Item>
            <div className="arbitration-rubric-item">
              <span>{pointLabel(point)}</span>
              <StatusTag tone={point.required ? "processing" : "neutral"}>{point.required ? "必选" : "可选"}</StatusTag>
            </div>
          </List.Item>
        )}
      />
    );
  };

  const renderScoreComparison = () => {
    if (!detail) {
      return null;
    }
    const task = detail.task;
    return (
      <div className="arbitration-score-compare">
        <div>
          <span>阅卷员 A</span>
          <strong>{formatScore(task.first_score)}</strong>
          <small>{task.first_reviewer_id}</small>
          <em>批注：当前 API 未返回</em>
        </div>
        <div>
          <span>阅卷员 B</span>
          <strong>{formatScore(task.second_score)}</strong>
          <small>{task.second_reviewer_id}</small>
          <em>批注：当前 API 未返回</em>
        </div>
        <div>
          <span>分差</span>
          <strong>{formatScore(task.score_difference)}</strong>
          <small>{task.difference_reason || "未返回分差原因"}</small>
          <em>{task.allow_same_arbitrator ? "允许同人仲裁" : "禁止前两名阅卷员仲裁"}</em>
        </div>
      </div>
    );
  };

  const renderAudit = () => {
    if (auditLoading) {
      return <LoadingState label="正在读取审计记录" />;
    }
    if (auditError) {
      return <ErrorState message={auditError} onRetry={() => detail?.task && void loadAudits(detail.task, finalGrade ?? undefined)} />;
    }
    if (auditLogs.length === 0) {
      return <EmptyState title="暂无审计记录" description="当前任务尚未返回匹配的 audit_log。" />;
    }
    return (
      <List
        size="small"
        dataSource={auditLogs}
        renderItem={(item) => (
          <List.Item>
            <div className="arbitration-audit-item">
              <strong>{auditActionLabels[item.action] ?? item.action}</strong>
              <span>{item.reason || item.target_type}</span>
              <small>{formatTime(item.created_at)}</small>
            </div>
          </List.Item>
        )}
      />
    );
  };

  return (
    <div className="arbitration-shell">
      <section className="arbitration-topbar">
        <div>
          <Space>
            <h1>双评仲裁</h1>
            <StatusTag tone="success">真实 API</StatusTag>
          </Space>
          <p>处理 arbitration_task，对照双评分差、答案证据和 Rubric 后提交最终分。</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} onClick={() => void refreshCurrent()} loading={loadingTasks || detailLoading}>
            刷新
          </Button>
          <Button icon={<UserCheck size={16} />} disabled={!canAssign || !detail || detail.task.status === "submitted"} loading={actioning === "assign"} onClick={() => void assignToMe()}>
            分配给我
          </Button>
          <Button type="primary" icon={<Save size={16} />} disabled={!canSubmit || !detail || detail.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void submitDecision()}>
            提交仲裁
          </Button>
        </Space>
      </section>

      {!hasSession ? (
        <Alert
          type="warning"
          showIcon
          message="未检测到真实后端访问令牌"
            description="对照两次评分、答题材料和评分细则完成最终裁定。"
        />
      ) : null}

      {actorError ? <Alert type="warning" showIcon message="未能读取真实登录用户" description={`我的任务过滤暂用前端会话用户：${actorError}`} /> : null}

      <section className="arbitration-filterbar">
        <Select className="toolbar-select" value={scope} options={scopeOptions} onChange={setScope} />
        <Select className="toolbar-select" value={statusFilter} options={statusOptions} onChange={setStatusFilter} />
        <Input prefix={<Search size={16} />} placeholder="搜索考试、匿名码、题号、任务 ID" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <span className="muted">{filteredTasks.length} / {tasks.length} 个仲裁任务</span>
      </section>

      <section className="arbitration-queue-panel">
        {loadingTasks ? (
          <LoadingState label="正在读取 arbitration_task" />
        ) : taskError ? (
          <ErrorState message={taskError} onRetry={() => void loadTasks()} />
        ) : (
          <Table
            rowKey="id"
            size="small"
            columns={columns}
            dataSource={filteredTasks}
            pagination={{ pageSize: 6, showSizeChanger: false }}
            scroll={{ x: 860 }}
            rowClassName={(record) => (record.id === selectedTaskId ? "arbitration-row-active" : "")}
            locale={{ emptyText: <Empty description="当前筛选下没有后端返回的仲裁任务" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
            onRow={(record) => ({
              onClick: () => setSelectedTaskId(record.id)
            })}
          />
        )}
      </section>

      <section className="arbitration-workspace">
        {detailLoading ? (
          <main className="arbitration-main-empty">
            <LoadingState label="正在读取仲裁详情" />
          </main>
        ) : detailError ? (
          <main className="arbitration-main-empty">
            <ErrorState message={detailError} onRetry={() => void loadDetail(selectedTaskId)} />
          </main>
        ) : !detail ? (
          <main className="arbitration-main-empty">
            <EmptyState title="请选择仲裁任务" description="选择上方任务后展示双评分差和裁决表单。" />
          </main>
        ) : (
          <main className="arbitration-main">
            {detail.warnings.length > 0 ? <Alert type="warning" showIcon message="仲裁上下文不完整" description={detail.warnings.join("；")} /> : null}

            <section className="arbitration-context-row">
              <Descriptions bordered size="small" column={4}>
                <Descriptions.Item label="考试">{examNames[detail.task.exam_id] ?? detail.task.exam_id}</Descriptions.Item>
                <Descriptions.Item label="题号">{detail.task.question_no}</Descriptions.Item>
                <Descriptions.Item label="匿名码">{detail.task.anonymous_code}</Descriptions.Item>
                <Descriptions.Item label="状态">{statusLabels[detail.task.status] ?? detail.task.status}</Descriptions.Item>
                <Descriptions.Item label="任务 ID">{detail.task.id}</Descriptions.Item>
                <Descriptions.Item label="分配给">{detail.task.assigned_to || "未分配"}</Descriptions.Item>
                <Descriptions.Item label="创建时间">{formatTime(detail.task.created_at)}</Descriptions.Item>
                <Descriptions.Item label="更新时间">{formatTime(detail.task.updated_at)}</Descriptions.Item>
              </Descriptions>
            </section>

            <section className="arbitration-evidence-panel">
              <div className="panel-head">
                <div>
                  <h2>证据与评分标准</h2>
                  <p>满分 {maxScore || "未返回"}，当前分差 {formatScore(detail.task.score_difference)}</p>
                </div>
                <Tooltip title="普通阅卷员没有 arbitration:manage 权限时无法进入该页面">
                  <ShieldCheck size={18} />
                </Tooltip>
              </div>
              <Tabs
                size="small"
                items={[
                  { key: "context", label: "答案/OCR", children: renderContextTab() },
                  { key: "ai", label: "AI 建议", children: renderAiTab() },
                  { key: "rubric", label: "Rubric", children: renderRubricTab() }
                ]}
              />
            </section>

            <section className="arbitration-reviewer-panel">
              <div className="panel-head">
                <div>
                  <h2>双评分差</h2>
                  <p>{detail.task.difference_reason || "后端未返回分差说明"}</p>
                </div>
                <Gavel size={18} />
              </div>
              {renderScoreComparison()}
            </section>
          </main>
        )}

        <aside className="arbitration-decision-panel">
          <div className="panel-head">
            <div>
              <h2>仲裁裁决</h2>
              <p>{selectedTask ? `${selectedTask.anonymous_code} · ${selectedTask.question_no}` : "未选择任务"}</p>
            </div>
            <ClipboardCheck size={18} />
          </div>

          <div className="score-input-row">
            <InputNumber
              min={0}
              max={maxScore || undefined}
              precision={1}
              value={draft.finalScore}
              placeholder="最终分"
              disabled={!detail || detail.task.status === "submitted"}
              onChange={(value) => setDraft((current) => ({ ...current, finalScore: value === null ? null : Number(value) }))}
            />
            <span>/ {maxScore || "-"}</span>
          </div>

          <Input.TextArea
            rows={4}
            placeholder="仲裁说明"
            value={draft.reason}
            disabled={!detail || detail.task.status === "submitted"}
            onChange={(event) => setDraft((current) => ({ ...current, reason: event.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="学生可见反馈"
            value={draft.studentFeedback}
            disabled={!detail || detail.task.status === "submitted"}
            onChange={(event) => setDraft((current) => ({ ...current, studentFeedback: event.target.value }))}
          />

          <Space wrap>
            <Button icon={<UserCheck size={16} />} disabled={!canAssign || !detail || detail.task.status === "submitted"} loading={actioning === "assign"} onClick={() => void assignToMe()}>
              分配给我
            </Button>
            <Button type="primary" icon={<CheckCircle2 size={16} />} disabled={!canSubmit || !detail || detail.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void submitDecision()}>
              提交
            </Button>
          </Space>

          {finalGrade ? (
            <div className="final-grade-result">
              <StatusTag tone="success">final_grade</StatusTag>
              <strong>
                {formatScore(finalGrade.score)} / {formatScore(finalGrade.max_score)}
              </strong>
              <span>{finalGrade.source} · {finalGrade.locked ? "已锁定" : "未锁定"}</span>
            </div>
          ) : detail?.task.final_score !== undefined ? (
            <div className="final-grade-result">
              <StatusTag tone="success">已提交</StatusTag>
              <strong>{formatScore(detail.task.final_score)}</strong>
              <span>{detail.task.reason || "仲裁说明未返回"}</span>
            </div>
          ) : null}

          <section className="arbitration-audit-panel">
            <div className="panel-head compact">
              <div>
                <h2>审计记录</h2>
                <p>读取真实 audit_log</p>
              </div>
              <ScrollText size={18} />
            </div>
            {renderAudit()}
          </section>
        </aside>
      </section>
    </div>
  );
}
