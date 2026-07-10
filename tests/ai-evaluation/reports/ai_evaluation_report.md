# AI Evaluation Report

> Synthetic evaluation only. Do not present this report as proof of real model production capability.

## Dataset

- Samples: 5
- Predictions: 15
- Synthetic only: True

## Variant Comparison

| model_version | prompt_version | n | MAE | RMSE | exact | adjacent | bias | low-conf routed | review rate |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| mock-llm-v1 | prompt-v1 | 5 | 0.400 | 0.632 | 60.0% | 100.0% | -0.400 | 100.0% | 60.0% |
| mock-llm-v1 | prompt-v2-guarded | 5 | 0.100 | 0.224 | 80.0% | 100.0% | -0.100 | 100.0% | 40.0% |
| mock-llm-v2 | prompt-v2-guarded | 5 | 0.800 | 0.894 | 20.0% | 100.0% | 0.000 | 100.0% | 60.0% |

## Model Version Comparison

| model_version | n | MAE | RMSE | exact | adjacent | bias |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| mock-llm-v1 | 10 | 0.250 | 0.474 | 70.0% | 100.0% | -0.250 |
| mock-llm-v2 | 5 | 0.800 | 0.894 | 20.0% | 100.0% | 0.000 |

## Prompt Version Comparison

| prompt_version | n | MAE | RMSE | exact | adjacent | bias |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| prompt-v1 | 5 | 0.400 | 0.632 | 60.0% | 100.0% | -0.400 |
| prompt-v2-guarded | 10 | 0.450 | 0.652 | 50.0% | 100.0% | -0.050 |

## Low Confidence Routing

### mock-llm-v1 / prompt-v1

- Low confidence threshold: 0.8
- Low confidence samples: 3 (60.0%)
- Low confidence routed to human: 3 (100.0%)
- Overall human review rate: 60.0%
- Low confidence not routed sample IDs: none
- Risk-flagged but not routed sample IDs: none

### mock-llm-v1 / prompt-v2-guarded

- Low confidence threshold: 0.8
- Low confidence samples: 2 (40.0%)
- Low confidence routed to human: 2 (100.0%)
- Overall human review rate: 40.0%
- Low confidence not routed sample IDs: none
- Risk-flagged but not routed sample IDs: none

### mock-llm-v2 / prompt-v2-guarded

- Low confidence threshold: 0.8
- Low confidence samples: 3 (60.0%)
- Low confidence routed to human: 3 (100.0%)
- Overall human review rate: 60.0%
- Low confidence not routed sample IDs: none
- Risk-flagged but not routed sample IDs: none

## Limitations

- This report uses synthetic samples only.
- Included prediction fixtures may be mock baselines and must not be presented as real model capability.
- Metrics measure agreement with provided human_score labels; they do not prove fairness, robustness, or production readiness.
