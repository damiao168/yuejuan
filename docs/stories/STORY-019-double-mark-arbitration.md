# STORY-019 双评与仲裁模块

## 状态

Approved

## 目标

实现后端双评与仲裁基础闭环：支持考试级或题目级 double_mark 策略配置，为同一 `answer_segment` 创建两个独立且双盲的 `review_task`，两名阅卷员提交后自动比较分差，分差不超过阈值时按配置策略写入 `final_grade`，分差超过阈值时创建 `arbitration_task`，仲裁员提交最终分后写入 `final_grade`，并记录全流程审计。

本 Story 只实现后端 API、存储、策略与测试，不实现 Web 仲裁页面、不实现成绩发布审批、不实现学生端成绩查看、不实现真实 OCR/AI 上下文聚合页面。

## Plan

- 新增 migration：
  - `arbitration:manage` 权限，并分配给管理员、教师、阅卷员、仲裁员、审计员等已有角色。
  - `double_mark_policy` 表，支持考试级和题目级策略，字段包含 `enabled`、`threshold`、`resolution_strategy`、`allow_same_arbitrator`。
  - `double_mark_session` 表，记录同一 `answer_segment` 的双评会话、两个 review_task、两名阅卷员、分差、状态和 final_grade。
  - `arbitration_task` 表，记录超阈值仲裁任务、仲裁员、两次评分、分差原因、状态和说明。
  - `final_grade` 表，记录双评自动合分或仲裁最终分，明确 `source` 与 `locked=false`，成绩发布锁定留给后续 Story。
- 扩展 `internal/review`：
  - 类型定义：双评策略、双评会话、仲裁任务、最终分、输入输出类型。
  - Store 方法：
    - 配置/查询 double_mark policy。
    - 创建 double_mark session，并生成两个独立 `review_task`。
    - 两个评分完成后自动比较分差。
    - 分差不超过阈值时按 `average`、`first`、`second`、`higher`、`lower` 生成 `final_grade`。
    - 分差超过阈值时创建 `arbitration_task`。
    - 查询和提交仲裁任务。
  - MemoryStore 与 PostgresStore 实现。
  - Handler 与路由。
- 双盲规则：
  - 双评任务由两个独立 `review_task` 表示。
  - 阅卷员提交 `review_task` 时只能看到自己的任务和自己的提交响应。
  - 普通 `GET /review-tasks` 和 `GET /review-tasks/{id}` 不返回另一名阅卷员评分。
  - 仲裁任务详情可以返回两名阅卷员分数，但该接口受 `arbitration:manage` 权限保护。
- 仲裁规则：
  - `arbitrator_id` 不能为空。
  - 默认不允许仲裁员等于前两名阅卷员之一；仅当 `allow_same_arbitrator=true` 才允许。
  - 仲裁最终分必须在 0 到题目满分之间。
  - 仲裁提交后写 `final_grade`，并将 `arbitration_task` 与 `double_mark_session` 标记完成。
- 接口：
  - `PUT /api/v1/exams/{examId}/double-mark-policy`
  - `PUT /api/v1/questions/{id}/double-mark-policy`
  - `GET /api/v1/double-mark-policies?exam_id=&question_id=`
  - `POST /api/v1/double-mark-sessions`
  - `GET /api/v1/double-mark-sessions`
  - `GET /api/v1/double-mark-sessions/{id}`
  - `POST /api/v1/arbitration-tasks`
  - `GET /api/v1/arbitration-tasks`
  - `GET /api/v1/arbitration-tasks/{id}`
  - `POST /api/v1/arbitration-tasks/{id}/assign`
  - `POST /api/v1/arbitration-tasks/{id}/submit`
- 更新文档：
  - `docs/api/arbitration.md`
  - 根 README 与 API Gateway README 的能力说明。
  - `system/info` capabilities 与 not_implemented 边界。
- 测试：
  - 双评会话生成两个独立 review_task。
  - 两名阅卷员不能相同。
  - 双盲期间普通 review_task 查询不暴露对方评分。
  - 两评完成后分差 `<= threshold` 自动写 final_grade。
  - 五种合分策略均覆盖。
  - 分差 `> threshold` 自动创建 arbitration_task。
  - 仲裁员默认不能是前两个阅卷员之一。
  - `allow_same_arbitrator=true` 时允许同人仲裁。
  - 仲裁提交写 final_grade。
  - 权限与审计覆盖。

## Plan Review

