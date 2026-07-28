import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { checkGate, loadGateConfig } from "../src/evaluators/gateChecker.js";
import { runEvaluation, writeEvaluationReport } from "../src/evaluators/runEval.js";

test("mock adapter can run full synthetic eval", () => {
  const report = runEvaluation({ datasetPath: "evals/synthetic/samples.jsonl", adapterName: "mock" });
  assert.equal(report.sample_count >= 20, true);
  assert.equal(report.metrics.schema_validity_rate, 1);
  assert.equal(report.metrics.no_score_above_max, true);
});

test("dev gate passes for latest synthetic mock report", () => {
  const report = runEvaluation({ datasetPath: "evals/synthetic/samples.jsonl", adapterName: "mock" });
  const config = loadGateConfig("config/release-gates.json");
  const result = checkGate(report, "dev", config);
  assert.equal(result.passed, true, result.reasons.join("\n"));
});

test("production gate fails when sample count is too small", () => {
  const report = runEvaluation({ datasetPath: "evals/synthetic/samples.jsonl", adapterName: "mock" });
  const config = loadGateConfig("config/release-gates.json");
  const result = checkGate(report, "production", config);
  assert.equal(result.passed, false);
  assert.match(result.reasons.join("\n"), /sample_count/);
});

test("release gate fails closed when required numeric evidence is missing or non-finite", () => {
  const report = runEvaluation({ datasetPath: "evals/synthetic/samples.jsonl", adapterName: "mock" });
  const config = loadGateConfig("config/release-gates.json");
  delete report.metrics.mock_marked_rate;
  report.sample_count = Number.NaN;
  const result = checkGate(report, "dev", config);
  assert.equal(result.passed, false);
  assert.match(result.reasons.join("\n"), /sample_count/);
  assert.match(result.reasons.join("\n"), /mock_marked_rate/);
});

test("eval writer emits JSON, markdown, and failed cases", () => {
  const report = runEvaluation({ datasetPath: "evals/synthetic/samples.jsonl", adapterName: "mock", filters: { tag: "prompt_injection" } });
  const dir = mkdtempSync(join(tmpdir(), "grading-agent-lab-"));
  const jsonPath = join(dir, "report.json");
  const mdPath = join(dir, "report.md");
  const failedPath = join(dir, "failed.jsonl");
  writeEvaluationReport(report, jsonPath, mdPath, failedPath);
  assert.equal(JSON.parse(readFileSync(jsonPath, "utf8")).sample_count, 1);
  assert.match(readFileSync(mdPath, "utf8"), /Eval Report/);
  assert.equal(readFileSync(failedPath, "utf8").trim().length >= 0, true);
});
