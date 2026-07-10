# STORY-021 自审审批记录

## Story

STORY-021 申诉流程

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 可创建 appeal | `CreateAppeal`、route test | 通过 |
| 申诉对象支持考试/题目/扣分点 | `TestCreateAppealRequiresPublishedGradeAndSupportsTargets` | 通过 |
| 未发布成绩不能申诉 | Store 校验 `published` + `locked` | 通过 |
| 学生只能申诉自己的成绩 | Handler `DataScope.student_id` 校验、route test | 通过 |
| 申诉包含 reason 和可选 attachment | migration、route test | 通过 |
| 支持全部申诉状态 | migration check、`Statuses()` | 通过 |
| 教师/仲裁员/管理员可处理申诉 | `appeal:manage` 路由与 migration | 通过 |
| 审计员只读申诉 | `000016_appeal_workflow.sql` | 通过 |
| 申诉详情包含评分证据和历史改分 | `withDetails`、`loadEvidence`、Store/route 测试 | 通过 |
| 改分生成 `score_adjustment` | Store/route 测试 | 通过 |
| 改分更新 `final_grade` 和 `submission_grade` | Store/Postgres 实现、Store 测试 | 通过 |
| 终态申诉不能被二次 review 覆盖 | `TestReviewAppealDoesNotReopenTerminalStatus` | 通过 |
| 学生可查看处理结果 | route test | 通过 |
| 管理员可查看申诉统计 | route test | 通过 |
| 创建、处理、改分、关闭写审计 | route test 断言 audit action | 通过 |
| 权限不足返回 403 | `TestAppealRoutesRequirePermission` | 通过 |
| API 文档更新 | `docs/api/appeals.md` | 通过 |
| README 状态更新 | 根 README、API Gateway README | 通过 |
| 不越界 | 未实现 Web 申诉中心、通知、多级审批、图片级答卷渲染 | 通过 |

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

- 学生账号与学生档案绑定仍依赖 `auth.User.DataScope.student_id`，完整绑定后台留给后续账号/学生端增强。
- 申诉证据只返回当前系统已保存的文本和结构化数据，不包含图片级原始答卷渲染。
- Web 申诉中心、通知、多级审批、申诉导出尚未实现。
- 真实 OCR/真实主观题模型/视觉证据核验仍在后续 Story 范围。

## 下一步

进入 `STORY-022 学情报告`。
