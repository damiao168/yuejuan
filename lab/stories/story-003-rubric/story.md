# STORY-003 Rubric DSL

## Spec v1

Create a structured rubric format with points, deductions, equivalent answers, examples, scoring notes, and version hashing.

## Spec Review

Rubric total score, required point fields, essay dimensions, and calculation steps must be enforceable.

## Spec v2

Reuse schema validation and add stable hash generation for version tracking.

## Implementation

Implemented `src/rubric.js` and example rubrics under `examples/rubrics`.

## Implementation Review

Hash tests prove rubric changes produce a different version hash.

## Implementation Fixes

Added explicit example rubrics for math, physics, English essay, and Chinese reading.
