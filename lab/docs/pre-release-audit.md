# Pre-Release Audit

Audit date: 2026-07-06

## Critical Issues

None found in the isolated lab after the current tests and dev gate.

## High-Risk Issues

- The lab is still mock-only. This is acceptable for local eval mechanics, but it is not evidence of real model grading quality.
- The dataset is synthetic-only. Pilot or production readiness needs teacher-labeled anonymized samples.

## Medium-Risk Issues

- Prompt injection detection is rule-based. It catches the current suite but should later support stronger detectors.
- Question graders are thin wrappers around the adapter and verifier. Future stories should specialize fill-blank and numeric rules.
- The OpenAPI contract is intentionally skeletal and needs schema component expansion before platform integration.

## Low-Risk Issues

- `npm.ps1` is blocked by local PowerShell execution policy; `npm.cmd` works.
- Current eval metrics are strong because the mock adapter and synthetic aliases are intentionally aligned.

## Verified Controls

- Mock outputs use `mock=true` and include `MOCK_OUTPUT`.
- Synthetic samples use `synthetic=true`.
- Schema validation prevents overscore, negative score, missing versions, and missing evidence.
- Essay and discussion outputs require human review.
- OCR low confidence requires human review.
- Prompt injection is flagged.
- Eval metrics are computed from run records.
- Dev release gate passes.
- Production gate fails when sample count is too small.
- Regression runner executes the placeholder failed case.

## Recommended Next Step

Do not integrate into the main platform yet. First expand human-label import examples, add more regression cases, and only then configure a real OpenAI-compatible adapter if the user approves external model access.
