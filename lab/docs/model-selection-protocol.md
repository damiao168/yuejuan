# Fixed-Set Local Model Selection

Status: Story L accepted as the preliminary synthetic local-baseline decision.

## Fixed Comparison

All candidates use `evals/model-selection/fixed-synthetic-v1.jsonl`, the same structured prompt, CPU-only six-thread runtime, 4096-token context, temperature zero, one concurrent request, and the same schema/evidence checks. Expected labels are read only after model output and never enter the prompt.

Candidates are deterministic alias rules, Qwen3 4B Q4_K_M, and Qwen3 8B Q4_K_M. Reports include dataset SHA256, MAE, exact/adjacent agreement, evidence and schema validity, failures, first-pass rate, and p50/p95 latency.

## Decision Rule

On this computer, 8B replaces 4B only if it materially improves quality or failure rate enough to justify memory and latency. A synthetic tie goes to 4B. Neither model may receive a product-quality claim until the governed real Gold test set exists.

## Commands

```powershell
node scripts/run-model-selection.js --candidate rules_alias --out evals/reports/model-selection-rules_alias.json
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/prepare-model-candidate.ps1 -Candidate qwen3_8b
```

Local model runs require starting the matching server candidate before running `run-model-selection.js`.

## Measured Comparison

| Candidate | Completion | MAE on completed | Exact | Evidence/schema | p95 latency | Decision |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Alias-rule reference | 4/4 | 0.00 | 100% | 100% | 4 ms | Deterministic reference only; fixtures are alias-aligned |
| Qwen3 4B Q4_K_M | 4/4 | 0.50 | 75% | 100% | 106,768 ms | Selected local LLM |
| Qwen3 8B Q4_K_M | 1/4 | 0.00 | 100% | 100% on one success | 108,322 ms | Rejected: three failures |

The 4B error was a serious overgrade: an answer containing correct transformation `2x=6` but incorrect result `x=4` received 3/4 instead of 1/4. This proves evidence presence alone does not prove semantic correctness and is a required Story O regression.

The 8B server used about 6.48 GB working set at load and left about 1.06 GB physical memory free. It failed one sample after invalid evidence links and two samples after two 120-second timeouts. Its completed-sample MAE is not comparable because completion was only 25%.

## Decision

Use Qwen3 4B Q4_K_M as the only local LLM baseline on this computer. Keep deterministic scoring for eligible rule questions. Do not use or auto-load 8B on this hardware. The downloaded 8B file remains under D-drive lab storage solely for future stronger-hardware retesting.

## External Evidence Scope

Model-selection reports now bind the source ID, evidence scope, evaluated dataset hash, full-source hash, and whether the complete dataset was evaluated. Smoke subsets cannot become eligible candidates. Pilot qualification additionally requires `pilot_in_domain_gold`, the same source as the governed real Gold manifest, and an exact match to its frozen test SHA256. An external-real decision cannot unlock Pilot even if Qwen3 4B wins it.

The governed JorGPT test set provides an external semantic baseline. Rules scored MAE `4.383/10` over all 452 records. A three-question 4B smoke scored MAE `1.333/10`, but one output failed evidence grounding and p95 latency reached 189.9 seconds. This is diagnostic evidence, not a model-selection decision.
