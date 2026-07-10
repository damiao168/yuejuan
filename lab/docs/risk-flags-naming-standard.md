# Risk Flags Naming Standard

Status: Story E accepted after self-review.

Scope: documentation only. This standard defines risk flag naming and compatibility mapping between the lab and the current Go platform. It does not change code, schemas, database values, APIs, or runtime behavior.

## 1. Purpose

Risk flags drive human review, evidence checks, release gates, regression analysis, and quality reports. The lab currently uses uppercase enum names such as `OCR_LOW_CONFIDENCE`, while the Go platform uses lower snake case strings such as `low_ocr_confidence`.

The goal of this standard is to define a stable canonical vocabulary before any platform-output eval converter, real model adapter, or release gate integration is implemented.

## 2. Current Sources

### Lab Flags

Defined in `src/schemas/gradingSchema.js`:

| Lab Flag |
| --- |
| `OCR_LOW_CONFIDENCE` |
| `OCR_TEXT_EMPTY_REVIEW_REQUIRED` |
| `AMBIGUOUS_ANSWER` |
| `INSUFFICIENT_EVIDENCE` |
| `POSSIBLE_OFF_TOPIC` |
| `SCORE_NEEDS_REVIEW` |
| `SCHEMA_REPAIRED` |
| `PROMPT_INJECTION_SUSPECTED` |
| `HUMAN_REVIEW_REQUIRED` |
| `MOCK_OUTPUT` |

### Current Go / Platform Flags And Issue Codes

Observed in the current Go chain and docs:

| Platform Value | Source / Meaning |
| --- | --- |
| `mock_llm_output` | Subjective mock adapter output. |
| `low_model_confidence` | Model confidence below policy threshold. |
| `invalid_model_output` | Adapter output failed validation or adapter failed. |
| `long_form_subjective_requires_review` | Essay/discussion forced review. |
| `low_ocr_confidence` | OCR confidence below threshold. |
| `evidence_verification_failed` | Evidence verifier failure. |
| `true_false_parse_failed` | Objective true/false parse issue. |
| `multiple_choice_empty_answer` | Objective multiple-choice issue. |
| `numeric_parse_failed` | Objective numeric parse issue. |
| `score_exceeds_max` | Evidence issue code. |
| `matched_score_mismatch` | Evidence issue code. |
| `rubric_missing` | Evidence issue code. |
| `rubric_point_not_found` | Evidence issue code. |
| `empty_evidence` | Evidence issue code. |
| `empty_evidence_text` | Evidence issue code. |
| `evidence_text_not_found` | Evidence issue code. |
| `evidence_bbox_out_of_segment` | Evidence issue code. |

## 3. Canonical Naming Decision

Use **lower snake case** as the canonical platform-facing name.

Reasons:

- Go platform already uses lower snake case in persisted `risk_flags`.
- API responses already expose lower snake case values.
- Lower snake case is easier to keep stable across Go, TypeScript, JSON, SQL, and reports.
- Lab uppercase values can remain internal enum labels until a future code migration is approved.

Canonical names should be stable, descriptive, and action-oriented enough for review routing.

## 4. Canonical Flag Catalog

