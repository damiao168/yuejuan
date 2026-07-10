# Model Optimization Roadmap

## Phase 1: Prompt + Rubric + Schema + Eval

Do not train a model. Establish baseline prompts, structured rubrics, schema validation, synthetic evals, evidence verification, release gates, and regression cases.

## Phase 2: Human-Labeled Data

Collect teacher labels only after anonymization rules are approved. Use double scoring and adjudication for disputed samples.

## Phase 3: Prompt Iteration

Add few-shot examples, subject-specific prompts, question-type prompts, and bad-case regression samples.

## Phase 4: Model Selection

Compare large models, small models, local models, and OpenAI-compatible models by quality, latency, cost, privacy, and operational risk.

## Phase 5: Optional Fine-Tuning, Distillation, Or Local Training

Only consider training after evals prove that prompt and rubric engineering are insufficient. Training data must be anonymized, teacher review must remain, and the full eval suite must be rerun.

## Phase 6: Online Monitoring

Monitor AI-human score difference, teacher correction rate, appeal success rate, OCR failure rate, group bias, prompt drift, and model drift.

Conclusion: enterprise grading quality comes from evaluability, explainability, replayability, and auditability before training.
