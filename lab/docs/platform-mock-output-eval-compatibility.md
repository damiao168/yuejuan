# Platform Mock Output Eval Compatibility Spec

Status: Story D accepted after self-review.

Scope: documentation only. This spec defines how current main-platform mock/AI grade outputs can later be evaluated by the lab CLI. It does not implement an exporter, converter, API, CI job, or runtime integration.

## 1. Purpose

The lab is currently an offline evaluation and standards environment. The main platform already produces Go `AiGrade` records through:

- objective rule grading;
- subjective mock LLM grading;
- evidence verification jobs.

To make the lab useful without touching the platform runtime, future work should be able to take a platform output snapshot and convert it into a lab eval record. This story defines that compatibility contract.

## 2. Non-Goals

- Do not modify `services/`.
- Do not modify `apps/`.
- Do not modify `ai-services/`.
- Do not add `lab` to the root workspace.
- Do not implement a converter yet.
- Do not call the main platform API from lab.
- Do not treat platform mock output as real AI quality.
- Do not use real student private data.

## 3. Source Data Shapes

### 3.1 Frontend `AiGrade` Shape

From `apps/web-admin/src/api/review.ts`, current `AiGrade` includes:

| Field | Notes |
| --- | --- |
| `id` | Platform grade id. |
| `tenant_id` | Tenant id. Must not be exported into ordinary lab reports unless needed for anonymized grouping. |
| `answer_segment_id` | Segment id. |
| `question_id` | Question id. |
| `question_no` | Display question number. |
| `question_type` | Grading type. |
| `answer_version` | Objective answer version, optional. |
| `grader_type` | `rule_based_objective`, `mock_llm_subjective`, or `llm_subjective`. |
| `rule_version` | Objective rule version, optional. |
| `model_version` | Subjective model version, optional in type but required for subjective AI eval. |
| `prompt_version` | Subjective prompt version, optional in type but required for subjective AI eval. |
| `suggested_score` | Suggested score. |
| `max_score` | Max score. |
| `confidence` | Confidence. |
| `matched_points` | `PointResult[]`. |
| `missing_points` | `PointResult[]`. |
| `evidence` | `GradeEvidence[]`. |
| `risk_flags` | Platform flags. |
| `needs_human_review` | Review trigger. |
| `auto_pass` | Objective grading convenience flag. |
| `mock` | Mock marker. |
| `status` | `succeeded` or `failed`. |
| `failure_reason` | Failure reason. |
| `student_feedback` | Optional. |
| `teacher_note` | Optional. |
| `raw_output` | Adapter raw output. |

### 3.2 Go Subjective `AdapterOutput`

Current Go `subjective.AdapterOutput` contains:

- `SuggestedScore`
- `Confidence`
- `MatchedPoints`
- `MissingPoints`
- `Evidence`
- `RiskFlags`
- `NeedsHumanReview`
- `StudentFeedback`
- `TeacherNote`
- `RawOutput`
- `Mock`

`model_version` and `prompt_version` are owned by `ModelPolicy` and grade persistence, not directly by `AdapterOutput`.

### 3.3 Go Evidence Job

Current evidence job result contains:

- `passed`
- `failed`
- `warnings`
- `corrected_flags`
- `needs_human_review`

This is not the same as lab `EvidenceVerificationResult`, but it can be mapped for eval metrics.

## 4. Target Lab Snapshot Format

Future compatibility work should use a JSONL snapshot format under lab-managed input folders. This spec proposes the logical shape only:

```json
{
  "snapshot_id": "platform-snapshot-001",
  "synthetic": true,
  "source": "platform_mock_export",
  "platform_grade": {},
  "platform_evidence_job": {},
  "expected": {
    "expected_score": 0,
    "expected_matched_points": [],
    "expected_missing_points": [],
    "should_need_human_review": true,
    "human_rationale": "Mock output is not a real score."
  },
  "privacy": {
    "contains_real_student_data": false,
    "anonymized": true,
    "answer_text_included": false
  }
}
```

