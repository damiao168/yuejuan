# Human Labeling Guideline

This stage defines the format and tools only. Do not import real student private data until an approved anonymization workflow exists.

Teachers should label:

- `human_score`
- `human_matched_points`
- `human_missing_points`
- `human_deductions`
- `human_rationale`
- evidence excerpts for each matched point
- label confidence

Double scoring:

- A second label may be added under `second_label`.
- If scores differ beyond the configured tolerance, an adjudicator writes `adjudicated_score` and `adjudication_note`.

Privacy:

- Use anonymized IDs only.
- Remove names, student numbers, phone numbers, addresses, and other identifiers.
- Do not place raw private identifiers in `answer_text`.
- Rejected imports must preserve the rejection reason without copying sensitive text into ordinary logs.

Data that cannot enter training or eval:

- non-anonymized samples
- samples without rubric version
- samples without labeler metadata
- samples containing private identifiers in raw answer text
- disputed labels without adjudication
