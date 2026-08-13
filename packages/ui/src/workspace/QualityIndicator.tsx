import { StatusBadge } from "./StatusBadge";

/** Makes the blocker/warning distinction scannable without hiding the counts. */
export function QualityIndicator({ blockers, warnings }: { blockers: number; warnings: number }) {
  if (blockers > 0) return <StatusBadge tone="danger">{blockers} 项阻断</StatusBadge>;
  if (warnings > 0) return <StatusBadge tone="warning">{warnings} 项需关注</StatusBadge>;
  return <StatusBadge tone="success">质量正常</StatusBadge>;
}
