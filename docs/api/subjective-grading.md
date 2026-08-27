# 主观题智能阅卷 API

## 产品边界

主项目通过受治理的内部 `grading-agent` 获取主观题评分建议。建议不会直接写入 `final_grade`，不会自动发布，也不会对学生直接可见。当前 `local-pilot-v1` 全部要求教师复核；作文和论述题仅允许影子记录。

浏览器 API 保持不变：

```text
POST /api/v1/answer-segments/{id}/subjective-ai-grade
```

只有具备 `grading:manage` 权限的登录用户可以调用。Go API 网关负责租户隔离、上下文读取、持久化和审计；浏览器不能直接访问内网智能体接口。

## 受治理上下文

网关从数据库读取：

- 考试学科 `exam.subject`；
- 考试绑定班级的唯一年级层级，7-9 年级映射为 `junior_middle`；
- 题目、最新已批准 Rubric 及其版本；
- 最新答案文本、答案版本和 OCR 置信度；
- Prompt 注入检测结果。

学段缺失、跨学段或不在能力矩阵内时，不调用模型，只保存失败的 AI 记录并要求人工处理。

发送给智能体的请求不包含 `tenant_id`、学生姓名、学号、学生 id、最终成绩或已发布成绩，也不发送答案图片引用。

## 模型策略

启用 `EDUGRADE_AI_SERVICE_URL` 后，模型版本、Prompt 版本、置信度阈值和超时由服务端环境变量锁定。请求体中的 `model_policy` 仅保留向后兼容，不能选择实际模型。

未配置智能体服务时，开发环境继续使用明确标记为 `mock=true` 的占位适配器，不冒充真实推理。

## 成功响应

响应为 `201 Created`。真实智能体建议包含：

```json
{
  "grade": {
    "status": "succeeded",
    "grader_type": "llm_subjective",
    "answer_version": "answer-uuid",
    "model_version": "Qwen/Qwen3-4B-GGUF:Q4_K_M",
    "prompt_version": "subjective-governed-cn-subject-routing-v5",
    "rubric_version": "rubric-v3",
    "delivery_mode": "teacher_suggestion",
    "capability_profile": "local-pilot-v1",
    "suggested_score": 4,
    "confidence": 0,
    "risk_flags": ["score_needs_review", "human_review_required", "low_model_confidence"],
    "needs_human_review": true,
    "mock": false,
    "adapter_name": "local_llama_cpp",
    "adapter_attempts": 1,
    "adapter_latency_ms": 106000,
    "adapter_repair_attempted": false
  }
}
```

分数由服务和 Go 网关分别根据 `matched_points` 重算。每个命中采分点必须引用答案原文中的证据，证据必须绑定 Rubric point。

## 失败语义

鉴权失败、能力不支持、模型不可用、超时、JSON/Schema 错误、采分点不完整、分数越界或证据不成立时，不生成有效建议。平台仍写入一条 `status=failed`、`suggested_score=0`、`needs_human_review=true` 的 `ai_grade`，但不保存未经验证的模型证据。

失败响应仍为 `201 Created`，以保持现有业务 API 的“尝试记录已落库”语义。调用方应检查 `grade.status`，不能只检查 HTTP 状态码。

## 审计

- `subjective.ai_grade_created`
- `subjective.ai_grade_failed`

普通系统日志只记录 request id、状态、尝试次数、耗时和错误码，不记录完整答案、学生身份或最终成绩。
