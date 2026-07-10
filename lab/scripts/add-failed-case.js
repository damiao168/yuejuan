#!/usr/bin/env node
import { appendFileSync, readFileSync } from "node:fs";

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
if (!args.case) throw new Error("Usage: add-failed-case --case case.json --out evals/regression/failed_cases.jsonl");
const out = args.out ?? "evals/regression/failed_cases.jsonl";
const failedCase = JSON.parse(readFileSync(args.case, "utf8"));
for (const key of ["case_id", "source", "discovered_at", "subject", "question_type", "prompt_version", "model_version", "input", "bad_output", "expected_output", "failure_type", "severity", "human_note"]) {
  if (failedCase[key] === undefined) throw new Error(`failed case missing required field: ${key}`);
}
appendFileSync(out, `${JSON.stringify(failedCase)}\n`, "utf8");
console.log(JSON.stringify({ added: failedCase.case_id, out }, null, 2));
