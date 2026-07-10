import test from "node:test";
import assert from "node:assert/strict";
import { createAdapter, LocalModelAdapter, OpenAICompatibleGradingAdapter } from "../src/adapters/index.js";
import { validateGradingOutput } from "../src/schemas/gradingSchema.js";
import { baseInput } from "./helpers.js";

test("mock adapter produces valid marked output", () => {
  const input = baseInput();
  const adapter = createAdapter("mock");
  const output = adapter.grade(input);
  assert.equal(output.mock, true);
  assert.ok(output.risk_flags.includes("MOCK_OUTPUT"));
  assert.equal(validateGradingOutput(output, input).valid, true);
});

test("adapter factory selects mock", () => {
  const adapter = createAdapter("mock");
  assert.equal(adapter.get_model_info().adapter, "mock");
});

test("openai-compatible adapter fails clearly without env", () => {
  assert.throws(() => new OpenAICompatibleGradingAdapter({}), /GRADING_OPENAI_API_KEY/);
});

test("local adapter fails clearly while placeholder is not configured", () => {
  assert.throws(() => new LocalModelAdapter(), /placeholder/);
});
