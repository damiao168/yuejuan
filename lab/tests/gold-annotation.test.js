import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  buildGoldRecord,
  checkAgreementGate,
  computeInterRaterAgreement,
  loadAnnotationPolicy,
  validateAnnotationBundle
} from "../src/annotations/gold.js";

const bundles = readFileSync("evals/annotations/synthetic-double-label.jsonl", "utf8").split(/\r?\n/).filter(Boolean).map((line) => JSON.parse(line));

test("synthetic double labels validate and disputed case is adjudicated", () => {
  const validations = bundles.map((bundle) => validateAnnotationBundle(bundle));
  assert.ok(validations.every((result) => result.valid));
  assert.equal(validations[1].adjudication_required, true);
  assert.equal(validations[0].adjudication_required, false);
});

test("gold record selects adjudication for disputed labels", () => {
  const gold = buildGoldRecord(bundles[1]);
  assert.equal(gold.gold_score, 1);
  assert.equal(gold.gold_provenance.adjudicated, true);
  assert.equal(gold.gold_provenance.adjudication_label_id, "adjudication-002");
});

test("bundle rejects self double-scoring", () => {
  const copy = structuredClone(bundles[0]);
  copy.labels[1].labeler_id = copy.labels[0].labeler_id;
  const result = validateAnnotationBundle(copy);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("different labelers")));
});

test("bundle rejects evidence outside the answer", () => {
  const copy = structuredClone(bundles[0]);
  copy.labels[0].point_decisions[0].evidence = ["invented evidence"];
  const result = validateAnnotationBundle(copy);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("not found")));
});

test("malformed label returns validation errors instead of throwing", () => {
  const copy = structuredClone(bundles[0]);
  delete copy.labels[1].point_decisions;
  const result = validateAnnotationBundle(copy);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("point_decisions")));
});

test("labels reject hidden model fields and malformed point entries", () => {
  const hiddenModelField = structuredClone(bundles[0]);
  hiddenModelField.labels[0].model_score = 2;
  let result = validateAnnotationBundle(hiddenModelField);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("model_score is not allowed")));

  const malformedPoint = structuredClone(bundles[0]);
  malformedPoint.labels[0].point_decisions[0] = null;
  result = validateAnnotationBundle(malformedPoint);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("must be an object")));
});

test("agreement metrics are normalized and pilot gate rejects tiny synthetic set", () => {
  const policy = loadAnnotationPolicy();
  const metrics = computeInterRaterAgreement(bundles, policy);
  assert.equal(metrics.double_scored_count, 3);
  assert.ok(metrics.qwk >= -1 && metrics.qwk <= 1);
  assert.ok(metrics.normalized_mae >= 0 && metrics.normalized_mae <= 1);
  assert.equal(checkAgreementGate(metrics, "dev", policy).passed, true);
  const pilot = checkAgreementGate(metrics, "pilot", policy);
  assert.equal(pilot.passed, false);
  assert.ok(pilot.reasons.some((reason) => reason.includes("double_scored_count")));
});
