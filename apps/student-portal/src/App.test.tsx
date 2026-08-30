import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

const studentUser = {
  id: "student-user-1",
  tenant_id: "tenant-1",
  tenant_code: "school-a",
  username: "student-001",
  display_name: "测试学生",
  roles: ["student"],
  permissions: ["student:grade:read"],
  data_scope: { student_id: "student-1" }
};

const releasedResult = {
  exam_id: "exam-1",
  release_id: "release-7",
  release_version: 7,
  total_score: 82,
  max_score: 100,
  questions: [{
    question_id: "question-1",
    question_no: "1",
    score: 14,
    max_score: 20
  }],
  appeal_window: {
    open: true,
    allowed_reason_codes: ["recognition_error", "rubric_disagreement"]
  }
};

interface ApiCall {
  url: URL;
  init: RequestInit;
}

type ApiHandler = (call: ApiCall) => Response | undefined | Promise<Response | undefined>;

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  window.location.hash = "/";
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" }
  });
}

function installApi(handler: ApiHandler) {
  fetchMock.mockImplementation(async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const rawURL = typeof input === "string"
      ? input
      : input instanceof URL
        ? input.toString()
        : input.url;
    const call = { url: new URL(rawURL, "http://portal.test"), init };
    if (call.url.pathname === "/api/v1/auth/me") {
      return jsonResponse({ user: studentUser });
    }
    const response = await handler(call);
    if (response) return response;
    if (call.url.pathname === "/api/v1/student/question-appeals") {
      return jsonResponse({ appeals: [] });
    }
    throw new Error(`Unexpected API request: ${init.method ?? "GET"} ${call.url.pathname}`);
  });
}

function resultRoute(call: ApiCall, result: unknown = releasedResult) {
  if (call.url.pathname === "/api/v1/student/exams/exam-1/result") {
    return jsonResponse({ result });
  }
  return undefined;
}

async function renderReleasedResult(handler: ApiHandler = (call) => resultRoute(call)) {
  window.location.hash = "/exams/exam-1";
  installApi(handler);
  render(<App />);
  await screen.findByText("本次考试成绩");
}

