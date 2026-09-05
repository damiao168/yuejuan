import type { PaperImportDraftQuestion, PaperImportJob, PaperImportRole, PaperImportSource } from "../../api/papers";

const supportedExtensions = [".pdf", ".docx", ".png", ".jpg", ".jpeg", ".tif", ".tiff"];
const supportedMimeTypes = new Set([
  "application/pdf",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "image/png",
  "image/jpeg",
  "image/tiff"
]);

export function isSupportedPaperImportFile(file: File) {
  const name = file.name.toLowerCase();
  return supportedMimeTypes.has(file.type.toLowerCase()) || supportedExtensions.some((extension) => name.endsWith(extension));
}

export function filesFromClipboard(data: Pick<DataTransfer, "files" | "items"> | null | undefined, now = new Date()) {
  if (!data) return [];
  const files: File[] = [];
  const seen = new Set<File>();
  const add = (file: File | null) => {
    if (file && !seen.has(file) && isSupportedPaperImportFile(file)) {
      const clipboardName = file.type.startsWith("image/") && (!file.name || file.name.toLowerCase() === "image.png")
        ? `clipboard-${clipboardTimestamp(now)}-${String(files.length + 1).padStart(2, "0")}.${imageExtension(file.type)}`
        : file.name;
      files.push(clipboardName === file.name ? file : new File([file], clipboardName, { type: file.type, lastModified: file.lastModified }));
      seen.add(file);
    }
  };
  const listedFiles = Array.from(data.files ?? []);
  if (listedFiles.length > 0) {
    for (const file of listedFiles) add(file);
  } else {
    for (const item of Array.from(data.items ?? [])) {
      if (item.kind === "file") add(item.getAsFile());
    }
  }
  return files;
}

export function isTextPasteTarget(target: EventTarget | null) {
  return typeof Element !== "undefined" && target instanceof Element && Boolean(target.closest("input, textarea, [contenteditable]:not([contenteditable='false']), [role='textbox']"));
}

function clipboardTimestamp(value: Date) {
  const part = (number: number) => String(number).padStart(2, "0");
  return `${value.getFullYear()}${part(value.getMonth() + 1)}${part(value.getDate())}-${part(value.getHours())}${part(value.getMinutes())}${part(value.getSeconds())}`;
}

function imageExtension(mime: string) {
  if (mime === "image/jpeg") return "jpg";
  if (mime === "image/tiff") return "tiff";
  return "png";
}

export function orderedSourcesAfterMove(sources: PaperImportSource[], sourceID: string, direction: -1 | 1) {
  const ordered = [...sources].sort((left, right) => left.document_index - right.document_index);
  const index = ordered.findIndex((source) => source.id === sourceID);
  const target = index + direction;
  if (index < 0 || target < 0 || target >= ordered.length) return ordered;
  [ordered[index], ordered[target]] = [ordered[target], ordered[index]];
  return ordered.map((source, documentIndex) => ({ ...source, document_index: documentIndex }));
}

export function orderedSourcesAfterRemoval(sources: PaperImportSource[], sourceID: string) {
  return sources
    .filter((source) => source.id !== sourceID)
    .sort((left, right) => left.document_index - right.document_index)
    .map((source, documentIndex) => ({ ...source, document_index: documentIndex }));
}

export function sourcesAfterRoleChange(sources: PaperImportSource[], sourceID: string, roleHint: PaperImportRole) {
  return sources.map((source) => (source.id === sourceID ? { ...source, role_hint: roleHint } : source));
}

export function paperImportSummary(job: PaperImportJob) {
  return {
    questions: job.question_candidates?.length ?? job.questions.length,
    answers: job.answer_candidates?.length ?? 0,
    solutions: job.solution_candidates?.length ?? 0,
    reviewIssues: (job.structured_issues ?? []).filter((issue) => issue.severity !== "info").length
  };
}

export function hasNoExamContentDetected(job: PaperImportJob) {
  return (job.structured_issues ?? []).some((issue) => issue.code === "NO_EXAM_CONTENT_DETECTED");
}

export function isPaperImportCancelled(job: PaperImportJob) {
  return job.status === "cancelled" || job.error_code === "paper_import_cancelled";
}

export function paperImportProgress(job: PaperImportJob) {
  if (job.status === "applied") return { percent: 100, label: "导入完成", detail: "题目与评分资料已写入考试" };
	if (isPaperImportCancelled(job)) return { percent: 100, label: "已停止识别", detail: "识别任务已手动停止，可重新识别" };
  if (job.status === "review_required" && hasNoExamContentDetected(job)) return { percent: 100, label: "未识别到考试内容", detail: "请检查是否上传了无关图片或错误文件" };
  if (job.status === "review_required") return { percent: 100, label: "等待人工核对", detail: "自动识别完成，请核对识别结果" };
  if (job.status === "failed") return { percent: 100, label: "识别失败", detail: job.issues[0] ?? "请检查资料后重试" };
  if (!job.sources.length) return { percent: 15, label: "准备资料", detail: "正在登记上传文件" };
  if (job.sources.some((source) => source.processing_status === "failed")) return { percent: 100, label: "文字识别失败", detail: "请检查源文件后重试" };
  if (job.sources.every((source) => source.processing_status === "processed")) return { percent: 85, label: "AI 内容解析", detail: "文字识别已完成，正在提取题目、答案与解析" };
  if (job.sources.some((source) => source.processing_status === "processing")) return { percent: 60, label: "文字识别", detail: "正在识别扫描页面中的文字" };
  return { percent: 30, label: "页面预处理", detail: "正在读取图片、校正页面并准备文字识别" };
}

export function hasBlockingImportIssues(job: PaperImportJob) {
  return job.questions.length === 0 || (job.structured_issues ?? []).some((issue) => issue.severity === "error");
}

export function markImportFieldConfirmed(draft: PaperImportDraftQuestion, field: string, patch: Partial<PaperImportDraftQuestion>) {
  return {
    ...draft,
    ...patch,
    human_confirmed_fields: Array.from(new Set([...(draft.human_confirmed_fields ?? []), field]))
  };
}
