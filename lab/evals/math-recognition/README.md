# MathBench v1

`run_math_bench.py` is the reproducible MATH-00 contract benchmark. Each
manifest row points to an immutable original image and a ground-truth JSON
document; predictions live in a named directory and the runner emits one JSON
metrics report.

```powershell
python lab/evals/math-recognition/run_math_bench.py `
  --manifest lab/evals/math-recognition/fixtures/manifest.jsonl `
  --predictions lab/evals/math-recognition/fixtures/predictions/smoke-v1 `
  --output lab/evals/math-recognition/reports/smoke-v1.json
```

Metrics cover formula exact match, restricted AST exact match, symbols,
spatial relations, solution steps/edges, equivalence precision, unsafe
suggestion rate, risky-case recall and Rubric evidence. Manifests accept only
the governed formula subjects `mathematics`, `physics`, and `chemistry`; they
reject humanities instead of silently invoking the formula pipeline. The included
single synthetic fixture proves runner reproducibility only. It is not a model
quality result and must not be used as a pilot gate.

Reports carry a `dataset` provenance label via `--dataset`; synthetic runs must
always pass `--dataset synthetic`.

## synthetic-v2 baseline (SYNTHETIC harness self-check — NOT real model evidence)

`generate_synthetic_fixtures.py` deterministically expands the corpus to 54
synthetic samples across the full category matrix (each category >= 2 samples).
Ground truth is hand-authored; the `synthetic-v2` predictions are derived from
ground truth by a fixed schedule of deliberate mistakes (LaTeX normalization
variants, AST swaps, spurious/missing symbols, relations, step merges, edge
kind errors, equivalence flips, rubric hallucinations, unsafe suggestions,
missed risky-case recalls). Every number below therefore validates that the
scoring harness discriminates hits from misses on every metric path. It says
nothing about any real model. The real MathBench (1000+ formula crops, 500+
full answers from de-identified real sheets) is out of scope here.

The `original` manifest fields for this corpus are virtual references — the
runner scores JSON documents only and never loads images.

```powershell
python lab/evals/math-recognition/generate_synthetic_fixtures.py
python lab/evals/math-recognition/run_math_bench.py `
  --manifest lab/evals/math-recognition/fixtures/manifest.jsonl `
  --predictions lab/evals/math-recognition/fixtures/predictions/synthetic-v2 `
  --output lab/evals/math-recognition/reports/synthetic-v2.json `
  --dataset synthetic
```

| metric | value | | metric | value |
| --- | --- | --- | --- | --- |
| formula_exact_accuracy | 0.9286 | | steps_precision | 0.9799 |
| ast_exact_accuracy | 0.9643 | | steps_recall | 0.9605 |
| symbols_precision | 0.9885 | | steps_f1 | 0.9701 |
| symbols_recall | 0.9885 | | edges_precision | 0.9691 |
| symbols_f1 | 0.9885 | | edges_recall | 0.9691 |
| relations_precision | 0.7500 | | edges_f1 | 0.9691 |
| relations_recall | 0.9231 | | rubric_evidence_precision | 0.9538 |
| relations_f1 | 0.8276 | | rubric_evidence_recall | 0.9538 |
| equivalence_precision | 0.9455 | | rubric_evidence_f1 | 0.9538 |
| equivalence_recall | 0.9455 | | unsafe_suggestion_rate | 0.0652 |
| equivalence_f1 | 0.9455 | | risky_case_recall | 0.6667 |

Category coverage (samples): fraction-stacked 3, fraction-inline 2, radical 2,
exponent 2, subscript 2, system-of-equations 2, inequality 2, absolute-value 2,
trigonometric 2, geometry-symbol 2, vector 2, limit 2, summation 2, integral 2,
matrix 2, chinese-mixed 2, horizontal-step 2, vertical-step 2, two-column 2,
insert 2, crossed-out 2, scratch 2, low-quality 3, alternative-method 3,
solution-chain 3 — 54 generated samples plus the preserved
`synthetic-equation-001` smoke entry (55 manifest rows).
