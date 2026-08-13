# A15 模型置信度校准与风险覆盖

模型返回的原始置信度不是评分结论。A15 只把它与 A16 已完成的、对齐的 Gold 或人工裁决观察配对，生成可复核的校准工件；不保存答题卡、OCR 文本、Gold 依据或模型密钥。

每个工件的身份轴固定为 `model_reference + prompt_version + rubric_version + subject + archetype + slice_key`。因此替换模型、提示词或 Rubric 后必须重新采样，不能继承旧工件。`slice_key` 至少区分 `all`，也可缩小到分数段或 OCR 质量。

流程是：创建草稿 → 为同一个已完成 A16 run 的响应登记原始置信度 → 计算 `isotonic`、`logistic`、`conformal`（`auto` 以 Brier score 选择）→ 审核人显式批准。只有 `approved` 状态才能成为 A14 的 `CalibrationEvidence`。完成不等于批准，批准也不等于可以直接给学生最终分。

工件包括：

- 原始置信度到校准置信度的映射；
- Brier score、ECE，以及部分得分（middle/partial）单独指标；
- 0–1 阈值下的 Risk-Coverage Curve，含覆盖率、经验错误率和严重错误率。

`model_score_candidate` 仅记录 `raw_confidence`、`calibrated_confidence`、`calibration_id` 与 `abstain_reason`。当没有批准工件或目标严重错误风险无法满足时，它会记录弃权，不会创建、覆盖或发布任何成绩。
