import { useCallback, useEffect, useMemo, useState } from "react";
import {
  App,
  Button,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Progress,
  Select,
  Space,
  Table,
  Tabs,
  Upload,
  type TableColumnsType,
  type UploadProps,
} from "antd";
import {
  Check,
  Combine,
  FileQuestion,
  FileUp,
  FolderOpen,
  Plus,
  RefreshCw,
  RotateCw,
  ScanLine,
  Scissors,
  SlidersHorizontal,
  Trash2,
  Undo2,
  UserCheck,
  Workflow,
} from "lucide-react";
import { ApiClientError } from "../api/client";
import {
  confirmPageMatch,
  confirmRegistration,
  confirmStudentMatch,
  completeCaptureBatch,
  createCaptureBatch,
  deleteCapturePage,
  getCaptureBatch,
  getMatchingQueue,
  getProcessingSummary,
  listCaptureBatches,
  markStudentUnknown,
  mergeCaptureSubmissions,
  processCaptureBatch,
  processSubmissionPages,
  registerCaptureFile,
  restoreCapturePage,
  retryRegistration,
  splitCaptureSubmission,
  updateCapturePage,
  type CaptureBatch,
  type CaptureBatchDetail,
  type CaptureFile,
  type CapturePage,
  type MatchingQueue,
  type ProcessingSummary,
} from "../api/capture";
import { downloadFileBlob, uploadFile } from "../api/files";
import { ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
import { RegistrationCorrectionWorkspace } from "../components/RegistrationCorrectionWorkspace";

const statusLabels: Record<string, string> = {
  draft: "等待上传",
  uploading: "正在上传",
  matching: "等待匹配",
  processing: "正在处理",
  needs_review: "需要确认",
  ready: "处理完成",
  completed: "已完成",
  cancelled: "已取消",
  uploaded: "已上传",
  queued: "等待处理",
  duplicate: "重复文件",
  failed: "处理失败",
  grouped: "已组织",
  registration: "正在配准",
};

function formatError(error: unknown) {
  if (error instanceof ApiClientError) return error.message;
  return error instanceof Error ? error.message : "操作失败，请重试";
}

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "processing" | "neutral" {
  if (status === "completed" || status === "ready" || status === "grouped")
    return "success";
  if (status === "failed" || status === "cancelled") return "danger";
  if (status === "needs_review" || status === "duplicate") return "warning";
  if (status === "processing" || status === "queued" || status === "uploading")
    return "processing";
  return "neutral";
}

function batchProgress(batch: CaptureBatch) {
  if (batch.status === "completed") return 100;
  if (batch.file_count === 0) return 0;
  const processed = Math.max(0, batch.file_count - batch.failed_count);
  return batch.status === "processing"
    ? Math.min(85, Math.round((processed / batch.file_count) * 70))
    : batch.page_count > 0
      ? 70
      : 25;
}

function MatchingWorkspace({
  queue,
  canManage,
  actioning,
  onPreview,
  onConfirmStudent,
  onUnknown,
  onConfirmPage,
  onSplit,
  onMerge,
}: {
  queue: MatchingQueue;
  canManage: boolean;
  actioning: boolean;
  onPreview: (page: CapturePage) => Promise<void>;
  onConfirmStudent: (
    submissionId: string,
    studentId: string,
    revision: number,
  ) => Promise<void>;
  onUnknown: (submissionId: string, revision: number) => Promise<void>;
  onConfirmPage: (page: CapturePage, pageNo: number) => Promise<void>;
  onSplit?: (submissionId: string, pageId: string) => Promise<void>;
  onMerge?: (targetId: string, sourceId: string) => Promise<void>;
}) {
  const [selectedId, setSelectedId] = useState(queue.submissions[0]?.id ?? "");
  const [candidateId, setCandidateId] = useState("");
  const [mergeTarget, setMergeTarget] = useState("");
  const selected =
    queue.submissions.find((item) => item.id === selectedId) ??
    queue.submissions[0];
  useEffect(() => {
    setCandidateId(selected?.student_id ?? "");
  }, [selected?.id, selected?.student_id]);
  const used = new Set(
    queue.submissions
      .filter(
        (item) =>
          item.id !== selected?.id && item.identity_status === "matched",
      )
      .map((item) => item.student_id),
  );
  const candidates = queue.candidates.filter(
    (item) => !used.has(item.id) || item.id === selected?.student_id,
  );
  if (!selected) return <Empty description="暂无待匹配答卷" />;
  return (
    <div className="matching-workspace">
      <aside className="matching-submissions">
        <div className="capture-pane-title">
          <strong>答卷</strong>
          <span>{queue.submissions.length}</span>
        </div>
        {queue.submissions.map((item, index) => (
          <button
            key={item.id}
            className={
              item.id === selected.id
                ? "capture-batch-row active"
                : "capture-batch-row"
            }
            onClick={() => {
              setSelectedId(item.id);
              setCandidateId(item.student_id ?? "");
            }}
          >
            <span>
              <strong>答卷 {index + 1}</strong>
              <small>{item.pages.length} 页</small>
            </span>
            <StatusTag
              tone={
                item.identity_status === "matched"
                  ? "success"
                  : item.identity_status === "unknown"
                    ? "danger"
                    : "warning"
              }
            >
              {item.identity_status === "matched"
                ? "已匹配"
                : item.identity_status === "unknown"
                  ? "未知"
                  : "待确认"}
            </StatusTag>
          </button>
        ))}
      </aside>
      <section className="matching-evidence">
        <h3>答卷页面</h3>
        {selected.pages.map((page, index) => (
          <div className="matching-page-row" key={page.id}>
            <Button
              icon={<FolderOpen size={15} />}
              onClick={() => void onPreview(page)}
            >
              第 {page.sequence_no} 页
            </Button>
            <InputNumber
              min={1}
              value={page.assigned_page_no}
              onChange={(value) => value && void onConfirmPage(page, value)}
              disabled={!canManage || actioning}
              aria-label="确认答卷页码"
            />
            <Space>
              <StatusTag tone={page.status === "ready" ? "success" : "warning"}>
                {page.status}
              </StatusTag>
              {selected.pages.length > 1 && index > 0 && onSplit ? (
                <Button
                  icon={<Scissors size={14} />}
                  aria-label="拆分为新答卷"
                  onClick={() => void onSplit(selected.id, page.id)}
                />
              ) : null}
            </Space>
          </div>
        ))}
        <div className="matching-evidence-note">
          <ScanLine size={17} />
          <span>
            姓名、学号或条码仅作为候选证据；人工确认后才写入正式身份。
          </span>
        </div>
      </section>
      <section className="matching-candidates">
        <h3>本次考试名册</h3>
        <Select
          showSearch
          value={candidateId || undefined}
          onChange={setCandidateId}
          placeholder="按姓名或学号查找"
          optionFilterProp="label"
          options={candidates.map((item) => ({
            value: item.id,
            label: `${item.name} · ${item.student_no} · ${item.class_name}`,
          }))}
        />
        <Space wrap>
          <Button
            type="primary"
            icon={<UserCheck size={16} />}
            loading={actioning}
            disabled={!canManage || !candidateId}
            onClick={() =>
              void onConfirmStudent(
                selected.id,
                candidateId,
                selected.identity_revision,
              )
            }
          >
            确认学生
          </Button>
          <Button
            danger
            icon={<FileQuestion size={16} />}
            loading={actioning}
            disabled={!canManage}
            onClick={() =>
              void onUnknown(selected.id, selected.identity_revision)
            }
          >
            标记未知
          </Button>
        </Space>
        {queue.submissions.length > 1 && onMerge ? (
          <Space.Compact>
            <Select
              value={mergeTarget || undefined}
              onChange={setMergeTarget}
              placeholder="合并到其他答卷"
              options={queue.submissions
                .filter((item) => item.id !== selected.id)
                .map((item, index) => ({
                  value: item.id,
                  label: `答卷 ${index + 1}`,
                }))}
            />
            <Button
              icon={<Combine size={15} />}
              disabled={!mergeTarget}
              onClick={() => void onMerge(mergeTarget, selected.id)}
            >
              合并
            </Button>
          </Space.Compact>
        ) : null}
        {selected.identity_status === "matched" ? (
          <div className="matching-confirmed">
            <Check size={17} />
            已确认，仍可重新选择并更正
          </div>
        ) : null}
      </section>
    </div>
  );
}

export function CaptureBatchPage({
  examId,
  canManage,
}: {
  examId: string;
  canManage: boolean;
}) {
  const { message, modal } = App.useApp();
  const [form] = Form.useForm<{
    name: string;
    source_type: CaptureBatch["source_type"];
  }>();
  const [batches, setBatches] = useState<CaptureBatch[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [detail, setDetail] = useState<CaptureBatchDetail>();
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [modalOpen, setModalOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [actioning, setActioning] = useState(false);
  const [preview, setPreview] = useState<{ url: string; page: CapturePage }>();
  const [matching, setMatching] = useState<MatchingQueue>();
  const [matchingLoading, setMatchingLoading] = useState(false);
  const [processingSummaries, setProcessingSummaries] = useState<Record<string, ProcessingSummary>>({});
  const [correctionRunId, setCorrectionRunId] = useState<string>();
  const batchCanManage = canManage && !["completed", "cancelled"].includes(detail?.batch.status ?? "");

  const loadBatches = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const result = await listCaptureBatches(examId);
      setBatches(result.batches);
      setSelectedId((current) => current || result.batches[0]?.id || "");
    } catch (currentError) {
      setError(formatError(currentError));
    } finally {
      setLoading(false);
    }
  }, [examId]);

  const loadDetail = useCallback(
    async (batchId: string, quiet = false) => {
      if (!batchId) {
        setDetail(undefined);
        return;
      }
      if (!quiet) setDetailLoading(true);
      try {
        const result = await getCaptureBatch(batchId);
        setDetail(result);
        setBatches((current) =>
          current.map((item) =>
            item.id === result.batch.id ? result.batch : item,
          ),
        );
      } catch (currentError) {
        if (!quiet) message.error(formatError(currentError));
      } finally {
        if (!quiet) setDetailLoading(false);
      }
    },
    [message],
  );

  const loadMatching = useCallback(
    async (batchId: string) => {
      if (!batchId) return;
      setMatchingLoading(true);
      try {
        setMatching(await getMatchingQueue(batchId));
      } catch (currentError) {
        message.error(formatError(currentError));
      } finally {
        setMatchingLoading(false);
      }
    },
    [message],
  );

  useEffect(() => {
    void loadBatches();
  }, [loadBatches]);
  useEffect(() => {
    void loadDetail(selectedId);
  }, [loadDetail, selectedId]);
  useEffect(() => {
    void loadMatching(selectedId);
  }, [loadMatching, selectedId]);
  useEffect(() => {
    if (!detail || !["uploading", "processing"].includes(detail.batch.status))
      return;
    const timer = window.setInterval(
      () => void loadDetail(detail.batch.id, true),
      3000,
    );
    return () => window.clearInterval(timer);
  }, [detail, loadDetail]);
  useEffect(
    () => () => {
      if (preview?.url) URL.revokeObjectURL(preview.url);
    },
    [preview],
  );

  async function submitBatch() {
    const values = await form.validateFields();
    setCreating(true);
    try {
      const result = await createCaptureBatch(examId, {
        ...values,
        idempotency_key: crypto.randomUUID(),
      });
      setModalOpen(false);
      form.resetFields();
      await loadBatches();
      setSelectedId(result.batch.id);
      message.success("采集批次已创建");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setCreating(false);
    }
  }

  const uploadProps: UploadProps = {
    multiple: true,
    showUploadList: false,
    accept: ".pdf,.png,.jpg,.jpeg,.tif,.tiff",
    beforeUpload: (file) => {
      void uploadSource(file);
      return false;
    },
  };

  async function uploadSource(file: File) {
    if (!detail) return;
    setUploading(true);
    try {
      const uploaded = await uploadFile(file, {
        owner_type: "capture_batch",
        owner_id: detail.batch.id,
        exam_id: examId,
      });
      await registerCaptureFile(
        detail.batch.id,
        uploaded.file.id,
        `${file.name}:${uploaded.file.hash_sha256}`,
      );
      await loadDetail(detail.batch.id);
      message.success(`${file.name} 已加入批次`);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setUploading(false);
    }
  }

  async function startProcessing() {
    if (!detail) return;
    setActioning(true);
    try {
      await processCaptureBatch(detail.batch.id);
      await loadDetail(detail.batch.id);
      message.success("文件已进入页面处理队列");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }

  function completeBatch() {
    if (!detail) return;
    modal.confirm({
      title: "确认完成批次",
      content: "完成后该批次将进入只读状态，请确认活动页面均已处理并完成学生匹配。",
      okText: "完成批次",
      cancelText: "返回检查",
      onOk: async () => {
        setActioning(true);
        try {
          await completeCaptureBatch(detail.batch.id, "所有活动页面处理完成并已人工确认");
          await Promise.all([loadBatches(), loadDetail(detail.batch.id)]);
          message.success("批次已完成");
        } catch (currentError) {
          message.error(formatError(currentError));
          throw currentError;
        } finally {
          setActioning(false);
        }
      },
    });
  }

  async function startPageProcessing(submissionId: string) {
    setActioning(true);
    try {
      const result = await processSubmissionPages(submissionId);
      await loadDetail(detail!.batch.id);
      message.success(`已提交 ${result.runs.length} 个页面配准任务`);
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }

  async function showPage(page: CapturePage) {
    try {
      const blob = await downloadFileBlob(page.decoded_file_asset_id);
      if (preview?.url) URL.revokeObjectURL(preview.url);
      setPreview({ url: URL.createObjectURL(blob.blob), page });
    } catch (currentError) {
      message.error(formatError(currentError));
    }
  }

  async function rotatePage(page: CapturePage) {
    try {
      await updateCapturePage(page.id, {
        revision: page.revision,
        rotation_degrees: (page.rotation_degrees + 90) % 360,
      });
      await loadDetail(page.capture_batch_id);
      message.success("页面旋转已保存，后续处理将重新校验");
    } catch (currentError) {
      message.error(formatError(currentError));
    }
  }

  async function matchStudent(
    submissionId: string,
    studentId: string,
    revision: number,
  ) {
    setActioning(true);
    try {
      await confirmStudentMatch(submissionId, studentId, revision);
      await Promise.all([
        loadMatching(selectedId),
        loadDetail(selectedId, true),
      ]);
      message.success("学生匹配已确认");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }

  async function markUnknown(submissionId: string, revision: number) {
    setActioning(true);
    try {
      await markStudentUnknown(
        submissionId,
        revision,
        "答卷上无法可靠识别学生身份",
      );
      await Promise.all([
        loadMatching(selectedId),
        loadDetail(selectedId, true),
      ]);
      message.warning("已标记为未知答卷");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }

  async function matchPage(page: CapturePage, pageNo: number) {
    setActioning(true);
    try {
      await confirmPageMatch(page.id, pageNo, page.revision);
      await Promise.all([
        loadMatching(selectedId),
        loadDetail(selectedId, true),
      ]);
      message.success("页码已确认");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }

  async function changePageLifecycle(page: CapturePage) {
    setActioning(true);
    try {
      if (page.status === "deleted")
        await restoreCapturePage(page.id, page.revision, "恢复误删页面");
      else await deleteCapturePage(page.id, page.revision, "移除非答卷页面");
      await Promise.all([
        loadMatching(selectedId),
        loadDetail(selectedId, true),
      ]);
      message.success(page.status === "deleted" ? "页面已恢复" : "页面已删除");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }
  async function splitSubmission(submissionId: string, pageId: string) {
    setActioning(true);
    try {
      setMatching(
        await splitCaptureSubmission(
          selectedId,
          submissionId,
          [pageId],
          "人工拆分混扫答卷",
        ),
      );
      await loadDetail(selectedId, true);
      message.success("已拆分为新答卷");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }
  async function mergeSubmissions(targetId: string, sourceId: string) {
    setActioning(true);
    try {
      setMatching(
        await mergeCaptureSubmissions(
          selectedId,
          targetId,
          sourceId,
          "人工合并散页答卷",
        ),
      );
      await loadDetail(selectedId, true);
      message.success("答卷已合并");
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setActioning(false);
    }
  }

  const fileColumns: TableColumnsType<CaptureFile> = [
    { title: "文件", dataIndex: "original_name", ellipsis: true },
    { title: "类型", dataIndex: "content_type", width: 150 },
    { title: "页数", dataIndex: "page_count", width: 72 },
    {
      title: "状态",
      width: 110,
      render: (_, item) => (
        <StatusTag tone={statusTone(item.status)}>
          {statusLabels[item.status] ?? item.status}
        </StatusTag>
      ),
    },
    {
      title: "问题",
      width: 160,
      render: (_, item) =>
        item.error_code ||
        (item.status === "duplicate" ? "内容与批次内文件重复" : "-"),
    },
  ];
  const pageColumns: TableColumnsType<CapturePage> = [
    { title: "顺序", dataIndex: "sequence_no", width: 70 },
    { title: "来源页", dataIndex: "source_index", width: 80 },
    { title: "答卷页码", dataIndex: "assigned_page_no", width: 90 },
    {
      title: "旋转",
      dataIndex: "rotation_degrees",
      width: 72,
      render: (value: number) => `${value}°`,
    },
    {
      title: "状态",
      width: 110,
      render: (_, item) => (
        <StatusTag tone={statusTone(item.status)}>
          {statusLabels[item.status] ?? item.status}
        </StatusTag>
      ),
    },
    {
      title: "操作",
      width: 190,
      render: (_, item) => (
        <Space>
          <Button
            size="small"
            icon={<FolderOpen size={14} />}
            onClick={() => void showPage(item)}
            aria-label="查看页面"
          />
          <Button
            size="small"
            icon={<RotateCw size={14} />}
            onClick={() => void rotatePage(item)}
            disabled={!batchCanManage || item.status === "deleted"}
            aria-label="顺时针旋转"
          />
          <Button
            size="small"
            danger={item.status !== "deleted"}
            icon={
              item.status === "deleted" ? (
                <Undo2 size={14} />
              ) : (
                <Trash2 size={14} />
              )
            }
            onClick={() => void changePageLifecycle(item)}
            disabled={!batchCanManage}
            aria-label={item.status === "deleted" ? "恢复页面" : "删除页面"}
          />
        </Space>
      ),
    },
  ];

  const summary = useMemo(
    () =>
      detail
        ? [
            { label: "文件", value: detail.batch.file_count },
            { label: "页面", value: detail.batch.page_count },
            { label: "答卷", value: detail.batch.submission_count },
            { label: "待确认", value: detail.batch.review_count },
            { label: "失败", value: detail.batch.failed_count },
          ]
        : [],
    [detail],
  );
  const submissions = useMemo(
    () =>
      detail
        ? (Array.from(
            new Set(
              detail.pages.map((item) => item.submission_id).filter(Boolean),
            ),
          ) as string[])
        : [],
    [detail],
  );
  const issuePages = useMemo(
    () =>
      detail
        ? detail.pages.filter((item) =>
            ["needs_review", "quality_rejected", "failed"].includes(
              item.status,
            ),
          )
        : [],
    [detail],
  );
  useEffect(() => { void Promise.all(submissions.map(async (id) => [id, await getProcessingSummary(id)] as const)).then((items) => setProcessingSummaries(Object.fromEntries(items))).catch(() => setProcessingSummaries({})); }, [submissions]);

  async function resolveRegistration(runId: string, action: "confirm" | "retry") { setActioning(true); try { if (action === "confirm") await confirmRegistration(runId, "人工核对配准边界与题区正确"); else await retryRegistration(runId); const items = await Promise.all(submissions.map(async (id) => [id, await getProcessingSummary(id)] as const)); setProcessingSummaries(Object.fromEntries(items)); await loadDetail(selectedId, true); message.success(action === "confirm" ? "配准已确认" : "配准已重新排队"); } catch (currentError) { message.error(formatError(currentError)); } finally { setActioning(false); } }

  if (loading) return <LoadingState label="正在加载采集批次" />;
  if (error)
    return (
      <ErrorState
        message={`采集批次加载失败：${error}`}
        onRetry={() => void loadBatches()}
      />
    );
  if (correctionRunId) return <RegistrationCorrectionWorkspace runId={correctionRunId} canManage={batchCanManage} onClose={() => setCorrectionRunId(undefined)} onChanged={async () => { await loadDetail(selectedId, true); }} />;
  return (
    <div className="capture-batch-page">
      <section className="page-heading">
        <div>
          <h1>答卷采集批次</h1>
          <p>批量导入扫描文件，拆分页面并处理匹配和质量问题。</p>
        </div>
        <Space>
          <Button
            icon={<RefreshCw size={16} />}
            onClick={() => void loadBatches()}
          >
            刷新
          </Button>
          <Button
            type="primary"
            icon={<Plus size={16} />}
            onClick={() => setModalOpen(true)}
            disabled={!canManage}
          >
            新建批次
          </Button>
        </Space>
      </section>
      {batches.length === 0 ? (
        <Empty description="尚未创建采集批次">
          <Button type="primary" onClick={() => setModalOpen(true)} disabled={!canManage}>
            创建第一个批次
          </Button>
        </Empty>
      ) : (
        <div className="capture-batch-layout">
          <aside className="capture-batch-list">
            <div className="capture-pane-title">
              <strong>批次</strong>
              <span>{batches.length}</span>
            </div>
            {batches.map((batch) => (
              <button
                key={batch.id}
                className={
                  batch.id === selectedId
                    ? "capture-batch-row active"
                    : "capture-batch-row"
                }
                onClick={() => setSelectedId(batch.id)}
              >
                <span>
                  <strong>{batch.name}</strong>
                  <small>{new Date(batch.created_at).toLocaleString()}</small>
                </span>
                <StatusTag tone={statusTone(batch.status)}>
                  {statusLabels[batch.status]}
                </StatusTag>
              </button>
            ))}
          </aside>
          <main className="capture-batch-workspace">
            {detailLoading || !detail ? (
              <LoadingState label="正在读取批次" />
            ) : (
              <>
                <header className="capture-batch-header">
                  <div>
                    <Space>
                      <ScanLine size={20} />
                      <h2>{detail.batch.name}</h2>
                      <StatusTag tone={statusTone(detail.batch.status)}>
                        {statusLabels[detail.batch.status]}
                      </StatusTag>
                    </Space>
                    <Progress
                      percent={batchProgress(detail.batch)}
                      size="small"
                    />
                  </div>
                  <Space wrap>
                    <Upload {...uploadProps}>
                      <Button
                        icon={<FileUp size={16} />}
                        loading={uploading}
                        disabled={!batchCanManage}
                      >
                        导入文件
                      </Button>
                    </Upload>
                    <Button
                      type="primary"
                      icon={<Workflow size={16} />}
                      onClick={() => void startProcessing()}
                      loading={actioning}
                      disabled={
                        !batchCanManage ||
                        !detail.files.some((item) => item.status === "uploaded")
                      }
                    >
                      开始处理
                    </Button>
                    {detail.batch.status === "ready" && (
                      <Button
                        type="primary"
                        icon={<Check size={16} />}
                        onClick={completeBatch}
                        loading={actioning}
                        disabled={!batchCanManage}
                      >
                        完成批次
                      </Button>
                    )}
                  </Space>
                </header>
                <div className="capture-summary-strip">
                  {summary.map((item) => (
                    <div key={item.label}>
                      <span>{item.label}</span>
                      <strong>{item.value}</strong>
                    </div>
                  ))}
                </div>
                <Tabs
                  items={[
                    {
                      key: "files",
                      label: `文件与页面 (${detail.files.length})`,
                      children: (
                        <div className="capture-detail-stack">
                          <Table
                            rowKey="id"
                            columns={fileColumns}
                            dataSource={detail.files}
                            pagination={false}
                            size="small"
                            scroll={{ x: 720 }}
                          />
                          <Table
                            rowKey="id"
                            columns={pageColumns}
                            dataSource={detail.pages}
                            pagination={{ pageSize: 20 }}
                            size="small"
                            scroll={{ x: 700 }}
                          />
                        </div>
                      ),
                    },
                    {
                      key: "processing",
                      label: `页面处理 (${submissions.length})`,
                      children: submissions.length ? (
                        <div className="capture-processing-list">
                          {submissions.map((submissionId, index) => {
                            const pages = detail.pages.filter(
                              (item) => item.submission_id === submissionId,
                            );
                            const complete = pages.every(
                              (item) => item.status === "ready",
                            );
                            const summary = processingSummaries[submissionId];
                            return (
                              <div
                                key={submissionId}
                                className="capture-processing-row"
                              >
                                <div>
                                  <strong>答卷 {index + 1}</strong>
                                  <span>
                                    {pages.length} 页 ·{" "}
                                    {complete
                                      ? "已完成配准与切题"
                                      : summary ? `就绪 ${summary.ready_pages} · 阻断 ${summary.blocked_pages} · 处理中 ${summary.pending_pages}` : "等待配准或人工确认"}
                                  </span>
                                </div>
                                <Space wrap>
                                  {summary?.blockers
                                    .filter((item) => item.registration_run_id && item.stage === "registration")
                                    .map((item) => (
                                      <Space key={item.page_id} wrap>
                                        {["confirm_registration", "retry_registration"].includes(item.action) && (
                                          <Button
                                            type={item.action === "confirm_registration" ? "primary" : "default"}
                                            loading={actioning}
                                            disabled={!batchCanManage}
                                            onClick={() =>
                                              void resolveRegistration(
                                                item.registration_run_id!,
                                                item.action === "confirm_registration" ? "confirm" : "retry",
                                              )
                                            }
                                          >
                                            {item.action === "confirm_registration"
                                              ? `确认第 ${item.page_no} 页`
                                              : `重试第 ${item.page_no} 页`}
                                          </Button>
                                        )}
                                        <Button
                                          icon={<SlidersHorizontal size={15} />}
                                          disabled={!batchCanManage}
                                          onClick={() => setCorrectionRunId(item.registration_run_id)}
                                        >
                                          校正第 {item.page_no} 页边界
                                        </Button>
                                      </Space>
                                    ))}
                                  <Button icon={<Workflow size={16} />} loading={actioning} disabled={!batchCanManage || complete} onClick={() => void startPageProcessing(submissionId)}>{complete ? "处理完成" : "配准并切题"}</Button>
                                </Space>
                              </div>
                            );
                          })}
                        </div>
                      ) : (
                        <Empty
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                          description="拆页完成后可执行页面配准与切题"
                        />
                      ),
                    },
                    {
                      key: "matching",
                      label: `学生匹配 (${matching?.submissions.filter((item) => item.identity_status !== "matched").length ?? 0})`,
                      children: matchingLoading ? (
                        <LoadingState label="正在加载考试名册" />
                      ) : matching ? (
                        <MatchingWorkspace
                          queue={matching}
                          canManage={batchCanManage}
                          actioning={actioning}
                          onPreview={showPage}
                          onConfirmStudent={matchStudent}
                          onUnknown={markUnknown}
                          onConfirmPage={matchPage}
                          onSplit={splitSubmission}
                          onMerge={mergeSubmissions}
                        />
                      ) : (
                        <Empty description="暂无匹配数据" />
                      ),
                    },
                    {
                      key: "issues",
                      label: `处理问题 (${issuePages.length + detail.batch.failed_count})`,
                      children: issuePages.length ? (
                        <Table
                          rowKey="id"
                          columns={pageColumns}
                          dataSource={issuePages}
                          pagination={false}
                          size="small"
                          scroll={{ x: 700 }}
                        />
                      ) : (
                        <Empty
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                          description="当前没有需要人工处理的页面"
                        />
                      ),
                    },
                  ]}
                />
              </>
            )}
          </main>
        </div>
      )}
      <Modal
        title="新建采集批次"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void submitBatch()}
        confirmLoading={creating}
        okText="创建批次"
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ source_type: "web_upload" }}
        >
          <Form.Item
            name="name"
            label="批次名称"
            rules={[
              { required: true, message: "请输入批次名称" },
              { max: 120 },
            ]}
          >
            <Input placeholder="例如：期末考试第一扫描批次" />
          </Form.Item>
          <Form.Item
            name="source_type"
            label="采集方式"
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { value: "web_upload", label: "网页文件导入" },
                { value: "scanner_upload", label: "扫描工作站" },
                { value: "folder_import", label: "文件夹导入" },
                { value: "desktop_sync", label: "离线工作站同步" },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title={preview ? `页面 ${preview.page.sequence_no}` : "页面预览"}
        open={Boolean(preview)}
        onCancel={() => {
          if (preview?.url) URL.revokeObjectURL(preview.url);
          setPreview(undefined);
        }}
        footer={null}
        width={900}
      >
        {preview ? (
          <img
            className="capture-page-preview"
            src={preview.url}
            alt={`采集页面 ${preview.page.sequence_no}`}
          />
        ) : null}
      </Modal>
    </div>
  );
}
