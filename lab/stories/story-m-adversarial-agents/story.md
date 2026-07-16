# Story M Three Offline Adversarial Agents

## Spec v1

Add prompt-injection, Rubric exploit, and evidence/OCR adversarial agents.

## Spec Review

Adversaries must remain offline, include benign controls, measure false positives, and test deterministic validators rather than ask models to debate. Existing Chinese regexes were corrupted and evidence IDs were not actually linked to evidence records.

## Spec v2

Implement exactly three deterministic attack generators, repair Unicode normalization, enforce Rubric alias uniqueness and evidence ID integrity, and gate both attack recall and control false-positive rate.

## Implementation

Added three adversarial generators, evaluator, gate config, CLI, tests, Unicode-safe injection detector, stricter Rubric checks, stronger evidence links, and suite documentation.

## Implementation Review

The starter suite detected 6/6 attacks per agent with zero observed false positives. The dev gate passed and Pilot failed on attack breadth. Review found that Pilot lacked a minimum benign-control count, which could make a false-positive rate based on one control look meaningful.

Post-expansion review found a fail-open edge case: an empty report array passed because no per-agent loop ran, and reported aggregates were trusted without reconciliation to case results. A missing or manually altered report could therefore satisfy the standalone gate despite the normal generator producing valid data.

## Implementation Fixes

Added minimum controls per agent to both gates, set pilot to 25 attacks and 25 controls per agent, and added gate regression coverage. Replaced corrupted Chinese normalization and enforced evidence ID and alias integrity.

Expanded every agent to 30 attacks across six attack families and 25 benign controls across five control families. The expanded suite achieved 100% attack recall and 0% control false-positive rate for all three agents, so the deterministic Pilot adversarial gate now passes.

Pinned the three required agent identities, made the gate reject missing, duplicate, unknown, and malformed agent reports, and recomputed all counts, family breadth, recall, and false-positive rates from unique case results. Added regression coverage for empty/missing reports, duplicate agents, and tampered aggregate fields.

## Acceptance

Accepted for the deterministic Pilot adversarial gate. Each of the three offline agents exceeds the 25-attack/25-control breadth threshold and passes the recall and false-positive limits. This remains synthetic validator evidence and does not establish real-data or model-level Pilot readiness.
