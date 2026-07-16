# Gold Annotation, Adjudication, And Reliability

Status: Story K accepted for workflow mechanics; real teacher reliability evidence remains unavailable.

## Blind Operations

The operational workflow is specified in `docs/blind-annotation-workflow.md`. A coordinator first creates exactly two subject/grade-qualified teacher assignments per governed sample. Deterministic selection balances workload and repeated pairs while preserving an independent adjudicator option. Per-teacher packets are individually hashed, use a whitelist-minimized sample, and contain no peer identity, peer label, expected/Gold/model score, prior rationale, or participant linkage.

Teacher submission merge binds plan, dataset, roster, assignment, bundle, sample, authenticated actor, labeler, and point-level evidence. Missing work remains pending; malformed, duplicated, spoofed, cross-assignment, or packet-tampered input fails closed. Policy disagreements generate one subject-qualified adjudication task assigned to a worker independent of both teachers. Finalization rechecks the dataset sample-ID set hash and the existing full bundle validator.

The lab cannot authenticate a person from JSON. Production must supply `authenticated_actor_id` from the main platform session and own authorization, encryption, and immutable audit events.

## Annotation Unit

Every answer receives exactly two independent pseudonymous teacher labels. Each label includes total score, confidence, rationale, and a complete decision for every Rubric point: status, point score, and answer-local evidence. Point totals must equal the label score.

Any score, point-status, point-score, or evidence disagreement requires an independent adjudicator. The adjudicator must reference both source label IDs and cannot be either original labeler. Gold records preserve this provenance.

## Reliability Metrics

The report computes quadratic weighted Kappa on scores normalized into 20 bins, exact agreement, 10%-of-max adjacent agreement, normalized MAE, label-order score bias, Rubric-point agreement, and adjudication rate.

## Gates

The development gate proves mechanics with three synthetic bundles. The pilot gate requires at least 500 real double-scored answers, QWK at least 0.80, normalized MAE at most 0.08, and point agreement at least 0.85. Synthetic reports can never satisfy real-data readiness.

## Command

```powershell
npm.cmd run annotations:gold -- --input evals/annotations/synthetic-double-label.jsonl --out evals/gold/synthetic.jsonl --report evals/reports/annotation-reliability.json --gate dev
npm.cmd run annotations:workflow:demo
```

For a real governed source, use `npm.cmd run annotations:workflow -- --action plan|merge|finalize ...`. Any output containing real answer text is forced under ignored `lab/.downloads`. `examples/annotation-roster.example.json` documents the roster contract.

## Acceptance

The synthetic workflow must validate independent labels, require adjudication for disagreement, reject invalid evidence, produce Gold provenance, pass the development gate, and fail the pilot minimum-sample gate.

## Execution Result

Three synthetic bundles produced three Gold records. Pre-adjudication metrics were QWK `0.80`, exact agreement `0.667`, normalized MAE `0.167`, point agreement `0.833`, and adjudication rate `0.333`. The development mechanics gate passed. The pilot gate failed on sample count, normalized MAE, and point agreement, which is the required outcome for a tiny synthetic-only set.

These numbers describe fixture behavior, not teacher reliability. Story Q remains blocked from pilot readiness until at least 500 governed real answers are independently double-scored.

The blind workflow demo planned six assignments for three samples, generated hash-valid blind teacher packets, merged six submissions, completed two agreed bundles, routed one disagreement to an independent adjudicator, and finalized three Gold-valid bundles. The development gate passes and the Pilot gate remains blocked by the same real-evidence requirements.
