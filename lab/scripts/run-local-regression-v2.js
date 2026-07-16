#!/usr/bin/env node
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { LocalModelAdapter } from "../src/adapters/index.js";
import { getApprovedDatasetSource, validateGovernedSamples } from "../src/datasets/governance.js";
import { applyEvidenceVerification, verifyEvidence } from "../src/evidenceVerifier.js";
import { readJsonl, sampleToInput } from "../src/evaluators/runEval.js";
import { validateGradingOutput } from "../src/schemas/gradingSchema.js";

const datasetPath = "evals/regression/prompt-rubric-v2.jsonl";
const reportPath = "evals/reports/local-regression-v2.json";
const datasetText = readFileSync(datasetPath, "utf8");
const samples = readJsonl(datasetPath);
const source = getApprovedDatasetSource("lab_synthetic", "regression");
const governance = validateGovernedSamples(samples, source);
if (!governance.valid) throw new Error(`Regression governance failed: ${governance.errors.join("; ")}`);
const adapter = new LocalModelAdapter({ modelVersion: "Qwen/Qwen3-4B-GGUF:Q4_K_M" });
const results = [];
for (const sample of samples) {
  const input = sampleToInput({ ...sample, prompt_version: "prompt-local-structured-v2" });
  const started = performance.now();
  try {
    const output = applyEvidenceVerification(input, await adapter.grade(input));
    const schema = validateGradingOutput(output, input);
    const evidence = verifyEvidence(input, output);
    const feedbackLanguageValid = sample.expected_feedback_language !== "zh" || /[\u3400-\u9fff]/u.test(output.student_feedback);
    results.push({
      sample_id: sample.sample_id,
      expected_score: sample.expected_score,
      suggested_score: output.suggested_score,
      score_exact: output.suggested_score === sample.expected_score,
      schema_valid: schema.valid,
      evidence_valid: evidence.verification_passed,
      feedback_language_valid: feedbackLanguageValid,
      matched_points: output.matched_points.map((point) => point.rubric_point_id),
      missing_points: output.missing_points.map((point) => ({ rubric_point_id: point.rubric_point_id, reason: point.reason })),
      student_feedback: output.student_feedback,
      adapter_run: adapter.get_last_run_info(),
      elapsed_ms: Math.round(performance.now() - started)
    });
  } catch (error) {
    results.push({ sample_id: sample.sample_id, error_code: error.code ?? "ERROR", error: error.message, elapsed_ms: Math.round(performance.now() - started) });
  }
}
const passed = results.every((result) => result.score_exact && result.schema_valid && result.evidence_valid && result.feedback_language_valid);
const report = {
  schema_version: "local-regression-v2",
  generated_at: new Date().toISOString(),
  synthetic_only: true,
  dataset_sha256: createHash("sha256").update(datasetText).digest("hex"),
  model_version: adapter.modelVersion,
  prompt_version: "prompt-local-structured-v2",
  baseline_failures: {
    contradictory_result_baseline_score: 3,
    contradictory_result_gold_score: 1,
    chinese_feedback_baseline_language: "inconsistent"
  },
  total: results.length,
  passed_count: results.filter((result) => result.score_exact && result.schema_valid && result.evidence_valid && result.feedback_language_valid).length,
  passed,
  results
};
writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`, "utf8");
console.log(JSON.stringify(report, null, 2));
if (!passed) process.exitCode = 1;
