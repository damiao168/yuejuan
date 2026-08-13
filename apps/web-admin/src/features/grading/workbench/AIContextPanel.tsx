import { Alert, Button, Collapse, Descriptions, Empty, Space, Tag } from "antd";
import type { ReviewTaskContext } from "../../../api/review";
import { requiresExplicitSecondOpinion, secondOpinionMetadata } from "./reviewContext";

function scalar(value: unknown): string {
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") return String(value);
  return "—";
}

function flagList(source: Record<string, unknown>): string[] {
  const candidate = source.risk_flags ?? source.risks ?? source.flags;
  return Array.isArray(candidate) ? candidate.filter((item): item is string => typeof item === "string") : [];
}

export function AIContextPanel({
  context,
  visible,
  onReveal
}: {
  context: ReviewTaskContext;
  visible: boolean;
  onReveal: () => void;
}) {
  const explicit = requiresExplicitSecondOpinion(context);
  if (explicit && !visible) {
    return (
      <section className="evidence-panel" aria-label="AI 第二意见">
        <Alert
          type="info"
          showIcon
          message="本题由教师独立评分"
          description="高风险题不会预填 AI 分数。如确有需要，可主动查看第二意见，系统会保留这一操作的明确语义。"
          action={context.ai_second_opinion?.available ? <Button onClick={onReveal}>查看第二意见</Button> : undefined}
        />
      </section>
    );
  }

  const metadata = secondOpinionMetadata(context, visible);
  const flags = metadata ? flagList(metadata) : [];
  const candidates = context.ai_candidates;
  return (
    <section className="evidence-panel" aria-label="评分候选与证据">
      <div className="panel-head">
        <div>
          <h2>{explicit ? "AI 第二意见" : "评分候选与证据"}</h2>
          <p>{candidates.length} 个候选 · {context.scoring_evidence.length} 条结构化证据</p>
        </div>
      </div>
      {metadata ? (
        <Descriptions size="small" column={1} colon={false}>
          <Descriptions.Item label="模型版本">{scalar(metadata.model_version ?? metadata.engine_version)}</Descriptions.Item>
          <Descriptions.Item label="校准状态">{scalar(metadata.calibration_status ?? metadata.status)}</Descriptions.Item>
          <Descriptions.Item label="置信度">{scalar(metadata.confidence)}</Descriptions.Item>
          <Descriptions.Item label="风险 / abstain">
            <Space wrap size={[4, 4]}>
              {flags.map((item) => <Tag color="orange" key={item}>{item}</Tag>)}
              {metadata.abstain === true ? <Tag color="red">已放弃自动判断</Tag> : null}
              {!flags.length && metadata.abstain !== true ? "无已报告风险" : null}
            </Space>
          </Descriptions.Item>
        </Descriptions>
      ) : null}
      {candidates.length ? (
        <Collapse
          ghost
          size="small"
          items={candidates.map((candidate) => ({
            key: candidate.id,
            label: `${candidate.source} · ${candidate.engine_version || "版本未知"}`,
            children: (
              <Descriptions size="small" column={1} colon={false}>
                <Descriptions.Item label="结论">{candidate.display_text || candidate.decision}</Descriptions.Item>
                <Descriptions.Item label="Profile">{candidate.profile_version || "—"}</Descriptions.Item>
                <Descriptions.Item label="置信度">{candidate.confidence ?? "—"}</Descriptions.Item>
                <Descriptions.Item label="当前候选">{candidate.is_current ? "是" : "否"}</Descriptions.Item>
                <Descriptions.Item label="证据">{Object.keys(candidate.evidence).join("、") || "无"}</Descriptions.Item>
              </Descriptions>
            )
          }))}
        />
      ) : metadata ? null : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无 AI 候选，按评分细则人工评分" />}
      {context.scoring_evidence.length ? (
        <div className="structured-evidence-list">
          {context.scoring_evidence.map((item) => (
            <Tag key={item.id}>{item.evidence_type}{typeof item.quality === "number" ? ` · ${Math.round(item.quality * 100)}%` : ""}</Tag>
          ))}
        </div>
      ) : null}
    </section>
  );
}
