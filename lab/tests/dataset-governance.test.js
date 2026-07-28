import test from "node:test";
import assert from "node:assert/strict";
import {
  auditSplitLeakage,
  getApprovedDatasetSource,
  loadDatasetSourceRegistry,
  questionGroupKey,
  scanSensitiveText,
  splitByQuestionGroup,
  validateDatasetSourceRegistry,
  validateGovernedSamples
} from "../src/datasets/governance.js";

function sample(id, questionId, questionText = "Why?") {
  return {
    sample_id: id,
    synthetic: true,
    question_id: questionId,
    question_text: questionText,
    answer_text: "Because evidence supports it.",
    human_rationale: "Synthetic rationale.",
    rubric: { rubric_version: "rubric-v1" }
  };
}

test("source registry allows only reviewed uses", () => {
  const registry = loadDatasetSourceRegistry();
  assert.deepEqual(validateDatasetSourceRegistry(registry), { valid: true, errors: [] });
  assert.equal(getApprovedDatasetSource("lab_synthetic", "evaluation", registry).source_type, "synthetic");
  assert.throws(() => getApprovedDatasetSource("semeval_2013_task_7", "evaluation", registry), /not license-approved/);
});

test("malformed source registry entry returns validation errors instead of throwing", () => {
  const registry = loadDatasetSourceRegistry();
  registry.sources[0] = null;
  const result = validateDatasetSourceRegistry(registry);
  assert.equal(result.valid, false);
  assert.match(result.errors.join("\n"), /must be an object/);
});

test("privacy scanner detects common direct identifiers", () => {
  assert.deepEqual(scanSensitiveText("student email is learner@example.com"), ["email"]);
  assert.ok(scanSensitiveText("手机号：13812345678").includes("cn_mobile"));
  assert.ok(scanSensitiveText("姓名：张三").includes("name_label"));
  assert.ok(scanSensitiveText("Name: Alice Smith\n").includes("name_label"));
  assert.equal(scanSensitiveText("They should have a descriptive name: it explains the test.").includes("name_label"), false);
  assert.equal(scanSensitiveText("* Name: the JSON field naming convention.").includes("name_label"), false);
});

test("governance rejects duplicate ids and sensitive text", () => {
  const source = getApprovedDatasetSource("lab_synthetic", "evaluation");
  const samples = [sample("same", "q1"), sample("same", "q2", "姓名：张三")];
  const result = validateGovernedSamples(samples, source);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("duplicate sample_id")));
  assert.ok(result.errors.some((error) => error.includes("name_label")));
});

test("grouped split never leaks a question and rubric across splits", () => {
  const samples = [];
  for (let question = 0; question < 12; question += 1) {
    samples.push(sample(`s-${question}-a`, `q-${question}`), sample(`s-${question}-b`, `q-${question}`));
  }
  const result = splitByQuestionGroup(samples, { seed: "fixed" });
  assert.equal(result.group_count, 12);
  assert.deepEqual(auditSplitLeakage(result.splits), { passed: true, leaked_groups: [], unique_groups: 12 });
  assert.ok(result.splits.train.length > 0);
  assert.ok(result.splits.validation.length > 0);
  assert.ok(result.splits.test.length > 0);
});

test("grouped split is deterministic", () => {
  const samples = Array.from({ length: 10 }, (_, index) => sample(`s-${index}`, `q-${index}`));
  const first = splitByQuestionGroup(samples, { seed: "fixed" });
  const second = splitByQuestionGroup([...samples].reverse(), { seed: "fixed" });
  for (const name of ["train", "validation", "test"]) {
    assert.deepEqual(first.splits[name].map(questionGroupKey).sort(), second.splits[name].map(questionGroupKey).sort());
  }
});

test("grouped split rejects negative or non-finite ratios", () => {
  const samples = [sample("s-1", "q-1")];
  assert.throws(
    () => splitByQuestionGroup(samples, { ratios: { train: 2, validation: -0.5, test: -0.5 } }),
    /between 0 and 1/
  );
  assert.throws(
    () => splitByQuestionGroup(samples, { ratios: { train: Number.NaN, validation: 0.5, test: 0.5 } }),
    /between 0 and 1/
  );
});

test("real samples require explicit question ids and anonymized status", () => {
  const source = { source_id: "real", source_type: "human" };
  const real = { ...sample("r1", undefined), synthetic: false };
  const result = validateGovernedSamples([real], source);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("privacy_status")));
  assert.ok(result.errors.some((error) => error.includes("question_id")));
});
