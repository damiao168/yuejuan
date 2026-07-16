# Product Capability Boundary

Status: Story G accepted after specification and implementation review.

## Product Position

The grading agent produces auditable suggestions for teachers. It cannot publish a final grade, change a student's official record, bypass a review policy, or silently expand to an unvalidated subject or grade level.

## Local Pilot Scope

The first profile is `local-pilot-v1` for `junior_middle` only.

| Subject | Question type | Grader | Delivery | Review |
| --- | --- | --- | --- | --- |
| Math | fill blank, numeric | deterministic rules | teacher suggestion | risk based |
| Physics | numeric | deterministic rules | teacher suggestion | risk based |
| Chemistry | numeric | deterministic rules | teacher suggestion | risk based |
| Math | calculation | rules plus LLM | teacher suggestion | always |
| Chinese | short answer | LLM | teacher suggestion | always |
| Any | essay, discussion | LLM | shadow only | always |

Every other combination routes to `human_only`. Shadow results are evaluation records and must not be shown as accepted grades.

## Output Contract

An in-scope model-backed suggestion must include rubric-point decisions, code-recomputed score, answer-local evidence, model/prompt/rubric versions, risk flags, and a review decision. Model prose and model-provided totals are not authoritative.

## Failure Semantics

- Invalid input, schema, rubric point, score bound, evidence, timeout, or adapter output produces no valid suggestion.
- Unsupported grade levels or subject/question combinations route to humans.
- OCR, injection, ambiguity, evidence, and calibration failures require review.
- A capability configuration that enables final grade publication is invalid.

## Explicit Non-Goals

- Nine-subject autonomous grading is not a first-release claim.
- Essay or discussion auto-acceptance is not supported.
- The lab does not publish grades, manage appeals, add UI, or replace platform permissions.
- Synthetic and mock metrics do not prove product quality.

## Machine-Enforced Source

`config/capability-matrix.json` is the executable capability source. `src/capabilities.js` validates it and resolves unsupported requests to the human-only default.

## Acceptance Evidence

- The matrix disables final grade publication.
- Every LLM-backed route requires review in the local pilot.
- Essay and discussion routes are shadow-only.
- Unsupported subjects, question types, and grade levels fail closed to humans.
- `tests/capabilities.test.js` covers the safety invariants.
