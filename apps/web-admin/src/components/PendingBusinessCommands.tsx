import { useEffect, useRef, useState } from "react";
import { Alert, Button, Space } from "antd";
import { apiClient, getUserErrorMessage } from "../api/client";
import { recoverBusinessCommand, type PersistedBusinessCommand } from "../api/businessCommand";

const operations: Record<string, { label: string; path: (id: string) => string }> = {
  "review.submit": { label: "人工评分", path: id => `/api/v1/review-tasks/${id}/submit` },
  "review.arbitrate": { label: "仲裁评分", path: id => `/api/v1/arbitration-tasks/${id}/submit` },
  "score.confirm": { label: "成绩确认", path: id => `/api/v1/exams/${id}/confirm-grades` },
  "score.publish": { label: "成绩发布", path: id => `/api/v1/exams/${id}/publish` },
  "report.export": { label: "报表导出", path: id => `/api/v1/exams/${id}/reports/export` }
};

export function PendingBusinessCommands({ tenant, actor }: { tenant: string; actor: string }) {
  const prefix = `business-command:${tenant}:${actor}:`;
  const [keys, setKeys] = useState<string[]>([]);
  const [busy, setBusy] = useState("");
  const busyRef = useRef(false);
  const [error, setError] = useState("");
  useEffect(() => {
    const refresh = () => setKeys(Object.keys(localStorage).filter(key => key.startsWith(prefix)));
    refresh();
    window.addEventListener("business-command-changed", refresh);
    window.addEventListener("storage", refresh);
    return () => { window.removeEventListener("business-command-changed", refresh); window.removeEventListener("storage", refresh); };
  }, [prefix]);

  async function resume(key: string) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(key); setError("");
    try {
      const [operation, ...targetParts] = key.slice(prefix.length).split(":");
      const target = targetParts.join(":");
      const entry = operations[operation];
      if (!entry) throw new Error("无法识别待确认操作");
      const raw = localStorage.getItem(key);
      if (!raw) return;
      const command = JSON.parse(raw) as PersistedBusinessCommand;
      const reject = () => {
        localStorage.removeItem(key);
        setKeys(current => current.filter(item => item !== key));
      };
      if (operation === "report.export") {
        const file = await recoverBusinessCommand(operation, command,
          id => apiClient.requestBlob(entry.path(encodeURIComponent(target)), { method: "POST", headers: { "Idempotency-Key": id } }),
          resultValue => {
            const result = resultValue as { Content: string; ContentType: string; Filename: string; Watermark?: string };
            return {
              blob: new Blob([Uint8Array.from(atob(result.Content), char => char.charCodeAt(0))], { type: result.ContentType }),
              contentType: result.ContentType,
              filename: result.Filename,
              watermark: result.Watermark,
            };
          }, reject);
        const url = URL.createObjectURL(file.blob);
        const link = document.createElement("a"); link.href = url; link.download = file.filename ?? "report.csv"; link.click();
        window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      } else await recoverBusinessCommand(operation, command,
        (id, original) => apiClient.request(entry.path(encodeURIComponent(target)), { method: "POST", headers: { "Idempotency-Key": id }, body: JSON.stringify(original) }),
        () => undefined, reject);
      localStorage.removeItem(key);
      setKeys(current => current.filter(item => item !== key));
    } catch (cause) { setError(getUserErrorMessage(cause, "确认操作结果失败，请稍后继续")); }
    finally { busyRef.current = false; setBusy(""); }
  }
  if (!keys.length) return null;
  return <Alert type="warning" showIcon message="有待确认的操作" description={<Space wrap>{keys.map(key => <Button key={key} disabled={Boolean(busy)} loading={busy === key} onClick={() => void resume(key)}>继续确认{operations[key.slice(prefix.length).split(":")[0]]?.label ?? "操作"}</Button>)}{error && <span>{error}</span>}</Space>} />;
}
