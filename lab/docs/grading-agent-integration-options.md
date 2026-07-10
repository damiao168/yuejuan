# Grading Agent Integration Options

Status: Story C accepted after self-review.

Scope: documentation only. This document chooses a staged integration route. It does not implement APIs, adapters, services, workspace changes, or main-platform changes.

## 1. Executive Recommendation

Current decision:

Use **Option 3: lab CLI offline eval** now.

Do not connect `lab` to the production runtime yet. Do not add `lab` to the root workspace. Do not replace the Go gateway subjective grading path. Do not create a new `ai-services/grading-agent-service` implementation yet. Do not connect a real model adapter until P0 gaps from `current-grading-gap-analysis.md` are addressed.

Recommended route:

1. Phase 1: use `lab` for offline eval, schema baseline, release gates, and regression samples.
2. Phase 2: keep Go api-gateway embedded grading, but align it with lab rules: schema strictness, risk flags, prompt injection defense, evidence semantics, and release gate outputs.
3. Phase 3: consider `ai-services/grading-agent-service` only after a real adapter is stable, eval gates pass, and independent model operations are worth the deployment complexity.

## 2. Option 1: Continue Go API Gateway Embedded Adapter

### Description

The Go api-gateway continues to own subjective grading. `lab` remains outside the runtime path and acts as the source for eval standards, schema expectations, release gates, regression samples, and future proposals.

### Advantages

- Lowest disruption to current platform.
- Preserves existing auth, tenant isolation, audit, database writes, review tasks, and final grade boundaries.
- Keeps frontend APIs unchanged.
- Avoids new service deployment and model secret routing.
- Good short-term bridge while P0 safety gaps are analyzed.

### Disadvantages

- Lab logic cannot be imported directly because it is Node ESM and backend is Go.
- Go implementation can drift from lab standards unless mapped tests/gates are added.
- Prompt registry and eval runner remain outside runtime.
- Real model support must be implemented in Go or behind a Go adapter boundary.

### Suitable Stage

Short to medium term, especially while the platform still uses mock subjective grading and needs safer alignment before real models.

### Risks

- Standards drift if lab docs are not translated into Go tests.
- Pressure to copy JS logic into Go without preserving semantics.
- Existing API may accumulate model-specific concerns if not kept behind adapter boundaries.

### Implementation Cost

Low initially, medium as lab rules are translated into Go tests, guardrails, and schema fields.

### Main-Project Intrusion

Low to medium when future changes are made; none in the current documentation-only stage.

### Effect On Real Model Access

Useful only after P0 gaps are closed. It can support a first pilot if Go receives prompt injection guards, release-gate preflight, privacy-safe logging, and stronger schema mapping.

### CI/Eval Impact

Requires later CI wiring to run lab eval outputs or Go-equivalent tests. Not available today.

### Short-Term Fit

Good, but not enough by itself. It should be combined with Option 3.

## 3. Option 2: Independent `ai-services/grading-agent-service`

### Description

The lab eventually migrates into a real grading agent service. The Go api-gateway handles platform concerns: auth, tenant isolation, business orchestration, persistence, review tasks, final grades, and audit. The service handles grading, evidence verification, prompt/model adapters, eval support, and release-gated model versions.

### Advantages

- Clean model-service boundary.
- Easier to keep prompt registry, model adapters, evals, and release gates together.
- Can use a runtime better suited for model SDKs if needed.
- Decouples model iteration from Go platform releases.
- Allows independent monitoring for model latency, cost, parse failures, and quality metrics.

### Disadvantages

- Highest operational complexity.
- Requires new deployment, health checks, auth between services, timeout/retry policy, request idempotency, secret management, observability, and rollback.
- Current lab API differs from platform `/api/v1/...` APIs.
- Schema mapping is not code-proven.
- Release gates are not CI-bound.
- Real adapter has not been evaluated on anonymized human-labeled data.

### Suitable Stage

Medium to long term, after real adapter evaluation and platform mapping are mature.

### Risks

- Premature serviceization can make a mock/experimental system look production-ready.
- More failure modes: network, timeout, partial writes, inconsistent retries, model-provider outage.
- More places for private answer text to leak if logging and tracing are not designed carefully.

### Implementation Cost

High.

### Service Boundary

Potential future boundary:

- API gateway: permission, tenant, request lifecycle, answer/rubric loading, grade persistence, review tasks, final score.
- Grading service: structured model call, prompt registry, schema validation, evidence suggestions, risk flags, adapter metadata, eval support.

### API Contract

`lab/docs/platform-integration-contract.md` can be the seed, but must be reconciled with existing platform APIs. `/grading/...` should be treated as an internal service API, not a replacement for frontend-facing `/api/v1/...` endpoints.

### Deployment Complexity

High: requires service discovery, config, model credentials, network policy, rate limits, retries, and failure fallbacks.

### Monitoring Requirements

At minimum:

- model latency
- timeout rate
- parse failure rate
- schema failure rate
- evidence verification failure rate
- review-trigger recall
- cost by model/version
- model/prompt/rubric versions
- privacy-safe request tracing

