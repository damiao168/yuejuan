# Rubric Field Extension Proposal

Status: Story F accepted after self-review.

Scope: documentation only. This proposal does not change Go structs, TypeScript API types, database schema, runtime grading logic, prompts, release gates, or root workspace configuration.

## 1. Purpose

The current main platform can store and edit simple rubrics. The lab needs richer rubrics to support real-model grading, repeatable evals, evidence checks, essay dimensions, calculation steps, and teacher-auditable scoring.

This document proposes a conservative extension path. It defines what fields should exist before a real model pilot, how missing fields should default, and which changes must wait for a main-platform story.

The proposal is intentionally not an implementation plan for this conversation. The main platform is being worked on elsewhere, so this lab story only records the contract and migration shape.

## 2. Current Platform Rubric Surface

### Go Backend

Current source: `services/api-gateway/internal/paper/types.go`.

| Type | Field | JSON | Current Meaning |
| --- | --- | --- | --- |
| `RubricPoint` | `ID string` | `id` | Point identifier. |
| `RubricPoint` | `Description string` | `description` | Teacher-facing point description. |
| `RubricPoint` | `Score float64` | `score` | Point score. |
| `RubricPoint` | `Required bool` | `required` | Whether the point is required. |
| `Rubric` | `ID string` | `id` | Rubric id. |
| `Rubric` | `QuestionID string` | `question_id` | Owning question id. |
| `Rubric` | `Version string` | `version` | Rubric version. |
| `Rubric` | `Status string` | `status` | Rubric lifecycle status. |
| `Rubric` | `MaxScore float64` | `max_score` | Maximum score. |
| `Rubric` | `Points []RubricPoint` | `points` | Scoring points. |
| `Rubric` | `Deductions []any` | `deductions` | Untyped deduction data. |
| `Rubric` | `Examples []any` | `examples` | Untyped examples. |

Current `RubricInput` accepts `status`, `max_score`, `points`, `deductions`, and `examples`.

### Frontend API Types

Current source: `apps/web-admin/src/api/papers.ts`.

| Type | Field | Current Meaning |
| --- | --- | --- |
| `RubricPoint` | `id` | Point identifier. |
| `RubricPoint` | `description` | Point description. |
| `RubricPoint` | `score` | Point score. |
| `RubricPoint` | `required` | Required marker. |
| `Rubric` | `id`, `question_id`, `version`, `status`, `max_score` | Core metadata. |
| `Rubric` | `points` | Point list. |
| `Rubric` | `deductions` | Untyped array. |
| `Rubric` | `examples` | Untyped array. |
| `RubricPayload` | `status`, `max_score`, `points`, `deductions`, `examples` | Create/update payload shape. |

### Current Strengths

- The platform already has versioned rubric records.
- `points` are explicit and score-bearing.
- `max_score` exists and can be checked against the question score.
- `deductions` and `examples` already have placeholder fields, which lowers migration pressure.

### Current Limits

- Point aliases are not represented.
- Point-level evidence policy is not represented.
- Deductions and examples are untyped.
- Essay dimensions are not first-class.
- Calculation steps are not first-class.
- Subjective equivalent answers are not part of the rubric.
- Scoring notes are not stored.
- There is no explicit content hash or schema version.

## 3. Current Lab Rubric Requirements

Current source: `lab/src/schemas/gradingSchema.js`.

| Lab Field | Required Now | Meaning |
| --- | --- | --- |
| `rubric_id` | Yes | Stable rubric id used by lab input. |
| `rubric_version` | Yes | Version used for reproducible grading. |
| `max_score` | Yes | Maximum score. |
| `points` | Yes | Non-empty point list. |
| `points[].id` | Yes | Point id. |
| `points[].description` | Yes | Point description. |
| `points[].score` | Yes | Positive point score. |
| `points[].required` | Yes | Required marker. |
| `points[].aliases` | Yes | Alternative expressions or labels for matching/eval. |
| `points[].evidence_required` | Yes | Whether a matched required point must have evidence. |
| `deductions` | Yes | Array of deduction rules; objects require `id` and `max_deduction`. |
| `equivalent_answers` | Yes | Alternative correct answer forms. |
| `examples` | Yes | Examples for prompts/eval. |
| `scoring_notes` | Yes | Teacher scoring instructions. |
| `dimensions` | Required for essay | Essay/discussion scoring dimensions. |
| `steps` | Required for calculation | Calculation-step structure. |
| `allow_partial_total` | Optional | Allows point totals not to equal `max_score`. |

The lab also checks that point scores total `max_score` unless `allow_partial_total` is set.

## 4. Proposed Extension Summary

