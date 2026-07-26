import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { App, Button, Input, InputNumber, Space, Spin, Switch } from "antd";
import {
  ArrowLeft,
  Check,
  Eye,
  RotateCcw,
  Undo2,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { GlobalWorkerOptions, getDocument } from "pdfjs-dist";
import pdfWorker from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { ApiClientError } from "../api/client";
import {
  applyRegistrationCorrection,
  createRegistrationCorrection,
  getRegistrationCorrection,
  getRegistrationCorrectionContext,
  previewRegistrationCorrection,
  undoRegistrationCorrection,
  type NormalizedPoint,
  type RegistrationCorrection,
} from "../api/capture";
import { downloadFileBlob } from "../api/files";

GlobalWorkerOptions.workerSrc = pdfWorker;
const initialPoints: NormalizedPoint[] = [
  { x: 0, y: 0 },
  { x: 1, y: 0 },
  { x: 1, y: 1 },
  { x: 0, y: 1 },
];
const labels = ["左上", "右上", "右下", "左下"];
const clamp = (value: number) => Math.max(0, Math.min(1, value));
const errorText = (error: unknown) =>
  error instanceof ApiClientError
    ? error.message
    : error instanceof Error
      ? error.message
      : "操作失败，请重试";

async function imageURL(fileId: string, pageNo = 1) {
  const download = await downloadFileBlob(fileId);
  if (download.contentType !== "application/pdf")
    return URL.createObjectURL(download.blob);
  const pdf = await getDocument({ data: await download.blob.arrayBuffer() })
    .promise;
  try {
    const page = await pdf.getPage(Math.min(pageNo, pdf.numPages));
    const base = page.getViewport({ scale: 1 });
    const viewport = page.getViewport({
      scale: Math.min(1.5, 1400 / base.width),
    });
    const canvas = document.createElement("canvas");
    canvas.width = Math.ceil(viewport.width);
    canvas.height = Math.ceil(viewport.height);
    const context = canvas.getContext("2d");
    if (!context) throw new Error("浏览器无法创建模板画布");
    await page.render({ canvasContext: context, viewport, canvas }).promise;
    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, "image/png"),
    );
    if (!blob) throw new Error("模板页面渲染失败");
    return URL.createObjectURL(blob);
  } finally {
    await pdf.destroy();
  }
}

