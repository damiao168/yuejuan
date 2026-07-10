# STORY-031 自审审批记录

## Story

STORY-031 Web 申诉中心

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/appeals` 真实 API 页面 | `AppealCenterPage.tsx`、`routes.tsx mock=false` | 通过 |
| 缺 token 明确错误，不回退 mock | Playwright `/appeals` shell login 后清 token | 通过 |
| 申诉列表字段 | 学生、班级、考试、题号、原因、状态、提交时间 | 通过 |
| 申诉详情字段 | 申诉内容、原始答卷、OCR、AI/人工评分、最终分、Rubric、历史改分 | 通过 |
| 处理申诉 | 真实 `POST /appeals/{id}/review` | 通过 |
| 改分二次确认 | `score_adjusted` 前 `modal.confirm` | 通过 |
| 改分审计 | 后端 `appeal.reviewed` 与 `appeal.score_adjusted`，前端读取 audit_log | 通过 |
| 关闭申诉 | 真实 `POST /appeals/{id}/close` | 通过 |
| 学生可见结果 | status、result_reason、reviewed_at/closed_at | 通过 |
| 学生作用域 | 后端 data_scope 强制，前端权限面板说明 | 通过 |
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
/#/appeals after shell login, no edugrade.access_token -> passed
No-token state -> explicit error, empty list, actions disabled, no mock appeal rows
Desktop 1280 width -> scrollWidth equals viewport width
Mobile 390x844 -> scrollWidth equals viewport width
Console -> 0 errors, 0 warnings
Screenshots:
- output/playwright/appeals-desktop.png
- output/playwright/appeals-mobile.png
```

## 剩余风险

- 本 Story 不实现学生端申诉提交/查看页面。
- 本 Story 不预览附件文件，只展示 attachment 元数据。
- 当前 appeal evidence 不包含图片级原卷渲染，页面不伪造图片。
- Vite 构建仍有既有大 chunk warning。

## 下一步

进入 `STORY-032 Web 审计日志页面`。
