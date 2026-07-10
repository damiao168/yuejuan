# High-Risk Fixes

## Finding

The first eval run had `review_trigger_recall = 0.8`. Some non-empty answers with insufficient or ambiguous evidence were not forced into human review.

## Fix

`src/adapters/mockAdapter.js` now marks these cases for review:

- non-empty answer with zero matched rubric evidence
- uncertainty phrases such as `maybe`, `not sure`, `可能`, `大概`, or `不确定`

The output adds conservative flags:

- `INSUFFICIENT_EVIDENCE`
- `AMBIGUOUS_ANSWER`
- `HUMAN_REVIEW_REQUIRED`

## Result

After the fix:

- `npm.cmd test`: 43 passing tests
- `npm.cmd run eval`: `review_trigger_recall = 1`
- `npm.cmd run gate:dev`: passed
- `node scripts/run-regression.js`: 1 case, 0 failures
