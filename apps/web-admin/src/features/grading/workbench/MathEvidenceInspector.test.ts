import { describe, expect, it } from "vitest";
import { isFormulaEvidenceSubject } from "./MathEvidenceInspector";

describe("math evidence subject boundary", () => {
  it("enables formula evidence only for calibrated quantitative subjects", () => {
    expect(isFormulaEvidenceSubject("mathematics")).toBe(true);
    expect(isFormulaEvidenceSubject("physics")).toBe(true);
    expect(isFormulaEvidenceSubject("chemistry")).toBe(true);
  });

  it.each(["chinese", "history", "ethics_politics", "geography", "english", "biology"])(
    "does not call formula evidence for %s",
    (subject) => expect(isFormulaEvidenceSubject(subject)).toBe(false)
  );
});
