import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { evaluateCalibration, fitIsotonicCalibration, predictCalibratedProbability } from "../src/evaluators/calibration.js";
import { checkCalibrationFairnessGate, evaluateOperationalSlices } from "../src/evaluators/fairness.js";

const observations = JSON.parse(readFileSync("evals/calibration/synthetic-observations.json", "utf8"));
const calibrationSet = observations.filter((item) => item.partition === "calibration");
const evaluationSet = observations.filter((item) => item.partition === "evaluation");

test("isotonic calibration is monotonic", () => {
  const model = fitIsotonicCalibration(calibrationSet);
  const predictions = [0, 0.25, 0.5, 0.75, 1].map((signal) => predictCalibratedProbability(model, signal));
  for (let index = 1; index < predictions.length; index += 1) assert.ok(predictions[index] >= predictions[index - 1]);
});

test("calibration uses the conservative previous block between observed signals", () => {
  const model = { blocks: [
    { min_signal: 0.2, max_signal: 0.2, probability: 0.1 },
    { min_signal: 0.8, max_signal: 0.8, probability: 0.9 }
  ] };
  assert.equal(predictCalibratedProbability(model, 0.5), 0.1);
});

test("calibration evaluates only the held-out partition", () => {
  const result = evaluateCalibration(evaluationSet, fitIsotonicCalibration(calibrationSet));
  assert.equal(result.evaluation_records, 8);
  assert.ok(result.ece >= 0 && result.ece <= 1);
  assert.ok(result.brier_score >= 0 && result.brier_score <= 1);
  assert.equal(result.predictions.some((item) => item.observation.partition !== "evaluation"), false);
});

test("operational slices include subject, question type, OCR, and answer length", () => {
  const calibration = evaluateCalibration(evaluationSet, fitIsotonicCalibration(calibrationSet));
  const fairness = evaluateOperationalSlices(calibration.predictions, 2);
  assert.deepEqual(Object.keys(fairness.dimensions), ["subject", "question_type", "ocr_band", "answer_length_band"]);
  assert.equal(fairness.insufficient_slices.length, 0);
});

test("dev gate can validate mechanics while pilot rejects synthetic sample size", () => {
  const config = JSON.parse(readFileSync("config/calibration-gates.json", "utf8"));
  const calibration = evaluateCalibration(evaluationSet, fitIsotonicCalibration(calibrationSet));
  const devFairness = evaluateOperationalSlices(calibration.predictions, config.dev.minimum_records_per_slice);
  assert.equal(checkCalibrationFairnessGate(calibration, devFairness, "dev", config).passed, true);
  const pilotFairness = evaluateOperationalSlices(calibration.predictions, config.pilot.minimum_records_per_slice);
  const pilot = checkCalibrationFairnessGate(calibration, pilotFairness, "pilot", config);
  assert.equal(pilot.passed, false);
  assert.ok(pilot.reasons.some((reason) => reason.includes("evaluation_records")));
});

test("calibration and fairness gates reject missing or non-finite evidence", () => {
  const config = JSON.parse(readFileSync("config/calibration-gates.json", "utf8"));
  const result = checkCalibrationFairnessGate(
    { evaluation_records: 8, ece: Number.NaN, brier_score: Number.NaN },
    {},
    "dev",
    config
  );
  assert.equal(result.passed, false);
  assert.match(result.reasons.join("\n"), /ece/);
  assert.match(result.reasons.join("\n"), /insufficient_slices is missing/);
});

test("calibration evaluation rejects empty observations and invalid bins", () => {
  const model = fitIsotonicCalibration(calibrationSet);
  assert.throws(() => evaluateCalibration([], model), /observations are required/);
  assert.throws(() => evaluateCalibration(evaluationSet, model, 0), /positive integer/);
});
