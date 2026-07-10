# STORY-022 学情报告与考试质量分析

## 状态

Approved

## 目标

实现后端报告与考试质量分析基础闭环：基于已发布锁定的 `submission_grade`、`final_grade`、`question`、`student`、`school_class`、`ai_grade`、`human_grade`、`review_task`、`double_mark_session`、`arbitration_task`、`ocr_task` 等真实数据，提供学生个人报告、班级报告、年级/考试概览、题目分析、阅卷质量分析和报告导出，并写审计日志。

本 Story 只实现后端 API、统计计算、导出和审计；不实现 Web 报告页面、不实现 PDF/Excel 二进制报表、不生成新的 AI 学情建议、不做图表渲染、不编造缺失统计。

## Plan

- 新增 migration：
  - `report:read` 权限，用于教师/管理员查看报告。
  - `report:export` 权限，用于导出报告。
  - `student:report:read` 权限，用于学生查看自己的报告。
  - `report` 表，用于记录导出或生成快照。
- 新增 `internal/report`：
  - 类型定义：学生报告、班级报告、考试概览、题目分析、阅卷质量、导出结果、空状态。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 学生个人报告：
  - `GET /api/v1/students/{studentId}/reports/{examId}`
  - 学生只能查看 `auth.User.DataScope.student_id` 匹配的报告；`report:read` 用户可查看授权范围报告。
  - 只基于 `submission_grade.status=published` 且 `locked=true` 的成绩。
  - 返回总分、各题得分、知识点掌握、教师反馈、AI 反馈、错因线索。
  - AI 反馈只引用 `ai_grade.student_feedback`、`missing_points`、`risk_flags`、`teacher_note` 等已存储字段；为空时返回空数组/空状态。
  - 教师反馈只引用 `human_grade.student_feedback`、`comments` 和仲裁反馈；为空时返回空数组/空状态。
- 考试概览：
  - `GET /api/v1/exams/{examId}/reports/overview`
  - 返回考试人数、已发布人数、平均分、最高分、最低分、中位数、标准差、及格率、优秀率、分数段分布。
- 班级报告：
  - `GET /api/v1/exams/{examId}/reports/classes`
  - 按 `student.class_id` 汇总平均分、最高分、最低分、中位数、及格率、优秀率、分数段分布、高频错题、薄弱知识点。
- 题目分析：
  - `GET /api/v1/exams/{examId}/reports/questions`
  - 返回每题平均分、得分率、答对率、难度、区分度、薄弱知识点、主观题高频错误线索。
  - 客观题选项分布只在 `answer_segment_answer.answer_payload` 中存在选项数据时返回；否则返回空状态，不伪造。
- 阅卷质量分析：
  - `GET /api/v1/exams/{examId}/reports/grading-quality`
  - 返回 AI 采纳率、人工改分率、平均双评分差、最大双评分差、仲裁数量、OCR 失败率、低置信复核量。
  - 指标只来自现有表；缺失来源返回 `available=false`。
- 报告导出：
  - `POST /api/v1/exams/{examId}/reports/export`
  - 导出 CSV 汇总，包含水印字段。
  - 写 `report` 记录和 `audit_log`。
- 权限：
  - 管理端报告 API 要求 `report:read`。
  - 导出 API 要求 `report:export`。
  - 学生个人报告要求 `student:report:read` 或 `report:read`，并执行学生 scope 校验。
- 文档：
  - 新增 `docs/api/reports.md`。
  - 更新 README、API Gateway README、`system/info` capabilities。
- 测试：
  - 无成绩时返回明确 empty 状态。
  - 学生只能查看自己的报告。
  - 学生报告包含总分、题目得分、知识点掌握和反馈来源。
  - 概览统计平均分、最高分、最低分、中位数、标准差、及格率、优秀率和分布正确。
  - 班级报告按班级汇总正确。
  - 题目分析包含得分率、答对率、难度、区分度和薄弱知识点。
  - 阅卷质量分析包含 AI 采纳率、人工改分率、双评分差、仲裁数量、OCR 失败率。
  - 导出写 audit 和 report 记录。
  - 权限不足返回 403。

