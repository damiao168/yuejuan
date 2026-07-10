import test from "node:test";
import assert from "node:assert/strict";
import { listPrompts, loadPrompt, promptForQuestionType } from "../src/prompts/registry.js";

test("all registered prompt files load with checksums", () => {
  const prompts = listPrompts();
  assert.ok(prompts.length >= 6);
  for (const prompt of prompts) {
    assert.ok(prompt.text.length > 20);
    assert.ok(prompt.checksum.length === 64);
    assert.ok(prompt.prompt_version);
  }
});

test("prompt checksum is stable across loads", () => {
  assert.equal(loadPrompt("base_grading").checksum, loadPrompt("base_grading").checksum);
});

test("question type prompt lookup works", () => {
  assert.equal(promptForQuestionType("calculation").prompt_id, "calculation");
});

test("unknown prompt throws", () => {
  assert.throws(() => loadPrompt("missing"), /Unregistered prompt/);
});
