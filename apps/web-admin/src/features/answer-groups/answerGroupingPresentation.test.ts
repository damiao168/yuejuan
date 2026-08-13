import { describe, expect, it } from "vitest";
import type { AnswerGroup } from "@edugrade/sdk";
import { confirmBlockReason, sampleRoleLabels } from "./answerGroupingPresentation";

const base = {
  id: "g1", tenant_id: "t1", exam_id: "e1", question_id: "q1", exam_question_snapshot_id: "snap1",
  algorithm_version: "v1", representation_version: "r1", member_count: 2, representative_submission_id: "s1",
  homogeneity: 0.98, status: "sampling", minimum_sample: 2, reviewed_sample_count: 0, can_confirm: false,
  members: [], created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z"
} satisfies AnswerGroup;

describe("answer grouping presentation", () => {
  it("explains the sampling gate before confirmation", () => {
    expect(confirmBlockReason({ ...base, decision: { id: "d1", score_candidate: { score: 1 }, rubric_selection: { point: true }, sample_size: 0, minimum_sample: 2, revision: 1 } })).toContain("0 / 至少 2");
  });

  it("labels representative, boundary and outlier samples independently", () => {
    expect(sampleRoleLabels({ submission_id: "s", segment_id: "seg", similarity: 0.7, outlier_score: 0.4, representative: true, boundary: true, outlier: true, representation_hash: "hash" })).toEqual(["代表样本", "边界样本", "异常样本"]);
  });
});
