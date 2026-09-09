import { submitScoringCommand } from "./scoringCommand";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { App } from "antd";
import {
  cancelScoringRun,
  downloadScoringResultImage,
  getExamAutomationResults,
  getScoringReadiness,
  getScoringSummary,
  retryFailedScoringRun,

  type ExamAutomationResults,
  type ScoringReadiness,
  type ScoringRunItem,
  type ScoringSummary
} from "../../../../api/review";
import { formatAnswer, formatError } from "../gradingWorkbench.model";
import type { ScoringImagePreview, ScoringResultState, ScoringResultType } from "../gradingWorkbench.types";

export interface UseExamScoringOptions {
  initialExamId: string;
  currentUserId: string;
  currentTenantId: string;
  canGrade: boolean;
  onTasksChanged: () => Promise<void>;
}

export function useExamScoring({ initialExamId, currentUserId, currentTenantId, canGrade, onTasksChanged }: UseExamScoringOptions) {
  const { message } = App.useApp();
  const [summary, setSummary] = useState<ScoringSummary | null>(null);
  const [readiness, setReadiness] = useState<ScoringReadiness | null>(null);
  const [loading, setLoading] = useState(false);
  const [runDetail, setRunDetail] = useState<ExamAutomationResults | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [resultType, setResultType] = useState<ScoringResultType>("all");
  const [resultState, setResultState] = useState<ScoringResultState>("all");
  const [resultKeyword, setResultKeyword] = useState("");
  const [image, setImage] = useState<ScoringImagePreview | null>(null);
  const [imageLoading, setImageLoading] = useState("");
  const [actioning, setActioning] = useState<string | null>(null);
  const commandStorageKey = `scoring-command:${currentTenantId}:${currentUserId}:${initialExamId}`;
  const [pendingCommand, setPendingCommand] = useState(() => localStorage.getItem(commandStorageKey));
  const startBusy = useRef(false);
  useEffect(() => { setPendingCommand(localStorage.getItem(commandStorageKey)); }, [commandStorageKey]);
  const summaryRequestRef = useRef(0);
  const imageRequestRef = useRef(0);

  const blockingChecks = useMemo(() => readiness?.checks.filter((check) => check.severity === "blocker" && !check.passed) ?? [], [readiness]);
  const warningChecks = useMemo(() => readiness?.checks.filter((check) => check.severity === "warning" && !check.passed) ?? [], [readiness]);
  const hasUnresolvedRun = Boolean(summary?.run && ["queued", "processing", "needs_review", "failed", "cancelling"].includes(summary.run.status));
  const filteredItems = useMemo(() => {
    const text = resultKeyword.trim().toLowerCase();
    return (runDetail?.items ?? []).filter((item) => {
      const typeMatched = resultType === "all" ||
        (resultType === "choice" && ["single_choice", "multiple_choice", "true_false"].includes(item.question_type)) ||
        (resultType === "fill" && ["fill_blank", "numeric"].includes(item.question_type));
      const stateMatched = resultState === "all" ||
        (resultState === "processing" ? ["pending", "processing", "cancelling"].includes(item.state) : item.state === resultState);
      const keywordMatched = !text || [item.anonymous_code, item.question_no, item.recognized_answer, formatAnswer(item.standard_answer)]
        .some((value) => (value ?? "").toLowerCase().includes(text));
      return typeMatched && stateMatched && keywordMatched;
    });
  }, [resultKeyword, resultState, resultType, runDetail?.items]);
  const resultMetrics = useMemo(() => {
    const items = runDetail?.items ?? [];
    return {
      total: items.length,
      auto: items.filter((item) => item.state === "confirmed" && item.grade_source === "rule_confirmed").length,
      review: items.filter((item) => item.state === "review").length,
      failed: items.filter((item) => item.state === "failed").length
    };
  }, [runDetail?.items]);

  const loadSummary = useCallback(async () => {
    const requestId = ++summaryRequestRef.current;
    if (!initialExamId || !canGrade) {
      setSummary(null);
      setReadiness(null);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const [result, readinessResult] = await Promise.all([getScoringSummary(initialExamId), getScoringReadiness(initialExamId)]);
      if (requestId !== summaryRequestRef.current) return;
      setSummary(result.scoring_summary);
      setReadiness(readinessResult.scoring_readiness);
    } catch (error) {
      if (requestId === summaryRequestRef.current) message.error(formatError(error));
    } finally {
      if (requestId === summaryRequestRef.current) setLoading(false);
    }
  }, [canGrade, initialExamId, message]);

  const start = useCallback(async () => {
    if (!initialExamId || !canGrade || startBusy.current) return;
    startBusy.current = true;
    setActioning("start-scoring");
    try {
      const completed = await submitScoringCommand({
        examId: initialExamId, storageKey: commandStorageKey, storage: localStorage, pending: setPendingCommand,
        ready: async () => {
          const result = await getScoringReadiness(initialExamId);
          setReadiness(result.scoring_readiness);
          if (!result.scoring_readiness.ready) message.warning("评分准备检查未通过，请先处理阻塞项");
          return result.scoring_readiness.ready;
        }
      });
      if (!completed) return;
      message.success("评分任务已生成");
      setDetailOpen(true);
      const [nextSummary, detail, nextReadiness] = await Promise.all([
        getScoringSummary(initialExamId),
        getExamAutomationResults(initialExamId),
        getScoringReadiness(initialExamId)
      ]);
      setSummary(nextSummary.scoring_summary);
      setRunDetail(detail);
      setReadiness(nextReadiness.scoring_readiness);
      await onTasksChanged();
    } catch (error) {
      message.error(formatError(error));
      void getScoringReadiness(initialExamId).then((result) => setReadiness(result.scoring_readiness)).catch(() => undefined);
    } finally {
      startBusy.current = false;
      setActioning(null);
    }
  }, [canGrade, initialExamId, commandStorageKey, message, onTasksChanged]);

  const showDetail = useCallback(async () => {
    if (!initialExamId) return;
    setActioning("scoring-detail");
    try {
      const [nextSummary, detail] = await Promise.all([getScoringSummary(initialExamId), getExamAutomationResults(initialExamId)]);
      setSummary(nextSummary.scoring_summary);
      setRunDetail(detail);
      setDetailOpen(true);
    } catch (error) {
      message.error(formatError(error));
    } finally {
      setActioning(null);
    }
  }, [initialExamId, message]);

  const showImage = useCallback(async (item: ScoringRunItem) => {
    const requestId = ++imageRequestRef.current;
    setImageLoading(item.answer_segment_id);
    try {
      const file = await downloadScoringResultImage(item.answer_segment_id);
      if (requestId !== imageRequestRef.current) return;
      setImage((current) => {
        if (current?.url) URL.revokeObjectURL(current.url);
        return { url: URL.createObjectURL(file.blob), contentType: file.contentType, filename: file.filename, title: `${item.question_no} · ${item.anonymous_code}` };
      });
    } catch (error) {
      if (requestId === imageRequestRef.current) message.error(formatError(error));
    } finally {
      if (requestId === imageRequestRef.current) setImageLoading("");
    }
  }, [message]);

  const retry = useCallback(async () => {
    const runId = summary?.run?.id;
    if (!runId) return;
    setActioning("retry-scoring");
    try {
      const result = await retryFailedScoringRun(runId);
      result.requeued > 0 ? message.success(`已重新安排 ${result.requeued} 个失败项`) : message.info("当前没有可重新处理的失败项");
      await Promise.all([loadSummary(), onTasksChanged()]);
    } catch (error) {
      message.error(formatError(error));
    } finally {
      setActioning(null);
    }
  }, [loadSummary, message, onTasksChanged, summary?.run?.id]);

  const cancel = useCallback(async () => {
    const runId = summary?.run?.id;
    if (!runId) return;
    setActioning("cancel-scoring");
    try {
      await cancelScoringRun(runId);
      message.success("本次评分已取消，未完成任务不会继续写入结果");
      await Promise.all([loadSummary(), onTasksChanged()]);
    } catch (error) {
      message.error(formatError(error));
    } finally {
      setActioning(null);
    }
  }, [loadSummary, message, onTasksChanged, summary?.run?.id]);

  const closeImage = useCallback(() => {
    imageRequestRef.current += 1;
    setImageLoading("");
    setImage(null);
  }, []);

  useEffect(() => {
    summaryRequestRef.current += 1;
    imageRequestRef.current += 1;
    setSummary(null);
    setReadiness(null);
    setLoading(false);
    setRunDetail(null);
    setDetailOpen(false);
    setImage(null);
    setImageLoading("");
  }, [initialExamId]);

  useEffect(() => {
    void loadSummary();
  }, [loadSummary]);

  useEffect(() => {
    const status = summary?.run?.status;
    if (!detailOpen || !initialExamId || !status || !["queued", "processing"].includes(status)) return;
    let disposed = false;
    const refresh = () => {
      void Promise.all([getScoringSummary(initialExamId), getExamAutomationResults(initialExamId), getScoringReadiness(initialExamId)])
        .then(([nextSummary, detail, nextReadiness]) => {
          if (disposed) return;
          setSummary(nextSummary.scoring_summary);
          setRunDetail(detail);
          setReadiness(nextReadiness.scoring_readiness);
        })
        .catch(() => undefined);
    };
    const timer = window.setInterval(refresh, 2500);
    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, [detailOpen, initialExamId, summary?.run?.status]);

  useEffect(() => () => {
    if (image?.url) URL.revokeObjectURL(image.url);
  }, [image?.url]);

  return {
    pendingCommand,
    summary,
    readiness,
    loading,
    runDetail,
    detailOpen,
    setDetailOpen,
    resultType,
    setResultType,
    resultState,
    setResultState,
    resultKeyword,
    setResultKeyword,
    image,
    imageLoading,
    actioning,
    blockingChecks,
    warningChecks,
    hasUnresolvedRun,
    filteredItems,
    resultMetrics,
    loadSummary,
    start,
    showDetail,
    showImage,
    retry,
    cancel,
    closeImage
  };
}

export type ExamScoringController = ReturnType<typeof useExamScoring>;
