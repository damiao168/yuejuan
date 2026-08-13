import type { ReactNode } from "react";

export type StatusBadgeTone = "neutral" | "info" | "success" | "warning" | "danger" | "processing";

/** Status semantics without introducing another button/input abstraction. */
export function StatusBadge({ tone = "neutral", children }: { tone?: StatusBadgeTone; children: ReactNode }) {
  return <span className={`eg-status-badge is-${tone}`}>{children}</span>;
}