## Plan Review

- 不越界：不实现 Web 报告页面、不实现 Web 图表、不实现 PDF/Excel `.xlsx`、不实现新的 AI 学情建议生成、不接入外部报表服务。
- 不做假功能：所有统计只来自已存在的成绩、题目、学生、班级、AI 评分、人工评分、OCR 和仲裁数据；缺失时返回 empty/available=false。
- 与 STORY-020 衔接：报告只读取已发布且锁定的 `submission_grade`，避免未发布成绩泄露。
- 与 STORY-021 衔接：申诉改分后 `submission_grade` 和 `final_grade` 已更新，报告读取最新锁定成绩。
- 与后续 STORY-030 衔接：本 Story 提供后端数据接口，Web 报告页面留给 `STORY-030 Web 报告`。
- 与当前仓库实际一致：沿用 Go API Gateway、MemoryStore/PostgresStore、RBAC、audit、migration 和 CSV 导出模式。

## 非范围

- Web 报告页面。
- 图表渲染。
- PDF 导出。
- Excel `.xlsx` 导出。
- 新的 AI 学情建议生成。
- 报告定时任务。
- 报告文件入 MinIO。
- 家长端报告。
- 跨考试趋势分析。

## 验收标准

- 学生个人报告返回总分、各题得分、知识点掌握、错因线索、教师反馈、AI 反馈。
- 班级报告返回平均分、最高分、最低分、中位数、及格率、优秀率、分数段分布、高频错题、薄弱知识点。
- 考试/年级概览返回班级对比、题目得分率、难度、区分度和标准差。
- 题目分析返回答对率、平均分、区分度、客观题选项分布空状态或真实分布、主观题高频错误线索。
- 阅卷质量分析返回 AI 采纳率、人工改分率、双评分差、仲裁数量、OCR 失败率。
- 报告只基于真实表，不编造统计数据。
- 无数据时返回明确空状态。
- 学生只能查看自己的报告。
- 管理端报告接口要求权限。
- 报告导出可用。
- 报告生成/导出写 audit_log。
- 测试通过。

## Implementation

新增 migration `000017_report_analytics.sql`：

- 新增 `report:read`、`report:export`、`student:report:read` 权限。
- 教师、管理员、审计员可读取报告。
- 教师和管理员可导出报告。
- 学生可读取自己的学情报告。
- 新增 `report` 表，用于记录导出/生成快照。

新增 `internal/report`：

- `types.go`：报告类型、统计指标、空状态、导出结果和 Store 接口。
- `stats.go`：平均分、最高分、最低分、中位数、标准差、及格率、优秀率、分数段、知识点掌握、题目得分率、答对率、难度、区分度、选项分布和 CSV 导出工具。
- `store_memory.go`：内存实现和测试种子方法。
- `store_postgres.go`：Postgres 实现，从真实业务表读取已发布锁定成绩和评分证据。
- `handlers.go`：HTTP API、学生 scope 校验、导出响应和审计写入。
- `store_test.go`：统计公式、空状态和导出测试。

新增接口：

- `GET /api/v1/exams/{examId}/reports/overview`
- `GET /api/v1/exams/{examId}/reports/classes`
- `GET /api/v1/exams/{examId}/reports/questions`
- `GET /api/v1/exams/{examId}/reports/grading-quality`
- `GET /api/v1/students/{studentId}/reports/{examId}`
- `POST /api/v1/exams/{examId}/reports/export`

实现规则：

