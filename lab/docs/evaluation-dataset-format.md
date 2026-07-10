# Evaluation Dataset Format

Synthetic eval records are JSONL. Every line is one sample and must include `synthetic=true`.

Required fields:

- `sample_id`
- `synthetic`
- `subject`
- `grade_level`
- `question_type`
- `question_text`
- `max_score`
- `rubric`
- `answer_text`
- `expected_score`
- `expected_matched_points`
- `expected_missing_points`
- `human_rationale`
- `should_need_human_review`
- `tags`

Teacher-labeled real samples must be anonymized before entering the lab. Future human labels should use `docs/human-labeling-guideline.md` and must never include student names, student numbers, phone numbers, or raw private identifiers.