For real samples, `synthetic=false` is allowed only after anonymization and human-label approval. Until then, compatibility snapshots should use synthetic or de-identified fixtures.

## 5. Mapping: Platform `AiGrade` -> Lab Eval Result Record

| Platform Field | Lab Eval Result Field | Required? | Conversion | Do Not Fabricate |
| --- | --- | --- | --- | --- |
| `id` | `platform_grade_id` | Yes | Direct. | Do not use as `request_id` if it contains sensitive tenant semantics. |
| `question_id` | `question_id` | Yes | Direct. | No. |
| `answer_segment_id` | `answer_segment_id` | Yes | Direct if anonymized. | No. |
| `question_no` | `question_no` | Optional | Direct. | No. |
| `question_type` | `question_type` | Yes | Direct, must be one of lab-supported values or flagged unsupported. | No. |
| `grader_type` | `grader_type` | Yes | Direct. | No. |
| `rule_version` | `rule_version` | Optional | Direct for objective rule grading. | Do not invent for subjective grades. |
| `model_version` | `model_version` | Required for subjective | Direct from grade or `raw_output`. | Do not invent a real model version for mock output. |
| `prompt_version` | `prompt_version` | Required for subjective | Direct from grade or `raw_output`. | Do not invent. |
| `suggested_score` | `output.suggested_score` | Yes | Direct. | Do not clamp silently in eval conversion. |
| `max_score` | `output.max_score` | Yes | Direct. | No. |
| `confidence` | `output.confidence` | Yes | Direct. | Do not replace missing confidence with high confidence. |
| `matched_points` | `output.matched_points` | Yes | Convert point shape. | Do not add evidence ids unless source evidence can support them. |
| `missing_points` | `output.missing_points` | Yes | Convert point shape. | No. |
| `evidence` | `output.evidence` | Yes | Convert evidence shape. | Do not use full answer text as excerpt unless it is already evidence. |
| `risk_flags` | `output.risk_flags` | Yes | Map through canonical flag table. | Do not drop unknown flags. |
| `needs_human_review` | `output.needs_human_review` | Yes | Direct. | No. |
| `mock` | `output.mock` | Yes | Direct. | Never convert mock to false. |
| `status` | `platform_status` | Yes | Direct. Failed status should count as invalid/failed output, not score quality. | No. |
| `failure_reason` | `failure_reason` | Required when failed | Direct. | No. |
| `student_feedback` | `output.student_feedback` | Optional | Direct if privacy-safe. | Do not generate new feedback. |
| `teacher_note` | `output.teacher_note` | Optional | Direct. | Do not remove mock warning. |
| `raw_output` | `raw_output_summary` | Optional | Redacted summary only. | Do not export full private payloads. |

## 6. Mapping: Platform `PointResult` -> Lab Point Record

Platform:

```json
{ "code": "mock_not_scored", "label": "mock adapter does not perform real subjective scoring", "score": 5 }
```

Lab-compatible record:

```json
{
  "rubric_point_id": "mock_not_scored",
  "score": 5,
  "label": "mock adapter does not perform real subjective scoring",
  "evidence_ids": []
}
```

Rules:

- `code` maps to `rubric_point_id`.
- `label` may be retained as compatibility metadata.
- `score` maps directly.
- `evidence_ids` must remain empty unless platform evidence can be tied to the point.
- If `code` is not a real rubric point id, mark `rubric_point_known=false` in compatibility metadata.

## 7. Mapping: Platform `GradeEvidence` -> Lab Evidence Record

Platform:

```json
{
  "type": "mock",
  "answer_segment_id": "",
  "answer_text": "",
  "standard_answer": "",
  "rule": "mock output; not real model evidence",
  "bbox": []
}
```

Lab-compatible record:

```json
{
  "evidence_id": "generated-from-index-0",
  "rubric_point_id": null,
  "text_excerpt": "",
  "location": "mock",
  "confidence": null,
  "platform_rule": "mock output; not real model evidence",
  "bbox": []
}
```

Rules:

