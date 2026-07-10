# Subjective AI Grading API

当前文档属于 `STORY-016 主观题 AI 评分接口`。

## 边界

本模块实现主观题 AI 评分接口层，覆盖：

- `short_answer`
- `calculation`
- `essay`
- `discussion`

当前默认 adapter 是 `MockLLMAdapter`。它不会调用真实模型，不会生成可信 AI 分数；输出固定、低置信、`mock=true`，并强制人工复核。真实本地模型、OpenAI 兼容模型和私有模型接入属于后续工作。

## 权限

所有接口必须携带 Bearer token，并要求 `grading:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## POST /api/v1/answer-segments/{id}/subjective-ai-grade

对 answer segment 执行主观题 AI 评分接口调用。系统会读取：

- question
- 最新 rubric
- 最新 answer_segment_answer
- answer_image_ref
- ocr_confidence
- model_policy

请求：

```json
{
  "model_policy": {
    "model_version": "mock-llm-v1",
    "prompt_version": "subjective-mock-prompt-v1",
    "min_confidence": 0.8
  }
}
```

响应：`201 Created`

```json
{
  "grade": {
    "status": "succeeded",
    "grader_type": "mock_llm_subjective",
    "model_version": "mock-llm-v1",
    "prompt_version": "subjective-mock-prompt-v1",
    "suggested_score": 0,
    "confidence": 0.5,
    "risk_flags": ["mock_llm_output", "low_model_confidence"],
    "needs_human_review": true,
    "mock": true,
    "student_feedback": "MOCK: ...",
    "teacher_note": "MOCK LLM adapter ..."
  }
}
```

## 复核规则

- `confidence < model_policy.min_confidence` 时，`needs_human_review=true`。
- `essay` 和 `discussion` 默认 `needs_human_review=true`。
- `calculation` 如果 OCR 置信度低于阈值，`needs_human_review=true` 并写入 `low_ocr_confidence`。
- mock 输出必须写入 `mock_llm_output`。

## 失败落库

Adapter 输出必须通过 schema 校验：

- `suggested_score` 必须在 0 到题目满分之间。
- `confidence` 必须在 0 到 1 之间。
- `raw_output` 必须存在。

如果 adapter 报错或输出非法，系统不会把它作为有效评分，而是写入：

```json
{
  "status": "failed",
  "risk_flags": ["invalid_model_output"],
  "needs_human_review": true
}
```

## 审计

以下动作写入 `audit_log`：

- `subjective.ai_grade_created`
- `subjective.ai_grade_failed`
