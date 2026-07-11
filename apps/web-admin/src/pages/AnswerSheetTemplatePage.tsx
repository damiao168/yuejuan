import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import { Alert, App, Button, Input, List, Select, Space, Spin } from "antd";
import { ChevronLeft, ChevronRight, Copy, LockKeyhole, MousePointer2, Plus, RefreshCw, Save, Trash2, ZoomIn, ZoomOut } from "lucide-react";
import { GlobalWorkerOptions, getDocument, type PDFDocumentProxy } from "pdfjs-dist";
import pdfWorker from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { ApiClientError } from "../api/client";
import {
  cloneAnswerSheetTemplate,
  createAnswerSheetTemplate,
  listAnswerSheetTemplates,
  lockAnswerSheetTemplate,
  updateAnswerSheetTemplate,
  type AnswerSheetTemplate,
  type LayoutRegion,
  type TemplateLayout
} from "../api/configuration";
import { downloadFileBlob } from "../api/files";
import { listPapers, listQuestions, type PaperVersion, type Question } from "../api/papers";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";

GlobalWorkerOptions.workerSrc = pdfWorker;

type Interaction =
  | { kind: "draw"; startX: number; startY: number; x: number; y: number }
  | { kind: "move" | "resize"; regionId: string; startX: number; startY: number; original: LayoutRegion };

interface PreviewState {
  loading: boolean;
  error?: string;
  pageCount: number;
  width: number;
  height: number;
  imageUrl?: string;
}

function formatError(error: unknown) {
  if (error instanceof ApiClientError) return error.message;
  return error instanceof Error ? error.message : "操作失败，请重试";
}

function emptyLayout(pageCount: number, width: number, height: number): TemplateLayout {
  return {
    pages: Array.from({ length: pageCount }, (_, index) => ({
      page_no: index + 1,
      width,
      height,
      registration_marks: [],
      identity_regions: [],
      question_regions: []
    }))
  };
}

function clamp(value: number, min = 0, max = 1) {
  return Math.min(max, Math.max(min, value));
}

function roundCoordinate(value: number) {
  return Number(value.toFixed(6));
}

