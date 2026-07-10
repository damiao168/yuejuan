import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App, Button, DatePicker, Descriptions, Drawer, Input, Select, Space, Table, Tag, type TableColumnsType } from "antd";
import { Download, FileWarning, LockKeyhole, RefreshCw, Search, ShieldCheck } from "lucide-react";
import { ApiClientError } from "../api/client";
import { exportAuditLogs, listAuditLogs, type AuditLog, type AuditLogFilter } from "../api/audit";
import { listExams, type Exam } from "../api/exams";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";

interface AuditLogPageProps {
  canRead: boolean;
  canExport: boolean;
  tenantName: string;
}

const sensitiveKeys = ["password", "token", "authorization", "secret", "credential", "access_token", "refresh_token", "password_hash"];

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

function isSensitiveKey(key: string) {
  const normalized = key.toLowerCase();
  return sensitiveKeys.some((item) => normalized.includes(item));
}

function maskSensitive(value: unknown, key = ""): unknown {
  if (isSensitiveKey(key)) {
    return "***";
  }
  if (Array.isArray(value)) {
    return value.map((item) => maskSensitive(item));
  }
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value as Record<string, unknown>).map(([entryKey, entryValue]) => [entryKey, maskSensitive(entryValue, entryKey)]));
  }
  if (typeof value === "string" && /(bearer\s+|password=|token=)/i.test(value)) {
    return value.replace(/(bearer\s+)[^\s]+/i, "$1***").replace(/(password|token)=([^&\s]+)/gi, "$1=***");
  }
  return value;
}

function maskedJSON(value?: Record<string, unknown>) {
  if (!value || Object.keys(value).length === 0) {
    return "未记录";
  }
  return JSON.stringify(maskSensitive(value), null, 2);
}

function maskText(value?: string) {
  if (!value) {
    return "-";
  }
  return String(maskSensitive(value));
}

function actionTone(action: string) {
  if (action.includes("failed") || action.includes("rejected")) {
    return "danger" as const;
  }
  if (action.includes("export") || action.includes("publish") || action.includes("adjusted")) {
    return "warning" as const;
  }
  if (action.includes("created") || action.includes("succeeded") || action.includes("confirmed")) {
    return "success" as const;
  }
  return "processing" as const;
}

function buildTarget(record: AuditLog) {
  return record.target_id ? `${record.target_type}:${record.target_id}` : record.target_type;
}

