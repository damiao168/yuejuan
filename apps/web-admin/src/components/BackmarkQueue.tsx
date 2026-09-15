import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App, Button, Drawer, Empty, Input, List, Space, Spin, Tag } from "antd";
import { ClipboardCheck, RefreshCw } from "lucide-react";
import { getUserErrorMessage } from "../api/client";
import {
  claimBackmarkItem,
  downloadBackmarkSegmentImage,
  getBackmarkItemContext,
  listMyBackmarkItems,
  submitBackmarkItem,
  type BackmarkGraderContext,
  type BackmarkGraderItem
} from "../api/backmark";
import { SharedScoreControl, type SharedScoreValue } from "../features/grading/SharedScoreControl";

function errorMessage(error: unknown) {
  return getUserErrorMessage(error, "回标任务暂时无法处理，请稍后重试");
}

function rubricPoints(context?: BackmarkGraderContext) {
  const values = context?.frozen_rubric.points ?? [];
  return values.flatMap((value) => {
    if (!value || typeof value !== "object") return [];
    const point = value as Record<string, unknown>;
    const id = typeof point.id === "string" ? point.id : "";
    const score = typeof point.score === "number" ? point.score : Number(point.score);
    if (!id || !Number.isFinite(score)) return [];
    return [{ id, description: typeof point.description === "string" ? point.description : "未命名采分点", score, required: Boolean(point.required) }];
  });
}

export function BackmarkQueue({ canWork }: { canWork: boolean }) {
  const { message } = App.useApp();
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<BackmarkGraderItem[]>([]);
  const [nextCursor, setNextCursor] = useState("");
  const [loading, setLoading] = useState(false);
  const [activeItem, setActiveItem] = useState<BackmarkGraderItem>();
  const [context, setContext] = useState<BackmarkGraderContext>();
  const [imageUrl, setImageUrl] = useState("");
  const [score, setScore] = useState<SharedScoreValue>({ score: null, rubricSelections: {} });
  const [comments, setComments] = useState("");
  const [actioning, setActioning] = useState(false);

  const load = useCallback(async (cursor?: string) => {
    if (!canWork) return;
    setLoading(true);
    try {
      const response = await listMyBackmarkItems({ limit: 50, cursor });
      setItems((current) => cursor ? [...current, ...response.backmark_items] : response.backmark_items);
      setNextCursor(response.has_more ? response.next_cursor : "");
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [canWork, message]);

  useEffect(() => {
    if (open) void load();
  }, [load, open]);

  useEffect(() => () => {
    if (imageUrl) URL.revokeObjectURL(imageUrl);
  }, [imageUrl]);

  const points = useMemo(() => rubricPoints(context), [context]);

  const openItem = async (item: BackmarkGraderItem) => {
    setActioning(true);
    try {
      const claimed = await claimBackmarkItem(item.id);
      const response = await getBackmarkItemContext(item.id);
      let nextImageUrl = "";
      try {
        const image = await downloadBackmarkSegmentImage(item.id);
        nextImageUrl = URL.createObjectURL(image.blob);
      } catch {
        // Text/OCR evidence remains available even if the crop is not a browser-previewable image.
      }
      if (imageUrl) URL.revokeObjectURL(imageUrl);
      setActiveItem(claimed.backmark_item);
      setContext(response.backmark_context);
      setImageUrl(nextImageUrl);
      setScore({ score: null, rubricSelections: {} });
      setComments("");
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setActioning(false);
    }
  };

  const submit = async () => {
    if (!activeItem || !context || score.score === null) return;
    setActioning(true);
    try {
      await submitBackmarkItem(activeItem.id, {
        score: score.score,
        rubric_selections: Object.entries(score.rubricSelections).map(([point_id, value]) => ({ point_id, score: value })),
        comments,
        expected_revision: context.expected_revision
      });
      message.success("回标意见已提交，原成绩不会在此处直接修改");
      if (imageUrl) URL.revokeObjectURL(imageUrl);
      setImageUrl("");
      setActiveItem(undefined);
      setContext(undefined);
      await load();
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setActioning(false);
    }
  };

  if (!canWork) return null;

  return (
    <>
      <Button icon={<ClipboardCheck size={16} />} onClick={() => setOpen(true)}>回标任务</Button>
      <Drawer
        title={activeItem ? "独立回标" : "我的回标任务"}
        width={activeItem ? "min(1180px, 96vw)" : 520}
        open={open}
        onClose={() => setOpen(false)}
        extra={<Button icon={<RefreshCw size={15} />} loading={loading} onClick={() => void load()}>刷新</Button>}
      >
        {activeItem && context ? (
          <div className="backmark-workspace">
            <Alert showIcon type="info" message="独立回标" description="本页不显示原得分、原阅卷人或历史意见。提交只形成回标候选，后续是否仲裁或重评由质量流程决定。" />
            <div className="backmark-workspace-grid">
              <section>
                <h3>{context.question.question_no} · {context.question.question_type}</h3>
                {context.question.stem ? <p>{context.question.stem}</p> : null}
                {imageUrl ? <img className="backmark-answer-image" src={imageUrl} alt="当前答题区域" /> : <Alert type="warning" showIcon message="答题图片暂不可预览" description={context.answer.raw_answer || context.answer.ocr_text ? "可依据下方文字内容独立评分。" : "请联系管理员检查答题图处理状态。"} />}
                {(context.answer.raw_answer || context.answer.ocr_text) ? <div className="backmark-answer-text"><strong>答题内容</strong><p>{context.answer.raw_answer || context.answer.ocr_text}</p></div> : null}
              </section>
              <section>
                <h3>独立评分</h3>
                <SharedScoreControl value={score} maxScore={context.question.score} rubricPoints={points} onChange={setScore} disabled={actioning} />
                <Input.TextArea value={comments} onChange={(event) => setComments(event.target.value)} rows={4} maxLength={2000} showCount placeholder="可选：记录评分依据，供后续质量流程查看" />
                <Space className="backmark-submit-actions">
                  <Button onClick={() => { setActiveItem(undefined); setContext(undefined); }}>返回队列</Button>
                  <Button type="primary" disabled={score.score === null} loading={actioning} onClick={() => void submit()}>提交独立评分</Button>
                </Space>
              </section>
            </div>
          </div>
        ) : (
          <Spin spinning={loading}>
            {items.length ? <List loadMore={nextCursor ? <div style={{ textAlign: "center", marginTop: 16 }}><Button loading={loading} onClick={() => void load(nextCursor)}>加载更多</Button></div> : undefined} dataSource={items} renderItem={(item) => <List.Item actions={[<Button key="open" type="primary" loading={actioning} onClick={() => void openItem(item)}>开始回标</Button>]}><List.Item.Meta title={`独立回标 · 满分 ${item.max_score}`} description={`任务创建于 ${new Date(item.created_at).toLocaleString("zh-CN", { hour12: false })}`} /><Tag>{item.status === "in_progress" ? "处理中" : "待处理"}</Tag></List.Item>} /> : <Empty description="暂无分配给你的回标任务" />}
          </Spin>
        )}
      </Drawer>
    </>
  );
}