export function AnswerSheetTemplatePage({ examId, canManage, onExamChanged }: { examId: string; canManage: boolean; onExamChanged?: () => void }) {
  const { message, modal } = App.useApp();
  const canvasRef = useRef<HTMLDivElement>(null);
  const pdfRef = useRef<PDFDocumentProxy | undefined>(undefined);
  const imageObjectUrlRef = useRef<string | undefined>(undefined);
  const renderedObjectUrlRef = useRef<string | undefined>(undefined);
  const [papers, setPapers] = useState<PaperVersion[]>([]);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [templates, setTemplates] = useState<AnswerSheetTemplate[]>([]);
  const [selectedPaperId, setSelectedPaperId] = useState("");
  const [selectedTemplateId, setSelectedTemplateId] = useState("");
  const [selectedQuestionId, setSelectedQuestionId] = useState("");
  const [name, setName] = useState("答卷模板");
  const [layout, setLayout] = useState<TemplateLayout>({ pages: [] });
  const [revision, setRevision] = useState(0);
  const [pageNo, setPageNo] = useState(1);
  const [zoom, setZoom] = useState(1);
  const [interaction, setInteraction] = useState<Interaction>();
  const [selectedRegionId, setSelectedRegionId] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const [preview, setPreview] = useState<PreviewState>({ loading: false, pageCount: 1, width: 2480, height: 3508 });

  const selectedPaper = useMemo(() => papers.find((item) => item.id === selectedPaperId), [papers, selectedPaperId]);
  const selectedTemplate = useMemo(() => templates.find((item) => item.id === selectedTemplateId), [templates, selectedTemplateId]);
  const currentPage = layout.pages.find((item) => item.page_no === pageNo);
  const readonly = !canManage || selectedTemplate?.status === "locked";
  const coveredQuestions = useMemo(() => new Set(layout.pages.flatMap((page) => page.question_regions.map((region) => region.question_id))), [layout]);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const [paperResponse, questionResponse, templateResponse] = await Promise.all([listPapers(examId), listQuestions(examId), listAnswerSheetTemplates(examId)]);
      setPapers(paperResponse.papers);
      setQuestions(questionResponse.questions);
      setTemplates(templateResponse.templates);
      const latest = templateResponse.templates[0];
      setSelectedPaperId((current) => current || latest?.exam_paper_id || paperResponse.papers[0]?.id || "");
      setSelectedTemplateId((current) => current || latest?.id || "");
      setSelectedQuestionId((current) => current || questionResponse.questions[0]?.id || "");
    } catch (loadError) {
      setError(formatError(loadError));
    } finally {
      setLoading(false);
    }
  }, [examId]);

  useEffect(() => { void loadData(); }, [loadData]);

  useEffect(() => {
    if (!selectedTemplate) return;
    setName(selectedTemplate.name);
    setLayout(structuredClone(selectedTemplate.layout));
    setRevision(selectedTemplate.revision);
    setSelectedPaperId(selectedTemplate.exam_paper_id);
    setPageNo(1);
    setSelectedRegionId("");
  }, [selectedTemplate]);

  useEffect(() => {
    let active = true;
    async function loadSource() {
      if (!selectedPaper) {
        setPreview({ loading: false, pageCount: 1, width: 2480, height: 3508 });
        return;
      }
      setPreview((current) => ({ ...current, loading: true, error: undefined, imageUrl: undefined }));
      try {
        await pdfRef.current?.destroy();
        pdfRef.current = undefined;
        if (imageObjectUrlRef.current) URL.revokeObjectURL(imageObjectUrlRef.current);
        imageObjectUrlRef.current = undefined;
        const download = await downloadFileBlob(selectedPaper.file_asset_id);
        if (!active) return;
        if (download.contentType === "application/pdf" || selectedPaper.file.content_type === "application/pdf") {
          const pdfDocument = await getDocument({ data: await download.blob.arrayBuffer() }).promise;
          if (!active) { await pdfDocument.destroy(); return; }
          pdfRef.current = pdfDocument;
          setPreview((current) => ({ ...current, loading: false, pageCount: pdfDocument.numPages }));
        } else {
          const url = URL.createObjectURL(download.blob);
          imageObjectUrlRef.current = url;
          const dimensions = await new Promise<{ width: number; height: number }>((resolve, reject) => {
            const image = new Image();
            image.onload = () => resolve({ width: image.naturalWidth, height: image.naturalHeight });
            image.onerror = () => reject(new Error("无法读取答卷图片尺寸"));
            image.src = url;
          });
          if (!active) return;
          setPreview((current) => ({ ...current, loading: false, pageCount: 1, imageUrl: url, ...dimensions }));
        }
      } catch (loadError) {
        if (active) setPreview((current) => ({ ...current, loading: false, error: formatError(loadError) }));
      }
    }
    void loadSource();
    return () => { active = false; };
  }, [selectedPaper]);

  useEffect(() => {
    let active = true;
    async function renderPDFPage() {
      const pdfDocument = pdfRef.current;
      if (!pdfDocument) return;
      try {
        setPreview((current) => ({ ...current, loading: true, error: undefined }));
        const page = await pdfDocument.getPage(Math.min(pageNo, pdfDocument.numPages));
        const base = page.getViewport({ scale: 1 });
        const scale = Math.min(1.5, 1400 / base.width);
        const viewport = page.getViewport({ scale });
        const canvas = window.document.createElement("canvas");
        canvas.width = Math.ceil(viewport.width);
        canvas.height = Math.ceil(viewport.height);
        const context = canvas.getContext("2d");
        if (!context) throw new Error("浏览器无法创建 PDF 画布");
        await page.render({ canvasContext: context, viewport, canvas }).promise;
        const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/png"));
        if (!blob || !active) return;
        if (renderedObjectUrlRef.current) URL.revokeObjectURL(renderedObjectUrlRef.current);
        const url = URL.createObjectURL(blob);
        renderedObjectUrlRef.current = url;
        setPreview((current) => ({ ...current, loading: false, width: Math.round(base.width), height: Math.round(base.height), imageUrl: url }));
      } catch (renderError) {
        if (active) setPreview((current) => ({ ...current, loading: false, error: formatError(renderError) }));
      }
    }
    void renderPDFPage();
    return () => { active = false; };
  }, [pageNo, preview.pageCount]);

  useEffect(() => () => {
    void pdfRef.current?.destroy();
    if (imageObjectUrlRef.current) URL.revokeObjectURL(imageObjectUrlRef.current);
    if (renderedObjectUrlRef.current) URL.revokeObjectURL(renderedObjectUrlRef.current);
  }, []);

  function point(event: ReactPointerEvent) {
    const bounds = canvasRef.current?.getBoundingClientRect();
    if (!bounds) return { x: 0, y: 0 };
    return { x: clamp((event.clientX - bounds.left) / bounds.width), y: clamp((event.clientY - bounds.top) / bounds.height) };
  }

  function onCanvasPointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    if (readonly || !selectedQuestionId || !currentPage) return;
    const position = point(event);
    event.currentTarget.setPointerCapture(event.pointerId);
    setInteraction({ kind: "draw", startX: position.x, startY: position.y, x: position.x, y: position.y });
    setSelectedRegionId("");
  }

  function beginRegionInteraction(event: ReactPointerEvent, region: LayoutRegion, kind: "move" | "resize") {
    if (readonly) return;
    event.stopPropagation();
    const position = point(event);
    canvasRef.current?.setPointerCapture(event.pointerId);
    setSelectedRegionId(region.id);
    setInteraction({ kind, regionId: region.id, startX: position.x, startY: position.y, original: { ...region } });
  }

  function onCanvasPointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    if (!interaction || !currentPage) return;
    const position = point(event);
    if (interaction.kind === "draw") {
      setInteraction({ ...interaction, x: position.x, y: position.y });
      return;
    }
    const dx = position.x - interaction.startX;
    const dy = position.y - interaction.startY;
    updateRegion(interaction.regionId, interaction.kind === "move"
      ? { x: clamp(interaction.original.x + dx, 0, 1 - interaction.original.width), y: clamp(interaction.original.y + dy, 0, 1 - interaction.original.height) }
      : { width: clamp(interaction.original.width + dx, 0.01, 1 - interaction.original.x), height: clamp(interaction.original.height + dy, 0.01, 1 - interaction.original.y) });
  }

  function onCanvasPointerUp() {
    if (interaction?.kind === "draw" && currentPage && selectedQuestionId) {
      const x = Math.min(interaction.startX, interaction.x);
      const y = Math.min(interaction.startY, interaction.y);
      const width = Math.abs(interaction.x - interaction.startX);
      const height = Math.abs(interaction.y - interaction.startY);
      if (width >= 0.01 && height >= 0.01) {
        const question = questions.find((item) => item.id === selectedQuestionId);
        const region: LayoutRegion = { id: crypto.randomUUID(), question_id: selectedQuestionId, label: question?.question_no || "题目", x: roundCoordinate(x), y: roundCoordinate(y), width: roundCoordinate(width), height: roundCoordinate(height) };
        setLayout((current) => ({ pages: current.pages.map((page) => ({ ...page, question_regions: page.question_regions.filter((item) => item.question_id !== selectedQuestionId).concat(page.page_no === pageNo ? [region] : []) })) }));
        setSelectedRegionId(region.id);
      }
    }
    setInteraction(undefined);
  }

  function updateRegion(regionId: string, patch: Partial<LayoutRegion>) {
    setLayout((current) => ({ pages: current.pages.map((page) => ({ ...page, question_regions: page.question_regions.map((region) => region.id === regionId ? { ...region, ...patch } : region) })) }));
  }

  function removeRegion(regionId: string) {
    setLayout((current) => ({ pages: current.pages.map((page) => ({ ...page, question_regions: page.question_regions.filter((region) => region.id !== regionId) })) }));
    setSelectedRegionId("");
  }

  async function createDraft() {
    if (!selectedPaper) { message.error("请先上传并选择试卷版本"); return; }
    const pageCount = preview.pageCount || 1;
    setSaving(true);
    try {
      const response = await createAnswerSheetTemplate(examId, { exam_paper_id: selectedPaper.id, name: `${selectedPaper.file.original_name || "试卷"}答卷模板`, page_count: pageCount, layout: emptyLayout(pageCount, preview.width, preview.height) });
      setTemplates((current) => [response.template, ...current]);
      setSelectedTemplateId(response.template.id);
      onExamChanged?.();
      message.success("模板草稿已创建，可以开始框选题目区域");
    } catch (createError) { message.error(formatError(createError)); } finally { setSaving(false); }
  }

  async function saveDraft() {
    if (!selectedTemplate || readonly) return;
    setSaving(true);
    try {
      const response = await updateAnswerSheetTemplate(selectedTemplate.id, { exam_paper_id: selectedTemplate.exam_paper_id, name, page_count: layout.pages.length, layout }, revision);
      setTemplates((current) => current.map((item) => item.id === response.template.id ? response.template : item));
      setRevision(response.template.revision);
      onExamChanged?.();
      message.success("模板已保存");
    } catch (saveError) {
      message.error(formatError(saveError));
      if (saveError instanceof ApiClientError && saveError.status === 409) void loadData();
    } finally { setSaving(false); }
  }

  function confirmLock() {
    if (!selectedTemplate || readonly) return;
    const missing = questions.filter((question) => !coveredQuestions.has(question.id));
    if (missing.length) { message.error(`还有 ${missing.length} 道题未配置区域`); return; }
    modal.confirm({ title: "锁定答卷模板", content: "锁定后不能原地修改；如需调整必须克隆新版本。", okText: "确认锁定", cancelText: "取消", onOk: async () => {
      const response = await lockAnswerSheetTemplate(selectedTemplate.id);
      setTemplates((current) => current.map((item) => item.id === response.template.id ? response.template : item));
      onExamChanged?.();
      message.success("模板已锁定");
    }});
  }

  async function cloneTemplate() {
    if (!selectedTemplate) return;
    setSaving(true);
    try {
      const response = await cloneAnswerSheetTemplate(selectedTemplate.id);
      setTemplates((current) => [response.template, ...current]);
      setSelectedTemplateId(response.template.id);
      onExamChanged?.();
      message.success("已创建可编辑的新版本");
    } catch (cloneError) { message.error(formatError(cloneError)); } finally { setSaving(false); }
  }

  if (loading) return <LoadingState label="正在加载答卷模板" />;
  if (error) return <ErrorState message={error} onRetry={() => void loadData()} />;
  if (!papers.length) return <EmptyState title="尚未上传试卷" description="先在“试卷”步骤上传 PDF 或图片，再建立答卷模板。" />;

  const draftRect = interaction?.kind === "draw" ? { x: Math.min(interaction.startX, interaction.x), y: Math.min(interaction.startY, interaction.y), width: Math.abs(interaction.x - interaction.startX), height: Math.abs(interaction.y - interaction.startY) } : undefined;

  return (
    <div className="template-editor-page">
      <section className="template-toolbar">
        <div><h2>答卷模板</h2><p>选择题目后在试卷上拖拽框选答题区域</p></div>
        <Space wrap>
          <Select value={selectedPaperId} options={papers.map((paper) => ({ value: paper.id, label: `v${paper.version_no} ${paper.file.original_name}` }))} onChange={(value) => { setSelectedPaperId(value); setSelectedTemplateId(""); setLayout({ pages: [] }); }} />
          {templates.length ? <Select value={selectedTemplateId || undefined} placeholder="选择模板版本" options={templates.map((item) => ({ value: item.id, label: `v${item.version_no} ${item.name} · ${item.status === "locked" ? "已锁定" : "草稿"}` }))} onChange={setSelectedTemplateId} /> : null}
          <Button icon={<RefreshCw size={16} />} onClick={() => void loadData()}>刷新</Button>
          {!selectedTemplate ? <Button type="primary" icon={<Plus size={16} />} loading={saving} onClick={() => void createDraft()}>新建模板</Button> : null}
          {selectedTemplate?.status === "locked" ? <Button icon={<Copy size={16} />} loading={saving} onClick={() => void cloneTemplate()}>克隆新版本</Button> : null}
          {selectedTemplate?.status === "draft" ? <><Button icon={<Save size={16} />} loading={saving} onClick={() => void saveDraft()}>保存</Button><Button type="primary" icon={<LockKeyhole size={16} />} disabled={!questions.length} onClick={confirmLock}>锁定模板</Button></> : null}
        </Space>
      </section>

      {!selectedTemplate ? <Alert type="info" showIcon message="先创建模板草稿" description={`系统已读取试卷，共 ${preview.pageCount} 页。创建后即可按题框选区域。`} /> : (
        <section className="template-workbench">
          <aside className="template-question-pane">
            <div className="template-pane-head"><strong>题目</strong><span>{coveredQuestions.size}/{questions.length}</span></div>
            <List dataSource={questions} locale={{ emptyText: "尚未配置题目" }} renderItem={(question) => <List.Item className={selectedQuestionId === question.id ? "template-question active" : "template-question"} onClick={() => setSelectedQuestionId(question.id)}><div><strong>{question.question_no}</strong><span>{question.score} 分</span></div><StatusTag tone={coveredQuestions.has(question.id) ? "success" : "warning"}>{coveredQuestions.has(question.id) ? "已框选" : "待框选"}</StatusTag></List.Item>} />
          </aside>

          <main className="template-canvas-column">
            <div className="template-canvas-toolbar">
              <Space><Button icon={<ChevronLeft size={15} />} disabled={pageNo <= 1} onClick={() => setPageNo((value) => value - 1)} aria-label="上一页" /><span>第 {pageNo} / {layout.pages.length} 页</span><Button icon={<ChevronRight size={15} />} disabled={pageNo >= layout.pages.length} onClick={() => setPageNo((value) => value + 1)} aria-label="下一页" /></Space>
              <Space><Button icon={<ZoomOut size={15} />} disabled={zoom <= 0.7} onClick={() => setZoom((value) => Math.max(0.7, value - 0.1))} aria-label="缩小" /><span>{Math.round(zoom * 100)}%</span><Button icon={<ZoomIn size={15} />} disabled={zoom >= 1.5} onClick={() => setZoom((value) => Math.min(1.5, value + 0.1))} aria-label="放大" /></Space>
            </div>
            <div className="template-canvas-scroll">
              <div ref={canvasRef} className={readonly ? "template-canvas readonly" : "template-canvas"} style={{ width: `${zoom * 760}px`, aspectRatio: `${currentPage?.width || preview.width} / ${currentPage?.height || preview.height}` }} onPointerDown={onCanvasPointerDown} onPointerMove={onCanvasPointerMove} onPointerUp={onCanvasPointerUp}>
                {preview.loading ? <div className="template-preview-state"><Spin /><span>正在渲染页面</span></div> : preview.error ? <div className="template-preview-state error">{preview.error}</div> : preview.imageUrl ? <img src={preview.imageUrl} alt={`试卷第 ${pageNo} 页`} draggable={false} /> : null}
                {currentPage?.question_regions.map((region) => <div key={region.id} className={selectedRegionId === region.id ? "template-region selected" : "template-region"} style={{ left: `${region.x * 100}%`, top: `${region.y * 100}%`, width: `${region.width * 100}%`, height: `${region.height * 100}%` }} onPointerDown={(event) => beginRegionInteraction(event, region, "move")}><span>{region.label}</span>{!readonly ? <button className="template-resize-handle" onPointerDown={(event) => beginRegionInteraction(event, region, "resize")} aria-label="调整区域大小" /> : null}</div>)}
                {draftRect ? <div className="template-region drawing" style={{ left: `${draftRect.x * 100}%`, top: `${draftRect.y * 100}%`, width: `${draftRect.width * 100}%`, height: `${draftRect.height * 100}%` }} /> : null}
              </div>
            </div>
          </main>

          <aside className="template-inspector">
            <div className="template-pane-head"><strong>模板属性</strong>{selectedTemplate.status === "locked" ? <StatusTag tone="success">已锁定</StatusTag> : <StatusTag tone="processing">草稿</StatusTag>}</div>
            <label><span>模板名称</span><Input value={name} disabled={readonly} onChange={(event) => setName(event.target.value)} /></label>
            <div className="template-help"><MousePointer2 size={18} /><p>选择左侧题目，在页面空白处拖拽创建区域。拖动区域可移动，右下角控制点可调整大小。</p></div>
            {selectedRegionId ? <Button danger icon={<Trash2 size={16} />} disabled={readonly} onClick={() => removeRegion(selectedRegionId)}>删除所选区域</Button> : null}
            <div className="template-summary"><span>页面</span><strong>{layout.pages.length}</strong><span>题目区域</span><strong>{coveredQuestions.size}</strong><span>未配置</span><strong>{Math.max(questions.length - coveredQuestions.size, 0)}</strong></div>
          </aside>
        </section>
      )}
    </div>
  );
}
