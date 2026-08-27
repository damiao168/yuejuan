import { describe, expect, it } from "vitest";
import { canSubmitTeacherAppealRecommendation, canWorkTeacherAppeals } from "./capabilities";

const allowed = {
  experience: "teacher" as const,
  canWork: true,
  terminal: false,
  assignedTo: "teacher-1",
  currentUserId: "teacher-1",
};

describe("canSubmitTeacherAppealRecommendation", () => {
  it("allows only an assigned teacher with appeal work capability", () => {
    expect(canSubmitTeacherAppealRecommendation(allowed)).toBe(true);
  });

  it.each([
    { ...allowed, canWork: false },
    { ...allowed, terminal: true },
    { ...allowed, assignedTo: "teacher-2" },
    { ...allowed, assignedTo: null },
    { ...allowed, experience: "admin" as const },
  ])("rejects a context that cannot submit a teacher recommendation", (context) => {
    expect(canSubmitTeacherAppealRecommendation(context)).toBe(false);
  });
});

describe("canWorkTeacherAppeals", () => {
  it("requires both teacher experience and appeal:work", () => {
    expect(canWorkTeacherAppeals({ permissions: ["appeal:read", "appeal:work"] }, "teacher")).toBe(true);
    expect(canWorkTeacherAppeals({ permissions: ["appeal:read"] }, "teacher")).toBe(false);
    expect(canWorkTeacherAppeals({ permissions: ["appeal:work"] }, "admin")).toBe(false);
  });
});
