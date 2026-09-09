import { describe, expect, it, vi } from "vitest";
import type { Submission } from "../../api/submissions";
import { buildSubmissionView, loadCapturePage, type CaptureQueryDependencies } from "./captureQueries";

const submission: Submission = {
  id: "submission-1",
  tenant_id: "tenant-1",
  exam_id: "exam-1",
  student_id: "student-1",
  source_type: "image_upload",
  status: "created",
  expected_page_count: 1,
  actual_page_count: 1,
  quality_status: "unchecked",
  quality_issues: [],
  collected_by: "teacher-1",
  revision: 1,
  created_at: "2026-09-07T00:00:00Z",
  pages: []
};

function dependencies(overrides: Partial<CaptureQueryDependencies> = {}): CaptureQueryDependencies {
  return {
    listSubmissions: vi.fn(async () => ({ submissions: [submission], next_cursor: "next", has_more: true })),
    listStudents: vi.fn(async () => ({ students: [{ id: "student-1", name: "测试学生" }] })),
    listSubmissionPages: vi.fn(async () => ({ pages: [] })),
    listOcrTasks: vi.fn(async () => ({ tasks: [], next_cursor: "", has_more: false })),
    listAnswerSegments: vi.fn(async () => ({ segments: [] })),
    ...overrides
  } as unknown as CaptureQueryDependencies;
}

describe("capture query boundary", () => {
  it("loads the page and its permitted student projection as one query result", async () => {
    const deps = dependencies();
    const result = await loadCapturePage("exam-1", true, deps);
    expect(result.rows.map((row) => row.submission.id)).toEqual(["submission-1"]);
    expect(result.students.map((student) => student.id)).toEqual(["student-1"]);
    expect(result).toMatchObject({ nextCursor: "next", hasMore: true });
  });

  it("keeps a usable detail projection when an optional child query fails", async () => {
    const deps = dependencies({ listOcrTasks: vi.fn(async () => { throw new Error("ocr unavailable"); }) as CaptureQueryDependencies["listOcrTasks"] });
    const result = await buildSubmissionView(submission, (error) => (error as Error).message, deps);
    expect(result.submission.id).toBe("submission-1");
    expect(result.ocrTasks).toEqual([]);
    expect(result.detailError).toBe("ocr unavailable");
  });
});
