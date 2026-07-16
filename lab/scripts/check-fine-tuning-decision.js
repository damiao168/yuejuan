#!/usr/bin/env node
import { readFileSync, writeFileSync } from "node:fs";
import { collectFineTuningEvidence, evaluateFineTuningDecision } from "../src/training/decision.js";

const outputIndex = process.argv.indexOf("--out");
const output = outputIndex >= 0 ? process.argv[outputIndex + 1] : "evals/reports/fine-tuning-decision.json";
const gate = JSON.parse(readFileSync("config/fine-tuning-gates.json", "utf8"));
const evidence = collectFineTuningEvidence();
const decision = evaluateFineTuningDecision(evidence, gate);
const report = { schema_version: "fine-tuning-decision-v1", generated_at: new Date().toISOString(), evidence, gate, decision };
writeFileSync(output, `${JSON.stringify(report, null, 2)}\n`, "utf8");
console.log(JSON.stringify(report, null, 2));