| Field | Type | Applies To | Priority | Backward Compatibility | Why Needed |
| --- | --- | --- | --- | --- | --- |
| `points[].aliases` | `string[]` | All subjective rubrics; useful for objective-like items too | P1 | Missing means `[]` in eval/adapters. | Equivalent phrasing and deterministic matching. |
| `points[].evidence_required` | `boolean` | All subjective rubrics | P1 | Missing must be explicitly defaulted by the adapter/eval layer. | Point-specific evidence verification. |
| `equivalent_answers` | `unknown[]` initially; typed later | Short answer, calculation, fill blank, numeric | P1/P2 | Existing objective `AnswerKey.EquivalentAnswers` remains valid. | Subjective rubrics need accepted alternatives too. |
| `scoring_notes` | `string[]` | All subjective rubrics | P1 | Missing means `[]`. | Teacher intent in prompt/eval without changing point scores. |
| `dimensions` | typed objects | Essay, discussion | P1 | Only required for essay/discussion after migration gate. | Long-form scoring needs dimensions, not only flat points. |
| `steps` | typed objects | Calculation | P1 | Only required for calculation after migration gate. | Step scoring and partial credit audit. |
| `deductions` | typed objects | All rubrics | P2 | Keep `[]any` readable until typed migration. | Deductions need ids, caps, reasons, and evidence policy. |
| `examples` | typed objects | All rubrics | P2 | Keep `[]any` readable until typed migration. | Prompt examples and eval examples need labels and expected scores. |
| `rubric_schema_version` | `string` | All rubrics | P2 | Missing means legacy platform schema. | Contract evolution without guessing. |
| `rubric_hash` | `string` | All rubrics | P2 | Generated outside storage until platform adopts it. | Reproducibility and eval replay. |
| `allow_partial_total` | `boolean` | Exceptional rubrics only | P3 | Missing means false. | Rare cases where point totals intentionally differ from max. |

## 5. Detailed Field Proposal

### 5.1 `points[].aliases`

Type: `string[]`.

Recommended default: `[]`.

Validation:

- Must be an array.
- Items must be non-empty strings.
- Items should be trimmed during authoring.
- Duplicates inside the same point should be rejected in future UI validation.

Usage:

- Lab eval can use aliases for deterministic mock matching and expected point matching.
- Real-model prompts can include aliases as accepted forms, not as extra score-bearing points.
- Platform UI can keep aliases hidden in the first migration phase if needed.

Risk:

- If aliases are treated as separate points, scores can be double-counted.
- If aliases include answer leakage from private data, prompt/eval data can become unsafe.

Decision:

Aliases are alternate expressions for one point. They never add score.

### 5.2 `points[].evidence_required`

Type: `boolean`.

Recommended default:

- For lab eval fixtures, require the field explicitly.
- For legacy platform rubrics, do not silently rewrite stored data.
- In a future adapter compatibility layer, default should be explicit and visible in reports.

Suggested compatibility default:

| Point Condition | Suggested Adapter/Eval Default |
| --- | --- |
| `required=true` and field missing | `true`, with a compatibility note. |
| `required=false` and field missing | `false`, unless the rubric type requires evidence. |
| field present | Use the stored value. |

Validation:

- Must be boolean after normalization.
- Required matched points with `evidence_required=true` must link to evidence.

Usage:

- Evidence verifier can distinguish conceptual points that need citations from formatting or holistic points that may not.
- Eval reports can measure evidence validity by point.

Risk:

- Defaulting all points to `true` can over-reject holistic essay grading.
- Defaulting all points to `false` can allow unsupported model scoring.

Decision:

Make the field first-class before real model pilots. Use compatibility defaults only in conversion/eval reports, not as a hidden data migration.

### 5.3 `equivalent_answers`

Type: `unknown[]` initially; later typed by question type.

Recommended default: `[]`.

Relationship to current platform:

- Current `AnswerKey` already has `EquivalentAnswers`.
- Rubric-level equivalent answers are needed when subjective grading depends on accepted forms, explanation variants, formulas, or wording alternatives.

Suggested future typed variants:

| Question Type | Example Shape |
| --- | --- |
| `short_answer` | accepted phrases or semantic equivalents. |
| `calculation` | equivalent formulas, numeric forms, units, or intermediate values. |
| `fill_blank` | accepted strings or normalized variants. |
| `numeric` | accepted numeric values plus tolerance. |

Risk:

- Duplicating `AnswerKey.EquivalentAnswers` can create drift.
- Untyped arrays can become inconsistent.

Decision:

Keep objective answer-key equivalents as the source for objective grading. Add rubric-level equivalents only for subjective/AI grading after the platform defines ownership rules.

### 5.4 `scoring_notes`

Type: `string[]`.

Recommended default: `[]`.

Validation:

- Must be an array of non-empty strings.
- Should not include student private data.
- Should be versioned together with the rubric.

Usage:

- Prompt construction.
- Teacher review context.
- Eval fixture documentation.

Risk:

- Notes can become hidden scoring rules if not reviewed.
- Long free-form notes can make prompts unstable.

Decision:

Use concise, teacher-authored scoring instructions. Do not use notes to bypass explicit point/deduction structure.

### 5.5 `dimensions`

Type: typed objects. Proposed minimum:

```json
{
  "id": "argument_quality",
  "label": "Argument quality",
  "max_score": 4,
  "criteria": ["clear claim", "reasonable support"]
}
```

Applies to: `essay`, `discussion`.

Recommended default:

- Missing allowed for legacy rubrics.
- Required before essay/discussion real-model pilot.

Validation:

- Non-empty for essay/discussion when extended schema is enabled.
- Dimension max scores should total `max_score` or map clearly to point totals.
- Dimension ids should be stable.

Usage:

- Essay scoring.
- Teacher-facing review.
- Model prompt structure.
- Eval metrics by dimension.

Risk:

- Dimensions and points can conflict if both carry scores without a mapping.

Decision:

Future implementation must define whether dimensions are the primary score units or a grouping layer over points. Do not add both as independent score sources.

### 5.6 `steps`

Type: typed objects. Proposed minimum:

```json
{
  "id": "step_1",
  "description": "Set up the equation",
  "score": 2,
  "evidence_required": true,
  "aliases": []
}
```

Applies to: `calculation`.

Recommended default:

- Missing allowed for legacy rubrics.
- Required before calculation real-model pilot.

Validation:

- Non-empty for calculation when extended schema is enabled.
- Step scores should total `max_score` or map clearly to point totals.
- Each step should have stable ids.

Usage:

- Partial credit.
- Evidence verification.
- Model output structure.
- Regression cases for calculation mistakes.

Risk:

- If steps duplicate `points`, matched scores can diverge.

Decision:

In the first platform migration, calculation `steps` should either replace flat points for calculation or be generated as a typed projection from points. Avoid two independent scoring sources.

### 5.7 Typed `deductions`

Current platform field exists as `[]any`.

Proposed minimum:

```json
{
  "id": "missing_unit",
  "description": "Missing or incorrect unit",
  "max_deduction": 1,
  "applies_to": ["calculation"],
  "evidence_required": false
}
```

Priority: P2.

Recommended default: `[]`.

Validation:

- `id` required.
- `max_deduction >= 0`.
- Deductions should not drive score below zero.
- Deduction ids should be stable.

Usage:

- Auditable score reductions.
- Analytics on common mistakes.
- Teacher review explanation.

Risk:

- Untyped deductions can be interpreted differently by model, verifier, and UI.

Decision:

Keep current `[]any` readable. Add typed validation only after platform UI/API can author and display it.

### 5.8 Typed `examples`

Current platform field exists as `[]any`.

Proposed minimum:

```json
{
  "id": "full_credit_example",
  "answer": "Example answer text",
  "expected_score": 5,
  "notes": ["why this earns full credit"]
}
```

Priority: P2.

Recommended default: `[]`.

Validation:

- `id` required.
- `expected_score` should be within `[0, max_score]` when present.
- Example answer text must be synthetic or approved for use.

Usage:

- Prompt examples.
- Eval seed cases.
- Teacher calibration.

Risk:

- Real student answers must not be copied into examples unless anonymized and approved.

Decision:

Use synthetic or teacher-authored examples first.

### 5.9 `rubric_schema_version`

Type: `string`.

Recommended default:

- Missing means legacy platform rubric shape.
- New extended rubrics can use a value such as `rubric-schema-v2`.

Usage:

- Compatibility converters.
- UI conditional rendering.
- Eval report grouping.

Risk:

- Version labels without validation can create false confidence.

Decision:

Add only with concrete validation rules and migration docs.

### 5.10 `rubric_hash`

Type: `string`.

Recommended default:

- Generated in lab reports now.
- Not required in platform storage until a main-platform story approves it.

Usage:

- Reproducibility.
- Eval replay.
- Detecting scoring drift when `version` was not changed correctly.

Risk:

- Hashes depend on canonical serialization. Inconsistent serialization creates noisy diffs.

Decision:

Keep hash computation lab-side until platform has a canonical rubric serializer.

### 5.11 `allow_partial_total`

Type: `boolean`.

Recommended default: `false`.

Usage:

- Rare rubrics where point totals do not equal max score because of alternate paths or capped scoring.

Risk:

- Overuse can hide bad rubric authoring.

Decision:

Do not expose as a casual UI toggle. Require reviewer intent and validation notes.

## 6. Backward Compatibility Rules

