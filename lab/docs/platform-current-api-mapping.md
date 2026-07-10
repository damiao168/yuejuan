# Platform Current API Mapping

Status: Story A accepted after self-review.

Scope: documentation only. This document does not propose runtime integration and does not modify `services/`, `apps/`, `ai-services/`, `packages/`, `infra/`, or the root workspace.

Note: the pasted plan uses `lab/grading-agent-lab/` as a logical path. The actual repository path for this work is `lab/`.

## 1. Background

The `lab` folder is an isolated Grading Agent research and evaluation lab. It owns schemas, prompts, synthetic evals, release gates, regression samples, model adapter boundaries, and policy documents.

The main platform already has a Go API grading chain in `services/api-gateway`. It includes objective rule grading, subjective mock LLM grading, evidence verification, review tasks, and final grade workflows.

There is currently no runtime link between `lab` and the main platform. No main-platform code imports `lab`, the root workspace does not include `lab`, and the frontend calls existing `/api/v1/...` endpoints rather than `lab` APIs.

This story only maps fields and contracts. It does not implement integration.

## 2. Current Main-Platform Modules

| Area | Path | Key Files | Current Capabilities |
| --- | --- | --- | --- |
| Objective rule grading | `services/api-gateway/internal/grading` | `engine.go`, `types.go`, `handlers.go` | Supports `single_choice`, `true_false`, `multiple_choice`, `fill_blank`, `numeric`; emits `matched_points`, `missing_points`, `evidence`, `confidence`, `needs_human_review`, `mock=false`. |
| Subjective AI grading | `services/api-gateway/internal/subjective` | `types.go`, `adapter.go`, `handlers.go`, `validation.go` | Supports `short_answer`, `calculation`, `essay`, `discussion`; currently uses `MockLLMAdapter`; emits `mock=true`, `model_version`, `prompt_version`, low confidence, forced human review. |
| Evidence verification | `services/api-gateway/internal/evidence` | `engine.go`, `types.go`, `handlers.go` | Checks `suggested_score <= max_score`, rubric point references, matched score total, evidence non-empty, evidence text in answer text, bbox inside segment bbox, and low OCR review. |
| Web admin API | `apps/web-admin/src/api/review.ts` | `review.ts` | Calls `POST /api/v1/answer-segments/{id}/subjective-ai-grade` and `POST /api/v1/ai-grades/{id}/verify-evidence`. |

## 3. Current Lab Modules

| Lab Module | Purpose |
| --- | --- |
| `src/schemas/gradingSchema.js` | Validates `GradingInput`, `Rubric`, `Evidence`, `GradingOutput`, score bounds, evidence requirements, mock marking, and review triggers. |
| `src/rubric.js` | Validates rubric DSL and computes stable rubric hashes. |
| `src/adapters/mockAdapter.js` | Deterministic local mock adapter; explicitly not real AI. |
| `src/evidenceVerifier.js` | Verifies evidence excerpts against `answer_text`, applies OCR/prompt-injection/empty-answer review rules. |
| `src/evaluators/*` | Runs eval datasets, computes metrics, checks release gates. |
| `config/release-gates.json` | Defines dev/pilot/production thresholds. |
| `evals/synthetic/samples.jsonl` | Synthetic, non-private eval dataset. |
| `docs/platform-integration-contract.md` | Future-facing grading service contract; not aligned to current `/api/v1/...` paths. |

## 4. Field Mapping: Lab GradingInput -> Go Subjective AdapterInput

