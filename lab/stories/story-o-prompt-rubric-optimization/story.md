# Story O Failure-Driven Prompt And Rubric Optimization

## Spec v1

Fix the observed 4B math overgrade and Chinese feedback-language failure without degrading passing controls.

## Spec Review

Prompt wording alone cannot reliably prevent keyword-based overgrading. Numeric/result points need explicit deterministic semantics, baseline prompt hashes must be frozen, and feedback cannot remain model-owned when review language is mandatory.

## Spec v2

Add semantic contradiction instructions, explicit strict-alias Rubric policy, code-side downgrade and evidence removal, code-owned language feedback, and a three-case real-model regression with full and partial controls.

## Implementation

Added prompt v2, calculation prompt v2, Rubric match policy, strict post-validation, code-owned feedback, frozen regression data, runner, tests, and optimization documentation.

## Implementation Review

All three real 4B cases passed on the first attempt. The former overgrade changed from 3/4 to 1/4, full calculation stayed 4/4, and Chinese semantic grading stayed 2/2. Schema, evidence, exact score, and feedback language all passed. Review found that deterministic Chinese feedback still exposed English point IDs and `none`.

External JorGPT smoke review found one reproducible evidence-grounding failure on `jorgpt-en-0007`: the model scored 6 versus teacher 7, but cited text not present in the answer. One run repaired invalid JSON and the diagnostic rerun repaired a timeout. Prompt v2 already requires exact copied evidence, so the case is frozen as an open external regression rather than being used alone to justify Prompt v3 or unsafe evidence rewriting.

## Implementation Fixes

Changed feedback to Rubric descriptions with language-specific empty labels and added a formatting regression. Recorded v1/v2 prompt hashes and retained the observed 78–94 second latency range.

## Acceptance

Accepted. The real 4B regression is 3/3 with no retries or evidence failures, and deterministic feedback formatting tests pass.