1. Existing platform rubrics remain valid.
2. Missing `points[].aliases` normalizes to `[]` in lab/eval conversion.
3. Missing `points[].evidence_required` must be reported as a compatibility default, not silently persisted.
4. Missing `equivalent_answers` normalizes to `[]` for lab/eval conversion.
5. Missing `scoring_notes` normalizes to `[]`.
6. Existing untyped `deductions` and `examples` remain readable.
7. Extended fields must not be required by production APIs until the frontend can author them.
8. Extended rubric validation can be stricter in lab than in the current platform.
9. Question score, rubric max score, and point totals should continue to be cross-checked.
10. Rubric version must change when extended scoring semantics change.

## 7. Validation Proposal

### Legacy Validation

Legacy platform validation should continue to require:

- rubric exists when required by question type;
- `max_score > 0`;
- point ids are present and unique;
- point scores are positive;
- point score total matches `max_score` unless a future exception is explicitly enabled;
- rubric max score matches question score.

### Extended Validation

Extended validation should additionally require:

- `aliases` arrays contain strings;
- `evidence_required` is boolean;
- essay/discussion rubrics declare `dimensions`;
- calculation rubrics declare `steps`;
- typed deductions include `id` and `max_deduction`;
- examples with expected scores stay within score bounds;
- scoring notes are strings;
- extended fields are included in version/hash computation.

### Pilot Gate Validation

Before a real model pilot, every pilot rubric should prove:

- old rubrics can still load;
- new rubrics validate;
- score totals are deterministic;
- essay/discussion dimensions exist;
- calculation steps exist;
- evidence-required behavior is tested;
- aliases affect matching/eval without adding score;
- no examples or scoring notes contain private student data.

## 8. Migration Strategy

### Phase 1: Lab-Only Proposal

Current story. Document the extension fields and compatibility defaults. No runtime change.

### Phase 2: Lab Fixtures

Add example extended rubrics in `lab/examples` and tests that prove old/new rubric normalization behavior. This can happen without touching the platform.

### Phase 3: Platform Contract Proposal

In the main-platform conversation, propose API and frontend type changes. Keep the old fields readable and add extended fields behind compatibility handling.

### Phase 4: Database/API Migration

Only after contract approval, migrate storage/API shapes in the main platform. Preserve old rubrics and avoid rewriting existing records unless necessary.

### Phase 5: Pilot Enforcement

Require extended fields for real-model pilot rubrics only. Do not require every historical rubric to be rewritten.

### Phase 6: Production Enforcement

After UI, validation, eval gates, teacher review, and audit exports are stable, make extended rubric validation part of production readiness.

## 9. Do Not Do Now

- Do not edit `services/api-gateway/internal/paper/types.go`.
- Do not edit `apps/web-admin/src/api/papers.ts`.
- Do not migrate database schema.
- Do not add `lab` to root workspaces.
- Do not make the platform depend on Node lab code.
- Do not require all existing rubrics to be rewritten immediately.
- Do not use unvalidated extended fields for production scoring.
- Do not duplicate objective answer-key ownership without a field ownership decision.

## 10. Open Questions

1. Should rubric-level `equivalent_answers` exist only for subjective question types, or should it become a shared question-level concept?
2. Should `dimensions` be primary score units for essays, or a grouping layer over normal points?
3. Should `steps` replace points for calculation, or be a typed projection from points?
4. Should `evidence_required` default from `required`, question type, or teacher choice?
5. Should `rubric_hash` be stored in the database or generated in reports only?
6. Should typed examples be allowed to include anonymized historical student answers after a privacy workflow exists?
7. Should `allow_partial_total` be accepted at all in platform-authored rubrics?

## 11. Future Acceptance Criteria

A future implementation of this proposal should be accepted only when:

- legacy rubrics load through old and new API clients;
- extended rubrics round-trip through backend and frontend without field loss;
- score total validation is tested;
- aliases are tested and cannot add duplicate score;
- evidence-required points are tested in evidence verification;
- essay/discussion dimensions are required for pilot rubrics;
- calculation steps are required for pilot rubrics;
- typed deductions and examples are either validated or explicitly treated as legacy `unknown[]`;
- prompt/eval generation includes only approved fields;
- rubric version/hash behavior is deterministic;
- no private answer text is introduced into examples, notes, logs, or fixtures.

## 12. Story F Self-Review

No critical issue found. The proposal keeps the current platform stable, identifies the minimum P1 fields needed before real-model pilots, and avoids turning lab-only strictness into an immediate production migration.

Remaining risk: this is still a document. It does not enforce defaults, schema migration, UI authoring, or evidence verifier behavior. Those must be handled in separate stories after the main platform conversation is ready.

## 13. Next Story Input

Recommended next story: Prompt injection defense migration proposal.

That story should define how the lab prompt-injection detector maps to the current Go subjective/evidence chain before any real model adapter is enabled.
