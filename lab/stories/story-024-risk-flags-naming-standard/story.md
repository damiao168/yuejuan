# STORY-024 Risk Flags Naming Standard

## Spec v1

Define a unified risk flag naming standard for lab, platform output snapshots, release gates, and future review routing.

## Spec Review

The standard must respect the current Go platform's lower snake case values, preserve lab uppercase enums as lab-internal for now, avoid database/API changes, and support compatibility mapping.

## Spec v2

Use lower snake case as canonical platform-facing names. Define lab-to-canonical and platform-to-canonical mapping, compatibility rules, gate metrics, review routing, migration phases, and open questions.

## Implementation

Created `docs/risk-flags-naming-standard.md`.

## Implementation Review

The document keeps existing Go flags stable, does not rename code values, and blocks the dangerous shortcut of collapsing all causes into `human_review_required`.

## Implementation Fixes

Self-review added unknown-flag preservation, mapping coverage metric, and explicit do-not-do-yet constraints.

## Acceptance

Accepted as Story E. It provides input for future platform snapshot converter tests, release gate CI planning, and prompt-injection migration.
