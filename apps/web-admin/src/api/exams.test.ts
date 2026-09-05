import { afterEach, describe, expect, it, vi } from "vitest";
import { createExamSession, type ExamSessionPayload } from "./exams";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("createExamSession", () => {
  it("uses the same stable command ID in the header and business payload", async () => {
    const commandId = "exam-session-command-123";
    const fetchMock = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
      void init;
      return new Response(JSON.stringify({
        exam_session: { id: "session-1", school_id: "school-1", grade_id: "grade-1", name: "期中", exam_type: "midterm", status: "draft", exams: [] }
      }), { status: 201, headers: { "Content-Type": "application/json" } });
    });
    vi.stubGlobal("fetch", fetchMock);
    const payload: ExamSessionPayload = {
      school_id: "school-1", grade_id: "grade-1", name: "期中", exam_type: "midterm",
      grading_mode: "ai_assisted", appeal_enabled: true, publish_policy: "after_admin_approval",
      class_ids: ["class-1"], subjects: [{
        subject: "math", total_score: 100, duration_minutes: 90,
        candidate_rule: "all_selected_classes", class_ids: [],
        sections: [{ title: "全卷", question_type: "short_answer", question_count: 10, score_per_question: 10 }]
      }]
    };

    await createExamSession(payload, commandId);

    const init = fetchMock.mock.calls[0]?.[1];
    const headers = new Headers(init?.headers);
    expect(headers.get("Idempotency-Key")).toBe(commandId);
    expect(JSON.parse(String(init?.body))).toMatchObject({ command_id: commandId, name: "期中" });
  });
});
