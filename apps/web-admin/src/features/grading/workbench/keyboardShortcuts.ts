export type GradingShortcutIntent =
  | { type: "submit" }
  | { type: "set_score"; score: number }
  | { type: "toggle_criterion"; index: number }
  | { type: "adopt_ai" }
  | { type: "flag_exception" }
  | { type: "return_task" }
  | { type: "undo_score_change" }
  | { type: "zoom"; direction: -1 | 1 };

export interface GradingShortcutContext {
  key: string;
  code?: string;
  ctrlKey?: boolean;
  metaKey?: boolean;
  altKey?: boolean;
  isComposing?: boolean;
  repeat?: boolean;
  blocked?: boolean;
  quickSubmit?: boolean;
  isInputTarget: boolean;
  hasTask: boolean;
  canEditDraft: boolean;
  canSubmit: boolean;
  canReturn: boolean;
  canUndoScoreChange: boolean;
  hasAiSuggestion: boolean;
  maxScore: number;
  rubricPointCount: number;
}

/**
 * Maps a browser key event to a workbench action without touching React state.
 * The component owns the actual action and its confirmation/error handling.
 */
export function gradingShortcutIntent(context: GradingShortcutContext): GradingShortcutIntent | null {
  if (context.isInputTarget || context.isComposing || context.repeat || context.blocked || context.altKey) return null;

  const key = context.key.toLowerCase();
  if (key === "enter") {
    return context.canSubmit && (context.ctrlKey || context.metaKey || context.quickSubmit) ? { type: "submit" } : null;
  }

  // Keep Ctrl+Enter compatible with the older workbench, but do not hijack
  // browser shortcuts such as Ctrl+A.
  if (context.ctrlKey || context.metaKey) return null;

  if (key === "+" || context.code === "NumpadAdd") {
    return context.hasTask ? { type: "zoom", direction: 1 } : null;
  }
  if (key === "-" || context.code === "NumpadSubtract") {
    return context.hasTask ? { type: "zoom", direction: -1 } : null;
  }

  if (!context.canEditDraft) return null;

  if (/^[1-9]$/.test(key)) {
    const index = Number(key) - 1;
    if (context.rubricPointCount > 0) return index < context.rubricPointCount ? { type: "toggle_criterion", index } : null;
    const score = Number(key);
    return score <= context.maxScore ? { type: "set_score", score } : null;
  }
  if (key === "a") {
    return context.hasAiSuggestion ? { type: "adopt_ai" } : null;
  }
  if (key === "f") {
    return context.canReturn ? { type: "flag_exception" } : null;
  }
  if (key === "r") {
    return context.canReturn ? { type: "return_task" } : null;
  }
  if (key === "z") {
    return context.canUndoScoreChange ? { type: "undo_score_change" } : null;
  }
  return null;
}
