import { useEffect } from "react";
import { isInputTarget } from "../gradingWorkbench.model";
import { gradingShortcutIntent } from "../keyboardShortcuts";

export interface UseGradingKeyboardOptions {
  hasTask: boolean;
  canEditDraft: boolean;
  canSubmit: boolean;
  canReturn: boolean;
  canUndoScoreChange: boolean;
  hasAiSuggestion: boolean;
  maxScore: number;
  rubricPointCount: number;
  onSubmit: () => void;
  onSetScore: (score: number) => void;
  onToggleCriterion: (index: number) => void;
  onAdoptAi: () => void;
  onFlagException: () => void;
  onReturnTask: () => void;
  onUndoScoreChange: () => void;
  onZoom: (direction: -1 | 1) => void;
}

export function useGradingKeyboard(options: UseGradingKeyboardOptions) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const intent = gradingShortcutIntent({
        key: event.key,
        code: event.code,
        ctrlKey: event.ctrlKey,
        isInputTarget: isInputTarget(event.target),
        hasTask: options.hasTask,
        canEditDraft: options.canEditDraft,
        canSubmit: options.canSubmit,
        canReturn: options.canReturn,
        canUndoScoreChange: options.canUndoScoreChange,
        hasAiSuggestion: options.hasAiSuggestion,
        maxScore: options.maxScore,
        rubricPointCount: options.rubricPointCount
      });
      if (!intent) return;
      event.preventDefault();
      switch (intent.type) {
        case "submit":
          options.onSubmit();
          break;
        case "set_score":
          options.onSetScore(intent.score);
          break;
        case "toggle_criterion":
          options.onToggleCriterion(intent.index);
          break;
        case "adopt_ai":
          options.onAdoptAi();
          break;
        case "flag_exception":
          options.onFlagException();
          break;
        case "return_task":
          options.onReturnTask();
          break;
        case "undo_score_change":
          options.onUndoScoreChange();
          break;
        case "zoom":
          options.onZoom(intent.direction);
          break;
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [options]);
}
