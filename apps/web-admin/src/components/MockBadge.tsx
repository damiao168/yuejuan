import { Tag, Tooltip } from "antd";

export function MockBadge({ compact = false }: { compact?: boolean }) {
  return (
    <Tooltip title="框架阶段数据，后续 Story 将逐页接入真实 API">
      <Tag color="gold" className="mock-badge">
        {compact ? "MOCK" : "框架阶段 MOCK"}
      </Tag>
    </Tooltip>
  );
}
