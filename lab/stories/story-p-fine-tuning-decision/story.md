# Story P Fine-Tuning Decision And Optional QLoRA

## Spec v1

Decide whether prompt optimization is insufficient and, if so, define a reproducible QLoRA run.

## Spec Review

Training volume alone is insufficient. Data rights, privacy, group split, teacher reliability, real baseline quality, frozen test integrity, and a demonstrated prompt plateau must all pass. Synthetic success cannot authorize fine-tuning.

## Spec v2

Create a machine-readable no-train/eligible gate from current reports, require 5,000 real training and 500 real test records, and provide a QLoRA protocol that cannot execute while evidence is missing.

## Implementation

Added fine-tuning gates, evidence collector, decision engine, CLI, tests, and QLoRA protocol.

## Implementation Review

The current report correctly returned `do_not_train` with nine unmet conditions. Review found that prompt plateau count was hard-coded and that latest regression success was not an explicit gate, which would prevent or weaken future automatic eligibility.

## Implementation Fixes

Added versioned prompt-optimization history, trailing no-improvement calculation, and a latest-regression hard gate. Re-ran the decision: Prompt v2 is a material improvement, so plateau count remains zero and training remains forbidden.

## Acceptance

Accepted. Current evidence forbids training, three tests cover rejection and future eligibility, and no training artifacts were created.
