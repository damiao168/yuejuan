# Story I Real Local Model Adapter And Structured Output

## Spec v1

Connect the verified Qwen 4B runtime to the grading lab through an OpenAI-compatible local adapter.

## Spec Review

The existing pipeline assumed synchronous adapters and trusted adapter totals. A real adapter also needs bounded retries, loopback-only operation, dynamic rubric enums, privacy-safe failures, and evidence-link integrity before schema validation.

## Spec v2

Preserve synchronous Mock behavior while supporting Promise-returning adapters. Constrain the local model with JSON Schema, classify every rubric point, recompute totals in code, force review, and fail closed after one retry.

## Implementation

Added the local adapter, structured prompt, async eval path, server lifecycle scripts, unit tests, and adapter contract documentation.

## Implementation Review

The first real smoke passed after a repair attempt but took 203 seconds. Review found uncalibrated model confidence, a model-authored note contradicting mandatory review, unrestricted localhost CORS, and no API key. After fixes, the repeated smoke passed in one attempt in 85.05 seconds with valid schema and evidence. It also revealed English feedback on a Chinese question.

## Implementation Fixes

Added a random lab-local API key, localhost-only CORS, disabled CORS credentials, code-owned review notes, zero confidence until calibration, score review flags, retry telemetry, same-language prompt guidance, and schema-repair flags. The language adherence failure remains explicitly queued for Story O.

## Acceptance

Accepted. The real non-mock response passed schema and evidence verification; 57 tests, the 22-sample synthetic eval, dev gate, and regression suite also passed.