- `type` maps to `location`.
- `answer_text` maps to `text_excerpt`.
- `rule` should be preserved as `platform_rule`.
- `bbox` should be preserved when present.
- `rubric_point_id` is unknown unless encoded by a future convention.
- Empty `answer_text` should produce an evidence validity failure, not be hidden.
- `confidence` must not be invented.

## 8. Mapping: Evidence Job -> Lab Verification Summary

| Platform Evidence Job Field | Lab Verification Field | Rule |
| --- | --- | --- |
| `result.passed` | `verification_passed` | Direct. |
| `result.failed` | `invalid_points` / `failed_issues` | Preserve issue codes and messages. |
| `result.warnings` | `warnings` | Direct. |
| `result.corrected_flags` | `forced_risk_flags` | Map flags through canonical table. |
| `result.needs_human_review` | `forced_needs_human_review` | Direct. |
| `job.needs_human_review` | `job_needs_human_review` | Direct. |
| `job.status` | `evidence_job_status` | Direct. |

If no evidence job exists, the lab compatibility record must mark `evidence_job_missing=true`. It must not assume evidence passed.

## 9. Mock Output Rules

Platform mock output can be used to test compatibility mechanics only.

Required behavior:

- `mock=true` must remain true.
- `mock_llm_output` must map to lab `MOCK_OUTPUT` or a canonical equivalent.
- Suggested score from mock must not be interpreted as real model quality.
- Mock grades should usually have `needs_human_review=true`.
- Eval reports must clearly label `adapter_family=platform_mock`.

If a platform output has `grader_type=mock_llm_subjective` and `mock=false`, compatibility validation must fail.

## 10. Privacy Rules

Compatibility snapshots must be privacy-first:

- Do not export student name.
- Do not export student number.
- Do not export full answer text unless the sample is synthetic or explicitly anonymized for eval.
- Do not export raw image references unless they are synthetic or anonymized.
- Redact `raw_output` by default.
- Keep tenant id out of ordinary reports unless using anonymized tenant grouping.
- Keep grade ids only as internal trace ids for non-production lab work.

## 11. Metrics Enabled By Compatibility

Once a converter exists, lab should be able to compute:

- schema validity rate;
- score bounds validity;
- mock marking rate;
- evidence job pass rate;
- human review trigger rate;
- prompt/model/rubric version completeness;
- risk flag mapping coverage;
- failed-output rate;
- platform-to-lab conversion failure rate.

It should not compute MAE/RMSE unless expected human scores are present.

## 12. Required Snapshot Validation Rules

Future converter validation should reject or mark invalid when:

1. `suggested_score < 0`.
2. `suggested_score > max_score`.
3. `confidence < 0` or `confidence > 1`.
4. subjective grade lacks `model_version`.
5. subjective grade lacks `prompt_version`.
6. mock grader has `mock=false`.
7. non-mock grader contains mock risk flags.
8. failed grade has no `failure_reason`.
9. evidence job is missing when the eval requires evidence verification.
10. raw answer text appears in a non-anonymized snapshot.

## 13. Compatibility Acceptance Checklist

Before implementing any converter, the team should agree on:

- canonical risk flag mapping;
- whether platform grade id can be stored in lab reports;
- how to represent missing `rubric_version` on subjective grades;
- how to link evidence to rubric points;
- whether answer text is omitted, redacted, or included for each eval mode;
- whether objective rule grades are in scope or only subjective AI grades;
- how evidence job results are paired with grade records;
- how failed adapter outputs affect metrics.

## 14. Recommended Future Story Inputs

This story feeds:

- Story E: risk flag unified naming standard.
- Story H: release gate CI integration plan.
- A future converter story, after the user approves implementing lab-only tooling.

The next best story is Story E because risk flag compatibility is required before reliable platform-output eval.

## 15. Story D Self-Review

No critical issue found. The spec stays document-only and does not create runtime coupling.

Remaining risk: the exact platform export shape is not yet defined. This document assumes a future snapshot/export layer, not direct API calls from lab.
