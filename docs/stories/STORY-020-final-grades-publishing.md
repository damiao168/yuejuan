# STORY-020 最终成绩与成绩发布

## 状态

Approved

## 目标

实现后端最终成绩与成绩发布基础闭环：在已有 `final_grade` 题目分基础上，为每个 `submission` 汇总总分，维护成绩状态机，支持发布前质量检查、学科组长确认、年级主任或管理员发布、学生查询已发布成绩、成绩发布后锁定核心分数，并支持 CSV 导出及审计。

本 Story 只实现后端 API、存储、状态流转、质量检查和 CSV 导出，不实现 Web 成绩管理页面、不实现学生端页面、不实现申诉改分、不实现 Excel 二进制格式导出。

## Plan

- 新增 migration：
  - `score:manage` 权限，用于确认、发布和导出成绩。
  - `student:grade:read` 权限，用于学生查询已发布成绩。
  - 扩展 `final_grade`：
    - 支持 `source=rule_auto`。
    - 增加 `status` 字段，状态包含 `calculating`、`pending_confirmation`、`confirmed`、`pending_publish`、`published`、`locked`。
    - 保留 `locked` 字段，发布后置为 `true`。
  - 新增 `submission_grade` 表，记录每份答卷总分、状态、确认/发布时间和锁定状态。
- 新增 `internal/score`：
  - 类型定义：题目分、答卷总分、质量检查、导出结果。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- `POST /api/v1/exams/{examId}/finalize`：
  - 生成缺失题目最终分：
    - 优先保留已有 `final_grade`。
    - 可从已提交的单评 `human_grade` 生成 `source=single_review`。
    - 可从 `auto_pass=true`、`needs_human_review=false`、`mock=false` 的规则型 `ai_grade` 生成 `source=rule_auto`。
  - 为每个 submission 汇总总分到 `submission_grade`。
  - 初始状态置为 `pending_confirmation`。
- `GET /api/v1/exams/{examId}/grades`：
  - 返回 submission 级总分和题目分。
  - 未发布阶段只允许 `score:manage`。
- `POST /api/v1/exams/{examId}/confirm-grades`：
  - 要求 `score:manage`。
  - 代表学科组长确认成绩。
  - 状态从 `pending_confirmation` 变为 `confirmed`，并进入 `pending_publish`。
- `POST /api/v1/exams/{examId}/publish`：
  - 要求 `score:manage`。
  - 发布前执行质量检查：
    - 未完成 review_task。
    - 未完成 arbitration_task。
    - OCR 失败未处理。
    - 异常分未确认。
    - 缺失 final_grade。
    - 未确认的 submission_grade。
  - 检查通过后置 `published` 并锁定题目分和总分。
- `GET /api/v1/students/{studentId}/exams/{examId}/grade`：
  - 要求 `student:grade:read` 或 `score:manage`。
  - 只返回已发布成绩。
- `GET /api/v1/exams/{examId}/grades/export`：
  - 要求 `score:manage`。
  - 输出 CSV。
  - 每行包含水印字段：`exported_by`、`exported_at`、`watermark`。
  - 写导出审计。
- 更新文档：
  - 新增 `docs/api/scores.md`。
  - 更新 README、API Gateway README 和 `system/info` capabilities。
- 测试：
  - finalize 生成 submission_grade。
  - finalize 从 single_review 和 rule_auto 生成缺失 final_grade。
  - 未完成阅卷任务不能发布。
  - 未完成仲裁任务不能发布。
  - OCR 失败不能发布。
  - 缺失题目分不能发布。
  - 未确认不能发布。
  - 确认后可发布并锁定。
  - 发布后学生可查询。
  - 导出 CSV 包含水印字段并写审计。
  - 权限不足返回 403。

## Plan Review

- 不越界：不实现 Web 成绩页面、不实现学生端 UI、不实现申诉改分、不实现 Excel 二进制格式、不实现发布后改分申请。
- 不做假功能：本 Story 的导出是 CSV，不声称已经实现 Excel；真实 Excel 留给后续导出增强 Story。
- 与 STORY-019 衔接：复用 `final_grade`，并在 STORY-020 中扩展状态和发布锁定。
- 与 STORY-021 衔接：发布后改分和申诉处理留给申诉 Story。
- 与当前仓库实际一致：沿用 Go API Gateway、MemoryStore/PostgresStore、RBAC、audit 和 migration 模式。
- 风险控制：发布质量检查必须查询 review、arbitration、OCR、answer_segment、final_grade 和 submission_grade，Postgres 发布操作需要事务锁定成绩。

## 非范围

- Web 成绩管理与发布页面。
- 学生端 UI。
- 申诉流程。
- 改分申请。
- Excel `.xlsx` 二进制导出。
- 成绩排名和统计分析。
- 水印 PDF/Excel。
- 发布通知。

## 验收标准

- `final_grade` 支持题目最终分和发布状态。
- 每个 answer_segment 可生成最终题目分。
- 每个 submission 可汇总总分。
- 支持成绩状态：`calculating`、`pending_confirmation`、`confirmed`、`pending_publish`、`published`、`locked`。
- 支持成绩确认。
- 支持成绩发布。
- 发布前检查未完成阅卷任务。
- 发布前检查未完成仲裁任务。
- 发布前检查 OCR 失败未处理。
- 发布前检查异常分未确认或缺失最终分。
- 阅卷未完成时不能发布。
- 发布接口要求权限。
- 发布后学生可查询已发布成绩。
- 发布后核心分数锁定。
- 支持 CSV 导出。
- 导出包含水印字段。
- 发布和导出写审计。
- 测试通过。