- 不越界：不做 Web 仲裁页面、不做成绩发布审批、不做学生查看成绩、不做申诉、不实现真实 OCR/AI 上下文渲染。
- 不做假功能：仲裁任务详情可以承载 `raw_answer`、`ocr_text`、`ai_suggestion` 字段，但当前仅保存/返回已有输入或上下文占位，不声称已完成真实 OCR/AI 聚合。
- 与 STORY-018 衔接：复用 `review_task` 与 `human_grade`，双评本质是两个独立人工阅卷任务；提交 human_grade 后触发双评会话比较。
- 与 STORY-020 衔接：本 Story 写入 `final_grade`，但不负责整场考试最终确认、发布、锁定和导出。
- 与当前仓库实际一致：沿用 Go API Gateway、`internal/review`、MemoryStore/PostgresStore、RBAC、audit 和 migration 模式。
- 风险控制：Store 接口扩展会影响路由测试，需要同步更新测试 helper；Postgres 需要事务保证 review_task、double_mark_session、arbitration_task、final_grade 状态一致。

## 非范围

- Web 双评仲裁页面。
- 成绩发布审批。
- 学生端成绩查看。
- 申诉改分。
- 真实 OCR 内容聚合。
- 真实 AI 建议聚合。
- 三评、多评、多仲裁员投票。
- 阅卷员质量分析报表。

## 验收标准

- 可配置考试级 double_mark policy。
- 可配置题目级 double_mark policy，题目级优先于考试级。
- 同一 answer_segment 可生成两个独立 review_task。
- 两个 review_task 必须分配给不同阅卷员。
- 双盲期间普通阅卷员不能通过 review_task API 看到对方评分。
- 两个评分完成后自动比较分差。
- 分差 `<= threshold` 时按配置策略生成 final_grade。
- 支持 `average`、`first`、`second`、`higher`、`lower` 策略。
- 分差 `> threshold` 时创建 arbitration_task。
- 仲裁员可查询仲裁详情，包含匿名码、题号、两名阅卷员分数、分差和上下文字段。
- 仲裁员默认不能是前两个阅卷员之一。
- 配置允许时仲裁员可以是前两个阅卷员之一。
- 仲裁员提交最终分后写 final_grade。
- 双评、自动合分、仲裁创建、仲裁提交均写审计日志。
- 接口受 `review:manage` 或 `arbitration:manage` 权限保护。
- 测试通过。

## Implementation

新增 migration `000014_double_mark_arbitration.sql`：

- 新增 `arbitrator` 角色和 `arbitration:manage` 权限。
- 为平台管理员、租户管理员、学校管理员、教师、仲裁员和审计员分配仲裁权限。
- 为 `review_task` 增加 `grade_round`，支持 `single`、`first_mark`、`second_mark`、`appeal_review`。
- 新增 `double_mark_policy`。
- 新增 `double_mark_session`。
- 新增 `arbitration_task`。
- 新增 `final_grade`。

扩展 `internal/review`：

- `types.go` 增加 double mark policy、double mark session、arbitration task、review context 和 final grade 类型。
- `store_memory.go` 实现：
  - 考试级/题目级策略配置。
  - 题目级策略优先。
  - 双评会话创建。
  - 两条独立 review_task 创建。
  - `SubmitGrade` 后自动比较分差。
  - 五种合分策略。
  - 超阈值自动创建 arbitration_task。
  - 仲裁员限制。
  - 仲裁提交写 final_grade。
- `store_postgres.go` 实现同等 Store 能力，并使用事务保护双评会话、任务、仲裁和 final_grade 状态一致。
- `handlers.go` 增加 policy、session、arbitration API Handler，并补充审计。

扩展路由：

- `PUT /api/v1/exams/{examId}/double-mark-policy`
- `PUT /api/v1/questions/{id}/double-mark-policy`
- `GET /api/v1/double-mark-policies`
- `POST /api/v1/double-mark-sessions`
- `GET /api/v1/double-mark-sessions`
- `GET /api/v1/double-mark-sessions/{id}`
- `POST /api/v1/arbitration-tasks`
- `GET /api/v1/arbitration-tasks`
- `GET /api/v1/arbitration-tasks/{id}`
- `POST /api/v1/arbitration-tasks/{id}/assign`
- `POST /api/v1/arbitration-tasks/{id}/submit`

更新文档：

- 新增 `docs/api/arbitration.md`。
- 更新 `docs/api/review.md`，移除 STORY-018 时代的过期边界表述。
- 更新根 README 和 API Gateway README。
- 更新 `system/info` capabilities。

## Implementation Review

