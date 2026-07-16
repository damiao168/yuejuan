# STORY-057: Grading Agent Production Contract

Status: accepted after plan review and implementation review on 2026-07-15.

## Goal

Promote the safe, testable parts of `lab` into a versioned internal-service contract without importing lab runtime code or presenting the experimental model as production-ready.

## Scope

- Versioned request, suggestion, and error schemas under `contracts/grading-agent/v1`.
- A promoted `local-pilot-v1` capability matrix.
- Canonical lower-snake-case risk flags for platform APIs.
- Conformance fixtures for a valid suggestion and an invalid evidence link.
- A repository check for non-negotiable governance invariants.

## Boundary

- The browser continues to call `POST /api/v1/answer-segments/{id}/subjective-ai-grade`.
- Only the Go API gateway may call the internal grading-agent service.
- The request contains question, Rubric, answer text, OCR confidence, and opaque resource ids. It must not contain tenant or student identity, final grades, or published grades.
- A successful service response is a suggestion, never a final grade.
- Every response requires human review. Essay and discussion delivery is `shadow_only`.
- Invalid input, unsupported capability, timeout, malformed output, score mismatch, or evidence mismatch produces no valid suggestion.

## Platform Mapping

| Contract field | Platform source |
| --- | --- |
| `subject` | `exam.subject` |
| `grade_level` | Derived from the distinct `grade.level_no` values attached through `exam_class`; only levels 7-9 map to `junior_middle` |
| `question_*` | `question` |
| `answer_text`, `ocr_confidence` | Latest non-deleted `answer_segment_answer` |
| `rubric_version` | Latest `question_rubric` joined to `rubric_version` |
| Rubric aliases/policy | Conservative platform defaults until persisted fields exist: empty aliases, `evidence_required=true`, `match_policy=semantic` |
| `request_id` | API-generated idempotency key; never a student identifier |

Missing, mixed, or unsupported grade-level context must fail closed before model invocation.

## Plan Review

The initial idea to pass the existing Go `paper.Question` and `paper.Rubric` JSON directly was rejected. Those structures leak tenant ids and lack explicit grade level, evidence policy, aliases, and stable point linkage. A dedicated contract also prevents accidental coupling to future platform fields.

The existing lab uppercase risk flags are converted once at the promotion boundary. Platform-facing values remain lower snake case to match existing APIs.

## Implementation Review

Accepted invariants:

- `final_grade_publication_allowed=false` in both metadata and capability matrix.
- Contract deployment mode is `shadow`, student visibility is disabled, and human review is mandatory.
- Model-backed routes are limited to the lab-approved combinations.
- Request schema rejects unknown fields and explicitly excludes identity/final-score fields.
- Scores are recomputed from matched Rubric points by service code.
- Matched points require answer-local evidence links.
- Prompt, model, Rubric, capability, and schema versions are traceable.

Residual limitation: schema files describe structural constraints; cross-field constraints such as score recomputation and evidence linkage are intentionally implemented and tested in service code as well as fixtures.

## Verification

```powershell
npm run check:story057
```
