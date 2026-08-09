import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Input, Progress, Space, Tag } from "antd";
import { Play, RefreshCw, Sparkles } from "lucide-react";
import { ApiClientError } from "../api/client";
import { createSubjectiveGradingBatch, enqueueSubjectiveGradingBatch, getSubjectiveGradingBatch, type SubjectiveGradingBatch } from "../api/subjectiveGrading";

function errorMessage(error: unknown) {
  if (error instanceof ApiClientError) return error.message;
  return error instanceof Error ? error.message : "主观题批次操作失败";
}

function newBatchIdempotencyKey() {
  return `admin-${crypto.randomUUID()}`;
}

export function SubjectiveGradingBatchPage() {
  const [idempotencyKey, setIdempotencyKey] = useState(newBatchIdempotencyKey);
  const [segmentText, setSegmentText] = useState("");
  const [batch, setBatch] = useState<SubjectiveGradingBatch>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [enqueueNotice, setEnqueueNotice] = useState<string>();
  const [pollError, setPollError] = useState<string>();

  const progress = useMemo(() => {
    if (!batch || batch.total_count === 0) return 0;
    return Math.round(((batch.succeeded_count + batch.failed_count) / batch.total_count) * 100);
  }, [batch]);

  const batchId = batch?.id;
  const batchStatus = batch?.status;
  useEffect(() => {
    if (!batchId || !batchStatus || ["completed", "failed", "cancelled"].includes(batchStatus)) return;
    let cancelled = false;
    let inFlight = false;
    let timer: number | undefined;
    let controller: AbortController | undefined;

    const schedule = () => {
      if (!cancelled && document.visibilityState !== "hidden") {
        timer = window.setTimeout(() => void poll(), 5000);
      }
    };
    const poll = async () => {
      if (cancelled || document.visibilityState === "hidden" || inFlight) {
        schedule();
        return;
      }
      inFlight = true;
      const requestController = new AbortController();
      controller = requestController;
      try {
        const result = await getSubjectiveGradingBatch(batchId, requestController.signal);
        if (!cancelled) {
          setBatch(result.batch);
          setPollError(undefined);
        }
      } catch (refreshError) {
        if (!cancelled && !(refreshError instanceof DOMException && refreshError.name === "AbortError")) {
          setPollError(`自动刷新失败：${errorMessage(refreshError)}`);
        }
      } finally {
        if (controller === requestController) controller = undefined;
        inFlight = false;
        schedule();
      }
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        controller?.abort();
        return;
      }
      if (timer !== undefined) window.clearTimeout(timer);
      if (!inFlight) void poll();
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    schedule();
    return () => {
      cancelled = true;
      controller?.abort();
      if (timer !== undefined) window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [batchId, batchStatus]);

  async function createBatch() {
    setLoading(true);
    setError(undefined);
    setEnqueueNotice(undefined);
    try {
      const segmentIds = segmentText.split(/[\s,，]+/).map((value) => value.trim()).filter(Boolean);
      const result = await createSubjectiveGradingBatch(idempotencyKey.trim(), segmentIds);
      setBatch(result.batch);
      setIdempotencyKey(newBatchIdempotencyKey());
    } catch (createError) {
      setError(errorMessage(createError));
    } finally {
      setLoading(false);
    }
  }

  async function enqueue() {
    if (!batch) return;
    setLoading(true);
    setError(undefined);
    setEnqueueNotice(undefined);
    try {
      const result = await enqueueSubjectiveGradingBatch(batch.id);
      setBatch(result.batch);
      if (result.enqueue_result.failed_count > 0) {
        const prefix = result.enqueue_result.partial_success ? "批次已部分入队" : "批次暂未入队";
        setEnqueueNotice(`${prefix}：${result.enqueue_result.accepted_count} 个已接受，${result.enqueue_result.failed_count} 个失败，可安全重试。`);
      }
    } catch (enqueueError) {
      setError(errorMessage(enqueueError));
    } finally {
      setLoading(false);
    }
  }

  async function refresh() {
    if (!batch) return;
    setLoading(true);
    try {
      const result = await getSubjectiveGradingBatch(batch.id);
      setBatch(result.batch);
      setError(undefined);
      setPollError(undefined);
    } catch (refreshError) {
      setError(errorMessage(refreshError));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="page-stack">
      <section className="page-heading">
        <div><span className="dashboard-kicker">管理员</span><h1>主观题 AI 批次</h1><p>创建受治理的影子评分批次，AI 结果只进入教师复核，不直接改写最终成绩。</p></div>
        <Sparkles size={28} aria-hidden="true" />
      </section>
      {error ? <Alert type="error" showIcon message={error} /> : null}
      {enqueueNotice ? <Alert type="warning" showIcon message={enqueueNotice} /> : null}
      {pollError ? <Alert type="warning" showIcon message={pollError} action={<Button size="small" onClick={() => void refresh()}>立即重试</Button>} /> : null}
      <Card title="创建批次">
        <Space direction="vertical" size="middle" style={{ width: "100%" }}>
          <Input value={idempotencyKey} onChange={(event) => setIdempotencyKey(event.target.value)} addonBefore="幂等键" />
          <Input.TextArea value={segmentText} onChange={(event) => { setSegmentText(event.target.value); setIdempotencyKey(newBatchIdempotencyKey()); }} placeholder="输入答题片段 ID，使用换行或逗号分隔（最多 1000 个）" autoSize={{ minRows: 4, maxRows: 8 }} />
          <Button type="primary" loading={loading} onClick={() => void createBatch()}>创建批次</Button>
        </Space>
      </Card>
      {batch ? <Card title={<Space>批次进度 <Tag color={batch.status === "failed" ? "red" : batch.status === "completed" ? "green" : "blue"}>{batch.status}</Tag></Space>} extra={<Space><Button icon={<Play size={15} />} disabled={batch.status !== "planned" && batch.status !== "processing"} loading={loading} onClick={() => void enqueue()}>入队评分</Button><Button icon={<RefreshCw size={15} />} loading={loading} onClick={() => void refresh()}>刷新</Button></Space>}>
        <Progress percent={progress} status={batch.failed_count ? "exception" : undefined} />
        <div className="operations-counts"><span><strong>{batch.total_count}</strong>总量</span><span><strong>{batch.queued_count}</strong>排队</span><span><strong>{batch.processing_count}</strong>处理中</span><span><strong>{batch.succeeded_count}</strong>成功</span><span className={batch.failed_count ? "danger" : ""}><strong>{batch.failed_count}</strong>失败</span></div>
      </Card> : null}
    </div>
  );
}
