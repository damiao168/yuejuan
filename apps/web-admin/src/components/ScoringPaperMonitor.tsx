import { useEffect, useMemo, useState } from "react";
import { Button, Empty, Progress, Spin } from "antd";
import { CheckCircle2, CircleAlert, Clock3, FileStack, RefreshCw } from "lucide-react";
import { downloadFileBlob } from "../api/files";
import type { ScoringRun, ScoringRunItem } from "../api/review";
import { StatusTag } from "./StatusTag";

interface ScoringPaperMonitorProps {
  run?: ScoringRun;
  items: ScoringRunItem[];
  loading?: boolean;
  onRefresh: () => void;
}

interface CandidateGroup {
  code: string;
  items: ScoringRunItem[];
  processed: number;
  confirmed: number;
  review: number;
  failed: number;
  scored: number;
  totalScore: number;
  maxScore: number;
}

const stateLabel: Record<string, string> = {
  confirmed: "已评分",
  review: "待人工",
  failed: "失败",
  processing: "处理中",
  pending: "等待处理",
  cancelled: "已取消",
  cancelling: "正在取消"
};

function groupCandidates(items: ScoringRunItem[]) {
  const grouped = new Map<string, ScoringRunItem[]>();
  for (const item of items) {
    const current = grouped.get(item.anonymous_code) ?? [];
    current.push(item);
    grouped.set(item.anonymous_code, current);
  }
  return [...grouped.entries()]
    .map(([code, candidateItems]): CandidateGroup => ({
      code,
      items: candidateItems,
      processed: candidateItems.filter((item) => item.state !== "queued" && item.state !== "processing").length,
      confirmed: candidateItems.filter((item) => item.state === "confirmed").length,
      review: candidateItems.filter((item) => item.state === "review").length,
      failed: candidateItems.filter((item) => item.state === "failed").length,
      scored: candidateItems.filter((item) => typeof item.score === "number").length,
      totalScore: candidateItems.reduce((sum, item) => sum + (item.score ?? 0), 0),
      maxScore: candidateItems.reduce((sum, item) => sum + (item.max_score ?? 0), 0)
    }))
    .sort((left, right) => left.code.localeCompare(right.code));
}

function scoreText(item: ScoringRunItem) {
  if (typeof item.score !== "number") return "";
  const score = Number.isInteger(item.score) ? item.score : item.score.toFixed(1);
  const max = typeof item.max_score === "number"
    ? (Number.isInteger(item.max_score) ? item.max_score : item.max_score.toFixed(1))
    : "";
  return `${item.grade_source === "ai_suggestion" ? "≈" : ""}${score}${max === "" ? "" : `/${max}`}`;
}