describe("Student Portal released-score boundaries", () => {
  it("does not show scores or grading details before a release exists", async () => {
    installApi((call) => {
      if (call.url.pathname === "/api/v1/student/exams") {
        return jsonResponse({ exams: [] });
      }
      return undefined;
    });

    render(<App />);

    await screen.findByText("学校尚未发布你的成绩");
    expect(screen.queryByText("总分")).toBeNull();
    expect(screen.queryByText("教师反馈")).toBeNull();
    expect(screen.queryByText("INTERNAL_AI_SCORE")).toBeNull();
  });

  it("shows the existing unreleased state for a direct result request", async () => {
    window.location.hash = "/exams/exam-1";
    installApi((call) => {
      if (call.url.pathname === "/api/v1/student/exams/exam-1/result") {
        return jsonResponse({ error: { code: "not_found", message: "raw release lookup failed" } }, 404);
      }
      return undefined;
    });

    render(<App />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("学校尚未发布这场考试的成绩");
    expect(alert.textContent).not.toContain("raw release lookup failed");
    expect(screen.queryByText("总分")).toBeNull();
  });

  it("renders the immutable released snapshot instead of mutable grading values", async () => {
    const resultWithInternalLiveValues = {
      ...releasedResult,
      current_grading_total: 91,
      questions: [{
        ...releasedResult.questions[0],
        current_grading_score: 19
      }]
    };

    await renderReleasedResult((call) => resultRoute(call, resultWithInternalLiveValues));

    expect(screen.getByText("已发布成绩 · 第 7 版")).toBeTruthy();
    expect(screen.getByLabelText("总分").textContent).toContain("82");
    expect(screen.getByLabelText("总分").textContent).not.toContain("91");
    expect(screen.getByRole("button", { name: /14 \/ 20 分/ }).textContent).not.toContain("19");
  });

  it("renders public question feedback without exposing internal grading metadata", async () => {
    await renderReleasedResult((call) => {
      const released = resultRoute(call);
      if (released) return released;
      if (call.url.pathname === "/api/v1/student/exams/exam-1/questions/question-1") {
        return jsonResponse({
          question: {
            ...releasedResult.questions[0],
            feedback: "请复核第二步的计算过程。",
            rubric_summary: ["列式正确"],
            ai_internal_rationale: "INTERNAL_AI_RATIONALE",
            grader_identity: "INTERNAL_GRADER_IDENTITY",
            model_confidence: "INTERNAL_MODEL_CONFIDENCE",
            hidden_rubric_notes: "INTERNAL_HIDDEN_RUBRIC"
          }
        });
      }
      if (call.url.pathname.endsWith("/annotations")) {
        return jsonResponse({
          annotations: [{
            id: "annotation-1",
            answer_segment_id: "segment-1",
            submission_page_id: "page-1",
            type: "rectangle",
            geometry: { kind: "rectangle", x: 0.1, y: 0.1, width: 0.2, height: 0.2 },
            content: "公开批注内容",
            created_at: "2026-08-30T00:00:00Z",
            updated_at: "2026-08-30T00:00:00Z",
            private_note: "INTERNAL_PRIVATE_NOTE"
          }]
        });
      }
      return undefined;
    });

    fireEvent.click(screen.getByRole("button", { name: /14 \/ 20 分/ }));

    await screen.findByText("请复核第二步的计算过程。");
    expect(screen.getByText("公开批注内容")).toBeTruthy();
    for (const internalValue of [
      "INTERNAL_AI_RATIONALE",
      "INTERNAL_GRADER_IDENTITY",
      "INTERNAL_MODEL_CONFIDENCE",
      "INTERNAL_HIDDEN_RUBRIC",
      "INTERNAL_PRIVATE_NOTE"
    ]) {
      expect(screen.queryByText(internalValue)).toBeNull();
    }
  });

  it("submits an appeal with exam, release, question, reason, and student session context", async () => {
    let appealCall: ApiCall | undefined;
    await renderReleasedResult((call) => {
      const released = resultRoute(call);
      if (released) return released;
      if (call.url.pathname === "/api/v1/student/exams/exam-1/questions/question-1") {
        return jsonResponse({ question: releasedResult.questions[0] });
      }
      if (call.url.pathname.endsWith("/annotations")) {
        return jsonResponse({ annotations: [] });
      }
      if (call.url.pathname === "/api/v1/student/exams/exam-1/question-appeals" && call.init.method === "POST") {
        appealCall = call;
        return jsonResponse({ appeal: { id: "appeal-1", status: "submitted" } });
      }
      return undefined;
    });

    fireEvent.click(screen.getByRole("button", { name: /14 \/ 20 分/ }));
    const reason = await screen.findByRole("textbox", { name: "具体说明" });
    fireEvent.change(reason, { target: { value: "第二步计算过程已正确完成，请重新核对采分点。" } });
    fireEvent.click(screen.getByRole("button", { name: "确认提交（仅一次）" }));

    await waitFor(() => expect(appealCall).toBeTruthy());
    const payload = JSON.parse(String(appealCall?.init.body)) as Record<string, unknown>;
    expect(appealCall?.url.pathname).toBe("/api/v1/student/exams/exam-1/question-appeals");
    expect(appealCall?.init.credentials).toBe("include");
    expect(payload).toEqual({
      source_release_id: "release-7",
      question_id: "question-1",
      reason_code: "recognition_error",
      reason: "第二步计算过程已正确完成，请重新核对采分点。"
    });
    expect(payload.student_id).toBeUndefined();
  });

  it("does not offer a duplicate appeal for the same release and question", async () => {
    await renderReleasedResult((call) => {
      const released = resultRoute(call);
      if (released) return released;
      if (call.url.pathname === "/api/v1/student/exams/exam-1/questions/question-1") {
        return jsonResponse({ question: releasedResult.questions[0] });
      }
      if (call.url.pathname.endsWith("/annotations")) {
        return jsonResponse({ annotations: [] });
      }
      if (call.url.pathname === "/api/v1/student/question-appeals") {
        return jsonResponse({
          appeals: [{
            id: "appeal-existing",
            exam_id: "exam-1",
            source_release_id: "release-7",
            source_release_version: 7,
            question_id: "question-1",
            question_no: "1",
            source_score: 14,
            source_max_score: 20,
            reason_code: "recognition_error",
            reason: "已有复核说明",
            status: "under_review",
            created_at: "2026-08-30T00:00:00Z",
            updated_at: "2026-08-30T00:00:00Z"
          }]
        });
      }
      return undefined;
    });

    fireEvent.click(screen.getByRole("button", { name: /14 \/ 20 分/ }));

    await screen.findByText("本题已提交过复核申请，不能重复提交。处理结果会显示在页面下方。");
    expect(screen.queryByRole("button", { name: "确认提交（仅一次）" })).toBeNull();
  });

  it("shows a safe non-blank state when the result API fails", async () => {
    window.location.hash = "/exams/exam-1";
    installApi((call) => {
      if (call.url.pathname === "/api/v1/student/exams/exam-1/result") {
        throw new Error("postgresql password=SHOULD_NOT_LEAK");
      }
      return undefined;
    });

    render(<App />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("暂时无法连接服务，请稍后重试");
    expect(alert.textContent).not.toContain("postgresql");
    expect(alert.textContent).not.toContain("SHOULD_NOT_LEAK");
    expect(screen.getByRole("button", { name: "重试" })).toBeTruthy();
  });

  it("keeps the released question row visible when question detail is unavailable", async () => {
    await renderReleasedResult((call) => {
      const released = resultRoute(call);
      if (released) return released;
      if (call.url.pathname === "/api/v1/student/exams/exam-1/questions/question-1") {
        return jsonResponse({ error: { code: "not_found", message: "answer_segment_id=INTERNAL_SEGMENT" } }, 404);
      }
      return undefined;
    });

    const questionButton = screen.getByRole("button", { name: /14 \/ 20 分/ });
    fireEvent.click(questionButton);

    await screen.findByText("学校尚未发布这场考试的成绩。");
    expect(screen.getByRole("button", { name: /14 \/ 20 分/ })).toBeTruthy();
    expect(screen.queryByText(/INTERNAL_SEGMENT/)).toBeNull();
  });
});
