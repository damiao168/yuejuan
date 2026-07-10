# STORY-015 客观题与填空题判分

## 状态

Approved

## 目标

实现确定性规则判分能力，覆盖 `single_choice`、`multiple_choice`、`true_false`、`fill_blank`、`numeric`。判分结果写入 `ai_grade`，用于后续人工复核、证据校验和最终成绩流程。

本 Story 的核心原则：规则判分是真实能力，但不是模型推理；不能伪造 AI/LLM 输出。`ai_grade` 记录必须明确标记规则引擎来源，并保留答案来源、证据、风险标记和人工复核触发。

## Plan

- 新增 migration：
  - `grading:manage` 权限。
  - `answer_segment_answer`：给 answer_segment 绑定真实学生答案文本/结构化载荷，作为当前 Story 的可追踪答案来源。
  - `ai_grade`：保存规则判分结果、证据、风险、人工复核标记和审计引用。
- 新增 `internal/grading`：
  - 类型定义。
  - 规则评分 Engine，评分逻辑不写在 Handler。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 接口：
  - `PUT /api/v1/answer-segments/{id}/answer`
  - `POST /api/v1/answer-segments/{id}/rule-grade`
  - `GET /api/v1/answer-segments/{id}/ai-grades`
- 规则：
  - 从 answer_segment 关联的最新 `answer_segment_answer` 读取学生答案。
  - 从 `question_answer_key` 读取标准答案、等价答案和 tolerance。
  - `single_choice` 精确判分。
  - `true_false` 精确判分。
  - `multiple_choice`：
    - 全对得满分。
    - 错选不得分。
    - 少选在 `tolerance.allow_partial=true` 时按比例给分。
  - `fill_blank`：
    - 标准答案和等价答案匹配。
    - 支持 `tolerance.ignore_case`。
    - 支持 `tolerance.ignore_spaces`。
  - `numeric`：
    - 支持 `tolerance.absolute` 或 `tolerance.value` 数值容差。
    - 支持 `tolerance.unit_required` 和标准答案 unit。
  - 分数必须 clamp 到 `[0, question.score]`。
  - 结果包含 `suggested_score`、`confidence`、`matched_points`、`missing_points`、`evidence`、`risk_flags`、`needs_human_review`。
  - 低置信度、缺答案、缺标准答案、解析失败、单位不匹配等进入人工复核。
  - 高置信客观题可 `auto_pass=true`，但仍写 audit。

## Plan Review

- 不越界：不实现主观题 AI 评分，不调用模型，不实现证据校验 Agent，不生成最终成绩。
- 不做假功能：系统不会从图片/OCR 自动“猜出”学生答案；必须先通过真实接口给 answer_segment 记录答案文本或结构化载荷。
- 与 STORY-009 衔接：读取 question 和 question_answer_key。
- 与 STORY-013 衔接：答案绑定到 answer_segment。
- 与 STORY-014 衔接：后续可由 Orchestrator 创建 rule-based grading agent task，但本 Story 不自动调度 worker。

## 非范围

- 主观题 LLM 评分。
- OCR 文本自动归属到题目。
- 图像识别选择题涂点。
- 证据校验 Agent。
- human_grade、final_grade 和成绩发布。

## 验收标准

- 可以为 answer_segment 记录可追踪学生答案。
- 可以对 single_choice、true_false 精确判分。
- multiple_choice 支持全对、错选 0 分、少选按配置给部分分。
- fill_blank 支持等价答案、忽略大小写和忽略空格。
- numeric 支持数值容差和单位要求。
- 缺答案、缺 answer_key、解析失败等返回明确错误或风险标记。
- 生成 `ai_grade` 记录，分数不超过题目满分。
- 结果包含建议分、置信度、命中点、缺失点、证据、风险标记、是否人工复核。
- 所有接口受 `grading:manage` 权限保护。
- 记录答案、执行判分写审计。
- 评分逻辑不写在 Controller。
- 测试通过。

