# Grading Agent Specification

## Positioning

The Grading Agent only produces suggested scores. It does not publish final grades, replace teachers, bypass human review, or update production records.

## Supported Question Types

`fill_blank`, `numeric`, `short_answer`, `calculation`, `essay`, and `discussion`.

## Supported Subjects

`chinese`, `math`, `english`, `physics`, `chemistry`, `biology`, `history`, `politics`, and `geography`.

## Required Input Fields

- `request_id`
- `question_id`
- `answer_segment_id`
- `subject`
- `grade_level`
- `question_type`
- `question_text`
- `max_score`
- `rubric`
- `answer_text`
- `ocr_confidence`
- `model_policy`
- `prompt_version`
- `rubric_version`

Optional input fields: `tenant_id`, `exam_id`, `answer_image_ref`.

## Required Output Fields

- `request_id`
- `suggested_score`
- `max_score`
- `confidence`
- `matched_points`
- `missing_points`
- `deductions`
- `evidence`
- `risk_flags`
- `needs_human_review`
- `student_feedback`
- `teacher_note`
- `model_version`
- `prompt_version`
- `rubric_version`
- `mock`

## Rubric Structure

Each rubric has `rubric_id`, `rubric_version`, `max_score`, `points`, `deductions`, `equivalent_answers`, `examples`, and `scoring_notes`.

Each point has `id`, `description`, `score`, `required`, `aliases`, and `evidence_required`.

## Evidence Requirements

Every matched point must reference an evidence item. Evidence excerpts must be found in `answer_text` after normalized matching. If evidence is absent or hallucinated, the result is forced to human review.

## Confidence

Confidence is a model or adapter estimate, not an accuracy guarantee. Low confidence must trigger review.

## Human Review Triggers

- `essay` or `discussion`
- `ocr_confidence < 0.85`
- empty answer
- prompt injection suspected
- invalid or missing evidence
- schema repair or adapter failure

## Risk Flags

Supported flags include `OCR_LOW_CONFIDENCE`, `OCR_TEXT_EMPTY_REVIEW_REQUIRED`, `AMBIGUOUS_ANSWER`, `INSUFFICIENT_EVIDENCE`, `POSSIBLE_OFF_TOPIC`, `SCORE_NEEDS_REVIEW`, `SCHEMA_REPAIRED`, `PROMPT_INJECTION_SUSPECTED`, `HUMAN_REVIEW_REQUIRED`, and `MOCK_OUTPUT`.

## Feedback Rules

`student_feedback` must be respectful, specific, concise, and actionable. `teacher_note` may include professional review guidance and adapter limitations.

## Scoring Strategy

- Fill blank and numeric questions prefer deterministic rules.
- Calculation questions require steps, result, units, and tolerance evidence.
- Short answer questions require rubric-point evidence.
- Essay and discussion answers use dimensional rubrics and always require human review.

## Failure Handling

Invalid schema, parser failure, adapter configuration failure, or evidence failure must not become a valid final score.

## Versions

Every output must include `model_version`, `prompt_version`, and `rubric_version`.

## Eval Metrics

The lab calculates MAE, RMSE, exact agreement, adjacent agreement, score bias, high-score recall, low-score recall, review-trigger recall, evidence validity rate, rubric compliance rate, and schema validity rate.

## JSON Example

```json
{
  "request_id": "req-001",
  "suggested_score": 2,
  "max_score": 2,
  "confidence": 0.82,
  "matched_points": [
    { "rubric_point_id": "p1", "score": 1, "evidence_ids": ["ev-p1"] },
    { "rubric_point_id": "p2", "score": 1, "evidence_ids": ["ev-p2"] }
  ],
  "missing_points": [],
  "deductions": [],
  "evidence": [
    { "evidence_id": "ev-p1", "rubric_point_id": "p1", "text_excerpt": "2x=6", "location": "answer_text", "confidence": 0.95 },
    { "evidence_id": "ev-p2", "rubric_point_id": "p2", "text_excerpt": "x=3", "location": "answer_text", "confidence": 0.95 }
  ],
  "risk_flags": ["MOCK_OUTPUT"],
  "needs_human_review": false,
  "student_feedback": "The answer matched the available rubric evidence in the lab rules.",
  "teacher_note": "Mock adapter used deterministic alias matching only.",
  "model_version": "mock-rules-v1",
  "prompt_version": "prompt-base-v1",
  "rubric_version": "rubric-test-v1",
  "mock": true
}
```
