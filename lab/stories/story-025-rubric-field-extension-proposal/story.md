# STORY-025 Rubric Field Extension Proposal

## Spec v1

Define which rubric fields the platform should eventually add so the current simple Go/frontend rubric can support the lab grading-agent standard.

## Spec Review

The story must remain documentation-only. It must not alter Go structs, frontend API types, database schema, prompts, eval scripts, or root workspace settings. It must preserve current rubrics and avoid forcing a full rewrite before the main platform conversation is ready.

## Spec v2

Write a proposal covering current platform fields, current lab requirements, proposed extension fields, backward compatibility defaults, validation rules, migration phases, do-not-do-yet constraints, open questions, and future acceptance criteria.

## Implementation

Created `docs/rubric-field-extension-proposal.md`.

## Implementation Review

The proposal separates P1 pilot blockers (`aliases`, `evidence_required`, essay `dimensions`, calculation `steps`, `scoring_notes`) from P2/P3 improvements such as typed examples, typed deductions, schema version, hash, and partial-total exceptions.

## Implementation Fixes

Self-review added compatibility-default rules, explicit field ownership cautions for `equivalent_answers`, privacy constraints for examples/scoring notes, and a do-not-do-now section that prevents accidental platform edits.

## Acceptance

Accepted as Story F. It provides input for a future main-platform rubric contract story and for lab-only extended rubric fixtures.