## Implementation

- 新增 `000010_rule_grading.sql`：
  - `grading:manage` 权限。
  - `answer_segment_answer` 表，记录 answer_segment 的真实答案文本/结构化载荷。
  - `ai_grade` 表，保存规则判分结果、证据、风险、复核标记、`grader_type` 和 `rule_version`。
- 新增 `internal/grading`：
  - `Engine`：确定性规则评分，不依赖 HTTP Handler。
  - `MemoryStore`：测试和无数据库默认路由使用。
  - `PostgresStore`：从 answer_segment、question、question_answer_key、answer_segment_answer 读取真实上下文，并持久化 `ai_grade`。
  - `Handler`：答案记录、规则判分、ai_grade 列表接口。
- API Gateway 挂载：
  - `PUT /api/v1/answer-segments/{id}/answer`
  - `POST /api/v1/answer-segments/{id}/rule-grade`
  - `GET /api/v1/answer-segments/{id}/ai-grades`
- 更新 `system/info` capabilities：
  - `answer_segment_answer_capture`
  - `rule_based_objective_grading`
  - `ai_grade_recording`
  - `grading_low_confidence_review_trigger`
- 更新 `system/info` not_implemented：
  - `subjective_ai_grading`
- 新增 `docs/api/grading.md`，更新根 README 和 API Gateway README。

## Implementation Review

逐项检查结果：

- 可记录学生答案：`PUT /api/v1/answer-segments/{id}/answer` 保存答案文本/结构化载荷，测试覆盖。
- single_choice：精确判分，测试覆盖。
- true_false：支持 true/false 和中文“正确/错误”等等价输入，测试覆盖。
- multiple_choice：全对满分、错选 0 分、`allow_partial=true` 少选按比例给分，测试覆盖。
- fill_blank：支持等价答案、忽略大小写、忽略空格，测试覆盖。
- numeric：支持数值容差和单位要求，支持 `9.81m/s2` 这类无空格单位写法，测试覆盖。
- 缺答案/缺标准答案：分别返回 `answer_segment_answer_missing`、`question_answer_key_missing`，测试覆盖。
- ai_grade：生成 `grader_type=rule_based_objective`、`rule_version=objective-rules-v1`、`mock=false` 的记录。
- 分数上限：Engine 在 finalize 阶段 clamp 到 `[0, max_score]`，测试断言分数不超过满分。
- 权限：所有接口要求 `grading:manage`，401/403 测试覆盖。
- 审计：答案记录和规则判分均写 audit，测试覆盖。
- 不越界：未实现主观题 AI 评分、模型调用、OCR 文本自动归属、证据校验、最终成绩。

实现审阅中关注点：

- 评分逻辑在 `internal/grading/engine.go`，Handler 只做 HTTP 编排。
- `ai_grade` 使用统一结果结构，但不伪造模型能力；model/prompt 字段保留为空，规则来源通过 `grader_type`/`rule_version` 表达。
- PostgresStore 使用 LATERAL 子查询读取最新 answer_key 和最新 segment answer，确保数据来源真实且可追踪。

## Fixes

- 增加 numeric 无空格单位解析，覆盖 `9.81m/s2`。
- 增加显式缺答案和缺 answer_key 的 API 测试。
- 增加五类题型规则引擎测试，确保规则不依赖 Controller。

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
system/info -> capabilities include answer_segment_answer_capture, rule_based_objective_grading, ai_grade_recording
system/info -> not_implemented includes subjective_ai_grading, ocr_engine_inference, agent_worker_runtime
PUT /api/v1/answer-segments/{id}/answer without grading:manage -> 403 covered by TestGradingPermissionDeniedAndUnauthenticated
POST /api/v1/answer-segments/{id}/rule-grade without token -> 401 covered by TestGradingPermissionDeniedAndUnauthenticated
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-016 主观题 AI 评分接口`。
