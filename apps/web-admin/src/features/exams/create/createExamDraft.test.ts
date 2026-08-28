import { describe, expect, it } from "vitest";
import type { ExamTemplateSubject } from "../../../api/examTemplates";
import { subjectDraftFromTemplate, subjectScore } from "./createExamDraft";

describe("exam template draft hydration", () => {
  it("copies the server template structure into an editable exam draft", () => {
    const template: ExamTemplateSubject = {
      id: "subject-1",
      subject: "math",
      total_score: 150,
      duration_minutes: 120,
      candidate_rule: "all_selected_classes",
      sort_order: 1,
      sections: [
        { id: "section-1", title: "选择题", question_type: "single_choice", question_count: 10, score_per_question: 5, sort_order: 1 },
        { id: "section-2", title: "解答题", question_type: "calculation", question_count: 10, score_per_question: 10, sort_order: 2 }
      ]
    };

    const draft = subjectDraftFromTemplate(template);

    expect(draft).toMatchObject({ subject: "math", totalScore: 150, durationMinutes: 120 });
    expect(draft.sections.map((section) => section.title)).toEqual(["选择题", "解答题"]);
    expect(subjectScore(draft)).toBe(150);
    expect(draft.sections[0].id).not.toBe(template.sections[0].id);
  });
});