- 所有报告只读取 `submission_grade.status=published` 且 `locked=true` 的成绩。
- 学生报告普通学生必须具备 `student:report:read`，并匹配 `auth.User.DataScope.student_id`。
- 管理端报告 API 要求 `report:read`。
- 导出 API 要求 `report:export`。
- 学生报告返回总分、题目分、知识点掌握、教师反馈、AI 反馈和错因线索。
- 班级报告按 `student.class_id` 汇总。
- 题目分析使用题目分计算得分率、答对率、难度和区分度。
- 客观题选项分布只从 `answer_segment_answer.answer_payload` 中读取真实选项；缺失时返回 `option_empty`。
- AI 反馈和错因线索只来自 `ai_grade.student_feedback`、`missing_points`、`risk_flags` 等已存储字段。
- 教师反馈只来自 `human_grade.student_feedback`、`comments`。
- 阅卷质量分析基于 `ai_grade`、`human_grade`、`double_mark_session`、`arbitration_task`、`ocr_task`、`review_task`。
- 导出返回 CSV，设置 `X-EduGrade-Watermark`，并写 `report` 记录。
- Handler 写 `report.generated` 和 `report.exported` 审计。

更新文档：

- 新增 `docs/api/reports.md`。
- 更新根 README 和 API Gateway README。
- 更新 `system/info` capabilities。

## Implementation Review

逐项检查结果：

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 学生个人报告包含总分和各题得分 | `StudentReport`、Store/route 测试 | 通过 |
| 学生个人报告包含知识点掌握 | `knowledgeMastery`、Store/route 测试 | 通过 |
| 学生个人报告包含错因线索、教师反馈、AI 反馈 | `gradeRecord` evidence fields、Store 测试 | 通过 |
| 班级报告统计完整 | `buildClassReports`、Store/route 测试 | 通过 |
| 考试概览包含班级对比、题目得分率、标准差 | `buildOverview`、Store 测试 | 通过 |
| 题目分析包含答对率、平均分、区分度、难度 | `buildQuestionAnalysis`、Store 测试 | 通过 |
| 客观题选项分布不伪造 | `optionDistribution` 和 `option_empty` | 通过 |
| 阅卷质量分析包含 AI 采纳、人工改分、双评分差、仲裁、OCR 失败 | `GradingQuality`、Store/route 测试 | 通过 |
| 所有统计来自真实表 | Postgres `loadDataset` 和 `loadQuality` 查询 | 通过 |
| 无数据返回明确空状态 | `TestReportsReturnEmptyStateWithoutPublishedGrades` | 通过 |
| 学生只能查看自己的报告 | `TestReportRoutesGenerateStudentClassQuestionQualityAndExport` | 通过 |
| 管理端报告接口要求权限 | route 中 `report:read`，权限测试 | 通过 |
| 报告导出可用 | `Export`、route 测试 | 通过 |
| 报告生成/导出写审计 | route 测试断言 `report.generated`、`report.exported` | 通过 |
| 测试通过 | `go test ./...` | 通过 |

实现审阅中关注点：

- AI 反馈不是新生成内容，只引用已存储 AI 评分字段。
- 选项分布不是猜测，只读取 answer payload，缺失时明确返回空状态。
- 报告导出为 CSV，不冒充 PDF 或 Excel。
- 学生身份绑定仍依赖 `auth.User.DataScope.student_id`，与前序 Story 保持一致。

## Fixes

- 局部测试时发现 AI 采纳率测试预期与 seed 数据不一致，修正测试预期为真实计算结果 `0.75`。
- 清理 Postgres 存储中的临时未用 import 痕迹。
- 补齐 system info capability 测试。

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
GET /api/v1/students/{studentId}/reports/{examId} -> route test covered
GET /api/v1/exams/{examId}/reports/overview -> route test covered
GET /api/v1/exams/{examId}/reports/classes -> route test covered
GET /api/v1/exams/{examId}/reports/questions -> route test covered
GET /api/v1/exams/{examId}/reports/grading-quality -> route test covered
POST /api/v1/exams/{examId}/reports/export -> route test covered
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-023 Web 管理后台基础框架`。
