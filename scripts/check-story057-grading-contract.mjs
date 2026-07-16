import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const contractRoot = join(root, "contracts", "grading-agent", "v1");
const load = (path) => JSON.parse(readFileSync(join(contractRoot, path), "utf8"));

const contract = load("contract.json");
const matrix = load("capability-matrix.json");
const request = load("fixtures/valid-request.json");
const response = load("fixtures/valid-response.json");
const invalid = load("fixtures/invalid-response-evidence-mismatch.json");

assert.equal(contract.deployment_mode, "shadow");
assert.equal(contract.final_grade_publication_allowed, false);
assert.equal(contract.student_visible, false);
assert.equal(contract.human_review_required, true);
assert.equal(matrix.final_grade_publication_allowed, false);
assert.ok(matrix.capabilities.every((item) => item.review_policy === "always"));
assert.ok(matrix.capabilities.filter((item) => ["essay", "discussion"].includes(item.question_type)).every((item) => item.delivery === "shadow_only"));

for (const field of contract.forbidden_request_fields) {
  assert.equal(Object.hasOwn(request, field), false, `request leaks forbidden field ${field}`);
}
assert.equal(request.schema_version, "grading-agent-v1");
assert.equal(request.grade_level, "junior_middle");
assert.equal(request.model_policy.mode, "shadow");
assert.equal(request.prompt_guard.student_answer_is_untrusted, true);
assert.equal(request.rubric_version, request.rubric.rubric_version);
assert.equal(request.max_score, request.rubric.max_score);
assert.equal(request.rubric.points.reduce((sum, point) => sum + point.score, 0), request.max_score);

assert.equal(response.request_id, request.request_id);
assert.equal(response.status, "suggestion");
assert.equal(response.needs_human_review, true);
assert.equal(response.mock, false);
assert.equal(response.suggested_score, response.matched_points.reduce((sum, point) => sum + point.score, 0));
assert.equal(response.rubric_version, request.rubric_version);
assert.equal(response.prompt_version, request.prompt_version);
assert.ok(response.risk_flags.every((flag) => contract.risk_flags.includes(flag)));

const evidence = new Map(response.evidence.map((item) => [item.evidence_id, item]));
for (const point of response.matched_points) {
  for (const id of point.evidence_ids) {
    assert.equal(evidence.get(id)?.rubric_point_id, point.rubric_point_id);
  }
}
assert.ok(invalid.matched_points.some((point) => point.evidence_ids.some((id) => !invalid.evidence.some((item) => item.evidence_id === id))));

console.log("STORY-057 grading-agent contract invariants passed");
