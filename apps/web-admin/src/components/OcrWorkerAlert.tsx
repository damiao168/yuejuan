import { useCallback, useEffect, useState } from "react";
import { Alert, Button } from "antd";
import { Activity } from "lucide-react";
import { getOcrAvailability, type WorkerServiceStatus } from "../api/system";

export function OcrWorkerAlert({ enabled }: { enabled: boolean }) {
  const [worker, setWorker] = useState<WorkerServiceStatus | null>(null);
  const [statusError, setStatusError] = useState(false);

  const load = useCallback(async () => {
    if (!enabled) return;
    try {
      const status = await getOcrAvailability();
      setWorker(status.worker);
      setStatusError(false);
    } catch {
      setStatusError(true);
    }
  }, [enabled]);

  useEffect(() => {
    if (!enabled) return;
    void load();
    const timer = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(timer);
  }, [enabled, load]);

  if (!enabled || (!statusError && (!worker || worker.automation_available))) return null;

  const stale = worker?.availability === "stale";
  return (
    <Alert
      className="ocr-worker-alert"
      type={stale || statusError ? "warning" : "error"}
      showIcon
      icon={<Activity size={18} />}
      message={statusError ? "无法确认 OCR 自动处理状态" : stale ? "OCR Worker 心跳已失联" : "OCR 自动处理已暂停"}
      description={statusError ? (
        "系统状态暂时无法读取。依赖 OCR 的填空题请按人工流程处理，并检查系统状态。"
      ) : (
        <div className="ocr-worker-alert-detail">
          <span>{worker!.impact}</span>
          <strong>待处理 {worker!.queued_tasks} · 处理中 {worker!.in_flight_tasks} · 死信 {worker!.dead_letter_tasks}</strong>
          <span>处理建议：{worker!.action}</span>
        </div>
      )}
      action={<Button size="small" onClick={() => void load()}>重新检查</Button>}
    />
  );
}
