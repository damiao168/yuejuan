# STORY-022 自审审批记录

## Story

STORY-022 学情报告与考试质量分析

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 学生个人报告返回总分、各题得分 | `StudentReport`、Store/route 测试 | 通过 |
| 学生个人报告返回知识点掌握 | `knowledgeMastery`、Store 测试 | 通过 |
| 学生个人报告返回错因线索、教师反馈、AI 反馈 | Store 测试 | 通过 |
| 班级报告返回平均分、最高分、最低分、中位数、及格率、优秀率、分数段 | `buildClassReports`、Store 测试 | 通过 |
| 班级报告返回高频错题、薄弱知识点 | `frequentWrongQuestions`、`weakKnowledgePoints` | 通过 |
| 考试概览返回班级对比、题目得分率、难度、区分度和标准差 | `buildOverview`、`buildQuestionAnalysis` | 通过 |
| 题目分析返回答对率、平均分、区分度 | Store 测试 | 通过 |
| 客观题选项分布只使用真实 answer payload | `optionDistribution`、`option_empty` | 通过 |
| 阅卷质量分析返回 AI 采纳率、人工改分率、双评分差、仲裁数量、OCR 失败率 | `GradingQuality`、Store/route 测试 | 通过 |
| 不编造统计数据 | Postgres 只读真实业务表，缺失时 empty/available=false | 通过 |
| 无数据返回明确空状态 | `TestReportsReturnEmptyStateWithoutPublishedGrades` | 通过 |
| 学生只能查看自己的报告 | route 测试 | 通过 |
| 管理端报告接口要求权限 | `report:read` route 和权限测试 | 通过 |
| 报告导出可用 | route 测试 | 通过 |
| 报告生成/导出写 audit_log | route 测试断言 audit action | 通过 |
| API 文档更新 | `docs/api/reports.md` | 通过 |
| README 状态更新 | 根 README、API Gateway README | 通过 |
| 不越界 | 未实现 Web 页面、PDF、Excel、新 AI 建议、图表渲染 | 通过 |

## 运行命令与结果

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

## 剩余风险

- Web 报告页面和图表留给 `STORY-030 Web 报告`。
- 当前导出为 CSV，不是 PDF 或 Excel `.xlsx`。
- AI 反馈只引用已有评分记录，不生成新的教学建议。
- 学生账号与学生档案绑定仍依赖 `auth.User.DataScope.student_id`。
- 跨考试趋势分析、家长端报告和报告文件入 MinIO 尚未实现。

## 下一步

进入 `STORY-023 Web 管理后台基础框架`。
