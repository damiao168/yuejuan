import type { AssessmentRiskTier, AssessmentScoringMode, QuestionArchetypeCode } from "../../../api/assessment";

export function isAssessmentPolicyBlocked(
  riskTier?: AssessmentRiskTier,
  archetype?: QuestionArchetypeCode,
  scoringMode?: AssessmentScoringMode
) {
  return riskTier === "R3" && archetype === "extended_response" && scoringMode === "AI_FAST_CONFIRM";
}

export function defaultArchetypeForQuestionType(questionType: string): QuestionArchetypeCode {
  if (["single_choice", "multiple_choice", "true_false"].includes(questionType)) return "selected_response";
  if (["fill_blank"].includes(questionType)) return "exact_text";
  if (["numeric", "formula"].includes(questionType)) return "numeric_expression";
  if (["calculation"].includes(questionType)) return "structured_steps";
  if (["essay", "discussion", "coding"].includes(questionType)) return "extended_response";
  return "short_constructed";
}