| Canonical Flag | Severity Class | Triggers Human Review? | Description |
| --- | --- | --- | --- |
| `mock_output` | safety | Yes | Generic mock output marker. Use only when source-specific flag is unavailable. |
| `mock_llm_output` | safety | Yes | Subjective mock LLM output; not real model grading. |
| `low_model_confidence` | quality | Yes | Model/adaptor confidence is below threshold. |
| `low_ocr_confidence` | quality | Yes | OCR confidence is below threshold. |
| `ocr_text_empty_review_required` | quality | Yes | OCR text is empty while image reference exists or OCR is expected. |
| `insufficient_evidence` | evidence | Yes | Output lacks adequate evidence for score or matched points. |
| `evidence_verification_failed` | evidence | Yes | Evidence verifier failed at least one blocking rule. |
| `rubric_point_not_found` | evidence | Yes | Output references a rubric point not present in the rubric. |
| `evidence_text_not_found` | evidence | Yes | Evidence text is not found in answer text. |
| `evidence_bbox_out_of_segment` | evidence | Yes | Evidence bbox is outside the answer segment bbox. |
| `ambiguous_answer` | quality | Usually | Answer is ambiguous or uncertain enough to require teacher judgment. |
| `possible_off_topic` | quality | Usually | Answer appears unrelated to the prompt or rubric. |
| `score_needs_review` | score | Yes | Score is suspicious even if schema bounds pass. |
| `score_exceeds_max` | score | Yes | Suggested score exceeds max score; should normally fail validation. |
| `matched_score_mismatch` | score | Yes | Matched point total does not equal suggested score where strict equality is required. |
| `invalid_model_output` | schema | Yes | Adapter output failed validation or parse. |
| `schema_repaired` | schema | Yes | Output required automatic repair before validation. Do not use until repair flow exists. |
| `prompt_injection_suspected` | security | Yes | Student answer contains instruction-like text that attempts to override grading. |
| `long_form_subjective_requires_review` | policy | Yes | Essay/discussion require human review by policy. |
| `human_review_required` | policy | Yes | Generic fallback flag. Prefer explicit flags plus `needs_human_review=true`. |
| `numeric_parse_failed` | objective | Yes | Numeric answer could not be parsed. |
| `true_false_parse_failed` | objective | Yes | True/false answer could not be parsed. |
| `multiple_choice_empty_answer` | objective | Yes | Multiple-choice expected or actual answer is empty. |

## 5. Lab To Canonical Mapping

| Lab Flag | Canonical Flag | Mapping Type | Notes |
| --- | --- | --- | --- |
| `OCR_LOW_CONFIDENCE` | `low_ocr_confidence` | direct semantic | Already present in Go. |
| `OCR_TEXT_EMPTY_REVIEW_REQUIRED` | `ocr_text_empty_review_required` | direct semantic | New canonical flag. |
| `AMBIGUOUS_ANSWER` | `ambiguous_answer` | direct semantic | New platform flag candidate. |
| `INSUFFICIENT_EVIDENCE` | `insufficient_evidence` | direct semantic | May also accompany `evidence_verification_failed`. |
| `POSSIBLE_OFF_TOPIC` | `possible_off_topic` | direct semantic | New platform flag candidate. |
| `SCORE_NEEDS_REVIEW` | `score_needs_review` | direct semantic | New platform flag candidate. |
| `SCHEMA_REPAIRED` | `schema_repaired` | direct semantic | Reserve until repair exists. |
| `PROMPT_INJECTION_SUSPECTED` | `prompt_injection_suspected` | direct semantic | P0 before real LLM. |
| `HUMAN_REVIEW_REQUIRED` | `human_review_required` | fallback | Prefer boolean plus explicit reason flags. |
| `MOCK_OUTPUT` | `mock_output` or `mock_llm_output` | contextual | Use `mock_llm_output` for subjective LLM mock; `mock_output` for generic mock. |

## 6. Platform To Canonical Mapping

| Platform Value | Canonical Flag | Mapping Type | Notes |
| --- | --- | --- | --- |
| `mock_llm_output` | `mock_llm_output` | identity | Keep. |
| `low_model_confidence` | `low_model_confidence` | identity | Keep. |
| `invalid_model_output` | `invalid_model_output` | identity | Keep. |
| `long_form_subjective_requires_review` | `long_form_subjective_requires_review` | identity | Keep. |
| `low_ocr_confidence` | `low_ocr_confidence` | identity | Keep. |
| `evidence_verification_failed` | `evidence_verification_failed` | identity | Keep. |
| `true_false_parse_failed` | `true_false_parse_failed` | identity | Keep. |
| `multiple_choice_empty_answer` | `multiple_choice_empty_answer` | identity | Keep. |
| `numeric_parse_failed` | `numeric_parse_failed` | identity | Keep. |
| `score_exceeds_max` | `score_exceeds_max` | identity | Evidence issue code can be promoted to flag in reports. |
| `matched_score_mismatch` | `matched_score_mismatch` | identity | Evidence issue code can be promoted to flag in reports. |
| `rubric_missing` | `rubric_missing` | identity | Evidence issue code. |
| `rubric_point_not_found` | `rubric_point_not_found` | identity | Evidence issue code. |
| `empty_evidence` | `insufficient_evidence` + `empty_evidence` | composite | Preserve original issue code; add high-level flag in reports. |
| `empty_evidence_text` | `insufficient_evidence` + `empty_evidence_text` | composite | Preserve original issue code. |
| `evidence_text_not_found` | `evidence_text_not_found` | identity | Keep. |
| `evidence_bbox_out_of_segment` | `evidence_bbox_out_of_segment` | identity | Keep. |

