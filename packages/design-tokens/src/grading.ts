/**
 * Semantic grading colors. UI code should refer to these meanings rather than
 * choosing an ad-hoc red, amber, or blue for a score state.
 */
export const gradingColors = {
  correct: "var(--eg-grading-correct)",
  partial: "var(--eg-grading-partial)",
  incorrect: "var(--eg-grading-incorrect)",
  aiSuggestion: "var(--eg-grading-ai-suggestion)",
  lowConfidence: "var(--eg-grading-low-confidence)",
  disagreement: "var(--eg-grading-disagreement)",
  humanOverride: "var(--eg-grading-human-override)"
} as const;
