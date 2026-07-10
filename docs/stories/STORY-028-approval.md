# STORY-028 自审审批记录

## Story

STORY-028 Web 双评仲裁页面

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/arbitration` 真实 API 页面 | `ArbitrationPage.tsx`、`routes.tsx mock=false` | 通过 |
| 缺 token 明确错误，不回退 mock | Playwright `/arbitration` no-token snapshot | 通过 |
| 仲裁任务列表字段 | Table columns: exam、question_no、anonymous_code、A/B score、difference、status | 通过 |
| 仲裁详情字段 | 答案/OCR、AI 建议、Rubric、双评分差、裁决表单 | 通过 |
| 提交仲裁结果 | `submitArbitration` 调用真实 submit API | 通过 |
| `final_grade` 展示 | 提交响应保存并展示 `finalGrade` | 通过 |
| 审计记录展示 | 新增 `GET /api/v1/audit-logs` 与前端 `listAuditLogs` | 通过 |
| 普通阅卷员不可访问 | 路由要求 `arbitration:manage` | 通过 |
| 我的任务过滤 | 默认 `assigned_to=current backend user id` | 通过 |
| 分数校验 | 前端 0 到题目满分校验 + 后端校验 | 通过 |
| 不伪造阅卷员批注 | 页面明确显示当前 API 未返回 comments | 通过 |
| 后端测试 | `go test ./...` | 通过 |
| 后端构建 | `go build ...cmd/api-gateway` | 通过 |
| 前端类型检查 | `npm.cmd run typecheck` | 通过 |
| 前端构建 | `npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright viewport checks + screenshots | 通过 |

## 运行命令与结果

```powershell
Push-Location .\services\api-gateway
gofmt -w internal\auth\types.go internal\auth\store_memory.go internal\auth\store_postgres.go internal\auth\handlers.go internal\auth\audit.go internal\auth\handlers_test.go internal\server\server.go
go test ./...
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
npm.cmd run typecheck
npm.cmd run build
```

```text
go test ./... -> passed
go build -> passed
npm run typecheck -> passed
npm run build -> passed
```

## 浏览器检查

```text
/#/arbitration after shell login, no token -> passed
No-token state -> explicit error, submit disabled, no mock arbitration tasks
Desktop 1280x900 -> scrollWidth equals viewport width
Mobile 390x844 -> scrollWidth equals viewport width
Console -> 0 errors, 0 warnings
Screenshots:
- output/playwright/arbitration-desktop.png
- output/playwright/arbitration-mobile.png
```

## 剩余风险

- 当前 `arbitration_task` API 不返回两名阅卷员 comments，页面不伪造批注。
- 后端 `GET /api/v1/arbitration-tasks/{id}` 仍按 tenant + `arbitration:manage` 查询，未强制 assigned-only 读取；提交已校验 assigned_to。
- 本 Story 只补最小审计查询接口和仲裁页审计显示，完整 `/audit` 审计中心仍在后续 Web Story。
- Vite 构建仍有既有大 chunk warning。

## 下一步

进入 `STORY-029 Web 成绩管理与发布页面`。
