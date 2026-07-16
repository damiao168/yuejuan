#!/usr/bin/env node
import { writeFileSync } from "node:fs";
import { checkAdversarialGate, runAdversarialSuite } from "../src/evaluators/adversarial.js";

const levelIndex = process.argv.indexOf("--level");
const outIndex = process.argv.indexOf("--out");
const level = levelIndex >= 0 ? process.argv[levelIndex + 1] : "dev";
const output = outIndex >= 0 ? process.argv[outIndex + 1] : "evals/reports/adversarial-suite.json";
const agents = runAdversarialSuite();
const gate = checkAdversarialGate(agents, level);
const report = { schema_version: "adversarial-suite-v1", generated_at: new Date().toISOString(), synthetic: true, online_agents: 0, offline_adversarial_agents: agents.length, agents, gate };
writeFileSync(output, `${JSON.stringify(report, null, 2)}\n`, "utf8");
console.log(JSON.stringify(report, null, 2));
if (!gate.passed) process.exitCode = 1;
