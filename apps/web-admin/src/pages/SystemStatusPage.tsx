import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Descriptions, Space, Table, Tag, type TableColumnsType } from "antd";
import { Activity, Database, RefreshCw, ScanText, ShieldCheck, Signal, TimerReset } from "lucide-react";
import { ApiClientError } from "../api/client";
import { getSystemStatus, type DependencyStatus, type SystemStatus } from "../api/system";
import { ErrorState, LoadingState } from "../components/PageState";
import { ResponsiveTable } from "../components/ResponsiveTable";

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    return `${error.status} ${error.code}: ${error.message}`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "未知错误";
}

function formatTime(value: string) {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString("zh-CN", { hour12: false });
}

function formatUptime(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "-";
  }
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${days}天 ${hours}小时 ${minutes}分`;
}

function dependencyTone(status: DependencyStatus["status"]) {
  if (status === "ok") {
    return "success";
  }
  if (status === "not_configured") {
    return "warning";
  }
  return "error";
}

function dependencyText(status: DependencyStatus["status"]) {
  if (status === "ok") {
    return "正常";
  }
  if (status === "not_configured") {
    return "未配置/待接入";
  }
  return "异常";
}

export function SystemStatusPage() {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setStatus(await getSystemStatus());
    } catch (nextError) {
      setError(formatError(nextError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const dependencySummary = useMemo(() => {
    const items = status?.dependencies ?? [];
    return {
      total: items.length,
      ok: items.filter((item) => item.status === "ok").length,
      warning: items.filter((item) => item.status === "not_configured").length,
      error: items.filter((item) => item.status === "error").length
    };
  }, [status]);
  const ocrWorker = useMemo(
    () => status?.worker_services?.find((item) => item.name === "ocr_worker"),
    [status?.worker_services]
  );

  const columns: TableColumnsType<DependencyStatus> = [
    {
      title: "服务",
      dataIndex: "name",
      render: (value: string) => (
        <Space>
          <Database size={16} />
          <strong>{value}</strong>
        </Space>
      )
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 150,
      render: (value: DependencyStatus["status"]) => <Tag color={dependencyTone(value)}>{dependencyText(value)}</Tag>
    },
    { title: "检查耗时", dataIndex: "duration_ms", width: 120, render: (value: number) => `${value} ms` },
    { title: "最近检查", dataIndex: "checked_at", width: 190, render: (value: string) => formatTime(value) },
    {
      title: "说明",
      dataIndex: "detail",
      render: (_value, record) => record.error ?? record.detail ?? "-"
    }
  ];

  return (
    <div className="system-status-shell">
      <section className="system-status-topbar">
        <div>
          <h1>系统状态</h1>
          <p>查看 API Gateway 运行状态、核心依赖连通性和日志边界。</p>
        </div>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load()}>
          刷新
        </Button>
      </section>

      {loading && !status ? <LoadingState label="读取系统状态" /> : null}
      {error ? <ErrorState message={error} onRetry={() => void load()} /> : null}

      {status ? (
        <>
          <section className="system-status-summary">
            <div>
              <span>整体状态</span>
              <strong>{status.status === "healthy" ? "健康" : "降级"}</strong>
              <Tag color={status.status === "healthy" ? "success" : "warning"}>{status.status}</Tag>
            </div>
            <div>
              <span>服务</span>
              <strong>{status.service}</strong>
              <small>{status.environment}</small>
            </div>
            <div>
              <span>版本</span>
              <strong>{status.version}</strong>
              <small>启动 {formatTime(status.started_at)}</small>
            </div>
            <div>
              <span>运行时长</span>
              <strong>{formatUptime(status.uptime_sec)}</strong>
              <small>生成 {formatTime(status.generated_at)}</small>
            </div>
          </section>

          {ocrWorker ? (
            <section className={`system-worker-status ${ocrWorker.availability}`}>
              <div className="system-worker-heading">
                <div>
                  <Space>
                    <ScanText size={18} />
                    <h2>OCR 自动处理</h2>
                  </Space>
                  <p>{ocrWorker.worker_service} · {ocrWorker.queue_name} 队列 · 失联阈值 {ocrWorker.stale_after_sec} 秒</p>
                </div>
                <Tag color={ocrWorker.automation_available ? "success" : ocrWorker.availability === "stale" ? "warning" : "error"}>
                  {ocrWorker.automation_available ? "在线" : ocrWorker.availability === "stale" ? "心跳失联" : "不可用"}
                </Tag>
              </div>
              <div className="system-worker-metrics">
                <span>在线实例<strong>{ocrWorker.fresh_instances}</strong></span>
                <span>失联实例<strong>{ocrWorker.stale_instances}</strong></span>
                <span>待处理<strong>{ocrWorker.queued_tasks}</strong></span>
                <span>处理中<strong>{ocrWorker.in_flight_tasks}</strong></span>
                <span>近一小时失败<strong>{ocrWorker.failed_last_hour}</strong></span>
                <span>死信<strong>{ocrWorker.dead_letter_tasks}</strong></span>
              </div>
              <Alert
                type={ocrWorker.automation_available ? "success" : ocrWorker.availability === "stale" ? "warning" : "error"}
                showIcon
                message={ocrWorker.impact}
                description={`处理建议：${ocrWorker.action}${ocrWorker.last_seen_at ? ` 最近心跳：${formatTime(ocrWorker.last_seen_at)}` : ""}`}
                action={!ocrWorker.automation_available ? <Button size="small" href="#/capture">进入答卷处理</Button> : undefined}
              />
            </section>
          ) : (
            <Alert type="warning" showIcon message="未返回 OCR Worker 状态" description="请确认 API Gateway 与 Worker Runtime 已升级并连通。" />
          )}

          <section className="system-status-grid">
            <div className="system-status-panel">
              <div className="section-head">
                <div>
                  <Space>
                    <Signal size={18} />
                    <h2>依赖状态</h2>
                  </Space>
                  <p>
                    共 {dependencySummary.total} 项，正常 {dependencySummary.ok}，未配置 {dependencySummary.warning}，异常 {dependencySummary.error}
                  </p>
                </div>
              </div>
              <ResponsiveTable rowKey="name" columns={columns} dataSource={status.dependencies} pagination={false} />
            </div>

            <div className="system-status-panel">
              <div className="section-head">
                <div>
                  <Space>
                    <Activity size={18} />
                    <h2>日志与追踪</h2>
                  </Space>
                  <p>系统日志用于排障，审计日志用于业务追溯，两者分离。</p>
                </div>
              </div>
              <Descriptions bordered size="small" column={1}>
                <Descriptions.Item label="日志格式">{status.observability.log_format}</Descriptions.Item>
                <Descriptions.Item label="系统日志流">{status.observability.system_log_stream}</Descriptions.Item>
                <Descriptions.Item label="审计日志流">{status.observability.audit_log_stream}</Descriptions.Item>
                <Descriptions.Item label="请求 ID">{status.observability.request_id_header}</Descriptions.Item>
                <Descriptions.Item label="Trace ID">{status.observability.trace_id_header}</Descriptions.Item>
                <Descriptions.Item label="慢请求阈值">{status.observability.slow_request_threshold_ms} ms</Descriptions.Item>
                <Descriptions.Item label="慢查询日志">{status.observability.slow_query_log}</Descriptions.Item>
              </Descriptions>
            </div>
          </section>

          <Alert
            type="info"
            showIcon
            icon={<ShieldCheck size={18} />}
            message="日志隐私边界"
            description={status.observability.sensitive_log_policy}
          />
          <Alert
            type="warning"
            showIcon
            icon={<TimerReset size={18} />}
            message="慢查询日志预留"
          description="当前可识别响应较慢的请求，数据库语句级耗时分析尚未接入。"
          />
        </>
      ) : null}
    </div>
  );
}
