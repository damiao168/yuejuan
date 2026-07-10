# STORY-016 主观题 AI 评分接口

## 状态

Approved

## 目标

建立主观题 AI 评分接口层，覆盖 `short_answer`、`calculation`、`essay`、`discussion`。本 Story 只实现可替换 Adapter、输入输出 schema、mock adapter、失败落库和人工复核触发，不实现真实模型推理。

## Plan

- 新增 migration：
  - 扩展 `ai_grade` 支持主观题 question_type 和 LLM grader_type。
  - 增加 `status`、`failure_reason`、`model_version`、`prompt_version`、`student_feedback`、`teacher_note` 等字段。
- 新增 `internal/subjective`：
  - `LLMGradingAdapter` 接口。
  - `MockLLMAdapter`，固定输出、低置信、`mock=true`、必须人工复核。
  - 输入/输出类型和 schema validation。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 接口：
  - `POST /api/v1/answer-segments/{id}/subjective-ai-grade`
- 规则：
  - 输入包含 question、rubric、answer_text、answer_image_ref、ocr_confidence、model_policy。
  - 输出必须包含 suggested_score、confidence、matched_points、missing_points、evidence、risk_flags、needs_human_review、student_feedback、teacher_note。
  - 所有输出必须 schema 校验。
  - 非法模型输出不得作为有效评分落库，必须写 `status=failed` 的 ai_grade 记录。
  - 低置信度自动 `needs_human_review=true`。
  - `essay`、`discussion` 默认 `needs_human_review=true`。
  - `calculation` 如果 OCR 低置信度，必须 `needs_human_review=true`。
  - 所有 `model_version`、`prompt_version` 必须记录。
  - mock 结果必须 `mock=true` 且含 `mock_llm_output` 风险标记。

## Plan Review

- 不越界：不调用真实 LLM，不生成可自动通过的主观题最终分，不进入 final_grade。
- 不做假功能：mock adapter 输出固定、低置信、必须复核，并在 ai_grade 中标记 `mock=true`。
- 与 STORY-015 衔接：复用 `answer_segment_answer` 作为答案文本来源，复用 `ai_grade` 作为建议分记录。
- 与 STORY-017 衔接：evidence 字段只记录 adapter 输出的证据引用，后续证据校验 Agent 再验证。

## 非范围

- 真实 OpenAI/本地/私有模型调用。
- Prompt 管理后台。
- 证据真实性校验。
- 人工复核任务生成。
- final_grade 和成绩发布。

## 验收标准

- 定义 `LLMGradingAdapter`，后续可替换真实模型。
- mock adapter 输出固定、可测试、明确 `mock=true`。
- 支持 short_answer、calculation、essay、discussion。
- 保存 ai_grade，包含 model_version、prompt_version、student_feedback、teacher_note。
- 低置信度、作文/论述题、低 OCR 置信计算题会触发人工复核。
- 非法 adapter 输出写入 failed ai_grade，不作为有效评分。
- 所有接口受 `grading:manage` 权限保护。
- AI 评分动作写审计。
- 测试通过。

## Implementation

- 新增 `000011_subjective_ai_grading.sql`：
  - 扩展 `ai_grade.question_type` 支持 `short_answer`、`calculation`、`essay`、`discussion`。
  - 扩展 `ai_grade.grader_type` 支持 `mock_llm_subjective`、`llm_subjective`。
  - 增加 `status`、`failure_reason`、`model_version`、`prompt_version`、`student_feedback`、`teacher_note`。
- 新增 `internal/subjective`：
  - `LLMGradingAdapter` 接口。
  - `MockLLMAdapter` 固定低置信输出，`mock=true`，强制人工复核。
  - schema validation 和复核规则。
  - MemoryStore、PostgresStore。
  - Handler。
- API Gateway 挂载：
  - `POST /api/v1/answer-segments/{id}/subjective-ai-grade`
- 更新 `system/info` capabilities：
  - `subjective_ai_grading_interface`
  - `mock_llm_grading_adapter`
  - `subjective_ai_grade_failure_recording`
- 更新 `system/info` not_implemented：
  - `real_subjective_model_inference`
- 新增 `docs/api/subjective-grading.md`，更新根 README 和 API Gateway README。

## Implementation Review

逐项检查结果：

- Adapter 接口：`LLMGradingAdapter` 已定义，后续可替换真实模型。
- mock 标记：`MockLLMAdapter` 输出固定、低置信、`mock=true`、`mock_llm_output` 风险标记，测试覆盖。
- 题型支持：支持 `short_answer`、`calculation`、`essay`、`discussion`，非支持题型拒绝。
- ai_grade 保存：保存 model_version、prompt_version、student_feedback、teacher_note、status、failure_reason。
- 低置信复核：低于 `model_policy.min_confidence` 自动复核，测试覆盖。
- 作文/论述复核：`essay`、`discussion` 默认复核，测试覆盖 essay。
- 计算题 OCR 复核：`calculation` 低 OCR confidence 自动复核，测试覆盖。
- schema 校验：adapter 输出分数越界/置信度越界/缺 raw_output 会失败。
- 非法输出失败落库：非法 adapter 输出写 `status=failed` 的 ai_grade，不作为有效评分，测试覆盖。
- 权限：接口要求 `grading:manage`，401/403 测试覆盖。
- 审计：成功和失败均写 audit，测试覆盖。
- 不越界：未接入真实模型，不生成 final_grade，不自动通过主观题。

实现审阅中关注点：

- mock 输出不提供看似真实的理由或高置信评分，而是固定低置信并提醒人工复核。
- PostgresStore 从 answer_segment、question、rubric、answer_segment_answer 读取真实上下文。
- 失败输出仍落 `ai_grade.status=failed`，方便审计和后续排障。

## Fixes

- 修正 `system/info` 测试，使未实现项从泛化的 `subjective_ai_grading` 改为更准确的 `real_subjective_model_inference`。
- 为非法 adapter 输出补充 failed ai_grade 测试，确保不会落为有效评分。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
gofmt -w internal
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
gofmt -w internal -> passed
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

接口验证证据：

```text
system/info -> capabilities include subjective_ai_grading_interface, mock_llm_grading_adapter, subjective_ai_grade_failure_recording
system/info -> not_implemented includes real_subjective_model_inference
POST /api/v1/answer-segments/{id}/subjective-ai-grade without grading:manage -> 403 covered by TestSubjectivePermissionDeniedAndUnauthenticated
POST /api/v1/answer-segments/{id}/subjective-ai-grade without token -> 401 covered by TestSubjectivePermissionDeniedAndUnauthenticated
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-017 证据校验 Agent`。
