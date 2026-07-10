# Agent Overview

## 当前阶段

本文件属于项目骨架 Story，只定义 Agent 边界。真实 Agent Runtime、任务表、执行器和测试会在后续 Story 实现。

## 原则

Agent 是受控工作流节点，不是多角色聊天。每个 Agent 的输入、输出、版本、错误和审计链路必须可记录。

## Agent 类型

- `ocr_agent`：OCR 识别和置信度输出。
- `layout_agent`：版面解析、题号、页码和答题区域识别。
- `rubric_parser_agent`：Rubric 结构化。
- `objective_grading_agent`：客观题判分。
- `fill_blank_grading_agent`：填空题、同义答案、容差。
- `short_answer_grading_agent`：简答题采分点建议。
- `calculation_grading_agent`：计算题步骤和公式建议。
- `essay_grading_agent`：作文多维度建议分。
- `evidence_verifier_agent`：证据位置和采分点一致性检查。
- `consistency_checker_agent`：同类答案分数一致性检查。
- `anomaly_detector_agent`：异常答案、异常得分、疑似作弊检测。
- `analytics_agent`：学情诊断和考试质量分析。

## 输出要求

AI 阅卷输出必须包含：

- `suggested_score`
- `max_score`
- `confidence`
- `matched_points`
- `missing_points`
- `evidence`
- `risk_flags`
- `needs_human_review`
- `model_version`
- `prompt_version`
- `rubric_version`
- `mock`，仅当结果来自 mock/stub 时必须为 `true`

## 人工复核触发

- OCR 置信度低于阈值。
- AI 置信度低于阈值。
- 缺少证据位置。
- AI 与 Rubric 或规则冲突。
- 同类答案分差异常。
- 双评分差超过阈值。
