import type { WorkspaceStage } from "./types";
import { StatusBadge, type StatusBadgeTone } from "./StatusBadge";

function toneFor(state: WorkspaceStage["state"]): StatusBadgeTone {
  if (state === "completed") return "success";
  if (state === "current") return "processing";
  return "neutral";
}

/** Small status fact for page headers and lists; it is not a second navigator. */
export function ExamStageIndicator({ stage }: { stage?: WorkspaceStage }) {
  if (!stage) return <StatusBadge>阶段待确认</StatusBadge>;
  return <StatusBadge tone={toneFor(stage.state)}>{stage.label}</StatusBadge>;
}
