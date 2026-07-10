# STORY-019 High-Risk Fixes

## Spec v1

Fix high-risk issues found during implementation review.

## Spec Review

The key issue was review-trigger recall below the desired threshold in synthetic eval.

## Spec v2

Make the mock adapter more conservative without pretending to be smarter than it is.

## Implementation

Updated `src/adapters/mockAdapter.js` to force review for zero-evidence non-empty answers and obvious uncertainty phrases.

## Implementation Review

Eval improved `review_trigger_recall` from `0.8` to `1`.

## Implementation Fixes

Documented the fix in `docs/high-risk-fixes.md` and this story.
