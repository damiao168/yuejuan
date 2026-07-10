# STORY-021 申诉流程

## 状态

Approved

## 目标

实现后端学生申诉处理基础闭环：学生只能对自己已发布且锁定的成绩提交申诉，申诉对象可指向整场考试、某道题或某个扣分点；教师/仲裁员可查看申诉详情和评分证据，处理申诉并在需要改分时生成 `score_adjustment` 记录、更新最终分和总分、写审计；学生可查看处理结果；管理员可查看申诉统计。

本 Story 只实现后端 API、存储、状态流转、评分证据快照和审计，不实现 Web 申诉页面、不实现通知、不实现多级改分审批、不实现学生账号与学生档案的完整绑定后台。

## Plan

- 新增 migration：
  - `appeal:create`、`appeal:read`、`appeal:manage` 权限。
  - 为学生分配创建/读取权限，为教师、仲裁员、管理员、审计员分配读取/处理权限。
  - `appeal` 表。
  - `score_adjustment` 表。
- 新增 `internal/appeal`：
  - 类型定义：申诉、申诉证据、改分记录、统计、输入类型。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 创建申诉：
  - `POST /api/v1/appeals`
  - 必须引用 `exam_id` 和 `student_id`。
  - `target_type` 支持 `exam`、`question`、`deduction_point`。
  - `question` 和 `deduction_point` 类型必须关联 `final_grade_id`。
  - `deduction_point` 必须提供 `deduction_point_id`。
  - 只能对 `submission_grade.status=published` 且 `locked=true` 的成绩申诉。
  - 普通学生请求必须匹配 `auth.User.DataScope.student_id`。
- 查询申诉：
  - `GET /api/v1/appeals`
  - `GET /api/v1/appeals/{id}`
  - 学生只能看到自己的申诉。
  - `appeal:manage` 用户可按状态、考试、学生筛选。
- 申诉详情证据：
  - 原始答案：来自 `answer_segment_answer` 最新记录。
  - OCR：来自已有 OCR/答案文本，缺失时为空。
  - AI 评分：来自 `ai_grade` 摘要。
  - 人工评分：来自 `human_grade` 摘要。
  - Rubric：来自最新 `question_rubric`。
  - 最终分：来自 `final_grade`。
  - 历史修改记录：来自 `score_adjustment`。
  - 不声称实现图片级原始答卷渲染或真实 OCR 聚合页面。
- 处理申诉：
  - `POST /api/v1/appeals/{id}/review`
  - 支持状态：`under_review`、`need_more_info`、`accepted`、`rejected`、`score_adjusted`。
  - `score_adjusted` 必须提供 `adjusted_score`，并生成 `score_adjustment`。
  - 改分只能在 0 到题目满分之间。
  - 改分后更新对应 `final_grade.score`，保持 `locked=true`，并重算 `submission_grade.total_score`。
- 关闭申诉：
  - `POST /api/v1/appeals/{id}/close`
  - 状态置为 `closed`。
- 统计：
  - `GET /api/v1/appeals/statistics`
  - 返回总数、各状态数量、已改分数量、平均处理时长。
- 更新文档：
  - `docs/api/appeals.md`
  - README、API Gateway README、`system/info` capabilities。
- 测试：
  - 未发布成绩不能申诉。
  - 学生只能申诉自己的成绩。
  - 可提交考试/题目/扣分点申诉。
  - 查询详情包含评分证据和历史改分记录。
  - 教师/仲裁员可处理申诉。
  - 改分生成 `score_adjustment` 并更新 final/submission grade。
  - 学生可查看处理结果。
  - 管理员可查看统计。
  - 权限不足返回 403。
  - 审计覆盖创建、处理、改分、关闭。

## Plan Review

- 不越界：不实现 Web 申诉页面、不实现通知、不实现多级审批、不实现申诉导出、不实现完整学生账号绑定后台。
- 不做假功能：原始答卷、OCR、AI 评分只返回当前系统已有文本/结构化记录；没有真实图像渲染或 OCR 聚合时为空。
- 与 STORY-020 衔接：只能对已发布并锁定的 `submission_grade` 创建申诉；改分通过 `score_adjustment` 留痕，并更新锁定成绩。
- 与后续 Story 衔接：Web 申诉页面、学生端 UI、通知和报表留给后续 Web/报告 Story。
- 与当前仓库实际一致：沿用 Go API Gateway、MemoryStore/PostgresStore、RBAC、audit 和 migration 模式；使用 `auth.User.DataScope.student_id` 作为学生身份绑定边界。

## 非范围

- Web 申诉中心页面。
- 学生端 UI。
- 申诉通知。
- 多级审批。
- 批量申诉处理。
- 申诉导出。
- 图片级原始答卷查看器。
- 完整学生账号绑定后台。

## 验收标准

- 可创建 appeal。
- 申诉对象支持整场考试、某一道题、某个扣分点。
- 未发布成绩不能申诉。
- 学生只能申诉自己的成绩。
- 申诉包含 reason 和可选 attachment。
- 支持状态：`submitted`、`under_review`、`need_more_info`、`accepted`、`rejected`、`score_adjusted`、`closed`。
- 教师/仲裁员可处理申诉。
- 申诉详情包含原始答案、OCR、AI 评分、人工评分、Rubric、最终分、历史修改记录。
- 改分生成 score_adjustment。
- 改分更新 final_grade 和 submission_grade。
- 申诉创建、处理、改分、关闭写审计。
- 学生可查看处理结果。
- 管理员可查看申诉统计。
- 测试通过。

## Implementation

新增 migration `000016_appeal_workflow.sql`：