## Implementation

新增 migration `000015_final_grades_publishing.sql`：

- 新增 `student` 角色。
- 新增 `score:manage` 权限。
- 新增 `student:grade:read` 权限。
- 为管理员、学校管理员、教师、审计员分配 `score:manage`。
- 为学生角色分配 `student:grade:read`，并添加 demo 学生账号。
- 扩展 `final_grade`：
  - 支持 `source=rule_auto`。
  - 增加 `status` 字段。
  - 状态约束包含 `calculating`、`pending_confirmation`、`confirmed`、`pending_publish`、`published`、`locked`。
- 新增 `submission_grade`：
  - 汇总每份 submission 总分。
  - 记录状态、确认人、发布时间和锁定状态。

新增 `internal/score`：

- `types.go`：最终题目分、答卷总分、质量检查、导出结果和 Store 接口。
- `store_memory.go`：内存实现和测试种子方法。
- `store_postgres.go`：Postgres 事务实现。
- `handlers.go`：HTTP API 与审计。

新增接口：

- `POST /api/v1/exams/{examId}/finalize`
- `GET /api/v1/exams/{examId}/grades`
- `POST /api/v1/exams/{examId}/confirm-grades`
- `POST /api/v1/exams/{examId}/publish`
- `GET /api/v1/exams/{examId}/grades/export`
- `GET /api/v1/students/{studentId}/exams/{examId}/grade`

实现规则：

- `finalize` 保留已有 `final_grade`。
- `finalize` 可从已提交单评 `human_grade` 生成 `source=single_review`。
- `finalize` 可从 `auto_pass=true`、`needs_human_review=false`、`mock=false` 的规则型 `ai_grade` 生成 `source=rule_auto`。
- `confirm-grades` 将成绩置为 `confirmed`。
- `publish` 发布前执行质量检查，检查通过后 `submission_grade.status=published` 且 `locked=true`，`final_grade.status=locked` 且 `locked=true`。
- `student grade` 只返回已发布且锁定的成绩。
- `export` 输出 CSV，并包含水印字段。

更新文档：

- 新增 `docs/api/scores.md`。
- 更新根 README 和 API Gateway README。
- 更新 `system/info` capabilities。

## Implementation Review

逐项检查结果：

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `final_grade` 支持题目最终分和发布状态 | migration 扩展 `status` 和 `source=rule_auto` | 通过 |
| 每个 answer_segment 可生成最终题目分 | `FinalizeExam` 从 human/rule 来源生成缺失 final_grade，测试覆盖 | 通过 |
| 每个 submission 可汇总总分 | `submission_grade` 和 `TestFinalizeAggregatesSubmissionGradesFromHumanAndRuleGrades` | 通过 |
| 支持所有成绩状态 | migration 约束、`Statuses()`、测试断言 `calculating` 和 `locked` | 通过 |
| 支持成绩确认 | `ConfirmGrades`、路由测试 | 通过 |
| 支持成绩发布 | `PublishGrades`、路由测试 | 通过 |
| 发布前检查未完成阅卷任务 | `qualityLocked` / `qualityReportTx`，测试覆盖 | 通过 |
| 发布前检查未完成仲裁任务 | Store 测试覆盖 | 通过 |
| 发布前检查 OCR 失败未处理 | Store 测试覆盖 | 通过 |
| 发布前检查异常分未确认或缺失最终分 | Store 测试覆盖 | 通过 |
| 阅卷未完成不能发布 | `TestPublishBlocksUntilGradesConfirmedAndQualityPasses` | 通过 |
| 发布接口要求权限 | `TestScoreRoutesRequirePermission` | 通过 |
| 发布后学生可查询已发布成绩 | `GetStudentGrade` 和路由测试 | 通过 |
| 发布后核心分数锁定 | Store/route 测试断言 `locked=true` | 通过 |
| 支持 CSV 导出 | `ExportGradesCSV` 和路由测试 | 通过 |
| 导出包含水印字段 | Store/route 测试断言 `watermark`、`EduGrade export` | 通过 |
| 发布和导出写审计 | 路由测试断言 `score.published`、`score.exported` | 通过 |
| 测试通过 | `go test ./...` | 通过 |

实现审阅中关注点：

- `confirm-grades` 原本直接进入 `pending_publish`，审阅后修正为显式 `confirmed`，发布门禁接受 `confirmed` 或 `pending_publish`。
- 学生端查询是后端 API 能力；真实学生账号与学生档案绑定、学生端 UI 留给后续 Story。
- 当前导出为 CSV，不冒充 Excel `.xlsx`。
- 发布后不提供直接改分接口；后续改分必须走申诉/改分 Story。

## Fixes

- 补充 `student` 角色和 demo 学生账号迁移。
- 将确认后的成绩状态修正为 `confirmed`。
- 发布质量门禁调整为接受 `confirmed` 或 `pending_publish`。
- 补充 Store 测试和路由测试。
- 补充 `system/info` capabilities 测试。

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
POST /api/v1/exams/{examId}/finalize -> route test covered
POST /api/v1/exams/{examId}/publish before confirmation -> 409 quality gate covered
POST /api/v1/exams/{examId}/confirm-grades -> confirmed covered
POST /api/v1/exams/{examId}/publish -> published and locked covered
GET /api/v1/students/{studentId}/exams/{examId}/grade -> published grade covered
GET /api/v1/exams/{examId}/grades/export -> CSV watermark covered
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-021 申诉流程`。
