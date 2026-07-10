# Rule Grading API

当前文档属于 `STORY-015 客观题与填空题判分`。

## 边界

本模块实现确定性规则判分，覆盖：

- `single_choice`
- `multiple_choice`
- `true_false`
- `fill_blank`
- `numeric`

本模块会生成 `ai_grade` 记录，但评分来源是 `grader_type=rule_based_objective`、`mock=false` 的规则引擎，不是 LLM 或真实模型推理。主观题 AI 评分、证据校验 Agent、OCR 文本自动归属和最终成绩生成属于后续 Story。

## 权限

所有接口必须携带 Bearer token，并要求 `grading:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## PUT /api/v1/answer-segments/{id}/answer

给 answer segment 记录真实学生答案。判分必须先有答案来源，系统不会从图片或 OCR 中伪造答案。

请求：

```json
{
  "answer_text": "A,C",
  "answer_payload": {
    "answers": ["A", "C"]
  },
  "source": "manual_entry",
  "confidence": 0.99
}
```

`source` 支持：

- `manual_entry`
- `ocr_text`
- `imported_answer`

## POST /api/v1/answer-segments/{id}/rule-grade

执行规则判分并生成 `ai_grade`。

规则：

- 从 answer segment 最新答案记录读取学生答案。
- 从 `question_answer_key` 读取标准答案、等价答案和 tolerance。
- `single_choice`、`true_false` 精确匹配。
- `multiple_choice` 全对满分，错选 0 分；`tolerance.allow_partial=true` 时少选按比例给分。
- `fill_blank` 支持标准答案、等价答案、`ignore_case`、`ignore_spaces`。
- `numeric` 支持 `tolerance.absolute` 或 `tolerance.value`，以及 `unit_required`。
- 建议分永远不会超过题目满分。

响应：`201 Created`

```json
{
  "grade": {
    "grader_type": "rule_based_objective",
    "rule_version": "objective-rules-v1",
    "suggested_score": 5,
    "max_score": 5,
    "confidence": 0.99,
    "matched_points": [],
    "missing_points": [],
    "evidence": [],
    "risk_flags": [],
    "needs_human_review": false,
    "auto_pass": true,
    "mock": false
  }
}
```

## GET /api/v1/answer-segments/{id}/ai-grades

列出 answer segment 下的 `ai_grade` 记录，按创建时间倒序返回。

## 错误与复核

- 没有答案：`409 answer_segment_answer_missing`
- 没有标准答案：`409 question_answer_key_missing`
- 不支持题型：`400 unsupported_rule_grading_question_type`
- 数值解析失败等风险会写入 `risk_flags`，并设置 `needs_human_review=true`。

## 审计

以下动作写入 `audit_log`：

- `grading.answer_recorded`
- `grading.rule_grade_created`
