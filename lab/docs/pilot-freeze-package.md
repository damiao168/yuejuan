# Pilot Freeze Package

Status: Story Q accepted with readiness decision `NOT_READY`.

## Package Contents

The package freezes the suggestion-only capability profile, llama.cpp build, Qwen3 4B path/bytes/SHA256, Prompt versions and checksums, regression Rubric hashes, data/report hashes, training decision, and confidence policy. It references the D-drive GGUF but does not copy the model into the package.

## Readiness Semantics

`READY_FOR_SHADOW_PILOT` means AI and teachers run in parallel without publishing AI grades. It never means autonomous grading. Any failed requirement produces `NOT_READY` with explicit blockers.

Required gates are capability safety, model integrity, frozen regression, governed real Gold data, real-data model selection, teacher agreement, adversarial breadth, calibration/fairness, and the local single-request latency ceiling. Fine-tuning is not required if a prompted base model eventually passes real gates.

## Acceptance

The package must verify the complete 4B SHA256, hash every referenced artifact, forbid grade publication, keep confidence disabled, and truthfully report all current pilot blockers.

## Current Freeze

`pilot-candidate-v1` is stored under `evals/pilot/pilot-candidate-v1` on D drive. The 2,497,280,256-byte Qwen3 4B file matched SHA256 `7485fe6f11af29433bc51cab58009521f205840f5b4ae3a32fa7f92e8534fdf5`.

Five checks pass: capability safety, model integrity, 3/3 local regression, the expanded deterministic adversarial gate, and p95 local latency 106,768 ms under the 120,000 ms single-request ceiling. Four checks block Pilot: no governed in-domain real Gold dataset, no model selection bound to that Gold test hash, teacher reliability gate failure, and calibration/fairness gate failure.

The governed JorGPT source does not alter readiness. Its manifest is `external_real_benchmark`, its single-teacher holistic labels are not Gold agreement evidence, and its three-record 4B run is marked `complete_dataset=false`. Pilot model selection now requires the exact `pilot_in_domain_gold` source and frozen test SHA256.

Final-grade publication is false, confidence runtime is disabled, training decision is `do_not_train`, and readiness is `NOT_READY`.
