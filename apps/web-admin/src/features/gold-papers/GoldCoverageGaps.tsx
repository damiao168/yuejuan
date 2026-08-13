import { useEffect, useState } from "react";
import { Alert, Button, List, Spin, Tag } from "antd";
import type { GoldCoverage } from "../../api/goldPapers";
import { getGoldCoverage } from "../../api/goldPapers";
import { listQuestions } from "../../api/papers";
import { goldCoverageGapLabel } from "./goldPaperPresentation";

export function GoldCoverageGaps({ examId }: { examId?: string }) {
  const [items, setItems] = useState<Array<{ questionNo: string; coverage: GoldCoverage }>>([]);
  const [unavailableCount, setUnavailableCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);

  const load = async () => {
    if (!examId) return;
    setLoading(true); setFailed(false);
    try {
      const questions = (await listQuestions(examId)).questions;
      const results = await Promise.all(questions.map(async (question) => {
        try { return { questionNo: question.question_no, coverage: (await getGoldCoverage(examId, question.id)).coverage }; }
        catch { return null; }
      }));
      setUnavailableCount(results.filter((item) => item === null).length);
      setItems(results.flatMap((item) => item && item.coverage.gaps.length ? [item] : []));
    } catch { setFailed(true); setUnavailableCount(0); }
    finally { setLoading(false); }
  };

  useEffect(() => { void load(); }, [examId]);
  if (!examId) return null;
  if (loading) return <Spin size="small" />;
  if (failed) return <Alert type="warning" showIcon message="Gold 覆盖暂时无法读取" action={<Button size="small" onClick={() => void load()}>重试</Button>} />;
  if (unavailableCount > 0 && !items.length) return <Alert type="warning" showIcon message={`${unavailableCount} 道题尚无冻结评估快照，无法判断 Gold 覆盖`} />;
  if (!items.length) return <Alert type="success" showIcon message="Gold 标准卷覆盖暂无阻断缺口" />;
  return (
    <Alert
      type="warning"
      showIcon
      message={`${items.length} 道题的 Gold 标准卷覆盖不完整${unavailableCount ? `；另有 ${unavailableCount} 道题无法判断` : ""}`}
      description={<List size="small" dataSource={items} renderItem={(item) => <List.Item><strong>{item.questionNo}</strong><span>{item.coverage.gaps.map((gap) => <Tag key={gap}>{goldCoverageGapLabel(gap)}</Tag>)}</span></List.Item>} />}
    />
  );
}
