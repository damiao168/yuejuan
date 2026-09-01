import { describe, expect, it } from "vitest";
import { readinessCheckRoute } from "./ExamPreparationPage";

describe("readiness check routes", () => {
  it.each(["students", "paper", "questions", "template"])("routes %s checks to the existing exam workspace section", (section) => {
    expect(readinessCheckRoute("exam/一", section)).toBe(`/exams/exam%2F%E4%B8%80/${section}`);
  });

  it("keeps unknown backend sections inside the readiness page", () => {
    expect(readinessCheckRoute("exam-1", "unknown")).toBe("/exams/exam-1/settings");
  });
});
