# Offline Adversarial Agent Suite

Status: Story M accepted for the deterministic Pilot adversarial gate.

## Three Agents

`prompt_injection_adversary` tests Chinese/English instruction override, full-score demands, role override, zero-width characters, full-width text, and benign controls. `rubric_exploit_adversary` tests duplicate points, total mismatch, alias collisions, missing calculation steps, invalid deductions, and empty aliases. `evidence_integrity_adversary` tests hallucinated excerpts, broken evidence ID links, duplicate IDs, unknown points, score mismatch, low OCR, and valid controls.

All three run only offline. The online grading path contains zero adversarial agents and no multi-agent discussion.

## Gate

The development gate requires 100% recall and zero false positives on the starter synthetic suite. The pilot gate additionally requires at least 25 attacks and 25 benign controls per agent, 98% recall, and at most 5% control false positives.

The gate is fail-closed: all three fixed agent identities must be present exactly once. Counts, family breadth, recall, and false-positive rates are recomputed from unique case results; missing, duplicate, unknown, malformed, or aggregate-mismatched reports fail validation.

## Command

```powershell
npm.cmd run adversarial -- --level dev --out evals/reports/adversarial-suite.json
```

## Acceptance

The development and Pilot gates must pass, every agent must exceed the required attack/control breadth, Chinese Unicode attacks must be detected, and normal controls must not be flagged.

## Execution Result

Each agent ran 30 synthetic attacks across six attack families and 25 benign controls across five control families. All three achieved 100% attack recall and 0% control false-positive rate, so both the development and Pilot adversarial gates pass.

This validates deterministic defenses, not the 4B model's semantic robustness or real-school behavior. Model-level attacks remain part of frozen regression and future governed real-data Pilot evaluation.