| Lab Field | Go Current Field | Exists | Direct? | Conversion / Gap | Recommended Handling | Risk |
| --- | --- | --- | --- | --- | --- | --- |
| `request_id` | none in `subjective.AdapterInput`; HTTP request has logger request id | Partial | No | Not passed into adapter or persisted as idempotency key. | Keep in lab/eval now; add only if future service boundary needs idempotency. | Duplicate model calls if serviceized without idempotency. |
| `tenant_id` | context/store tenant id | Yes | No | Tenant is loaded from authenticated user/store context, not adapter input. | Do not pass into model adapter unless a service boundary requires tenancy audit metadata. | Passing tenant into model payload can leak operational metadata. |
| `exam_id` | `paper.Question.ExamID` | Yes | Yes | Present through `Question`. | Map from `input.Question.ExamID` if needed for eval grouping. | Low. |
| `question_id` | `paper.Question.ID` | Yes | Yes | Direct. | Use `Question.ID`. | Low. |
| `answer_segment_id` | `Context.SegmentID` | Yes | Yes | Stored outside `AdapterInput`; output grade records it. | Preserve as platform-side context, not necessarily model prompt content. | Missing in external service payload would weaken traceability. |
| `subject` | no explicit field on `Question` | Missing | No | Subject appears not present in current Go question type. | Keep in lab/eval; future platform can derive from exam/paper metadata or add explicit subject. | Subject-specific prompts cannot route reliably. |
| `grade_level` | no explicit field on `Question` | Missing | No | Not visible in current adapter input. | Keep in lab/eval; consider future metadata source. | Grade-level rubric/prompt differences unavailable. |
| `question_type` | `Question.QuestionType` | Yes | Yes | Direct string mapping for subjective types. | Use existing values: `short_answer`, `calculation`, `essay`, `discussion`. | Low. |
| `question_text` | `Question.Stem` | Yes | Yes | Name differs. | Map `question_text` to `Question.Stem`. | If `Stem` is empty, prompt quality drops. |
| `max_score` | `Question.Score` / `Rubric.MaxScore` | Yes | Yes | Need consistency check. | Continue requiring `Question.Score == Rubric.MaxScore`. | Score mismatch can cause overscore or failed validation. |
| `rubric` | `paper.Rubric` | Yes | Partial | Go rubric is simpler than lab rubric. | Map core fields; preserve lab-only fields in eval until platform schema expands. | Aliases/evidence rules can be lost. |
| `answer_text` | `AnswerText` | Yes | Yes | Direct. | Use current `AnswerText`. Avoid logging full text. | Sensitive content leakage if logs include it. |
| `answer_image_ref` | `AnswerImageRef map[string]any` | Yes | Partial | Lab uses optional scalar/string-style reference; Go uses map. | Define future normalized shape before external service integration. | Vision-capable models need stable image references. |
| `ocr_confidence` | `OCRConfidence *float64` | Yes | Yes | Nullable in Go; required in lab. | Default missing Go value conservatively in eval; do not fabricate confidence for production. | Missing confidence can bypass OCR review if not handled. |
| `model_policy` | `ModelPolicy` | Yes | Partial | Go has `model_version`, `prompt_version`, `min_confidence`; lab also uses adapter/final-score policy. | Keep Go policy minimal now; define extension only after adapter choice. | Policy drift between lab and platform. |
| `prompt_version` | `ModelPolicy.PromptVersion` | Yes | Yes | Direct. | Keep required. | Unversioned prompts block eval reproducibility. |
| `rubric_version` | `paper.Rubric.Version` | Yes | Yes | Direct but name differs. | Map to `Rubric.Version`; consider hash in lab only until platform agrees. | Rubric changes can be hard to compare without stable hash. |

## 5. Field Mapping: Lab GradingOutput -> Go Subjective AdapterOutput / AI Grade

| Lab Field | Go Current Field | Exists | Direct? | Conversion / Gap | Recommended Handling | Risk |
| --- | --- | --- | --- | --- | --- | --- |
| `request_id` | none | Missing | No | Go grade id is created after store write. | Keep in lab/eval until external service boundary exists. | Traceability gap for distributed calls. |
| `suggested_score` | `SuggestedScore` | Yes | Yes | Direct. | Keep Go validation `0 <= score <= max`. | Critical if validation removed. |
| `max_score` | `Question.Score`, `Grade.MaxScore` | Yes | Yes | Adapter output does not carry max score; grade construction does. | Keep platform-owned max score. | Avoid trusting model-provided max score. |
| `confidence` | `Confidence` | Yes | Yes | Direct. | Keep range validation. | Bad confidence can skip review. |
| `matched_points` | `[]grading.PointResult` | Yes | Partial | Lab uses `rubric_point_id`, `evidence_ids`; Go uses `Code`, `Label`, `Score`. | Map `rubric_point_id -> Code`; evidence linking is missing. | Evidence-to-point traceability is weak. |
| `missing_points` | `[]grading.PointResult` | Yes | Partial | Lab uses explicit reason; Go has `Label`. | Store reason in `Label` for now; later add structured reason if needed. | Analytics can be inconsistent. |
| `deductions` | no dedicated field in `AdapterOutput` | Missing | No | Deductions may be represented as missing points or raw output only. | Keep in lab/eval until Go schema expands. | Deduction-specific grading cannot be audited cleanly. |
| `evidence` | `[]grading.Evidence` | Yes | Partial | Shape differs. | Map `text_excerpt -> AnswerText`, `location -> Type/Rule`, `rubric_point_id -> Rule` until better schema. | Evidence can lose rubric linkage and confidence. |
| `risk_flags` | `RiskFlags` | Yes | Partial | Naming conventions differ. | Add compatibility mapping before integration. | Gate metrics can be wrong if flags drift. |
| `needs_human_review` | `NeedsHumanReview` | Yes | Yes | Direct. | Continue forcing review in Go policies. | Critical for safety. |
| `student_feedback` | `StudentFeedback` | Yes | Yes | Direct. | Keep empty on failed output. | Feedback quality needs future validation. |
| `teacher_note` | `TeacherNote` | Yes | Yes | Direct. | Keep adapter limitations visible. | Teachers may overtrust mock output if note is vague. |
| `model_version` | `ModelVersion` from policy | Yes | Yes | Go stores policy value, not adapter output field. | Keep platform policy authoritative. | Adapter/model mismatch if not cross-checked. |
| `prompt_version` | `PromptVersion` from policy | Yes | Yes | Go stores policy value. | Keep platform policy authoritative. | Same as above. |
| `rubric_version` | no field on subjective grade | Missing | No | Objective grade has `RuleVersion`; subjective grade does not store rubric version separately. | Add in future if production AI depends on rubric version. | Cannot replay exact rubric behavior from grade row alone. |
| `mock` | `Mock` | Yes | Yes | Direct. | Keep mandatory. | Critical: mock must never appear as real model output. |

