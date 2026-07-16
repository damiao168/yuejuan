#!/usr/bin/env node
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { LocalModelAdapter } from "../src/adapters/localModelAdapter.js";
import { applyEvidenceVerification, verifyEvidence } from "../src/evidenceVerifier.js";
import { LAB_ROOT } from "../src/runtime/localRuntime.js";
import { validateGradingInput, validateGradingOutput } from "../src/schemas/gradingSchema.js";

const input = {
  request_id: "local-smoke-001",
  question_id: "local-smoke-question-001",
  answer_segment_id: "local-smoke-answer-001",
  subject: "chinese",
  grade_level: "junior_middle",
  question_type: "short_answer",
  question_text: "绿色植物为什么被称为生态系统中的生产者？",
  max_score: 2,
  rubric: {
    rubric_id: "local-smoke-rubric",
    rubric_version: "local-smoke-rubric-v1",
    max_score: 2,
    points: [
      {
        id: "photosynthesis",
        description: "指出植物通过光合作用制造有机物",
        score: 1,
        required: true,
        aliases: ["通过光合作用制造有机物"],
        evidence_required: true
      },
      {
        id: "energy_conversion",
        description: "指出将太阳能转化为化学能",
        score: 1,
        required: true,
        aliases: ["将太阳能转化为化学能"],
        evidence_required: true
      }
    ],
    deductions: [],
    equivalent_answers: [],
    examples: [],
    scoring_notes: []
  },
  answer_text: "绿色植物能够通过光合作用制造有机物，并将太阳能转化为化学能。",
  ocr_confidence: 0.99,
  model_policy: { adapter: "local", final_score_allowed: false },
  prompt_version: "prompt-local-structured-v1",
  rubric_version: "local-smoke-rubric-v1"
};

const inputValidation = validateGradingInput(input);
if (!inputValidation.valid) throw new Error(`smoke input invalid: ${inputValidation.errors.join("; ")}`);
const adapter = new LocalModelAdapter();
const started = performance.now();
const rawOutput = await adapter.grade(input);
const output = applyEvidenceVerification(input, rawOutput);
const elapsedMs = Math.round(performance.now() - started);
const schemaValidation = validateGradingOutput(output, input);
const evidenceVerification = verifyEvidence(input, output);
const report = {
  schema_version: "local-adapter-smoke-v1",
  generated_at: new Date().toISOString(),
  synthetic: true,
  adapter: adapter.get_model_info(),
  adapter_run: adapter.get_last_run_info(),
  sample_id: input.request_id,
  elapsed_ms: elapsedMs,
  output,
  schema_validation: schemaValidation,
  evidence_verification: evidenceVerification,
  passed: schemaValidation.valid && evidenceVerification.verification_passed && output.mock === false
};
const reportPath = join(LAB_ROOT, "evals", "reports", "local-adapter-smoke.json");
mkdirSync(join(LAB_ROOT, "evals", "reports"), { recursive: true });
writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`, "utf8");
console.log(JSON.stringify({ report: reportPath, ...report }, null, 2));
if (!report.passed) process.exitCode = 1;
