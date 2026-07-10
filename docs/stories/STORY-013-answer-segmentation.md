# STORY-013 答题区域切分模块

## 状态

Approved

## 目标

基于已配置的题目 `answer_area` 和答卷页面，生成结构化 `answer_segment` 元数据，为后续 AI 阅卷、人工阅卷和 OCR 证据对齐提供单题答案入口。

## Plan

- 新增 migration：
  - `answer_segment`
  - `segment:manage` 权限。
- 新增 `internal/segment`：
  - 类型定义和校验。
  - Store interface。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 接口：
  - `POST /api/v1/submissions/{id}/segment-answers`
  - `GET /api/v1/submissions/{id}/answer-segments`
  - `PATCH /api/v1/answer-segments/{id}`
- 规则：
  - 所有接口要求 `segment:manage`。
  - submission 必须至少完成 OCR 任务结果回写，或者处于 `ready_for_ocr` 之后的可切分状态。
  - 题目必须有 `answer_area`。
  - `answer_area.page` 必须能匹配 submission page。
  - 同一 submission + question 只生成一个 active segment。
  - 生成结果返回 issues，不只返回 true/false。
  - 人工可修正 bbox、状态和 notes。
  - 所有关键动作写审计。

## Plan Review

- 不越界：不做真实图像裁剪，不创建 cropped image 文件，不做模型切分，不做 AI 阅卷。
- 不做假功能：只基于题目配置的 `answer_area` 生成 segment 元数据，不伪造视觉识别能力。
- 与 STORY-009 衔接：使用题目配置中的 `answer_area`。
- 与 STORY-011 衔接：使用 submission page。
- 与 STORY-012 衔接：可在 OCR 完成后执行，但不依赖 OCR 文本生成答案。

## 非范围

- 图像裁剪和保存。
- 坐标自动校准。
- 模型版面识别。
- OCR 文本归属算法。
- AI 阅卷。

## 验收标准

- 可以为已配置题目和已上传页面的 submission 生成 answer_segment。
- 缺少题目、缺少 answer_area、缺少对应页码会返回 issue。
- 重复执行不会生成重复 active segment。
- 可以人工修正 bbox、状态和 notes。
- 所有接口受 `segment:manage` 权限保护。
- 生成和人工修正写审计。
- 测试通过。

## Implementation

- 新增 `000008_answer_segment.sql`：
  - `segment:manage` 权限。
  - `answer_segment` 表。
  - submission/question 索引。
- 新增 `internal/segment`：
  - 类型定义和 bbox 校验。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- API Gateway 挂载：
  - `POST /api/v1/submissions/{id}/segment-answers`
  - `GET /api/v1/submissions/{id}/answer-segments`
  - `PATCH /api/v1/answer-segments/{id}`
- 更新 `system/info` capabilities：
  - `answer_segmentation_metadata`
  - `answer_segment_manual_review`
- 新增 `docs/api/answer-segments.md`，更新 README。

## Implementation Review

逐项检查结果：

- 生成 answer_segment：基于 question `answer_area` 和 submission page 生成元数据，测试覆盖。
- 缺配置问题：缺题目、缺 `answer_area`、缺页面会返回 issues。
- 幂等性：同一 submission + question 不重复生成，测试覆盖。
- 人工修正：支持修正 bbox、状态、notes，bbox 修正后 source 变为 `manual`。
- 权限：所有接口要求 `segment:manage`，测试覆盖权限不足。
- 审计：生成和人工修正均调用 audit。
- 不越界：未实现图片裁剪、坐标自动校准、模型切分、OCR 文本归属和 AI 阅卷。

实现审阅中关注点：

- Postgres 通过唯一键约束 `(tenant_id, submission_id, question_id)` 保证幂等。
- Handler 明确要求 submission 为 `ready_for_ocr`，避免质量门禁前生成 segment。
- 响应包含 issue 列表，不只返回 true/false。

## Fixes

- 增加生成幂等性测试。
- 增加缺页和缺 answer_area issue 测试。
- 增加人工修正 source/status/notes 测试。
- 增加未 ready submission 和权限不足测试。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

启动验证：

```powershell
GET http://127.0.0.1:18093/api/v1/system/info
GET http://127.0.0.1:18093/api/v1/submissions/submission-1/answer-segments
```

结果：

```text
system/info -> capabilities include answer_segmentation_metadata, answer_segment_manual_review
system/info -> not_implemented includes ocr_engine_inference, ai_grading
GET /api/v1/submissions/submission-1/answer-segments without token -> 401
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-014 多智能体 Orchestrator`。