### Model Secret Management

Model API keys must live only in service-side secret management, never frontend, logs, eval reports, or raw issue exports.

### Failure Degradation

On service failure, the platform should create a failed AI grade or review task, never a valid score.

### Short-Term Fit

Poor. Do not choose now.

## 4. Option 3: Lab CLI Offline Eval

### Description

Keep `lab` independent. Use synthetic datasets now and anonymized human-labeled datasets later. Run CLI eval, release gates, and regression checks before any real adapter is promoted. The lab produces reports only and does not participate in online scoring.

### Advantages

- Zero runtime intrusion.
- Keeps current platform stable.
- Best fit for the current state because `lab` already has eval runner, synthetic data, release gates, and regression samples.
- Allows schema, rubric, prompt, evidence, and risk-flag decisions to mature before integration.
- Creates a quality baseline before real model access.
- Makes bad cases reproducible.

### Disadvantages

- Does not improve production grading immediately.
- Requires exporting or simulating adapter outputs for platform comparison later.
- Offline metrics can drift from production unless datasets and mappings stay current.
- Synthetic data can overstate readiness.

### Suitable Stage

Current stage.

### Risks

- Team may overtrust mock/synthetic metrics.
- If mapping is not maintained, lab gates may not reflect platform behavior.

### Implementation Cost

Low.

### Main-Project Intrusion

None now.

### Value For Real Model Readiness

High. It creates the baseline and release gate that should be passed before real model rollout.

### Current Fit

Best current option.

## 5. Decision Table

| Criterion | Option 1: Go Embedded | Option 2: Independent Service | Option 3: Lab CLI Offline Eval |
| --- | --- | --- | --- |
| Current feasibility | Medium | Low | High |
| Runtime risk | Medium | High | Low |
| Implementation cost | Medium | High | Low |
| Main-platform intrusion now | Low if deferred | High | None |
| Uses existing Go workflow value | High | Medium | High, indirectly |
| Supports eval/release gate now | Low without work | Medium after service work | High |
| Real model readiness | Medium after P0 fixes | High long term | High as preflight, not runtime |
| Deployment complexity | Low/Medium | High | Low |
| Secret management burden | Medium | High | Low until real adapter eval |
| Recommended timing | Short/mid term alignment | Long term | Now |

## 6. Why Not Option 2 Now

Do not create an independent grading-agent service now because:

1. Lab and main APIs are not aligned.
2. Schema mapping is documented but not code-proven.
3. Release gates are not connected to CI.
4. A real model adapter has not passed enough evals.
5. The platform already has a Go embedded grading and evidence chain.
6. Serviceization adds deployment, monitoring, secret management, retries, timeout, and fallback complexity.
7. The current lab dataset is synthetic and mock-heavy.
8. Premature serviceization can make experimental mock behavior look like production capability.

## 7. Final Route

### Phase 1: Adopt Option 3 Now

Use `lab` as:

- offline eval runner
- schema baseline
- release gate source
- regression set
- synthetic dataset manager
- future human-label format validator
- prompt/rubric standard source

No runtime integration.

### Phase 2: Strengthen Option 1

Keep Go api-gateway embedded grading, but align it with lab outputs:

- canonical risk flags
- prompt injection defense
- rubric version traceability
- evidence-to-rubric-point traceability
- stricter schema validation
- privacy-safe model logging rules
- release gate reports before enabling real adapters

### Phase 3: Consider Option 2 Later

Create `ai-services/grading-agent-service` only when:

- real adapter quality has passed gates;
- enough anonymized human-labeled data exists;
- the platform needs independent model iteration;
- API mapping is stable;
- service operations and secret management are ready.

## 8. Current Next Stories

Do not implement these now. They are the next planning candidates:

1. Story D: lab eval runner compatibility spec for current platform mock output.
2. Story E: risk flag unified naming standard.
3. Story F: Rubric field extension proposal.
4. Story G: Prompt injection defense migration proposal.
5. Story H: Release Gate CI integration plan.
6. Story I: `ai-services/grading-agent-service` migration plan.

## 9. Current Do / Do Not

### Do Now

- Keep `lab` independent.
- Use lab eval and release gates offline.
- Expand mapping and gap analysis into future specs.
- Treat lab as the standard source for real adapter readiness.

### Do Not Now

- Do not add `lab` to the root workspace.
- Do not modify `api-gateway`.
- Do not modify frontend calls.
- Do not create runtime code in `ai-services/grading-agent-service`.
- Do not replace `MockLLMAdapter` with a real model.
- Do not expose `/grading/...` endpoints as production APIs.
- Do not treat synthetic/mock metrics as production readiness.

## 10. Story C Self-Review

No critical issues found after review. The document is intentionally conservative: it values the existing Go chain, keeps the lab out of runtime, and recommends offline eval first.

Remaining risk: this decision must be revisited after human-labeled anonymized data and a real adapter evaluation exist.