逐项检查结果：

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 可配置考试级 double_mark policy | `SetExamDoubleMarkPolicy`、`PUT /api/v1/exams/{examId}/double-mark-policy`、`TestDoubleMarkAndArbitrationRoutes` | 通过 |
| 可配置题目级 double_mark policy，题目级优先 | `SetQuestionDoubleMarkPolicy`、`TestQuestionDoubleMarkPolicyOverridesExamPolicy` | 通过 |
| 同一 answer_segment 生成两个独立 review_task | `CreateDoubleMarkSession`、`TestDoubleMarkAutoFinalizesByResolutionStrategy`、路由测试断言两个任务 | 通过 |
| 两个 review_task 分配给不同阅卷员 | Store 校验 `FirstReviewerID != SecondReviewerID` | 通过 |
| 双盲期间普通阅卷员不能通过 review_task API 看到对方评分 | review_task 响应不返回 human_grade 对方分数，`TestDoubleMarkAndArbitrationRoutes` 覆盖 | 通过 |
| 两个评分完成后自动比较分差 | `resolveDoubleMarkAfterGradeLocked`、`resolveDoubleMarkAfterGradeTx` | 通过 |
| 分差 `<= threshold` 按策略生成 final_grade | `TestDoubleMarkAutoFinalizesByResolutionStrategy` 覆盖 | 通过 |
| 支持五种策略 | `average`、`first`、`second`、`higher`、`lower` 子测试覆盖 | 通过 |
| 分差 `> threshold` 创建 arbitration_task | `TestDoubleMarkCreatesArbitrationAndBlocksSameArbitratorByDefault`、路由测试覆盖 | 通过 |
| 仲裁详情包含匿名码、题号、两名阅卷员分数、分差和上下文字段 | `ArbitrationTask` 类型和路由测试覆盖 `raw_answer`、`first_score`、`second_score` | 通过 |
| 仲裁员默认不能是前两个阅卷员之一 | Store 和路由测试覆盖 403/ErrForbidden | 通过 |
| 配置允许时可同人仲裁 | `TestAllowSameArbitratorPolicyAllowsReviewerAsArbitrator` | 通过 |
| 仲裁提交写 final_grade | `SubmitArbitration` 和测试覆盖 | 通过 |
| 审计 | 路由测试覆盖 `review.double_mark_policy_set`、`review.double_mark_session_created`、`arbitration.task_created`、`arbitration.task_assigned`、`arbitration.submitted`、`final_grade.created` | 通过 |
| 权限保护 | `review:manage` 与 `arbitration:manage` 路由保护，权限测试覆盖 arbitration 403 | 通过 |
| 不越界 | 未实现 Web 仲裁页面、成绩发布审批、申诉、真实 OCR/AI 聚合 | 通过 |

实现审阅中关注点：

- Postgres Store 的双评提交、自动合分、仲裁创建和仲裁提交均在事务中执行。
- 仲裁详情的上下文只返回已有 `answer_segment_answer` 和最新 `ai_grade` 摘要；没有真实数据时为空，不冒充真实 OCR/AI。
- `final_grade.locked=false`，成绩锁定和发布留给 STORY-020。
- 当前没有实现三评、多仲裁员投票或阅卷员质量分析，属于后续质量控制/报告 Story。

## Fixes

- 补充题目级 `enabled=false` 覆盖考试级启用策略的测试，明确题目级优先不只包含阈值覆盖。
- 更新 `docs/api/review.md`、根 README 和 API Gateway README，删除“双评仲裁尚未实现”的过期表述。
- 补充 `system/info` capabilities 和对应测试。
- 修正 Postgres `double_mark_policy` UPDATE 参数编号，避免真实数据库执行时出现未使用参数风险。
- 将 `human_grade.grade_round` 从固定 `single` 改为跟随 `review_task.grade_round`。

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
Select-String -Path .\README.md,.\services\api-gateway\README.md,.\docs\api\*.md -Pattern '双评仲裁、最终成绩|双评仲裁.*尚未实现|final_grade、成绩确认'
```

结果：

```text
gofmt -w internal -> passed
go test ./... -> passed
docker compose config -> passed
go build -> passed
stale docs search -> no matches
```

接口验证证据：

```text
PUT /api/v1/exams/{examId}/double-mark-policy -> covered by TestDoubleMarkAndArbitrationRoutes
POST /api/v1/double-mark-sessions -> covered by TestDoubleMarkAndArbitrationRoutes
POST /api/v1/review-tasks/{id}/submit first mark -> no second_score exposed
POST /api/v1/review-tasks/{id}/submit second mark with large diff -> arbitration_task returned
GET /api/v1/arbitration-tasks/{id} -> first_score/second_score/context returned only on arbitration endpoint
POST /api/v1/arbitration-tasks/{id}/assign to first reviewer -> 403 covered
POST /api/v1/arbitration-tasks/{id}/submit -> final_grade returned
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-020 最终成绩与成绩发布`。
