# Blind Double-Teacher Annotation Workflow

Status: Story K operational remediation specification.

## Spec v1

Generate two teacher assignments for every governed answer, collect labels, send disagreements to an adjudicator, and produce bundles accepted by the existing Gold builder.

## Spec Review

A pair count alone is not sufficient. The workflow must bind every artifact to one immutable dataset hash, assign only active teachers qualified for the sample subject, balance work without repeatedly using one pair, and keep coordinator data separate from teacher packets. A teacher packet must not disclose the peer labeler, peer assignment, peer label, aggregate score, model score, expected score, or Gold label.

Submission identity cannot be trusted from a label body alone. Assignment IDs, plan IDs, sample IDs, labeler IDs, and roles must agree with the coordinator plan. Unknown, duplicated, missing, or cross-assignment submissions must fail closed. Teacher timestamps must be recorded, but wall-clock order must not determine which label becomes authoritative.

Disagreement detection must use the existing score, point, and evidence policy. A disputed pair is pending, not invalid Gold and not complete Gold. It must receive an independent adjudicator qualified for the subject and different from both teachers. Finalization must validate the full bundle with the existing Gold validator before output.

Real annotation packets contain governed answer text and therefore belong in ignored coordinator storage, not tracked reports. Tracked evidence may contain hashes, counts, pseudonymous workload statistics, and failure codes, but no answer text or evidence excerpts.

## Spec v2

### Inputs

- A governed JSONL dataset with an approved `annotation` source/use pair.
- An `annotation-roster-v1` roster of pseudonymous active workers with roles, subjects, and grade-level qualifications.
- At least two subject-qualified workers with role `teacher` for every sample.
- At least one subject-qualified worker with role `adjudicator` who can be independent of the assigned pair.
- A fixed plan ID and deterministic assignment seed.

### Plan Output

The coordinator plan records dataset path/hash, source/evidence scope, roster hash, blind mode, assignment seed, exactly two assignments per sample, slot A/B, assigned worker, workload counts, pair counts, and every teacher-packet SHA256. It contains no labels. Pair selection minimizes workload first and repeated pair use second.

Each per-teacher packet contains only that teacher's identity, plan ID, assignment ID, bundle ID, slot, a minimized governed sample, and an empty label template. It contains no peer identity or label and no expected/model score fields. Fields used only for evaluation or prior labeling, including `expected_*`, `gold_*`, `human_rationale`, external model/judge outputs, and source participant linkage, are stripped before packet creation.

### Teacher Merge

Every submission references one assignment, names an `authenticated_actor_id` supplied by a trusted upstream, and contains one teacher label. Merge validates assignment ownership, actor/labeler identity, plan/sample/bundle identity, role, label completeness, point totals, and answer-local evidence. Duplicate submissions fail. Missing submissions remain explicit pending assignments.

Pairs with no policy disagreement become complete bundles. Pairs with score, point, or evidence disagreement become pending bundles and receive one independent adjudication assignment. No disputed bundle enters completed output before adjudication.

### Adjudication

The adjudicator packet contains the sample and both source labels because resolving disagreement requires them. It does not contain model output or expected/Gold labels. The adjudicator label must identify both source label IDs and pass the same point/evidence validation. Finalization accepts only the assigned independent adjudicator and runs the full existing bundle validator.

### State Machine

`planned -> teacher_pending -> teacher_complete -> adjudication_pending|complete -> complete`

Any identity, hash, qualification, schema, privacy, or evidence failure moves the operation to `rejected`; there is no automatic score fallback.

## Security Boundary

The lab workflow cannot authenticate a person from a JSON field. In production, the main platform must derive `authenticated_actor_id` from its authenticated session, authorize assignment access, encrypt packets and submissions in transit and at rest, and retain an immutable audit event. The offline lab CLI treats that actor ID as trusted operator input and proves only assignment consistency, blindness of generated packets, and data integrity hashes.

## Acceptance

1. Every sample receives exactly two distinct qualified teachers.
2. Workload differs by at most one assignment when all teachers share eligibility.
3. Teacher packets contain no peer or expected/Gold/model/prior-rationale information and are individually hashed.
4. Dataset and roster hashes are verified before merge/finalize.
5. Unknown, duplicate, cross-labeler/actor, malformed, and incomplete submissions fail closed.
6. Agreement completes without adjudication; every policy disagreement creates one queue item.
7. Adjudicators are qualified and independent from both teachers.
8. Final bundles pass `validateAnnotationBundle` and can enter the existing Gold builder.
9. Tracked workflow reports contain no student answer or evidence excerpts.
10. Synthetic end-to-end evidence passes while the real Pilot gate remains blocked until 500 real bundles exist.
