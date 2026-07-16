# Story K Gold Annotation And Agreement

## Spec v1

Create a double-teacher labeling format with adjudication and inter-rater reliability metrics.

## Spec Review

Total scores alone are not auditable. Different max-score questions cannot be compared with raw errors, and disputed labels must not silently enter Gold. Labelers and adjudicators also need independence checks.

## Spec v2

Require complete Rubric-point decisions and answer-local evidence, normalize reliability metrics by max score, force adjudication on score/point/evidence disagreement, preserve provenance, and separate synthetic development gates from real pilot gates.

## Implementation

Added annotation policy, bundle validator, Gold builder, reliability metrics, gate checker, CLI, synthetic double-label fixtures, tests, and protocol documentation.

## Implementation Review

Three synthetic bundles produced Gold output, one adjudication, QWK 0.80, normalized MAE 0.167, and point agreement 0.833. The development gate passed and the pilot gate correctly failed. Review also found that malformed labels without point decisions could reach disagreement comparison and throw instead of returning validation errors.

The governed JorGPT external benchmark adds 3,041 real answers but exposes only one corrected holistic teacher grade per answer and no original point-level teacher Rubric. Review confirmed that it cannot be counted as double-scored Gold or reliability evidence.

Operational review found that the original implementation accepted completed bundles but could not create blind assignments, prove packet integrity, bind submissions to an authenticated actor, track missing work, or assign independent adjudication. The first workflow implementation also used `map(structuredClone)`, which Node 24 interpreted as an invalid options argument. A second review found that final bundle count alone did not prevent replacing one sample with a duplicate.

## Implementation Fixes

Guarded disagreement calculation behind structural label validation and added a malformed-label regression test. Recorded the synthetic-only limitation and 500-real-double-score requirement.

Added the reviewed blind workflow specification, qualified balanced pair planner, whitelist-minimized hash-bound packets, coordinator/teacher separation, strict submission merge, missing-work state, independent adjudicator packets, and final sample-ID-set binding. Replaced the unsafe clone callback, added generic plan/merge/finalize CLI actions, forced real answer artifacts under ignored storage, and added a no-content tracked demo report.

## Acceptance

Accepted for executable workflow mechanics. Core and disk-protocol tests cover blindness, balancing, packet tampering, actor spoofing, missing submissions, plan/state tampering, adjudicator independence, and Gold finalization. The synthetic demo completes 3/3; Pilot still requires 500 governed real double scores.
