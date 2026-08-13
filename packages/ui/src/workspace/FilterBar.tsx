import type { ReactNode } from "react";

/** A responsive, non-opinionated home for existing Ant Design filters. */
export function FilterBar({ children, label = "筛选条件" }: { children: ReactNode; label?: string }) {
  return <section className="eg-filter-bar" aria-label={label}>{children}</section>;
}