function PointCanvas({
  title,
  url,
  points,
  editable,
  zoom,
  onChange,
}: {
  title: string;
  url?: string;
  points: NormalizedPoint[];
  editable: boolean;
  zoom: number;
  onChange: (points: NormalizedPoint[]) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState<number>();
  function move(index: number, clientX: number, clientY: number) {
    const bounds = ref.current?.getBoundingClientRect();
    if (!bounds) return;
    const next = points.map((point) => ({ ...point }));
    next[index] = {
      x: clamp((clientX - bounds.left) / bounds.width),
      y: clamp((clientY - bounds.top) / bounds.height),
    };
    onChange(next);
  }
  function keyMove(index: number, event: KeyboardEvent<HTMLButtonElement>) {
    const delta = event.shiftKey ? 0.01 : 0.002;
    const offset =
      event.key === "ArrowLeft"
        ? [-delta, 0]
        : event.key === "ArrowRight"
          ? [delta, 0]
          : event.key === "ArrowUp"
            ? [0, -delta]
            : event.key === "ArrowDown"
              ? [0, delta]
              : undefined;
    if (!offset) return;
    event.preventDefault();
    const next = points.map((point) => ({ ...point }));
    next[index] = {
      x: clamp(next[index].x + offset[0]),
      y: clamp(next[index].y + offset[1]),
    };
    onChange(next);
  }
  return (
    <section className="correction-canvas-column">
      <header>
        <strong>{title}</strong>
        <span>{editable ? "拖动或使用方向键调整" : "模板锚点已锁定"}</span>
      </header>
      <div className="correction-canvas-scroll">
        <div
          ref={ref}
          className="correction-canvas"
          style={{ width: `${zoom * 620}px` }}
          onPointerMove={(event) =>
            dragging === undefined
              ? undefined
              : move(dragging, event.clientX, event.clientY)
          }
          onPointerUp={() => setDragging(undefined)}
        >
          {url ? (
            <img src={url} alt={title} draggable={false} />
          ) : (
            <div className="correction-loading">
              <Spin />
            </div>
          )}
          {points.map((point, index) => (
            <button
              key={labels[index]}
              className={`correction-point point-${index + 1}`}
              style={{ left: `${point.x * 100}%`, top: `${point.y * 100}%` }}
              disabled={!editable}
              aria-label={`${title}${labels[index]}点`}
              onKeyDown={(event) => keyMove(index, event)}
              onPointerDown={(event: PointerEvent<HTMLButtonElement>) => {
                if (!editable) return;
                event.currentTarget.setPointerCapture(event.pointerId);
                setDragging(index);
              }}
            >
              <span>{index + 1}</span>
            </button>
          ))}
        </div>
      </div>
    </section>
  );
}

export function RegistrationCorrectionWorkspace({
  runId,
  canManage,
  onClose,
  onChanged,
}: {
  runId: string;
  canManage: boolean;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const { message } = App.useApp();
  const [sourceURL, setSourceURL] = useState<string>();
  const [templateURL, setTemplateURL] = useState<string>();
  const [previewURL, setPreviewURL] = useState<string>();
  const [context, setContext] =
    useState<
      Awaited<ReturnType<typeof getRegistrationCorrectionContext>>["context"]
    >();
  const [sourcePoints, setSourcePoints] = useState(initialPoints);
  const [templatePoints, setTemplatePoints] = useState(initialPoints);
  const [advanced, setAdvanced] = useState(false);
  const [zoom, setZoom] = useState(0.8);
  const [correction, setCorrection] = useState<RegistrationCorrection>();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  useEffect(() => {
    let active = true;
    const urls: string[] = [];
    void (async () => {
      try {
        const result = await getRegistrationCorrectionContext(runId);
        if (!active) return;
        setContext(result.context);
        const [source, template] = await Promise.all([
          imageURL(result.context.source_file_asset_id),
          imageURL(
            result.context.template_file_asset_id,
            result.context.page_no,
          ),
        ]);
        urls.push(source, template);
        if (active) {
          setSourceURL(source);
          setTemplateURL(template);
        }
      } catch (error) {
        message.error(errorText(error));
      }
    })();
    return () => {
      active = false;
      urls.forEach(URL.revokeObjectURL);
    };
  }, [runId, message]);
  useEffect(
    () => () => {
      if (previewURL) URL.revokeObjectURL(previewURL);
    },
    [previewURL],
  );
  async function preview() {
    if (!context) return;
    setBusy(true);
    try {
      const created = await createRegistrationCorrection(runId, {
        page_revision: context.page_revision,
        source_points: sourcePoints,
        template_points: templatePoints,
        advanced_anchor_mode: advanced,
      });
      if (!mountedRef.current) return;
      let current = (
        await previewRegistrationCorrection(
          created.correction.id,
          created.correction.revision,
        )
      ).correction;
      if (!mountedRef.current) return;
      setCorrection(current);
      for (
        let i = 0;
        i < 60 && ["queued", "draft"].includes(current.status);
        i++
      ) {
        await new Promise((resolve) => setTimeout(resolve, 1500));
        if (!mountedRef.current) return;
        current = (await getRegistrationCorrection(current.id)).correction;
        if (!mountedRef.current) return;
        setCorrection(current);
      }
      if (
        current.status !== "preview_ready" ||
        !current.preview_registered_file_asset_id
      )
        throw new Error(current.error_code || "校正预览未完成");
      const url = await imageURL(current.preview_registered_file_asset_id);
      if (!mountedRef.current) {
        URL.revokeObjectURL(url);
        return;
      }
      if (previewURL) URL.revokeObjectURL(previewURL);
      setPreviewURL(url);
      message.success("校正预览已生成");
    } catch (error) {
      if (!mountedRef.current) return;
      message.error(errorText(error));
    } finally {
      setBusy(false);
    }
  }
  async function apply() {
    if (!correction || !reason.trim()) return;
    setBusy(true);
    try {
      const result = await applyRegistrationCorrection(
        correction.id,
        correction.revision,
        reason.trim(),
      );
      setCorrection(result.correction);
      setReason("");
      await onChanged();
      message.success("人工校正已应用");
    } catch (error) {
      message.error(errorText(error));
    } finally {
      setBusy(false);
    }
  }
  async function undo() {
    if (!correction || !reason.trim()) return;
    setBusy(true);
    try {
      const result = await undoRegistrationCorrection(
        correction.id,
        correction.revision,
        reason.trim(),
      );
      setCorrection(result.correction);
      setReason("");
      await onChanged();
      message.success("已恢复应用前的配准证据");
    } catch (error) {
      message.error(errorText(error));
    } finally {
      setBusy(false);
    }
  }
  function updatePoint(
    target: "source" | "template",
    index: number,
    axis: "x" | "y",
    value: number | null,
  ) {
    const setter = target === "source" ? setSourcePoints : setTemplatePoints;
    setter((points) =>
      points.map((point, current) =>
        current === index ? { ...point, [axis]: clamp(value ?? 0) } : point,
      ),
    );
  }
  return (
    <div className="correction-workspace">
      <header className="correction-workspace-head">
        <Button icon={<ArrowLeft size={16} />} onClick={onClose}>
          返回批次
        </Button>
        <div>
          <span>人工配准校正</span>
          <h1>第 {context?.page_no ?? "-"} 页边界</h1>
        </div>
        <Space>
          <Button
            icon={<ZoomOut size={16} />}
            disabled={zoom <= 0.6}
            onClick={() => setZoom((value) => Math.max(0.6, value - 0.1))}
            aria-label="缩小"
          />
          <strong>{Math.round(zoom * 100)}%</strong>
          <Button
            icon={<ZoomIn size={16} />}
            disabled={zoom >= 1.2}
            onClick={() => setZoom((value) => Math.min(1.2, value + 0.1))}
            aria-label="放大"
          />
        </Space>
      </header>
      <div className="correction-layout">
        <PointCanvas
          title="标准化答卷"
          url={sourceURL}
          points={sourcePoints}
          editable={canManage && correction?.status !== "applied"}
          zoom={zoom}
          onChange={setSourcePoints}
        />
        <PointCanvas
          title="锁定模板"
          url={templateURL}
          points={templatePoints}
          editable={canManage && advanced && correction?.status !== "applied"}
          zoom={zoom}
          onChange={setTemplatePoints}
        />
        <aside className="correction-inspector">
          <div className="correction-inspector-head">
            <strong>对应点</strong>
            <Button
              icon={<RotateCcw size={15} />}
              onClick={() => {
                setSourcePoints(initialPoints);
                setTemplatePoints(initialPoints);
              }}
              disabled={!canManage || correction?.status === "applied"}
            >
              重置
            </Button>
          </div>
          <label className="correction-mode">
            <span>高级模板锚点</span>
            <Switch
              checked={advanced}
              onChange={setAdvanced}
              disabled={!canManage || correction?.status === "applied"}
            />
          </label>
          {labels.map((label, index) => (
            <div className="correction-point-row" key={label}>
              <strong>
                <i className={`point-swatch point-${index + 1}`} />
                {index + 1} {label}
              </strong>
              <span>源</span>
              <InputNumber
                min={0}
                max={1}
                step={0.001}
                precision={3}
                value={sourcePoints[index].x}
                disabled={!canManage || correction?.status === "applied"}
                onChange={(value) => updatePoint("source", index, "x", value)}
              />
              <InputNumber
                min={0}
                max={1}
                step={0.001}
                precision={3}
                value={sourcePoints[index].y}
                disabled={!canManage || correction?.status === "applied"}
                onChange={(value) => updatePoint("source", index, "y", value)}
              />
              {advanced ? (
                <>
                  <span>模板</span>
                  <InputNumber
                    min={0}
                    max={1}
                    step={0.001}
                    precision={3}
                    value={templatePoints[index].x}
                    disabled={!canManage || correction?.status === "applied"}
                    onChange={(value) =>
                      updatePoint("template", index, "x", value)
                    }
                  />
                  <InputNumber
                    min={0}
                    max={1}
                    step={0.001}
                    precision={3}
                    value={templatePoints[index].y}
                    disabled={!canManage || correction?.status === "applied"}
                    onChange={(value) =>
                      updatePoint("template", index, "y", value)
                    }
                  />
                </>
              ) : null}
            </div>
          ))}
          <div
            className={`correction-validation ${correction?.status ?? "draft"}`}
          >
            <strong>
              {correction?.status === "preview_ready"
                ? "预览检查通过"
                : correction?.status === "applied"
                  ? "校正已应用"
                  : correction?.status === "undone"
                    ? "已恢复原证据"
                    : correction?.status === "failed"
                      ? "预览检查未通过"
                      : "等待生成预览"}
            </strong>
            {correction?.coverage ? (
              <span>页面覆盖 {Math.round(correction.coverage * 100)}%</span>
            ) : (
              <span>预览不会改变正式题区证据</span>
            )}
            {correction?.error_code ? (
              <span>{correction.error_code}</span>
            ) : null}
          </div>
          {previewURL ? (
            <div className="correction-preview">
              <span>校正结果</span>
              <img src={previewURL} alt="人工校正预览" />
            </div>
          ) : null}
          <Input.TextArea
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={2}
            placeholder="应用或撤销原因"
            maxLength={300}
            disabled={!canManage}
          />
          <Space wrap>
            <Button
              icon={<Eye size={16} />}
              onClick={() => void preview()}
              loading={busy}
              disabled={!canManage || correction?.status === "applied"}
            >
              预览校正
            </Button>
            <Button
              type="primary"
              icon={<Check size={16} />}
              onClick={() => void apply()}
              loading={busy}
              disabled={
                !canManage ||
                correction?.status !== "preview_ready" ||
                !reason.trim()
              }
            >
              应用校正
            </Button>
            <Button
              icon={<Undo2 size={16} />}
              onClick={() => void undo()}
              loading={busy}
              disabled={
                !canManage || correction?.status !== "applied" || !reason.trim()
              }
            >
              撤销应用
            </Button>
          </Space>
        </aside>
      </div>
    </div>
  );
}