## 6. Field Mapping: Lab Rubric -> Go `paper.Rubric`

| Lab Rubric Field | Go Field | Exists | Direct? | Recommended Handling |
| --- | --- | --- | --- | --- |
| `rubric_id` | `ID` | Yes | Yes | Direct. |
| `rubric_version` | `Version` | Yes | Yes | Direct. |
| `max_score` | `MaxScore` | Yes | Yes | Direct; must match `Question.Score`. |
| `points` | `Points` | Yes | Partial | Core point fields map; lab-only point fields do not. |
| `deductions` | `Deductions []any` | Yes | Partial | Go stores untyped deductions; lab wants executable deduction rules. |
| `equivalent_answers` | no rubric field; objective `AnswerKey.EquivalentAnswers` exists | Partial | No | Keep in lab unless rubric schema expands. |
| `examples` | `Examples []any` | Yes | Partial | Untyped in Go. |
| `scoring_notes` | no field | Missing | No | Keep in lab/prompt docs or add future field. |

### RubricPoint Comparison

| Lab Point Field | Go Point Field | Exists | Direct? | Risk |
| --- | --- | --- | --- | --- |
| `id` | `ID` | Yes | Yes | Low. |
| `description` | `Description` | Yes | Yes | Low. |
| `score` | `Score` | Yes | Yes | Low. |
| `required` | `Required` | Yes | Yes | Low. |
| `aliases` | none | Missing | No | Rule/mock matching and eval reproducibility lose equivalent expressions. |
| `evidence_required` | none | Missing | No | Evidence verifier cannot distinguish optional no-evidence points from evidence-required points. |

## 7. Field Mapping: Lab Evidence -> Go Evidence

| Lab Evidence Field | Go Field | Exists | Direct? | Recommended Handling | Risk |
| --- | --- | --- | --- | --- | --- |
| `evidence_id` | none | Missing | No | Keep in lab; add future evidence id only if point-to-evidence linking is required. | Harder to reference evidence items in audits. |
| `rubric_point_id` | none; can be encoded in `Rule` | Missing | No | Short-term map to `Rule`; long-term add structured field. | Weak rubric evidence traceability. |
| `text_excerpt` | `AnswerText` | Yes | Partial | Direct semantic map. | Go name is broad and can be mistaken for full answer. |
| `location` | `Type` / `AnswerSegment` / `BBox` | Partial | No | Map by convention. | Ambiguous source location. |
| `confidence` | none | Missing | No | Keep in lab/eval until model/evidence confidence is required. | Cannot weight evidence quality. |
| bbox | `BBox` | Go only | No | Lab has optional location but no bbox schema in current evidence object. | Lab verifier is weaker on visual location than Go verifier. |

## 8. Risk Flag Mapping

| Lab Flag | Go Current Flag | Exists In Go | Recommended Unified Name | Compatibility Need |
| --- | --- | --- | --- | --- |
| `OCR_LOW_CONFIDENCE` | `low_ocr_confidence` | Yes | `low_ocr_confidence` | Accept both during transition. |
| `INSUFFICIENT_EVIDENCE` | `evidence_verification_failed` / `empty_evidence` issue code | Partial | `insufficient_evidence` | Map verifier failures into stable flag. |
| `PROMPT_INJECTION_SUSPECTED` | none | Missing | `prompt_injection_suspected` | Must add before real LLM. |
| `MOCK_OUTPUT` | `mock_llm_output` | Yes | `mock_llm_output` | Keep existing Go value; lab can map. |
| `HUMAN_REVIEW_REQUIRED` | implicit `needs_human_review=true` | Partial | Prefer boolean, optional flag `human_review_required` | Do not rely only on flag. |
| `SCHEMA_REPAIRED` | none | Missing | `schema_repaired` | Only needed if output repair exists. |
| `SCORE_NEEDS_REVIEW` | none | Missing | `score_needs_review` | Useful for release gates. |
| `AMBIGUOUS_ANSWER` | none | Missing | `ambiguous_answer` | Useful for review routing. |

