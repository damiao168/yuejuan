# STORY-A18：不可变成绩发布版本

## 已实现

`internal/scorerelease` 将对外成绩与仍可变化的 `submission_grade` / `final_grade` 分离：

- 每次发布使用独立 `score_release`，版本在同一考试内单调递增；
- `score_release_item` 与 `score_release_question` 固化总分、题级分、评分来源和 SHA-256 快照；
- 已发布版本及其题级/学生级快照由数据库触发器禁止修改或删除；申诉、重评、回滚都必须创建后续版本；
- 回滚不是把旧版本改回“当前”，而是从旧的已发布版本复制出一个带 `source_release_id` 的新草稿，再重新通过当前发布门禁；
- 发布在考试 advisory lock 下重新计算门禁，并将 `gate_snapshot` 与发布版本一同冻结；
- 门禁覆盖未完成阅卷/仲裁、OCR 失败、身份/页数对账、未确认成绩、总分一致性、发布快照完整性；接入 A13 后还会阻断质量看板的 blocking 项、未完成回标，以及 R3 的 Gold/标定/Seed 证据缺口；
- 学生查询只能读取 `score_release_current` 指向的已发布版本，并且只返回总分、按可见策略允许的题分、教师公开反馈/公开 rubric 摘要和申诉窗口。DTO 结构不含内部 `final_grade_id`、来源 ID、阅卷教师、私密批注、质量事件、模型提示词或 AI 原始输出。

## 接线点

`services/api-gateway/internal/server/server.go` 的真实 PostgreSQL 组装处已创建：

```go
scoreReleaseStore := scorerelease.NewPostgresStore(postgresDB, qualityDashboardService)
scoreReleaseService := scorerelease.NewService(scoreReleaseStore)
scoreReleaseHandler := scorerelease.NewHandler(scoreReleaseService, authStore)
```

并在现有 `requireScoreManage`、`requireStudentGradeAccess` 与 `withScopedExam` 的附近挂载 `scorerelease.RegisterRoutes`。考试范围路由继续使用 `withScopedExam`；学生路由保留 `auth.ScopedStudentID` 校验，不以客户端的 `student_id` 为准。

管理端路由：

- `POST /api/v1/exams/{examId}/score-releases`
- `GET /api/v1/exams/{examId}/score-releases`
- `GET /api/v1/exams/{examId}/release-gate`
- `GET /api/v1/exams/{examId}/score-releases/current-published`
- `GET /api/v1/score-releases/{id}`
- `GET /api/v1/score-releases/{id}/diff?base={releaseId}`
- `POST /api/v1/score-releases/{id}:publish`
- `POST /api/v1/exams/{examId}/score-releases:rollback`

学生安全路由：

- `GET /api/v1/student/exams/{examId}/result`
- `GET /api/v1/student/exams/{examId}/questions/{questionId}`

旧 `POST /publish` 不与版本发布入口宣称同一语义；A19 重评和 A22 题目申诉只生成后续 release，而不覆盖已发布事实。

## 与申诉的边界

A18 固化了每个 release 的 `appeal_window`，供学生端明确当前是否可申诉及允许的原因代码。现有 `appeal` 表尚未有 `source_release_id`/`question_id` 的发布锚定字段；该 schema 和“申诉改分只能进入新 release”的改造由 A22 接续，不能在旧 `score_adjustment` 上直接覆盖已发布事实。

## 验证

`go test ./internal/scorerelease -count=1 -v` 覆盖：

1. 已发布版本不会因后续改分而变化，改正通过第二个版本和题级 diff 对外；
2. 草稿创建后出现质量阻断时，发布时会重新拒绝；
3. 回滚产生新的可审计版本，而不是修改旧版本；
4. 学生 DTO 不泄漏内部评分或模型字段，并遵循冻结的申诉窗口。

## 已知后续依赖

- A19 将 regrade job 的已核准题级事实作为 `source=regrade` 新版本的来源；
- A20 可把目前的发布门禁扩展为统一质量 action route，而不改变发布版本模型；
- A21/A22 将接入独立 Student Portal 和 release-anchored question appeal，而不复用管理端评分 DTO。
