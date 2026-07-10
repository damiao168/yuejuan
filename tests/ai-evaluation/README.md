# AI Evaluation

This directory contains the first offline evaluation framework for EduGrade AI grading quality.

The included dataset and predictions are synthetic. They must not be presented as real model performance and must not contain real student identities, names, student numbers, answer sheets, or private school data.

## Files

- `samples/synthetic_subjective_v1.jsonl`: synthetic evaluation samples.
- `predictions/synthetic_mock_predictions.jsonl`: synthetic/mock prediction fixtures for comparing model and prompt versions.
- `evaluate.py`: dependency-free evaluator that produces JSON and Markdown reports.
- `test_evaluate.py`: unit tests for metrics, grouping, routing stats, and report generation.

## Sample Format

Each sample is a JSON object with:

- `sample_id`
- `synthetic: true`
- `question`
- `rubric`
- `answer`
- `human_score`
- `human_rationale`
- `expected_points`

The nested `answer` object must also include `synthetic: true`.

## Prediction Format

Each prediction is a JSON object with:

- `sample_id`
- `synthetic: true`
- `model_version`
- `prompt_version`
- `suggested_score`
- `confidence`
- `needs_human_review`
- `risk_flags`
- `matched_points`

## Run

```powershell
python .\tests\ai-evaluation\evaluate.py `
  --samples .\tests\ai-evaluation\samples\synthetic_subjective_v1.jsonl `
  --predictions .\tests\ai-evaluation\predictions\synthetic_mock_predictions.jsonl `
  --out-dir .\tests\ai-evaluation\reports
```

Outputs:

- `tests/ai-evaluation/reports/ai_evaluation_report.json`
- `tests/ai-evaluation/reports/ai_evaluation_report.md`

## Extend The Evaluation Set

1. Add only synthetic samples or de-identified samples approved by the data governance process.
2. Keep one scenario per `sample_id`.
3. Include the full human scoring rationale, expected rubric points, and max score.
4. Add prediction records for each model/prompt pair being compared.
5. Review low-confidence routing and risk flags. A high score agreement alone is not enough for production approval.
6. Update tests when changing required schema fields or metric definitions.

## Metric Notes

- `MAE`: average absolute score error.
- `RMSE`: square root of mean squared score error.
- `exact agreement`: suggested score equals human score within exact tolerance.
- `adjacent agreement`: suggested score is within the configured adjacent tolerance, default `1.0`.
- `score bias`: mean `suggested_score - human_score`; positive means over-scoring.
- low-confidence routing: measures whether low-confidence predictions are sent to human review.