## 7. Compatibility Rules

1. Canonical reports should use lower snake case.
2. Lab uppercase flags may be accepted as input in lab-only data.
3. Platform lower snake case flags must be accepted in platform-output eval snapshots.
4. Unknown flags must not be dropped. They should be preserved under `unknown_risk_flags`.
5. Unknown flags should count against `risk_flag_mapping_coverage`.
6. `needs_human_review=true` must not be inferred solely from `human_review_required`; explicit reason flags should be preferred.
7. `mock=true` must always imply a mock flag: `mock_llm_output` or `mock_output`.
8. `mock=false` must never include mock flags.
9. Evidence issue codes can be copied into reports as issue codes, but only stable, review-routing values should become `risk_flags`.
10. Release gates should use canonical names only.

## 8. Release Gate Metrics

Risk flag normalization enables these metrics:

| Metric | Definition |
| --- | --- |
| `risk_flag_mapping_coverage` | Fraction of input flags mapped to canonical values. |
| `mock_marked_rate` | Fraction of mock outputs containing `mock_llm_output` or `mock_output`. |
| `prompt_injection_detection` | Fraction of prompt-injection samples containing `prompt_injection_suspected`. |
| `ocr_low_confidence_review_rate` | Fraction of low-OCR samples with `low_ocr_confidence` and review. |
| `evidence_failure_review_rate` | Fraction of evidence failures with `evidence_verification_failed` or evidence-specific flags and review. |
| `invalid_output_failure_rate` | Fraction of invalid outputs recorded as failed with `invalid_model_output`. |

## 9. Review Routing Guidance

| Flag Family | Suggested Route |
| --- | --- |
| mock flags | Teacher review; never auto-accept as real AI. |
| prompt injection | Teacher review plus security/regression tracking. |
| OCR flags | OCR/manual transcription review. |
| evidence flags | Evidence review before score use. |
| score flags | Senior teacher or grading lead review. |
| schema/model flags | Engineering triage; no valid score. |
| long-form policy flags | Normal essay/discussion teacher review. |

## 10. Migration Strategy

### Phase 1: Lab-Only Normalization

Use this mapping in lab docs, eval reports, and future platform snapshot converters. No platform code changes.

### Phase 2: Platform Compatibility Layer

When the main platform is ready, normalize incoming/outgoing flags at adapter boundaries while preserving existing values for backward compatibility.

### Phase 3: Canonical Platform Values

After UI, reports, API docs, release gates, and migrations are aligned, use lower snake case canonical values as the only new persisted values. Existing old values must remain readable.

## 11. Do Not Do Yet

- Do not rename existing Go flags now.
- Do not change database values now.
- Do not change frontend display logic now.
- Do not change lab schema enum now without a deliberate migration story.
- Do not make uppercase lab flags production API values.
- Do not collapse all review causes into `human_review_required`.

## 12. Open Questions

1. Should evidence issue codes become persisted `risk_flags`, or remain verifier issue codes only?
2. Should `mock_output` be allowed, or should every mock source use a source-specific flag such as `mock_llm_output`?
3. Should `rubric_missing` be a release-blocking flag or only an evidence verifier failure code?
4. Should prompt injection flags trigger a separate security audit event in the future?
5. How should multiple flags be ordered in API responses and reports?

## 13. Acceptance Criteria For Future Implementation

Any future implementation of this standard should prove:

- all lab flags map to canonical lower snake case;
- all current Go flags map to canonical lower snake case;
- unknown flags are preserved and reported;
- mock marking rules are enforced;
- prompt injection and OCR flags feed release gate metrics;
- review routing can distinguish evidence, OCR, schema, score, mock, and security causes;
- no existing platform API consumers break during transition.

## 14. Story E Self-Review

No critical issue found. The standard is conservative: it keeps current Go lower snake case values, treats lab uppercase values as internal, and avoids immediate platform migration.

Remaining risk: this document is not yet enforced by code. Story D-style compatibility snapshots should not be implemented until this mapping is converted into tests.
