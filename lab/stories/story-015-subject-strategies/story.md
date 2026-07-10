# STORY-015 Subject Strategies

## Spec v1

Create subject-specific grading strategies for nine subjects.

## Spec Review

Strategies must state review policy, risk flags, scoring constraints, and prompt template.

## Spec v2

Implement a registry that can be used by future graders and prompts.

## Implementation

Implemented `src/subjects.js` and `docs/subject-grading-strategies.md`.

## Implementation Review

Tests cover English essay review, math step evidence, physics concept evidence, and Chinese length-only risk.

## Implementation Fixes

Added strategy entries for chemistry, biology, history, politics, and geography.
