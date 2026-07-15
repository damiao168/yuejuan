import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import { Alert, App, Button, Drawer, Input, InputNumber, List, Modal, Progress, Select, Space, Spin } from "antd";
import { ChevronLeft, ChevronRight, Copy, LockKeyhole, MousePointer2, Plus, RefreshCw, Save, Trash2, ZoomIn, ZoomOut } from "lucide-react";
import { GlobalWorkerOptions, getDocument, type PDFDocumentProxy } from "pdfjs-dist";
import pdfWorker from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { ApiClientError } from "../api/client";
import {
	approveOMRCalibration,
  cloneAnswerSheetTemplate,
	createOMRCalibration,
  createAnswerSheetTemplate,
	discardOMRCalibration,
	downloadOMRCalibrationCaseImage,
	getOMRCalibration,
	labelOMRCalibrationCase,
  listAnswerSheetTemplates,
	listOMRCalibrations,
  lockAnswerSheetTemplate,
	revokeOMRCalibration,
  updateAnswerSheetTemplate,
  type AnswerSheetTemplate,
  type LayoutRegion,
	type OMRCalibrationCase,
	type OMRCalibrationDetail,
	type OMRCalibrationSession,
	type OMRCalibrationStatus,
  type OptionRegion,
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
		omr_profile: { mode: "manual_only", version: "opencv-fill-v1" },
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

function calibrationStatusLabel(status: OMRCalibrationStatus) {
	return { draft: "待标注", approved: "已批准", revoked: "已撤销", discarded: "已弃用" }[status];
}

function calibrationStatusTone(status: OMRCalibrationStatus): "warning" | "success" | "danger" | "neutral" {
	switch (status) {
		case "approved": return "success";
		case "revoked": return "danger";
		case "discarded": return "neutral";
		default: return "warning";
	}
}

function calibrationBlockerLabel(code: string) {
	return {
		sample_count_below_minimum: "合格历史样本不足 100 份",
		calibration_labels_pending: "仍有样本尚未人工标注",
		calibration_mismatch_detected: "人工标注与 OMR 结果出现不一致",
		option_coverage_incomplete: "各选项的人工样本覆盖不足 10 份",
		calibration_not_draft: "该校准已不处于可审批状态"
	}[code] ?? code;
}

function clamp(value: number, min = 0, max = 1) {
  return Math.min(max, Math.max(min, value));
}

function roundCoordinate(value: number) {
  return Number(value.toFixed(6));
}

export function AnswerSheetTemplatePage({ examId, canManage, canCalibrate = false, onExamChanged }: { examId: string; canManage: boolean; canCalibrate?: boolean; onExamChanged?: () => void }) {
  const { message, modal } = App.useApp();
  const canvasRef = useRef<HTMLDivElement>(null);
  const pdfRef = useRef<PDFDocumentProxy | undefined>(undefined);
  const imageObjectUrlRef = useRef<string | undefined>(undefined);
  const renderedObjectUrlRef = useRef<string | undefined>(undefined);
	const calibrationImageObjectUrlRef = useRef<string | undefined>(undefined);
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
	const [calibrations, setCalibrations] = useState<OMRCalibrationSession[]>([]);
	const [calibrationLoading, setCalibrationLoading] = useState(false);
	const [calibrationError, setCalibrationError] = useState<string>();
	const [calibrationDetail, setCalibrationDetail] = useState<OMRCalibrationDetail>();
	const [calibrationDrawerOpen, setCalibrationDrawerOpen] = useState(false);
	const [calibrationCaseId, setCalibrationCaseId] = useState("");
	const [calibrationQuestionId, setCalibrationQuestionId] = useState("");
	const [calibrationImageUrl, setCalibrationImageUrl] = useState<string>();
	const [calibrationImageLoading, setCalibrationImageLoading] = useState(false);
	const [calibrationBusy, setCalibrationBusy] = useState(false);
	const [calibrationAction, setCalibrationAction] = useState<"approve" | "revoke" | "discard">();
	const [calibrationActionReason, setCalibrationActionReason] = useState("");

  const selectedPaper = useMemo(() => papers.find((item) => item.id === selectedPaperId), [papers, selectedPaperId]);
  const selectedTemplate = useMemo(() => templates.find((item) => item.id === selectedTemplateId), [templates, selectedTemplateId]);
  const currentPage = layout.pages.find((item) => item.page_no === pageNo);
  const selectedRegion = currentPage?.question_regions.find((item) => item.id === selectedRegionId);
  const selectedQuestion = questions.find((item) => item.id === selectedRegion?.question_id);
  const readonly = !canManage || selectedTemplate?.status === "locked";
  const coveredQuestions = useMemo(() => new Set(layout.pages.flatMap((page) => page.question_regions.map((region) => region.question_id))), [layout]);
	const isTemplateDifference = selectedTemplate?.status === "locked" && selectedTemplate.layout.omr_profile?.mode === "template_difference";
	const calibrationQuestions = useMemo(() => questions.filter((item) => item.question_type === "single_choice" || item.question_type === "true_false"), [questions]);
	const selectedCalibrationCase = useMemo<OMRCalibrationCase | undefined>(() => calibrationDetail?.cases.find((item) => item.id === calibrationCaseId) ?? calibrationDetail?.cases[0], [calibrationCaseId, calibrationDetail]);

	const loadOMRCalibrationList = useCallback(async (templateId: string) => {
		setCalibrationLoading(true);
		setCalibrationError(undefined);
		try {
			const response = await listOMRCalibrations(templateId);
			setCalibrations(response.calibrations);
		} catch (loadError) {
			setCalibrations([]);
			setCalibrationError(formatError(loadError));
		} finally {
			setCalibrationLoading(false);
		}
	}, []);

	function showCalibrationDetail(detail: OMRCalibrationDetail) {
		setCalibrationDetail(detail);
		setCalibrationDrawerOpen(true);
		setCalibrationCaseId((current) => {
			if (detail.cases.some((item) => item.id === current)) return current;
			return detail.cases.find((item) => !item.expected_options)?.id ?? detail.cases[0]?.id ?? "";
		});
	}

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
		if (calibrationQuestions.some((item) => item.id === calibrationQuestionId)) return;
		setCalibrationQuestionId(calibrationQuestions.find((item) => item.id === selectedQuestionId)?.id ?? calibrationQuestions[0]?.id ?? "");
	}, [calibrationQuestionId, calibrationQuestions, selectedQuestionId]);

	useEffect(() => {
		setCalibrationDetail(undefined);
		setCalibrationDrawerOpen(false);
		setCalibrationCaseId("");
		if (!selectedTemplate?.id || !isTemplateDifference || !canCalibrate) {
			setCalibrations([]);
			setCalibrationError(undefined);
			return;
		}
		void loadOMRCalibrationList(selectedTemplate.id);
	}, [canCalibrate, isTemplateDifference, loadOMRCalibrationList, selectedTemplate?.id]);

	useEffect(() => {
		let active = true;
		if (calibrationImageObjectUrlRef.current) {
			URL.revokeObjectURL(calibrationImageObjectUrlRef.current);
			calibrationImageObjectUrlRef.current = undefined;
		}
		setCalibrationImageUrl(undefined);
		if (!selectedCalibrationCase) {
			setCalibrationImageLoading(false);
			return () => { active = false; };
		}
		setCalibrationImageLoading(true);
		void downloadOMRCalibrationCaseImage(selectedCalibrationCase.answer_segment_id).then((download) => {
			if (!active) return;
			const url = URL.createObjectURL(download.blob);
			calibrationImageObjectUrlRef.current = url;
			setCalibrationImageUrl(url);
		}).catch((imageError) => {
			if (active) message.error(`无法加载校准裁图：${formatError(imageError)}`);
		}).finally(() => {
			if (active) setCalibrationImageLoading(false);
		});
		return () => { active = false; };
	}, [message, selectedCalibrationCase?.answer_segment_id]);

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
        const region: LayoutRegion = { id: crypto.randomUUID(), question_id: selectedQuestionId, label: question?.question_no || "题目", x: roundCoordinate(x), y: roundCoordinate(y), width: roundCoordinate(width), height: roundCoordinate(height), option_regions: [] };
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

  function updateOption(optionId: string, patch: Partial<OptionRegion>) {
    if (!selectedRegionId) return;
    setLayout((current) => ({ pages: current.pages.map((page) => ({
      ...page,
      question_regions: page.question_regions.map((region) => region.id === selectedRegionId
        ? { ...region, option_regions: (region.option_regions ?? []).map((option) => option.id === optionId ? { ...option, ...patch } : option) }
        : region)
    })) }));
  }

  function addOptionRegion() {
    if (!selectedRegionId || !selectedRegion) return;
    const existing = selectedRegion.option_regions ?? [];
    if (existing.length >= 12) {
      message.warning("每题最多配置 12 个选项区域");
      return;
    }
    const index = existing.length;
    const label = String.fromCharCode(65 + index);
    const width = 0.12;
    const gap = 0.04;
    const x = Math.min(0.84, 0.04 + index * (width + gap));
    const option: OptionRegion = { id: crypto.randomUUID(), label, x, y: 0.25, width, height: 0.5 };
    updateRegion(selectedRegionId, { option_regions: existing.concat(option) });
  }

  function removeOptionRegion(optionId: string) {
    if (!selectedRegionId || !selectedRegion) return;
    updateRegion(selectedRegionId, { option_regions: (selectedRegion.option_regions ?? []).filter((option) => option.id !== optionId) });
  }

	function setOMRProfile(mode: "manual_only" | "template_difference") {
		setLayout((current) => ({
			...current,
			omr_profile: mode === "template_difference"
				? { mode: "template_difference", version: "opencv-template-difference-bubble-v1" }
				: { mode: "manual_only", version: "opencv-fill-v1" }
		}));
	}

	async function openCalibration(calibrationId: string) {
		setCalibrationBusy(true);
		try {
			const response = await getOMRCalibration(calibrationId);
			showCalibrationDetail(response.calibration);
		} catch (openError) {
			message.error(formatError(openError));
		} finally {
			setCalibrationBusy(false);
		}
	}

	async function startCalibration() {
		if (!selectedTemplate || !calibrationQuestionId) {
			message.error("请选择一个单选题或判断题后再创建校准样本");
			return;
		}
		setCalibrationBusy(true);
		try {
			const response = await createOMRCalibration(selectedTemplate.id, calibrationQuestionId);
			showCalibrationDetail(response.calibration);
			await loadOMRCalibrationList(selectedTemplate.id);
			message.success("已冻结服务器抽取的校准样本；请逐份查看裁图并人工标注");
		} catch (createError) {
			message.error(formatError(createError));
		} finally {
			setCalibrationBusy(false);
		}
	}

	async function labelCalibrationCase(expectedOption: string) {
		if (!calibrationDetail || !selectedCalibrationCase || calibrationDetail.session.status !== "draft") return;
		setCalibrationBusy(true);
		try {
			const response = await labelOMRCalibrationCase(calibrationDetail.session.id, selectedCalibrationCase.id, expectedOption);
			showCalibrationDetail(response.calibration);
			setCalibrationCaseId(response.calibration.cases.find((item) => !item.expected_options)?.id ?? selectedCalibrationCase.id);
			await loadOMRCalibrationList(calibrationDetail.session.template_id);
			if (response.calibration.session.summary.mismatch_count > 0) {
				message.warning("发现人工标签与 OMR 不一致；该草稿不能获批，可保留证据后弃用并重新抽样");
			} else {
				message.success("人工标签已写入，不可覆盖");
			}
		} catch (labelError) {
			message.error(formatError(labelError));
		} finally {
			setCalibrationBusy(false);
		}
	}

	async function submitCalibrationAction() {
		if (!calibrationDetail || !calibrationAction) return;
		const reason = calibrationActionReason.trim();
		if (reason.length < 10) {
			message.error("请填写至少 10 个字符的审计说明");
			return;
		}
		setCalibrationBusy(true);
		try {
			let response: { calibration: OMRCalibrationDetail };
			if (calibrationAction === "approve") {
				response = await approveOMRCalibration(calibrationDetail.session.id, reason);
			} else if (calibrationAction === "revoke") {
				response = await revokeOMRCalibration(calibrationDetail.session.id, reason);
			} else {
				response = await discardOMRCalibration(calibrationDetail.session.id, reason);
			}
			showCalibrationDetail(response.calibration);
			await loadOMRCalibrationList(calibrationDetail.session.template_id);
			setCalibrationAction(undefined);
			message.success(calibrationAction === "approve" ? "校准已批准，后续符合范围的 OMR 任务可自动确认" : calibrationAction === "revoke" ? "校准已撤销，已排队任务将在回调时转人工复核" : "校准草稿已弃用，原始证据将保留审计记录");
		} catch (actionError) {
			message.error(formatError(actionError));
		} finally {
			setCalibrationBusy(false);
		}
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
	const selectedCalibrationIndex = calibrationDetail && selectedCalibrationCase ? calibrationDetail.cases.findIndex((item) => item.id === selectedCalibrationCase.id) : -1;
	const calibrationProgress = calibrationDetail ? Math.round((calibrationDetail.session.summary.labeled_count / Math.max(calibrationDetail.session.summary.total_count, 1)) * 100) : 0;

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
                {currentPage?.question_regions.map((region) => <div key={region.id} className={selectedRegionId === region.id ? "template-region selected" : "template-region"} style={{ left: `${region.x * 100}%`, top: `${region.y * 100}%`, width: `${region.width * 100}%`, height: `${region.height * 100}%` }} onPointerDown={(event) => beginRegionInteraction(event, region, "move")}><span>{region.label}</span>{(region.option_regions ?? []).map((option) => <i key={option.id} className="template-option-region" title={`选项 ${option.label}`} style={{ left: `${option.x * 100}%`, top: `${option.y * 100}%`, width: `${option.width * 100}%`, height: `${option.height * 100}%` }}>{option.label}</i>)}{!readonly ? <button className="template-resize-handle" onPointerDown={(event) => beginRegionInteraction(event, region, "resize")} aria-label="调整区域大小" /> : null}</div>)}
                {draftRect ? <div className="template-region drawing" style={{ left: `${draftRect.x * 100}%`, top: `${draftRect.y * 100}%`, width: `${draftRect.width * 100}%`, height: `${draftRect.height * 100}%` }} /> : null}
              </div>
            </div>
          </main>

          <aside className="template-inspector">
            <div className="template-pane-head"><strong>模板属性</strong>{selectedTemplate.status === "locked" ? <StatusTag tone="success">已锁定</StatusTag> : <StatusTag tone="processing">草稿</StatusTag>}</div>
            <label><span>模板名称</span><Input value={name} disabled={readonly} onChange={(event) => setName(event.target.value)} /></label>
            <div className="template-help"><MousePointer2 size={18} /><p>选择左侧题目，在页面空白处拖拽创建区域。拖动区域可移动，右下角控制点可调整大小。</p></div>
            {selectedRegion && ["single_choice", "multiple_choice", "true_false"].includes(selectedQuestion?.question_type ?? "") ? <section className="template-option-editor">
              <div className="template-pane-head"><strong>选项标记区域</strong><Button size="small" icon={<Plus size={14} />} disabled={readonly} onClick={addOptionRegion}>添加</Button></div>
              <p>坐标相对当前题目裁图。请让每个框只覆盖一个填涂位置。</p>
              {(selectedRegion.option_regions ?? []).map((option) => <div className="template-option-row" key={option.id}>
                <Input aria-label="选项标签" value={option.label} disabled={readonly} maxLength={16} onChange={(event) => updateOption(option.id, { label: event.target.value.toUpperCase() })} />
                {(["x", "y", "width", "height"] as const).map((key) => <label key={key}><span>{key}</span><InputNumber aria-label={`选项 ${option.label} ${key}`} min={0} max={1} step={0.01} precision={3} disabled={readonly} value={option[key]} onChange={(value) => updateOption(option.id, { [key]: Number(value ?? 0) })} /></label>)}
                <Button danger type="text" icon={<Trash2 size={14} />} aria-label={`删除选项 ${option.label}`} disabled={readonly} onClick={() => removeOptionRegion(option.id)} />
              </div>)}
              {(selectedRegion.option_regions ?? []).length === 0 ? <Alert type="warning" showIcon message="尚未配置选项框" description="此题不能进入自动 OMR，将转人工处理。" /> : null}
            </section> : null}
			{selectedTemplate ? <section className="template-option-editor">
				<div className="template-pane-head"><strong>选择题识别方式</strong><StatusTag tone="warning">人工复核</StatusTag></div>
				<Select
					value={layout.omr_profile?.mode ?? "manual_only"}
					disabled={readonly}
					onChange={setOMRProfile}
					options={[
						{ value: "manual_only", label: "原始填涂建议（人工复核）" },
						{ value: "template_difference", label: "空白模板差分建议（人工复核）" }
					]}
				/>
				<p>模板差分会使用当前试卷原件中冻结的同版空白页消除印刷文字和框线；在校准获批前，任何结果都不会自动写入成绩。</p>
				{layout.omr_profile?.reference ? <div className="template-help"><MousePointer2 size={18} /><p>参考资产已冻结：{layout.omr_profile.reference.file_asset_id.slice(0, 8)} · {layout.omr_profile.reference.hash_sha256.slice(0, 12)}</p></div> : layout.omr_profile?.mode === "template_difference" ? <Alert type="info" showIcon message="保存后将绑定当前试卷原件" description="锁定模板时服务端会再次校验参考文件哈希。" /> : null}
			</section> : null}
			{isTemplateDifference ? <section className="template-option-editor calibration-panel">
				<div className="template-pane-head"><strong>差分 OMR 校准</strong><StatusTag tone={calibrations.some((item) => item.status === "approved") ? "success" : "warning"}>{calibrations.some((item) => item.status === "approved") ? "有批准证据" : "仅人工复核"}</StatusTag></div>
				<p>自动确认必须基于同一模板、题目、参考资产和算法配置的固定样本。系统随机抽取历史高置信结果；人工标注时会隐藏 OMR 建议，避免确认偏差。</p>
				{!canCalibrate ? <Alert type="info" showIcon message="需要“评分管理”权限才能创建、标注或审批校准" /> : <>
					<Space.Compact block>
						<Select value={calibrationQuestionId || undefined} placeholder="选择单选题或判断题" options={calibrationQuestions.map((item) => ({ value: item.id, label: `${item.question_no} · ${item.question_type === "single_choice" ? "单选" : "判断"}` }))} onChange={setCalibrationQuestionId} />
						<Button type="primary" loading={calibrationBusy} disabled={!calibrationQuestionId} onClick={() => void startCalibration()}>创建/打开样本</Button>
					</Space.Compact>
					{calibrationError ? <Alert type="warning" showIcon message="校准状态暂不可读取" description={calibrationError} action={<Button size="small" onClick={() => selectedTemplate && void loadOMRCalibrationList(selectedTemplate.id)}>重试</Button>} /> : null}
					{calibrationLoading ? <Spin size="small" /> : calibrations.length ? <List className="calibration-session-list" size="small" dataSource={calibrations.slice(0, 4)} renderItem={(item) => <List.Item actions={[<Button key="open" type="link" size="small" loading={calibrationBusy} onClick={() => void openCalibration(item.id)}>查看</Button>]}>
						<div className="calibration-session-row"><div><strong>{questions.find((question) => question.id === item.question_id)?.question_no ?? item.question_id.slice(0, 8)}</strong><span>{item.summary.labeled_count}/{item.summary.total_count} 已标注 · {item.summary.mismatch_count} 不一致</span></div><StatusTag tone={calibrationStatusTone(item.status)}>{calibrationStatusLabel(item.status)}</StatusTag></div>
					</List.Item>} /> : <Alert type="info" showIcon message="尚无校准记录" description="收集到至少 100 份高置信差分 OMR 结果后，可创建服务器固定的人工校准样本。" />}
				</>}
			</section> : null}
            {selectedRegionId ? <Button danger icon={<Trash2 size={16} />} disabled={readonly} onClick={() => removeRegion(selectedRegionId)}>删除所选区域</Button> : null}
            <div className="template-summary"><span>页面</span><strong>{layout.pages.length}</strong><span>题目区域</span><strong>{coveredQuestions.size}</strong><span>未配置</span><strong>{Math.max(questions.length - coveredQuestions.size, 0)}</strong></div>
          </aside>
        </section>
      )}
		<Drawer title="差分 OMR 校准证据" width={760} open={calibrationDrawerOpen} onClose={() => setCalibrationDrawerOpen(false)} destroyOnClose={false}>
			{calibrationDetail ? <div className="calibration-drawer">
				<Alert type={calibrationDetail.session.status === "approved" ? "success" : calibrationDetail.session.status === "revoked" ? "error" : "info"} showIcon message={`状态：${calibrationStatusLabel(calibrationDetail.session.status)}`} description={calibrationDetail.session.status === "approved" ? "该范围的新 OMR 任务将采用已冻结的证据哈希和 98% 最低置信度。" : calibrationDetail.session.status === "revoked" ? "撤销已生效：任何携带此审批快照但尚未回调的任务都会转入人工复核。" : "人工标签写入后不可修改；发现不一致时请保留证据并弃用草稿。"} />
				<div className="calibration-metrics">
					<div><span>已标注</span><strong>{calibrationDetail.session.summary.labeled_count}/{calibrationDetail.session.summary.total_count}</strong></div>
					<div><span>一致</span><strong>{calibrationDetail.session.summary.match_count}</strong></div>
					<div><span>不一致</span><strong>{calibrationDetail.session.summary.mismatch_count}</strong></div>
					<div><span>最低置信度</span><strong>{Math.round(calibrationDetail.session.minimum_confidence * 100)}%</strong></div>
				</div>
				<Progress percent={calibrationProgress} status={calibrationDetail.session.summary.mismatch_count > 0 ? "exception" : calibrationProgress === 100 ? "success" : "active"} />
				{calibrationDetail.session.status === "draft" && calibrationDetail.session.summary.blockers.length ? <Alert type="warning" showIcon message="尚不能批准自动确认" description={<ul className="calibration-blockers">{calibrationDetail.session.summary.blockers.map((item) => <li key={item}>{calibrationBlockerLabel(item)}</li>)}</ul>} /> : null}
				<div className="calibration-coverage">{Object.entries(calibrationDetail.session.summary.option_coverage).map(([option, count]) => <span key={option}>{option}: {count}/{calibrationDetail.session.minimum_samples_per_option}</span>)}</div>
				{selectedCalibrationCase ? <section className="calibration-case">
					<div className="template-pane-head"><strong>样本 {selectedCalibrationIndex + 1}/{calibrationDetail.cases.length}</strong><Space><Button size="small" disabled={selectedCalibrationIndex <= 0 || calibrationBusy} onClick={() => setCalibrationCaseId(calibrationDetail.cases[selectedCalibrationIndex - 1].id)}>上一份</Button><Button size="small" disabled={selectedCalibrationIndex < 0 || selectedCalibrationIndex >= calibrationDetail.cases.length - 1 || calibrationBusy} onClick={() => setCalibrationCaseId(calibrationDetail.cases[selectedCalibrationIndex + 1].id)}>下一份</Button></Space></div>
					<div className="calibration-crop">{calibrationImageLoading ? <Spin tip="加载受保护裁图" /> : calibrationImageUrl ? <img src={calibrationImageUrl} alt={`校准样本 ${selectedCalibrationIndex + 1}`} /> : <Alert type="error" showIcon message="裁图加载失败" />}</div>
					{selectedCalibrationCase.expected_options?.length ? <Alert type={selectedCalibrationCase.matches ? "success" : "error"} showIcon message={`人工标签：${selectedCalibrationCase.expected_options.join(", ")}`} description={`OMR 建议：${selectedCalibrationCase.observed_options.join(", ")} · 置信度 ${Math.round(selectedCalibrationCase.observed_confidence * 100)}%${selectedCalibrationCase.matches ? " · 一致" : " · 不一致，草稿不可批准"}`} /> : <><Alert type="info" showIcon message="先基于裁图独立判断，再写入人工标签" description="在标签提交前，系统不展示 OMR 预测，避免人工判断被模型结果锚定。" /><Space wrap className="calibration-label-actions">{calibrationDetail.session.option_labels.map((option) => <Button key={option} type="primary" disabled={calibrationDetail.session.status !== "draft" || calibrationBusy} loading={calibrationBusy} onClick={() => void labelCalibrationCase(option)}>标注为 {option}</Button>)}</Space></>}
				</section> : <EmptyState title="没有可用样本" description="需要先完成服务器固定样本抽取。" />}
				<div className="calibration-actions">
					{calibrationDetail.session.status === "draft" ? <><Button type="primary" disabled={!calibrationDetail.session.summary.ready_to_approve || calibrationBusy} onClick={() => { setCalibrationActionReason(""); setCalibrationAction("approve"); }}>由独立审批人批准</Button><Button danger disabled={calibrationBusy} onClick={() => { setCalibrationActionReason(""); setCalibrationAction("discard"); }}>弃用草稿</Button></> : null}
					{calibrationDetail.session.status === "approved" ? <Button danger disabled={calibrationBusy} onClick={() => { setCalibrationActionReason(""); setCalibrationAction("revoke"); }}>立即撤销自动确认</Button> : null}
				</div>
				<div className="calibration-evidence"><span>模板哈希：{calibrationDetail.session.template_content_hash.slice(0, 16)}</span><span>运行配置：{calibrationDetail.session.profile_hash.slice(0, 16)}</span><span>参考资产：{calibrationDetail.session.reference_sha256.slice(0, 16)}</span>{calibrationDetail.session.evidence_hash ? <span>证据哈希：{calibrationDetail.session.evidence_hash.slice(0, 24)}</span> : null}</div>
			</div> : <Spin />}
		</Drawer>
		<Modal open={Boolean(calibrationAction)} title={calibrationAction === "approve" ? "批准自动确认校准" : calibrationAction === "revoke" ? "撤销自动确认校准" : "弃用校准草稿"} okText={calibrationAction === "approve" ? "批准并冻结证据" : calibrationAction === "revoke" ? "立即撤销" : "弃用草稿"} okButtonProps={{ danger: calibrationAction !== "approve" }} confirmLoading={calibrationBusy} onOk={() => void submitCalibrationAction()} onCancel={() => !calibrationBusy && setCalibrationAction(undefined)}>
			<Alert type={calibrationAction === "approve" ? "warning" : "info"} showIcon message={calibrationAction === "approve" ? "审批人与任何标注人必须不同；服务端会再次强制校验。" : calibrationAction === "revoke" ? "撤销会在下一次 OMR 回调前生效，排队任务不会再自动写入成绩。" : "弃用不会篡改已写入的人工标签，只会终止这份草稿。"} />
			<Input.TextArea autoFocus rows={4} value={calibrationActionReason} onChange={(event) => setCalibrationActionReason(event.target.value)} placeholder="填写至少 10 个字符的审计说明" />
		</Modal>
    </div>
  );
}