export function ScoringPaperMonitor({ run, items, loading, onRefresh }: ScoringPaperMonitorProps) {
  const candidates = useMemo(() => groupCandidates(items), [items]);
  const [selectedCode, setSelectedCode] = useState("");
  const [pageUrls, setPageUrls] = useState<Record<string, string>>({});
  const [pageLoading, setPageLoading] = useState(false);

  useEffect(() => {
    if (!candidates.length) {
      setSelectedCode("");
      return;
    }
    if (!candidates.some((candidate) => candidate.code === selectedCode)) {
      setSelectedCode(candidates[0].code);
    }
  }, [candidates, selectedCode]);

  const selected = candidates.find((candidate) => candidate.code === selectedCode) ?? candidates[0];
  const pages = useMemo(() => {
    const unique = new Map<string, { pageNo: number; assetId: string }>();
    for (const item of selected?.items ?? []) {
      if (item.page_file_asset_id) {
        unique.set(item.page_file_asset_id, { pageNo: item.page_no, assetId: item.page_file_asset_id });
      }
    }
    return [...unique.values()].sort((left, right) => left.pageNo - right.pageNo);
  }, [selected]);

  useEffect(() => {
    let disposed = false;
    const urls: string[] = [];
    setPageUrls({});
    if (!pages.length) return;
    setPageLoading(true);
    void Promise.all(pages.map(async (page) => {
      const file = await downloadFileBlob(page.assetId);
      const url = URL.createObjectURL(file.blob);
      urls.push(url);
      return [page.assetId, url] as const;
    }))
      .then((entries) => {
        if (!disposed) setPageUrls(Object.fromEntries(entries));
      })
      .finally(() => {
        if (!disposed) setPageLoading(false);
      });
    return () => {
      disposed = true;
      urls.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [pages]);

  const processed = items.filter((item) => item.state !== "queued" && item.state !== "processing").length;
  const progress = items.length ? Math.round((processed / items.length) * 100) : 0;
  const activeItems = (selected?.items ?? [])
    .filter((item) => item.state !== "confirmed")
    .sort((left, right) => left.page_no - right.page_no || left.question_no.localeCompare(right.question_no))
    .slice(0, 8);

  if (!items.length) {
    return <div className="scoring-paper-empty"><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="评分启动后，这里会逐卷显示处理进度与红笔得分" /></div>;
  }

  return (
    <div className="scoring-paper-monitor">
      <header className="scoring-monitor-header">
        <div>
          <span className="scoring-monitor-kicker">实时阅卷监控</span>
          <h3>{run?.status === "needs_review" ? "自动识别完成，等待教师确认" : run?.status === "completed" ? "本批答卷已处理完成" : "正在逐卷识别与评分"}</h3>
          <p>红色批注定位到原答题区域；“≈”表示 AI 建议分，仍需教师确认。</p>
        </div>
        <div className="scoring-monitor-progress">
          <div><strong>{processed}</strong><span>/ {items.length} 题已识别 · {run?.review_count ?? 0} 题待人工</span></div>
          <Progress percent={progress} showInfo={false} strokeColor="#1677ff" trailColor="#e8edf5" />
          <Button icon={<RefreshCw size={15} />} loading={loading} onClick={onRefresh}>刷新</Button>
        </div>
      </header>

      <div className="scoring-monitor-body">
        <aside className="scoring-candidate-rail">
          <div className="scoring-rail-title"><FileStack size={16} /> 答卷队列 <span>{candidates.length}</span></div>
          {candidates.map((candidate, index) => {
            return (
              <button
                type="button"
                key={candidate.code}
                className={`scoring-candidate-item ${candidate.code === selected?.code ? "active" : ""}`}
                onClick={() => setSelectedCode(candidate.code)}
              >
                <span className="scoring-candidate-index">{String(index + 1).padStart(2, "0")}</span>
                <span className="scoring-candidate-copy">
                  <strong>{candidate.code}</strong>
                  <small>{candidate.processed}/{candidate.items.length} 已识别 · {candidate.review} 待人工</small>
                </span>
                {candidate.failed > 0
                  ? <CircleAlert size={16} className="danger" />
                  : candidate.confirmed === candidate.items.length
                    ? <CheckCircle2 size={16} className="success" />
                    : <Clock3 size={16} className="processing" />}
              </button>
            );
          })}
        </aside>

        <main className="scoring-paper-stage">
          <div className="scoring-paper-stage-head">
            <div>
              <strong>{selected?.code}</strong>
              <span>{pages.length} 页整卷 · {selected?.scored ?? 0} 题已有分数</span>
            </div>
            <div className="scoring-total-score">
              <span>当前合计</span>
              <strong>{selected?.totalScore ?? 0}</strong>
              <small>{selected?.maxScore ? `/ ${selected.maxScore}` : "分"}</small>
            </div>
          </div>
          <div className="scoring-paper-scroll">
            {pageLoading ? <div className="scoring-page-loading"><Spin /><span>正在装载整卷影像</span></div> : null}
            {pages.map((page) => {
              const pageItems = selected?.items.filter((item) => item.page_file_asset_id === page.assetId) ?? [];
              return (
                <figure className="scoring-paper-page" key={page.assetId}>
                  <figcaption>第 {page.pageNo} 页</figcaption>
                  {pageUrls[page.assetId] ? <div className="scoring-paper-image-wrap">
                    <img src={pageUrls[page.assetId]} alt={`${selected?.code} 第 ${page.pageNo} 页`} />
                    {pageItems.filter((item) => typeof item.score === "number").map((item) => {
                      const box = item.normalized_bbox;
                      if (!box) return null;
                      return <span
                        key={item.answer_segment_id}
                        className={`scoring-score-mark ${item.grade_source === "ai_suggestion" ? "suggestion" : ""}`}
                        title={`${item.question_no} · ${stateLabel[item.state] ?? item.state}`}
                        style={{
                          left: `${Math.min(98, (box.x + box.width) * 100)}%`,
                          top: `${Math.max(1, box.y * 100)}%`
                        }}
                      >{item.question_no} {scoreText(item)}</span>;
                    })}
                  </div> : null}
                </figure>
              );
            })}
          </div>
        </main>

        <aside className="scoring-activity-rail">
          <div className="scoring-activity-summary">
            <span>这张答卷</span>
            <div><strong>{selected?.confirmed ?? 0}</strong><small>已确认</small></div>
            <div><strong>{selected?.review ?? 0}</strong><small>待人工</small></div>
            <div><strong className={selected?.failed ? "danger" : ""}>{selected?.failed ?? 0}</strong><small>失败</small></div>
          </div>
          <div className="scoring-activity-list">
            <h4>当前流程</h4>
            {activeItems.length ? activeItems.map((item) => (
              <div className="scoring-activity-item" key={item.answer_segment_id}>
                <span className={`scoring-activity-dot ${item.state}`} />
                <div><strong>{item.question_no} · {stateLabel[item.state] ?? item.state}</strong><small>{item.reason_code || item.error_code || item.runtime_status || item.review_status || "等待评分结果"}</small></div>
                <StatusTag tone={item.state === "failed" ? "danger" : item.state === "review" ? "warning" : "processing"}>{`${item.page_no} 页`}</StatusTag>
              </div>
            )) : <div className="scoring-activity-done"><CheckCircle2 size={24} /><span>当前答卷已全部确认</span></div>}
          </div>
        </aside>
      </div>
    </div>
  );
}
