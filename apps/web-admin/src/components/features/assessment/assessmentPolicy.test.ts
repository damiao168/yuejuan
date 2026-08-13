import { describe, expect, it } from "vitest";
import { defaultArchetypeForQuestionType, isAssessmentPolicyBlocked } from "./assessmentPolicy";

describe("assessment policy", () => {
  it("blocks R3 extended responses from AI fast confirmation", () => {
    expect(isAssessmentPolicyBlocked("R3", "extended_response", "AI_FAST_CONFIRM")).toBe(true);
    expect(isAssessmentPolicyBlocked("R2", "extended_response", "AI_FAST_CONFIRM")).toBe(false);
    expect(isAssessmentPolicyBlocked("R3", "short_constructed", "AI_FAST_CONFIRM")).toBe(false);
  });

  it("maps existing question types to domain archetypes", () => {
    expect(defaultArchetypeForQuestionType("single_choice")).toBe("selected_response");
    expect(defaultArchetypeForQuestionType("calculation")).toBe("structured_steps");
    expect(defaultArchetypeForQuestionType("essay")).toBe("extended_response");
  });
});
