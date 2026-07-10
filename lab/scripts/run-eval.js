#!/usr/bin/env node
import { join } from "node:path";
import { writeEvaluationReport, runEvaluation } from "../src/evaluators/runEval.js";

function parseArgs(argv) {
  const args = {};
  for (let index = 0; index < argv.length; index += 1) {
    const item = argv[index];
    if (!item.startsWith("--")) continue;
    args[item.slice(2)] = argv[index + 1] && !argv[index + 1].startsWith("--") ? argv[index + 1] : true;
  }
  return args;
}

const args = parseArgs(process.argv.slice(2));
const datasetName = args.dataset ?? "synthetic";
const datasetPath = args.path ?? join("evals", datasetName, "samples.jsonl");
const adapter = args.adapter ?? "mock";
const filters = {
  subject: args.subject,
  question_type: args.question_type,
  tag: args.tag
};

const report = runEvaluation({ datasetPath, adapterName: adapter, filters });
writeEvaluationReport(
  report,
  join("evals", "reports", "latest-report.json"),
  join("evals", "reports", "latest-report.md"),
  join("evals", "reports", "failed-cases.jsonl")
);
console.log(JSON.stringify({ ok: true, sample_count: report.sample_count, metrics: report.metrics }, null, 2));
