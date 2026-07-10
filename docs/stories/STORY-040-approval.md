# STORY-040 Approval

结论：批准。

## 修改摘要

1. 新增 AI 阅卷离线评估框架，使用 synthetic 样本与 prediction JSONL。
2. 实现 MAE、RMSE、exact agreement、adjacent agreement、score bias。
3. 支持按 `model_version`、`prompt_version` 和组合维度对比。
4. 输出 JSON 与 Markdown 报告。
5. 增加低置信度转人工策略统计。
6. 添加单元测试和扩展评估集说明。

## 新增文件

1. `tests/ai-evaluation/README.md`
2. `tests/ai-evaluation/evaluate.py`
3. `tests/ai-evaluation/test_evaluate.py`
4. `tests/ai-evaluation/samples/synthetic_subjective_v1.jsonl`
5. `tests/ai-evaluation/predictions/synthetic_mock_predictions.jsonl`
6. `tests/ai-evaluation/reports/ai_evaluation_report.json`
7. `tests/ai-evaluation/reports/ai_evaluation_report.md`
8. `docs/stories/STORY-040-ai-evaluation-quality.md`
9. `docs/stories/STORY-040-approval.md`

## 修改文件

1. `docs/stories/README.md`
2. `tests/README.md`

## 运行命令

```powershell
$env:PYTHONDONTWRITEBYTECODE='1'; python -m unittest discover -s .\tests\ai-evaluation
python .\tests\ai-evaluation\evaluate.py --samples .\tests\ai-evaluation\samples\synthetic_subjective_v1.jsonl --predictions .\tests\ai-evaluation\predictions\synthetic_mock_predictions.jsonl --out-dir .\tests\ai-evaluation\reports
```

## 测试结果

1. Python 单元测试：通过，4 个测试通过。
2. 评估脚本报告生成：通过。
3. 报告内容检查：通过，包含 synthetic 声明、五类指标、模型版本对比、Prompt 版本对比和低置信度转人工统计。
4. `tests/ai-evaluation/__pycache__`：不存在。

## 剩余风险

1. 当前评估集是小型 synthetic set，不能代表真实考试全量场景。
2. 当前 prediction fixture 是 mock baseline，不代表真实模型能力。
3. 后续真实模型评估需要接入数据治理、样本版本管理和更严格的统计置信区间。

## 下一步建议

进入 STORY-041：全链路 E2E 测试。
