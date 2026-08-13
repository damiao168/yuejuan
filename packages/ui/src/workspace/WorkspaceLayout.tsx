import type { ReactNode } from "react";

/**
 * Keeps the examination context around embedded stage modules consistent while
 * allowing each stage to own its domain UI.
 */
export function WorkspaceLayout({
  header,
  stageRail,
  children
}: {
  header: ReactNode;
  stageRail: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="eg-workspace-layout">
      {header}
      {stageRail}
      {children}
    </div>
  );
}
