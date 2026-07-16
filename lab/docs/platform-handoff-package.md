# Main Platform Handoff Package

Status: Lab-only handoff contract. It does not authorize changes outside `lab`.

## What Is Ready To Hand Off

- Suggestion-only capability matrix and fail-closed routing.
- Qwen3 4B Q4_K_M local adapter contract with loopback API key, JSON Schema, code-recomputed scores, strict alias policy, and evidence verification.
- Prompt v2 and frozen local regression evidence.
- Dataset governance, teacher double-label/adjudication, model selection, adversarial, calibration/fairness, training-decision, and Pilot readiness tooling.
- A hash-locked JorGPT external-real benchmark for evaluation-pipeline and domain-shift diagnostics; it is not deployable Gold or training data.
- `pilot-candidate-v1` manifest and explicit `NOT_READY` blockers.

## What The Main Platform Must Own

The Go API gateway remains responsible for authentication, tenant isolation, answer/Rubric loading, request idempotency, audit, persistence, review tasks, teacher overrides, appeals, final-grade publication, and rollback. The grading service or adapter owns only structured suggestions and model-operation telemetry.

For blind annotation, the platform must derive `authenticated_actor_id` from the logged-in worker, enforce assignment authorization, hide peer labels until adjudication, encrypt answer packets/submissions, and append immutable audit events. The lab JSON workflow validates assignment consistency but is not an identity provider.

## Integration Sequence

1. Reconcile canonical lower-snake-case risk flags and extended Rubric fields in Go/API contracts.
2. Add an internal adapter boundary with timeout, one retry, idempotency key, loopback/service authentication, and failed-grade semantics.
3. Run historical replay with no database grade writes.
4. Enable shadow mode: AI output is stored separately and invisible to students.
5. Add teacher accept/edit/reject capture and reason codes before any assisted workflow.
6. Export anonymized outcomes back through the governed Gold pipeline.
7. Re-run real model selection, teacher agreement, adversarial, calibration, fairness, and Pilot freeze gates.
8. Consider a limited teacher-visible pilot only after `READY_FOR_SHADOW_PILOT`.

## Forbidden Until Gates Pass

- No automatic final-grade publication.
- No essay/discussion auto-acceptance.
- No synthetic calibration confidence in production.
- No QLoRA or synthetic fine-tuning.
- No 8B deployment on the current 14GB-RAM computer.
- No public dataset import until its license entry becomes approved.

## Required Artifacts

Use `evals/pilot/pilot-candidate-v1/manifest.json` as the artifact index and `readiness-report.json` as the go/no-go source. The main project must not import Node lab code directly; translate contracts into Go tests or call a separately governed internal service.
