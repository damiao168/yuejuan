#!/usr/bin/env node
import { readFileSync, writeFileSync } from "node:fs";
import { compareModelSelectionReports } from "../src/evaluators/modelSelection.js";

const args = process.argv.slice(2);
const outputIndex = args.indexOf("--out");
const output = outputIndex >= 0 ? args[outputIndex + 1] : "evals/reports/model-selection-decision.json";
const reportPaths = args.filter((item, index) => item !== "--out" && index !== outputIndex + 1);
if (reportPaths.length < 2) throw new Error("Usage: compare-model-selection report1.json report2.json [report3.json] --out decision.json");
const reports = reportPaths.map((path) => JSON.parse(readFileSync(path, "utf8")));
const decision = { generated_at: new Date().toISOString(), ...compareModelSelectionReports(reports) };
writeFileSync(output, `${JSON.stringify(decision, null, 2)}\n`, "utf8");
console.log(JSON.stringify(decision, null, 2));
