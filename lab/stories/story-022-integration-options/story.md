# STORY-022 Integration Options

## Spec v1

Compare three integration options: Go embedded adapter, independent grading-agent service, and lab CLI offline eval.

## Spec Review

The choice must be based on Story A field mapping and Story B gaps. It must not recommend immediate runtime integration or premature serviceization.

## Spec v2

Recommend a staged route: Option 3 now, strengthen Option 1 next, consider Option 2 later.

## Implementation

Created `docs/grading-agent-integration-options.md`.

## Implementation Review

The document compares feasibility, risk, cost, platform intrusion, release-gate value, secret management, and recommended timing.

## Implementation Fixes

Self-review added explicit "Do Not Now" decisions and next candidate stories D through I.

## Acceptance

Accepted as Story C. Current route: keep `lab` as offline eval and standard source; do not integrate into runtime yet.
