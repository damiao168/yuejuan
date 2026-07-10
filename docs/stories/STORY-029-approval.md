# STORY-029 自审审批记录

## Story

STORY-029 Web 成绩管理与发布页面

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/scores` 真实 API 页面 | `ScoreManagementPage.tsx`、`routes.tsx mock=false` | 通过 |
| 缺 token 明确错误，不回退 mock | Playwright `/scores` shell login 后清 token | 通过 |
| 成绩总览 | submissions、grades、quality report 派生 | 通过 |
| 成绩列表字段 | anonymous_code、学生姓名权限态、班级权限态、总分、各题分、状态、锁定 | 通过 |
| 发布前质量检查 | `GET /api/v1/exams/{examId}/grades/quality?stage=publish` | 通过 |
| 发布按钮门禁 | `can_publish` 控制启用/禁用 | 通过 |
| 确认成绩 | `POST /api/v1/exams/{examId}/confirm-grades` | 通过 |
| 发布成绩二次确认 | `modal.confirm` + `POST /publish` | 通过 |
| 导出审计与水印提示 | 导出前确认 + `X-EduGrade-Watermark` 展示 | 通过 |
| 发布后锁定提示 | `locked/status` 触发锁定提示 | 通过 |
| 审计记录 | 真实 `audit_log` 的 `score.*` 过滤展示 | 通过 |
| 后端测试 | `go test ./...` | 通过 |
| 后端构建 | `go build ...cmd/api-gateway` | 通过 |
| 前端类型检查 | `npm.cmd run typecheck` | 通过 |
| 前端构建 | `npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright viewport checks + screenshots | 通过 |

## 运行命令与结果

```powershell
npm.cmd run typecheck
npm.cmd run build
Push-Location .\services\api-gateway
go test ./...
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

```text
npm run typecheck -> passed
npm run build -> passed, with existing Vite large chunk warning
go test ./... -> passed
go build -> passed
```

## 浏览器检查

```text
/#/scores after shell login, no edugrade.access_token -> passed
No-token state -> explicit error, actions disabled, no mock grade rows
Desktop 1280 width -> scrollWidth equals viewport width
Mobile 390x844 -> scrollWidth equals viewport width
Console -> 0 errors, 0 warnings
Screenshots:
- output/playwright/scores-desktop.png
- output/playwright/scores-mobile.png
```

## 剩余风险

- 当前后端真实导出能力是 CSV，本 Story 不扩展为 XLSX。
- 学生姓名和班级不在 score API 直接返回；页面仅在具备 `org:manage` 时通过 org API 映射，否则脱敏显示。
- `/scores` 页面读取 submission 用于总览，因此路由要求 `submission:manage`。
- Vite 构建仍有既有大 chunk warning。

## 下一步

进入 `STORY-030 Web 学情报告页面`。
