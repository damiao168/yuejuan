# STORY-018 人工复核与阅卷工作流

## 状态

Approved

## 目标

实现后端人工复核与阅卷任务基础闭环：从 AI 低置信、OCR 低置信、主观题默认复核、证据校验失败、双评要求、分数异常或人工抽检等来源创建 `review_task`，支持分配阅卷员、阅卷员提交 `human_grade`、退回重评、列表和详情查询，并保证权限、分数校验、Rubric 校验和审计。

本 Story 只实现后端 API 与存储边界，不实现 Web 阅卷工作台、不生成 final_grade、不做双评仲裁自动比较。

## Plan

- 新增 migration：
  - `review:manage` 权限。
  - `review_task` 表。
  - `human_grade` 表。
- 新增 `internal/review`：
  - 类型定义。
  - 输入校验。
  - 状态流转。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 接口：
  - `POST /api/v1/review-tasks`
  - `GET /api/v1/review-tasks`
  - `GET /api/v1/review-tasks/{id}`
  - `POST /api/v1/review-tasks/{id}/assign`
  - `POST /api/v1/review-tasks/{id}/submit`
  - `POST /api/v1/review-tasks/{id}/return`
- 支持任务来源：
  - `ai_low_confidence`
  - `ocr_low_confidence`
  - `subjective_default_review`
  - `evidence_verification_failed`
  - `double_mark_required`
  - `score_anomaly`
  - `manual_sample`
- 支持任务状态：
  - `pending`
  - `assigned`
  - `in_progress`
  - `submitted`
  - `returned`
  - `completed`
- `human_grade` 输入：
  - `score`
  - `rubric_selections`
  - `comments`
  - `private_note`
  - `student_feedback`
  - `reason`
- 校验规则：
  - 阅卷员只能提交分配给自己的任务。
  - 分数必须在 0 到题目满分之间。
  - rubric_selections 必须能对应题目 Rubric points。
  - 任务必须处于 `assigned`、`in_progress` 或 `returned` 才能提交。
  - 退回必须提供 reason。
- 查询输出：
  - 包含 `anonymous_code`。
  - 默认不返回学生姓名，避免阅卷员看到学生身份。
- 写 audit：
  - 创建任务。
  - 分配任务。
  - 提交 human_grade。
  - 退回重评。

## Plan Review

- 不越界：不实现 Web 阅卷工作台、不实现双评比较、不生成 final_grade、不发布成绩。
- 不做假功能：不会声称已完成人工复核页面或最终成绩；只提供后端任务和人工评分记录。
- 与 STORY-017 衔接：证据校验失败可以作为 `source=evidence_verification_failed` 的人工复核任务来源。
- 与 STORY-019 衔接：双评要求可创建 review_task，但双评分差比较和 arbitration_task 留给下一 Story。
- 与当前仓库实际一致：沿用 Go API Gateway 的 `internal/<module>`、MemoryStore/PostgresStore、Handler、RBAC 和 audit 模式。

## 非范围

- Web 阅卷工作台。
- 双评分差比较。
- arbitration_task。
- final_grade。
- 成绩确认和发布。
- 学生端反馈展示。

## 验收标准

- 可创建人工复核任务。
- 支持任务来源枚举校验。
- 可按租户查询任务列表和任务详情。
- 可分配阅卷员。
- 阅卷员只能提交分配给自己的任务。
- 分数越界会失败。
- Rubric 选择不匹配会失败。
- 提交 human_grade 后记录人工评分并更新任务状态。
- 可退回重评并写明 reason。
- 接口受 `review:manage` 权限保护。
- 创建、分配、提交、退回写审计。
- 查询结果包含 anonymous_code，默认不暴露学生姓名。
- 测试通过。

## Implementation

- 新增 migration `000013_human_review_workflow.sql`：
  - 增加 `review:manage` 权限。
  - 为平台管理员、租户管理员、学校管理员、教师、阅卷员和审计员分配权限。
  - 新增 `review_task` 表。
  - 新增 `human_grade` 表。
- 新增 `internal/review`：
  - 类型定义：`ReviewTask`、`HumanGrade`、`RubricSelection`、输入类型和 Store 接口。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- API Gateway 挂载：
  - `POST /api/v1/review-tasks`
  - `GET /api/v1/review-tasks`
  - `GET /api/v1/review-tasks/{id}`
  - `POST /api/v1/review-tasks/batch-assign`
  - `POST /api/v1/review-tasks/{id}/assign`
  - `POST /api/v1/review-tasks/{id}/submit`
  - `POST /api/v1/review-tasks/{id}/return`
- 更新 `system/info` capabilities：
  - `human_review_task_management`
  - `human_grade_recording`
  - `review_assignment_workflow`
- 新增 `docs/api/review.md`，更新根 README 和 API Gateway README。

## Implementation Review

逐项检查结果：

- 创建人工复核任务：`TestReviewTaskWorkflowCreatesAssignsSubmitsAndReturns` 和路由测试覆盖。
- 来源枚举校验：`TestCreateTaskRejectsInvalidSource` 覆盖。
- 查询列表和详情：`TestReviewTaskRoutesCreateAssignSubmitAndAudit` 覆盖。
- 单任务分配：同一路由测试覆盖。
- 批量分配：`TestBatchAssignTasks` 和 `TestReviewTaskBatchAssignRoute` 覆盖。
- 阅卷员只能提交自己的任务：`TestReviewerCanOnlySubmitAssignedTask` 覆盖。
- 分数越界失败：`TestSubmitGradeValidatesScoreAndRubricSelections` 覆盖。
- Rubric 选择不匹配失败：同一测试覆盖。
- 提交 human_grade 并更新任务状态：store 和 route 测试覆盖。
- 退回重评：store 和 route 测试覆盖 `return_reason`。
- 权限保护：`TestReviewTaskRoutesRequirePermission` 覆盖 401/403。
- 审计：路由测试覆盖创建、分配、批量分配、提交、退回。
- 匿名码：路由测试断言返回 `anonymous_code` 且不包含 `student_name`。
- 不越界：未实现 Web 阅卷工作台、双评仲裁、final_grade、成绩发布。

实现审阅中关注点：

- Postgres 批量分配使用事务，避免部分任务成功、部分任务失败。
- `human_grade.private_note` 已保存，但只作为后端字段返回；更细粒度的可见性控制留给 Web/API 查询分层 Story。
- 当前 `review:manage` 是统一权限，后续可拆为 `review:assign`、`review:grade`、`review:audit`。

## Fixes

- 补充批量分配能力：新增 `BatchAssignInput`、Store 方法、MemoryStore/PostgresStore 实现和 `POST /api/v1/review-tasks/batch-assign`。
- 补充列表、详情、退回路由测试，确保验收标准直接覆盖。
- 补充 `system/info` capabilities 测试与实现。

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
system/info -> capabilities include human_review_task_management, human_grade_recording, review_assignment_workflow
POST /api/v1/review-tasks without review:manage -> 403 covered by TestReviewTaskRoutesRequirePermission
GET /api/v1/review-tasks without token -> 401 covered by TestReviewTaskRoutesRequirePermission
POST /api/v1/review-tasks/{id}/submit by unassigned reviewer -> ErrForbidden covered by TestReviewerCanOnlySubmitAssignedTask
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-019 双评与仲裁模块`。