export function AuditLogPage({ canRead, canExport, tenantName }: AuditLogPageProps) {
  const { message, modal } = App.useApp();
  const hasSession = true;
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [exams, setExams] = useState<Exam[]>([]);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [actionFilter, setActionFilter] = useState("");
  const [actorFilter, setActorFilter] = useState("");
  const [targetTypeFilter, setTargetTypeFilter] = useState("");
  const [targetIdFilter, setTargetIdFilter] = useState("");
  const [examFilter, setExamFilter] = useState("");
  const [ipFilter, setIpFilter] = useState("");
  const [createdFrom, setCreatedFrom] = useState("");
  const [createdTo, setCreatedTo] = useState("");
  const [keyword, setKeyword] = useState("");
  const [lastWatermark, setLastWatermark] = useState("");
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const filter = useMemo<AuditLogFilter>(
    () => ({
      action: actionFilter.trim() || undefined,
      actor_id: actorFilter.trim() || undefined,
      target_type: targetTypeFilter || undefined,
      target_id: targetIdFilter.trim() || undefined,
      exam_id: examFilter || undefined,
      ip_address: ipFilter.trim() || undefined,
      created_from: createdFrom || undefined,
      created_to: createdTo || undefined,
      limit: 200
    }),
    [actionFilter, actorFilter, createdFrom, createdTo, examFilter, ipFilter, targetIdFilter, targetTypeFilter]
  );

  const actionOptions = useMemo(
    () =>
      Array.from(new Set(logs.map((item) => item.action)))
        .sort()
        .map((action) => ({ value: action, label: action })),
    [logs]
  );

  const targetTypeOptions = useMemo(
    () =>
      Array.from(new Set(logs.map((item) => item.target_type)))
        .sort()
        .map((targetType) => ({ value: targetType, label: targetType })),
    [logs]
  );

  const filteredLogs = useMemo(() => {
    const text = keyword.trim().toLowerCase();
    if (!text) {
      return logs;
    }
    return logs.filter((item) => {
      const haystack = [item.actor_id, item.action, item.target_type, item.target_id, item.reason, item.ip_address, item.user_agent, item.request_id].join(" ").toLowerCase();
      return haystack.includes(text);
    });
  }, [keyword, logs]);

  const summary = useMemo(
    () => ({
      total: logs.length,
      actors: new Set(logs.map((item) => item.actor_id).filter(Boolean)).size,
      actions: new Set(logs.map((item) => item.action)).size,
      ips: new Set(logs.map((item) => item.ip_address).filter(Boolean)).size
    }),
    [logs]
  );

  const columns = useMemo<TableColumnsType<AuditLog>>(
    () => [
      { title: "时间", dataIndex: "created_at", width: 170, render: (value: string) => formatTime(value) },
      { title: "Actor", dataIndex: "actor_id", width: 180, render: (value?: string) => value || <span className="muted">system</span> },
      { title: "Action", dataIndex: "action", width: 190, render: (value: string) => <StatusTag tone={actionTone(value)}>{value}</StatusTag> },
      { title: "Target", width: 240, render: (_, record) => buildTarget(record) },
      { title: "Reason", dataIndex: "reason", ellipsis: true, render: (value?: string) => maskText(value) },
      { title: "IP", dataIndex: "ip_address", width: 130, render: (value?: string) => value || "-" }
    ],
    []
  );

  const loadLogs = useCallback(async () => {
    setLoading(true);
    setError(null);
    if (!hasSession) {
      setLogs([]);
      setExams([]);
      setError("当前没有有效登录会话，无法调用真实后端 API。");
      setLoading(false);
      return;
    }
    try {
      const [auditResult, examResult] = await Promise.allSettled([listAuditLogs(filter), listExams()]);
      if (auditResult.status === "fulfilled") {
        setLogs(auditResult.value.audit_logs);
      } else {
        throw auditResult.reason;
      }
      if (examResult.status === "fulfilled") {
        setExams(examResult.value.exams);
      }
    } catch (currentError) {
      setLogs([]);
      setError(formatError(currentError));
    } finally {
      setLoading(false);
    }
  }, [filter, hasSession]);

  useEffect(() => {
    void loadLogs();
  }, [loadLogs]);

  const openDetail = (record: AuditLog) => {
    setSelectedLog(record);
    setDrawerOpen(true);
  };

  const exportLogs = () => {
    modal.confirm({
      title: "导出审计日志",
      content: "导出会调用真实审计导出 API，并写入 audit.exported 审计。导出的 CSV 会包含水印。",
      okText: "确认导出",
      cancelText: "取消",
      onOk: async () => {
        setExporting(true);
        try {
          const result = await exportAuditLogs(filter);
          setLastWatermark(result.watermark ?? "");
          saveBlob(result.blob, result.filename ?? "audit-logs.csv");
          message.success("审计日志 CSV 已导出");
          await loadLogs();
        } catch (currentError) {
          message.error(formatError(currentError));
        } finally {
          setExporting(false);
        }
      }
    });
  };

  return (
    <div className="audit-shell">
      <section className="audit-topbar">
        <div>
          <Space align="center" wrap>
            <h1>审计日志</h1>
            <StatusTag tone="success">真实 API</StatusTag>
          </Space>
          <p>只读查看当前租户内的关键操作记录，筛选、追踪并导出带水印的审计 CSV。</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} onClick={loadLogs} loading={loading}>
            刷新
          </Button>
          <Button type="primary" icon={<Download size={16} />} disabled={!canExport || !hasSession || logs.length === 0} loading={exporting} onClick={exportLogs}>
            导出 CSV
          </Button>
        </Space>
      </section>

      {!hasSession ? (
        <Alert
          type="warning"
          showIcon
          message="未检测到真实后端访问令牌"
            description="按操作人、时间和业务对象追踪关键操作记录。"
        />
      ) : null}
      {!canRead ? <Alert type="error" showIcon message="无审计读取权限" description="当前账号缺少 audit:read，不能查看全局审计日志。" /> : null}
      {error ? <ErrorState message={error} onRetry={loadLogs} /> : null}

      <section className="audit-scope-panel">
        <LockKeyhole size={18} />
        <span>租户隔离：后端按当前用户 tenant_id 查询。当前前端租户范围为 {tenantName}，页面不提供跨租户切换、编辑或删除入口。</span>
      </section>

      <section className="audit-filter-panel">
        <DatePicker.RangePicker
          showTime
          onChange={(dates) => {
            setCreatedFrom(dates?.[0]?.toISOString() ?? "");
            setCreatedTo(dates?.[1]?.toISOString() ?? "");
          }}
        />
        <Input placeholder="操作人 actor_id" value={actorFilter} onChange={(event) => setActorFilter(event.target.value)} allowClear />
        <Select allowClear placeholder="操作类型" value={actionFilter || undefined} options={actionOptions} onChange={(value) => setActionFilter(value ?? "")} />
        <Select allowClear placeholder="目标类型" value={targetTypeFilter || undefined} options={targetTypeOptions} onChange={(value) => setTargetTypeFilter(value ?? "")} />
        <Select
          allowClear
          placeholder="考试"
          value={examFilter || undefined}
          options={exams.map((exam) => ({ value: exam.id, label: `${exam.name} · ${exam.subject}` }))}
          onChange={(value) => setExamFilter(value ?? "")}
        />
        <Input placeholder="目标 ID" value={targetIdFilter} onChange={(event) => setTargetIdFilter(event.target.value)} allowClear />
        <Input placeholder="IP" value={ipFilter} onChange={(event) => setIpFilter(event.target.value)} allowClear />
        <Button icon={<Search size={16} />} onClick={loadLogs} loading={loading}>
          查询
        </Button>
      </section>

      <section className="audit-summary-strip">
        <div>
          <span>返回日志</span>
          <strong>{summary.total}</strong>
        </div>
        <div>
          <span>操作人数</span>
          <strong>{summary.actors}</strong>
        </div>
        <div>
          <span>操作类型</span>
          <strong>{summary.actions}</strong>
        </div>
        <div>
          <span>IP 数</span>
          <strong>{summary.ips}</strong>
        </div>
      </section>

      <section className="audit-table-panel">
        <div className="panel-head">
          <div>
            <h2>审计日志列表</h2>
            <p>{filteredLogs.length} / {logs.length} 条日志，点击行查看详情。</p>
          </div>
          <Input className="audit-keyword" prefix={<Search size={16} />} placeholder="页内搜索 action、target、reason、IP" value={keyword} onChange={(event) => setKeyword(event.target.value)} allowClear />
        </div>
        {loading ? (
          <LoadingState label="正在读取审计日志" />
        ) : (
          <Table
            rowKey="id"
            size="small"
            columns={columns}
            dataSource={filteredLogs}
            pagination={{ pageSize: 12 }}
            scroll={{ x: 1080 }}
            onRow={(record) => ({ onClick: () => openDetail(record) })}
            locale={{ emptyText: <EmptyState title="暂无审计日志" description="当前筛选条件下没有真实审计记录。" /> }}
          />
        )}
      </section>

      <section className="audit-export-panel">
        <ShieldCheck size={18} />
        <span>{lastWatermark ? `最近导出水印：${lastWatermark}` : "导出需要 audit:export；导出行为本身会写 audit.exported 审计。"}</span>
      </section>

      <Drawer title="审计详情" width={520} open={drawerOpen} onClose={() => setDrawerOpen(false)}>
        {selectedLog ? (
          <div className="audit-detail-stack">
            <Descriptions size="small" column={1}>
              <Descriptions.Item label="actor">{selectedLog.actor_id || "system"}</Descriptions.Item>
              <Descriptions.Item label="action">{selectedLog.action}</Descriptions.Item>
              <Descriptions.Item label="target">{buildTarget(selectedLog)}</Descriptions.Item>
              <Descriptions.Item label="reason">{maskText(selectedLog.reason)}</Descriptions.Item>
              <Descriptions.Item label="ip_address">{selectedLog.ip_address || "-"}</Descriptions.Item>
              <Descriptions.Item label="user_agent">{maskText(selectedLog.user_agent)}</Descriptions.Item>
              <Descriptions.Item label="created_at">{formatTime(selectedLog.created_at)}</Descriptions.Item>
              <Descriptions.Item label="request_id">{selectedLog.request_id || "-"}</Descriptions.Item>
              <Descriptions.Item label="tenant_id">{selectedLog.tenant_id}</Descriptions.Item>
            </Descriptions>
            <div className="audit-json-block">
              <strong>before_value</strong>
              <pre>{maskedJSON(selectedLog.before_value)}</pre>
            </div>
            <div className="audit-json-block">
              <strong>after_value</strong>
              <pre>{maskedJSON(selectedLog.after_value)}</pre>
            </div>
            <Alert
              type="info"
              showIcon
              icon={<FileWarning size={18} />}
              message="只读记录"
              description="审计日志不可编辑、不可删除；敏感字段仅在前端显示时脱敏。"
            />
            <Tag color="blue">tenant scoped</Tag>
          </div>
        ) : (
          <EmptyState title="未选择日志" description="请选择一条审计日志查看详情。" />
        )}
      </Drawer>
    </div>
  );
}
