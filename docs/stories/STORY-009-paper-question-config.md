# STORY-009 试卷与题目配置模块

## 状态

Approved

## 目标

实现试卷文件元数据、题目配置、标准答案、Rubric 版本管理和试卷完整性检查。

## Plan

- 新增 migration：
  - `file_asset`
  - `exam_paper`
  - `question`
  - `question_answer_key`
  - `rubric_version`
  - `question_rubric`
- 新增 `internal/paper`：
  - types。
  - Store interface。
  - MemoryStore。
  - PostgresStore。
  - handlers。
- 接口：
  - `POST /api/v1/exams/{examId}/papers`
  - `GET /api/v1/exams/{examId}/papers`
  - `POST /api/v1/exams/{examId}/questions`
  - `GET /api/v1/exams/{examId}/questions`
  - `PATCH /api/v1/questions/{id}`
  - `DELETE /api/v1/questions/{id}`
  - `POST /api/v1/questions/{id}/rubric`
  - `POST /api/v1/exams/{examId}/validate-paper-config`
- 验证：
  - 题目总分等于考试总分。
  - Rubric 采分点总分等于题目分值。
  - 修改 Rubric 必须创建新版本。
  - locked Rubric 不可修改。
- 审计：
  - 试卷元数据登记、题目创建/修改/删除、Rubric 修改、完整性检查都写审计。

## Plan Review

- 不越界：不实现真实文件二进制上传，真实上传属于 STORY-010。
- 本 Story 保存 file metadata 和 storage_key，但不把文件内容写入数据库。
- 题型枚举必须受控。
- Rubric 版本必须独立记录，不能覆盖旧版本。
- 完整性检查必须返回问题列表，不能只返回 true/false。

## 非范围

- 不实现 MinIO 上传。
- 不实现 OCR。
- 不实现答卷采集。
- 不实现 AI 评分。

## 验收标准

- 题目题型受控。
- Rubric 分值校验生效。
- locked Rubric 不能修改。
- 每次 Rubric 修改产生新版本。
- 完整性检查能发现总分不匹配。
- 所有修改写审计。
- 测试通过。

## Implementation

- 新增 `000004_paper_question.sql`，建立 `file_asset`、`exam_paper`、`question`、`question_answer_key`、`rubric_version`、`question_rubric`。
- 新增 `internal/paper`：
  - 类型定义和 Store interface。
  - MemoryStore，用于单元测试和路由测试。
  - PostgresStore，用于真实 PostgreSQL 持久化。
  - Handler，提供试卷元数据、题目、Rubric 和配置校验接口。
  - validation，集中维护题型和 Rubric 状态枚举。
- 在 API Gateway 路由中挂载 STORY-009 接口，并统一要求 `exam:manage` 权限。
- 更新 `system/info` capabilities：
  - `paper_metadata`
  - `question_config`
  - `rubric_versioning`
  - `paper_config_validation`
- 保留 `file_upload` 在 `not_implemented`，明确真实二进制上传属于 STORY-010。
- 新增 `docs/api/paper-question.md`。
- 更新根 README 和 API Gateway README。

## Implementation Review

逐项检查结果：

- 题型受控：服务端和数据库 CHECK 双层限制，测试覆盖非法题型。
- Rubric 分值校验：`max_score` 和采分点总分都必须等于题目分值，测试覆盖 mismatch。
- Rubric 版本：每次创建生成递增版本，不覆盖旧版本。
- locked 保护：最新 Rubric 为 `locked` 时拒绝继续创建，返回 `409 rubric_locked`。
- 完整性检查：返回 `issues` 列表，测试覆盖总分不匹配。
- 权限：所有路由通过 `exam:manage` 中间件保护，测试覆盖权限不足。
- 审计：创建试卷元数据、创建/修改/删除题目、创建 Rubric、配置校验均调用 audit。
- 边界：未实现真实上传、OCR、答卷采集、AI 评分。

实现审阅中发现并修正的问题：

- `system/info` 未声明 STORY-009 已实现能力。
- 题目引用 `exam_paper_id` 时缺少同考试校验。
- Postgres `file_asset.owner_id` 原先会误用 `exam_id`，已改为绑定真实 `exam_paper.id`。
- `CreateQuestion` 对 store 业务错误曾返回 500，已接入统一错误映射。

## Fixes

- 增加同租户、同考试的 `exam_paper_id` 校验。
- 增加跨考试试卷引用失败测试。
- 增加 `system/info` 能力声明测试。
- 修正 Postgres paper/file 元数据写入顺序和归属关系。
- 修正创建题目时的业务错误 HTTP 映射。

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
GET http://127.0.0.1:18089/api/v1/system/info
GET http://127.0.0.1:18089/api/v1/exams/exam-1/questions
```

结果：

```text
system/info -> capabilities include paper_metadata, question_config, rubric_versioning, paper_config_validation
GET /api/v1/exams/exam-1/questions without token -> 401
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-010 文件上传与对象存储`。
