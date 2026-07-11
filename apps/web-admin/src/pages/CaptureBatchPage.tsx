import { useCallback, useEffect, useMemo, useState } from "react";
import { App, Button, Empty, Form, Input, Modal, Progress, Select, Space, Table, Tabs, Upload, type TableColumnsType, type UploadProps } from "antd";
import { FileUp, FolderOpen, Plus, RefreshCw, RotateCw, ScanLine, Workflow } from "lucide-react";
import { ApiClientError } from "../api/client";
import { createCaptureBatch, getCaptureBatch, listCaptureBatches, processCaptureBatch, processSubmissionPages, registerCaptureFile, updateCapturePage, type CaptureBatch, type CaptureBatchDetail, type CaptureFile, type CapturePage } from "../api/capture";
import { downloadFileBlob, uploadFile } from "../api/files";
import { ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";

const statusLabels: Record<string, string> = { draft: "等待上传", uploading: "正在上传", matching: "等待匹配", processing: "正在处理", needs_review: "需要确认", ready: "处理完成", completed: "已完成", cancelled: "已取消", uploaded: "已上传", queued: "等待处理", duplicate: "重复文件", failed: "处理失败", grouped: "已组织", registration: "正在配准" };

function formatError(error: unknown) {
  if (error instanceof ApiClientError) return error.message;
  return error instanceof Error ? error.message : "操作失败，请重试";
}

function statusTone(status: string): "success" | "warning" | "danger" | "processing" | "neutral" {
  if (status === "completed" || status === "ready" || status === "grouped") return "success";
  if (status === "failed" || status === "cancelled") return "danger";
  if (status === "needs_review" || status === "duplicate") return "warning";
  if (status === "processing" || status === "queued" || status === "uploading") return "processing";
  return "neutral";
}

function batchProgress(batch: CaptureBatch) {
  if (batch.status === "completed") return 100;
  if (batch.file_count === 0) return 0;
  const processed = Math.max(0, batch.file_count - batch.failed_count);
  return batch.status === "processing" ? Math.min(85, Math.round((processed / batch.file_count) * 70)) : batch.page_count > 0 ? 70 : 25;
}

export function CaptureBatchPage({ examId, canManage }: { examId: string; canManage: boolean }) {
  const { message } = App.useApp();
  const [form] = Form.useForm<{ name: string; source_type: CaptureBatch["source_type"] }>();
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

  const loadBatches = useCallback(async () => {
    setLoading(true); setError(undefined);
    try {
      const result = await listCaptureBatches(examId);
      setBatches(result.batches);
      setSelectedId((current) => current || result.batches[0]?.id || "");
    } catch (currentError) { setError(formatError(currentError)); }
    finally { setLoading(false); }
  }, [examId]);

  const loadDetail = useCallback(async (batchId: string, quiet = false) => {
    if (!batchId) { setDetail(undefined); return; }
    if (!quiet) setDetailLoading(true);
    try {
      const result = await getCaptureBatch(batchId);
      setDetail(result);
      setBatches((current) => current.map((item) => item.id === result.batch.id ? result.batch : item));
    } catch (currentError) { if (!quiet) message.error(formatError(currentError)); }
    finally { if (!quiet) setDetailLoading(false); }
  }, [message]);

  useEffect(() => { void loadBatches(); }, [loadBatches]);
  useEffect(() => { void loadDetail(selectedId); }, [loadDetail, selectedId]);
  useEffect(() => {
    if (!detail || !["uploading", "processing"].includes(detail.batch.status)) return;
    const timer = window.setInterval(() => void loadDetail(detail.batch.id, true), 3000);
    return () => window.clearInterval(timer);
  }, [detail, loadDetail]);
  useEffect(() => () => { if (preview?.url) URL.revokeObjectURL(preview.url); }, [preview]);

  async function submitBatch() {
    const values = await form.validateFields();
    setCreating(true);
    try {
      const result = await createCaptureBatch(examId, { ...values, idempotency_key: crypto.randomUUID() });
      setModalOpen(false); form.resetFields(); await loadBatches(); setSelectedId(result.batch.id); message.success("采集批次已创建");
    } catch (currentError) { message.error(formatError(currentError)); }
    finally { setCreating(false); }
  }

  const uploadProps: UploadProps = {
    multiple: true, showUploadList: false, accept: ".pdf,.png,.jpg,.jpeg,.tif,.tiff",
    beforeUpload: (file) => { void uploadSource(file); return false; }
  };

  async function uploadSource(file: File) {
    if (!detail) return;
    setUploading(true);
    try {
      const uploaded = await uploadFile(file, { owner_type: "capture_batch", owner_id: detail.batch.id, exam_id: examId });
      await registerCaptureFile(detail.batch.id, uploaded.file.id, `${file.name}:${uploaded.file.hash_sha256}`);
      await loadDetail(detail.batch.id); message.success(`${file.name} 已加入批次`);
    } catch (currentError) { message.error(formatError(currentError)); }
    finally { setUploading(false); }
  }

  async function startProcessing() {
    if (!detail) return;
    setActioning(true);
    try { await processCaptureBatch(detail.batch.id); await loadDetail(detail.batch.id); message.success("文件已进入页面处理队列"); }
    catch (currentError) { message.error(formatError(currentError)); }
    finally { setActioning(false); }
  }

  async function startPageProcessing(submissionId: string) {
    setActioning(true);
    try {
      const result = await processSubmissionPages(submissionId);
      await loadDetail(detail!.batch.id);
      message.success(`已提交 ${result.runs.length} 个页面配准任务`);
    } catch (currentError) { message.error(formatError(currentError)); }
    finally { setActioning(false); }
  }

  async function showPage(page: CapturePage) {
    try {
      const blob = await downloadFileBlob(page.decoded_file_asset_id);
      if (preview?.url) URL.revokeObjectURL(preview.url);
      setPreview({ url: URL.createObjectURL(blob.blob), page });
    } catch (currentError) { message.error(formatError(currentError)); }
  }

  async function rotatePage(page: CapturePage) {
    try {
      await updateCapturePage(page.id, { revision: page.revision, rotation_degrees: (page.rotation_degrees + 90) % 360 });
      await loadDetail(page.capture_batch_id); message.success("页面旋转已保存，后续处理将重新校验");
    } catch (currentError) { message.error(formatError(currentError)); }
  }

  const fileColumns: TableColumnsType<CaptureFile> = [
    { title: "文件", dataIndex: "original_name", ellipsis: true },
    { title: "类型", dataIndex: "content_type", width: 150 },
    { title: "页数", dataIndex: "page_count", width: 72 },
    { title: "状态", width: 110, render: (_, item) => <StatusTag tone={statusTone(item.status)}>{statusLabels[item.status] ?? item.status}</StatusTag> },
    { title: "问题", width: 160, render: (_, item) => item.error_code || (item.status === "duplicate" ? "内容与批次内文件重复" : "-") }
  ];
  const pageColumns: TableColumnsType<CapturePage> = [
    { title: "顺序", dataIndex: "sequence_no", width: 70 },
    { title: "来源页", dataIndex: "source_index", width: 80 },
    { title: "答卷页码", dataIndex: "assigned_page_no", width: 90 },
    { title: "旋转", dataIndex: "rotation_degrees", width: 72, render: (value: number) => `${value}°` },
    { title: "状态", width: 110, render: (_, item) => <StatusTag tone={statusTone(item.status)}>{statusLabels[item.status] ?? item.status}</StatusTag> },
    { title: "操作", width: 150, render: (_, item) => <Space><Button size="small" icon={<FolderOpen size={14} />} onClick={() => void showPage(item)} aria-label="查看页面" /><Button size="small" icon={<RotateCw size={14} />} onClick={() => void rotatePage(item)} disabled={!canManage} aria-label="顺时针旋转" /></Space> }
  ];

  const summary = useMemo(() => detail ? [{ label: "文件", value: detail.batch.file_count }, { label: "页面", value: detail.batch.page_count }, { label: "答卷", value: detail.batch.submission_count }, { label: "待确认", value: detail.batch.review_count }, { label: "失败", value: detail.batch.failed_count }] : [], [detail]);
  const submissions = useMemo(() => detail ? Array.from(new Set(detail.pages.map((item) => item.submission_id).filter(Boolean))) as string[] : [], [detail]);
  const issuePages = useMemo(() => detail ? detail.pages.filter((item) => ["needs_review", "quality_rejected", "failed"].includes(item.status)) : [], [detail]);

  if (loading) return <LoadingState label="正在加载采集批次" />;
  if (error) return <ErrorState message={`采集批次加载失败：${error}`} onRetry={() => void loadBatches()} />;
  return <div className="capture-batch-page">
    <section className="page-heading"><div><h1>答卷采集批次</h1><p>批量导入扫描文件，拆分页面并处理匹配和质量问题。</p></div><Space><Button icon={<RefreshCw size={16} />} onClick={() => void loadBatches()}>刷新</Button><Button type="primary" icon={<Plus size={16} />} onClick={() => setModalOpen(true)} disabled={!canManage}>新建批次</Button></Space></section>
    {batches.length === 0 ? <Empty description="尚未创建采集批次"><Button type="primary" onClick={() => setModalOpen(true)}>创建第一个批次</Button></Empty> : <div className="capture-batch-layout">
      <aside className="capture-batch-list"><div className="capture-pane-title"><strong>批次</strong><span>{batches.length}</span></div>{batches.map((batch) => <button key={batch.id} className={batch.id === selectedId ? "capture-batch-row active" : "capture-batch-row"} onClick={() => setSelectedId(batch.id)}><span><strong>{batch.name}</strong><small>{new Date(batch.created_at).toLocaleString()}</small></span><StatusTag tone={statusTone(batch.status)}>{statusLabels[batch.status]}</StatusTag></button>)}</aside>
      <main className="capture-batch-workspace">{detailLoading || !detail ? <LoadingState label="正在读取批次" /> : <>
        <header className="capture-batch-header"><div><Space><ScanLine size={20} /><h2>{detail.batch.name}</h2><StatusTag tone={statusTone(detail.batch.status)}>{statusLabels[detail.batch.status]}</StatusTag></Space><Progress percent={batchProgress(detail.batch)} size="small" /></div><Space wrap><Upload {...uploadProps}><Button icon={<FileUp size={16} />} loading={uploading} disabled={!canManage}>导入文件</Button></Upload><Button type="primary" icon={<Workflow size={16} />} onClick={() => void startProcessing()} loading={actioning} disabled={!canManage || !detail.files.some((item) => item.status === "uploaded")}>开始处理</Button></Space></header>
        <div className="capture-summary-strip">{summary.map((item) => <div key={item.label}><span>{item.label}</span><strong>{item.value}</strong></div>)}</div>
        <Tabs items={[{ key: "files", label: `文件与页面 (${detail.files.length})`, children: <div className="capture-detail-stack"><Table rowKey="id" columns={fileColumns} dataSource={detail.files} pagination={false} size="small" scroll={{ x: 720 }} /><Table rowKey="id" columns={pageColumns} dataSource={detail.pages} pagination={{ pageSize: 20 }} size="small" scroll={{ x: 700 }} /></div> }, { key: "processing", label: `页面处理 (${submissions.length})`, children: submissions.length ? <div className="capture-processing-list">{submissions.map((submissionId, index) => { const pages = detail.pages.filter((item) => item.submission_id === submissionId); const complete = pages.every((item) => item.status === "ready"); return <div key={submissionId} className="capture-processing-row"><div><strong>答卷 {index + 1}</strong><span>{pages.length} 页 · {complete ? "已完成配准与切题" : "等待配准或人工确认"}</span></div><Button icon={<Workflow size={16} />} loading={actioning} disabled={!canManage || complete} onClick={() => void startPageProcessing(submissionId)}>{complete ? "处理完成" : "配准并切题"}</Button></div>; })}</div> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="拆页完成后可执行页面配准与切题" /> }, { key: "matching", label: "学生匹配", children: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={detail.pages.length ? "学生匹配证据与候选确认将在本批次内完成" : "页面处理完成后可进行匹配"} /> }, { key: "issues", label: `处理问题 (${issuePages.length + detail.batch.failed_count})`, children: issuePages.length ? <Table rowKey="id" columns={pageColumns} dataSource={issuePages} pagination={false} size="small" scroll={{ x: 700 }} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有需要人工处理的页面" /> }]} />
      </>}</main>
    </div>}
    <Modal title="新建采集批次" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => void submitBatch()} confirmLoading={creating} okText="创建批次"><Form form={form} layout="vertical" initialValues={{ source_type: "web_upload" }}><Form.Item name="name" label="批次名称" rules={[{ required: true, message: "请输入批次名称" }, { max: 120 }]}><Input placeholder="例如：期末考试第一扫描批次" /></Form.Item><Form.Item name="source_type" label="采集方式" rules={[{ required: true }]}><Select options={[{ value: "web_upload", label: "网页文件导入" }, { value: "scanner_upload", label: "扫描工作站" }, { value: "folder_import", label: "文件夹导入" }, { value: "desktop_sync", label: "离线工作站同步" }]} /></Form.Item></Form></Modal>
    <Modal title={preview ? `页面 ${preview.page.sequence_no}` : "页面预览"} open={Boolean(preview)} onCancel={() => { if (preview?.url) URL.revokeObjectURL(preview.url); setPreview(undefined); }} footer={null} width={900}>{preview ? <img className="capture-page-preview" src={preview.url} alt={`采集页面 ${preview.page.sequence_no}`} /> : null}</Modal>
  </div>;
}