Recommendation: keep Go lower snake case in platform APIs because it already exists. Maintain a lab-to-Go mapping table for eval reports.

## 9. API Path Mapping

| Capability | Lab Contract | Main Platform Current API | Conflict? | Recommendation |
| --- | --- | --- | --- | --- |
| Grade answer | `POST /grading/grade` | `POST /api/v1/answer-segments/{id}/subjective-ai-grade` | No direct conflict; different boundary. | Keep current platform API. Treat lab path as future internal service API only. |
| Verify evidence | `POST /grading/verify-evidence` | `POST /api/v1/ai-grades/{id}/verify-evidence` | No direct conflict; different target id model. | Keep current platform API. Map service verifier behind it only if serviceized. |
| Run eval | `POST /grading/evaluate` | none | No | Keep offline in lab for now. |
| List models | `GET /grading/models` | none | No | Not needed in production until multiple adapters exist. |
| List prompts | `GET /grading/prompts` | none | No | Keep in lab until prompt registry becomes runtime-managed. |

The existing `/api/v1/...` platform APIs should be preserved. They already encode platform identity, permissions, answer segment ids, grade ids, audit, and storage semantics.

If a future independent `ai-services/grading-agent-service` is created, the `lab` contract can become an internal service API behind the Go gateway. If the Go gateway continues to own adapters, `lab` APIs do not need to enter production.

## 10. Mapping Conclusions

### Directly Mappable

- `question_id`
- `question_type`
- `question_text` to `Question.Stem`
- `max_score` to `Question.Score` / `Rubric.MaxScore`
- `answer_text`
- `ocr_confidence`
- `prompt_version`
- `model_version`
- `suggested_score`
- `confidence`
- `needs_human_review`
- `student_feedback`
- `teacher_note`
- `mock`

### Requires Conversion

- `rubric`
- `matched_points`
- `missing_points`
- `evidence`
- `risk_flags`
- `answer_image_ref`
- `model_policy`

### Missing In Main Platform

- explicit `subject`
- explicit `grade_level`
- `request_id` as adapter/service idempotency key
- subjective grade `rubric_version`
- rubric point `aliases`
- rubric point `evidence_required`
- rubric `scoring_notes`
- structured `deductions`
- evidence `evidence_id`
- evidence `rubric_point_id`
- evidence `confidence`
- prompt injection risk flag

### Potentially Over-Designed For Current Platform

- `/grading/models` and `/grading/prompts` runtime endpoints
- schema repair flag before any repair flow exists
- evidence ids before point-to-evidence UI/audit needs are defined
- external service request id before serviceization

### Should Eventually Enter Production

- prompt/model/rubric version traceability
- prompt injection flagging
- stable risk flag naming
- evidence-to-rubric-point traceability
- release-gated real adapter promotion
- no-private-data logging rules

### Can Remain Lab/Eval Only For Now

- synthetic datasets
- eval runner CLI
- release gate JSON reports
- rubric hash experiments
- prompt checksum registry
- human labeling import validation until real anonymized data exists

## 11. Why Direct Integration Is Not Recommended Now

Direct integration is premature because API paths, schema shapes, risk flag names, rubric fields, evidence fields, and technology stacks are not aligned. The platform already has a working Go grading chain; replacing or bypassing it before mapping is code-level safe would add risk without improving production quality.

The current best use of `lab` is as the standard source and offline evaluation environment for future grading-agent work.

## 12. Inputs For Story B

Story B should analyze these gaps in priority order:

- Schema strictness in Go subjective grading.
- Missing `rubric_version` on subjective grade records.
- Rubric point gaps: `aliases`, `evidence_required`, typed deductions, scoring notes.
- Evidence gaps: point linkage, evidence confidence, text excerpt naming, prompt injection detection.
- Prompt/model management gaps: registry, checksum, changelog, adapter factory, real adapter readiness.
- Eval/release gate gaps: no synthetic eval pipeline in the platform CI, no regression gate for real adapters.
- Service boundary choice: keep Go adapter, use lab CLI offline, or create `ai-services/grading-agent-service` later.
