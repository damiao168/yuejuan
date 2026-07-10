#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { checkGate, loadGateConfig } from "../src/evaluators/gateChecker.js";

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
const reportPath = args.report ?? "evals/reports/latest-report.json";
const level = args.level ?? "dev";
const configPath = args.config ?? "config/release-gates.json";
const report = JSON.parse(readFileSync(reportPath, "utf8"));
const config = loadGateConfig(configPath);
const result = checkGate(report, level, config);
console.log(JSON.stringify(result, null, 2));
if (!result.passed) process.exitCode = 1;
