import { GradingWorkbench, type GradingWorkbenchProps } from "../features/grading/workbench/GradingWorkbench";

/**
 * Route-level composition shell for the grading workspace.
 *
 * The operational state, task queue, preview controls and scoring panels live
 * with the grading feature so this route remains a stable permission boundary.
 */
export function GradingWorkbenchPage(props: GradingWorkbenchProps) {
  return <GradingWorkbench {...props} />;
}
