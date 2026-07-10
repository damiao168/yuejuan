# STORY-023 Platform Mock Output Eval Compatibility

## Spec v1

Define how current main-platform `AiGrade`, subjective `AdapterOutput`, and evidence job results can later be converted into lab eval-compatible records.

## Spec Review

The story must not implement a converter or call platform APIs. It must preserve mock markers, avoid privacy leaks, and avoid inventing missing model/rubric/evidence fields.

## Spec v2

Write a documentation-only compatibility contract covering platform grade fields, point results, evidence records, evidence job results, mock behavior, privacy, metrics, validation rules, and acceptance checklist.

## Implementation

Created `docs/platform-mock-output-eval-compatibility.md`.

## Implementation Review

The spec explicitly says platform mock outputs are only for compatibility mechanics and must not be treated as real AI quality.

## Implementation Fixes

Self-review added rejection rules for mock grader with `mock=false`, failed grade without failure reason, missing evidence job, and raw answer text in non-anonymized snapshots.

## Acceptance

Accepted as Story D. It provides input for Story E: risk flag unified naming standard.
