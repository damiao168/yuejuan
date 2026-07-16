# QLoRA Training Decision And Protocol

Status: Story P accepted with decision `do_not_train`.

## Decision Before Training

QLoRA is eligible only after all conditions pass: at least 5,000 governed real Gold training records, 500 frozen real test records, teacher QWK at least 0.80, three Prompt/Rubric iterations with no material improvement, approved license and privacy review, zero grouped-split leakage, and a real-data baseline that proves an unresolved model-learning gap.

Synthetic data, public data with unreviewed licenses, or model-generated labels cannot satisfy these gates.

## Eligible Training Configuration

Base model: selected Qwen3 4B checkpoint in BF16/4-bit QLoRA on Linux with NVIDIA GPU, not this Ryzen laptop. Search LoRA rank 16/32, alpha 32/64, dropout 0.05, learning rate `5e-5` to `2e-4`, effective batch 64, sequence length 4096, and 1–3 epochs with early stopping.

Target attention and MLP projections only after an ablation. Training targets contain auditable JSON fields: point decisions, point scores, evidence, risk flags, feedback, and review decision. Hidden chain-of-thought is neither requested nor stored.

Model selection uses grouped validation metrics, not training loss. The frozen test set is evaluated once after selection. Original, adapter, merged, and Q4 models must each rerun schema, evidence, adversarial, calibration, fairness, and regression gates.

## Current State

No training process, LoRA weight, merged model, or synthetic fine-tune is permitted while the decision is `do_not_train`.

The current report has 0 real Gold training records, 0 real Gold test records, no real teacher QWK, no license/privacy approval for a real corpus, no verified real grouped split, no real baseline, no frozen real test set, and 0 plateau iterations because Prompt v2 materially improved the observed regression. Nine conditions remain unmet. Latest Prompt v2 regression does pass.