- 新增 `appeal:create`、`appeal:read`、`appeal:manage` 权限。
- 学生默认获得申诉创建和读取权限。
- 教师、仲裁员、平台/租户/学校管理员默认获得申诉读取和处理权限。
- 审计员默认只获得申诉读取权限，不授予处理/改分权限。
- 新增 `appeal` 表，记录申诉对象、原因、附件、状态、处理人、关闭人和审计关联字段。
- 新增 `score_adjustment` 表，记录申诉改分前后分数、delta、原因和改分人。

新增 `internal/appeal`：

- `types.go`：申诉、证据快照、改分记录、筛选、统计和 Store 接口。
- `store_memory.go`：内存实现，用于单元测试和路由测试。
- `store_postgres.go`：Postgres 实现，创建申诉、读取详情、处理申诉、改分、关闭和统计。
- `handlers.go`：HTTP Handler、学生 scope 校验、错误映射和审计写入。
- `store_test.go`：Store 级业务规则测试。

新增接口：

- `POST /api/v1/appeals`
- `GET /api/v1/appeals`
- `GET /api/v1/appeals/statistics`
- `GET /api/v1/appeals/{id}`
- `POST /api/v1/appeals/{id}/review`
- `POST /api/v1/appeals/{id}/close`

更新服务路由：

- `POST /api/v1/appeals` 要求 `appeal:create`。
- `GET /api/v1/appeals` 和 `GET /api/v1/appeals/{id}` 要求 `appeal:read`。
- `GET /api/v1/appeals/statistics`、`POST /api/v1/appeals/{id}/review`、`POST /api/v1/appeals/{id}/close` 要求 `appeal:manage`。

实现规则：

- 普通学生必须具备 `auth.User.DataScope.student_id`，且只能创建/查看自己的申诉。
- 申诉只能针对 `submission_grade.status=published` 且 `locked=true` 的成绩。
- `target_type` 支持 `exam`、`question`、`deduction_point`。
- `question` 和 `deduction_point` 必须关联 `final_grade_id`。
- `deduction_point` 必须提供 `deduction_point_id`。
- 详情返回当前系统已存储的原始答案文本、OCR 文本、AI 评分摘要、人工评分摘要、Rubric、最终分和改分历史。
- `score_adjusted` 必须提供 `adjusted_score`，并校验分数范围。
- 改分生成 `score_adjustment`，更新 `final_grade.score`，保持 `final_grade.locked=true`，并调整 `submission_grade.total_score`，保持已发布成绩锁定。
- `accepted`、`rejected`、`score_adjusted` 是 review 终态，不能被再次 review 覆盖，只能通过 close 关闭。
- 审计覆盖 `appeal.created`、`appeal.reviewed`、`appeal.score_adjusted`、`appeal.closed`。

更新文档：

- 新增 `docs/api/appeals.md`。
- 更新根 README 和 API Gateway README。
- 更新 `system/info` capabilities。

## Implementation Review

逐项检查结果：

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 可创建 appeal | `CreateAppeal`、route test | 通过 |
| 申诉对象支持考试/题目/扣分点 | `validateCreateInput`、`TestCreateAppealRequiresPublishedGradeAndSupportsTargets` | 通过 |
| 未发布成绩不能申诉 | `submission_grade.status=published AND locked=true` 校验和 Store 测试 | 通过 |
| 学生只能申诉自己的成绩 | Handler `DataScope.student_id` 校验和 route 测试 | 通过 |
| 申诉包含 reason 和可选 attachment | `CreateAppealInput`、migration、route 测试 | 通过 |
| 支持全部申诉状态 | migration check、`Statuses()`、测试 | 通过 |
| 教师/仲裁员可处理申诉 | `appeal:manage` 路由和 migration | 通过 |
| 申诉详情包含评分证据 | `withDetails`、`loadEvidence`、Store/route 测试 | 通过 |
| 改分生成 score_adjustment | `applyAdjustmentTx`、Store/route 测试 | 通过 |
| 改分更新 final_grade 和 submission_grade | Store/Postgres 实现和测试 | 通过 |
| 创建、处理、改分、关闭写审计 | route 测试断言 audit action | 通过 |
| 学生可查看处理结果 | route 测试 | 通过 |
| 管理员可查看申诉统计 | `Statistics`、route test | 通过 |
| 权限不足返回 403 | `TestAppealRoutesRequirePermission` | 通过 |
| 测试通过 | `go test ./...` | 通过 |

实现审阅中发现并修正：

- 终态申诉原本仍可再次 review 覆盖结果，已补充状态流转校验。
- migration 原本给审计员授予 `appeal:manage`，与 PRD 中“审计员只读审计日志/合规记录”的边界不一致，已收紧为 `appeal:read`。
- README 原本仍写着“申诉业务尚未实现”，已改为“申诉后端已实现，Web 申诉中心未实现”。

## Fixes

- 新增 `canReviewTransition`，阻止 `accepted`、`rejected`、`score_adjusted` 和 `closed` 被 review 覆盖。
- `CloseAppeal` 对已关闭申诉返回 `ErrInvalidTransition`。
- 补充 `TestReviewAppealDoesNotReopenTerminalStatus`。
- 调整 `000016_appeal_workflow.sql`，审计员只授予 `appeal:read`。
- 新增 `docs/api/appeals.md`。
- 更新 README、API Gateway README。

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
POST /api/v1/appeals -> route test covered
GET /api/v1/appeals/{id} as student -> route test covered
POST /api/v1/appeals/{id}/review score_adjusted -> route test covered
GET /api/v1/appeals/statistics -> route test covered
POST /api/v1/appeals/{id}/close -> route test covered
appeal audit actions -> route test covered
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-022 学情报告`。
