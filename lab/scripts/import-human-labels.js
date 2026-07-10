#!/usr/bin/env node
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";
import { validateHumanLabeledSample } from "../src/humanLabels.js";

function parseArgs(argv) {
  const args = {};
  for (let index = 0; index < argv.length; index += 1) {
    const item = argv[index];
    if (!item.startsWith("--")) continue;
    args[item.slice(2)] = argv[index + 1] && !argv[index + 1].startsWith("--") ? argv[index + 1] : true;
  }
  return args;
}

function readJsonl(path) {
  return readFileSync(path, "utf8").split(/\r?\n/).filter(Boolean).map((line) => JSON.parse(line));
}

const args = parseArgs(process.argv.slice(2));
if (!args.input) throw new Error("Usage: import-human-labels --input input.jsonl --clean clean.jsonl --rejected rejected.jsonl");
const cleanPath = args.clean ?? "evals/human_labeled/clean.jsonl";
const rejectedPath = args.rejected ?? "evals/human_labeled/rejected.jsonl";
mkdirSync(dirname(cleanPath), { recursive: true });
mkdirSync(dirname(rejectedPath), { recursive: true });

const clean = [];
const rejected = [];
for (const sample of readJsonl(args.input)) {
  const validation = validateHumanLabeledSample(sample);
  if (validation.valid) clean.push(sample);
  else rejected.push({ sample_id: sample.sample_id, errors: validation.errors });
}
writeFileSync(cleanPath, clean.map((item) => JSON.stringify(item)).join("\n") + (clean.length ? "\n" : ""), "utf8");
writeFileSync(rejectedPath, rejected.map((item) => JSON.stringify(item)).join("\n") + (rejected.length ? "\n" : ""), "utf8");
console.log(JSON.stringify({ clean: clean.length, rejected: rejected.length }, null, 2));
