# STORY-032 自审审批记录

## Story

STORY-032 Web 审计日志页面

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/audit` 真实 API 页面 | `AuditLogPage.tsx`、`routes.tsx mock=false` | 通过 |
| 缺 token 明确错误，不回退 mock | Playwright `/audit` shell login 后清 token | 通过 |
| 普通教师不能查看全局审计 | 路由要求 `audit:read` | 通过 |
| 审计列表只读 | 无编辑/删除入口，后端无编辑/删除 API | 通过 |
| 筛选条件 | created_from/to、actor_id、action、target_type、target_id、exam_id、ip_address | 通过 |
| 详情字段 | actor、action、target、before/after、reason、ip、user_agent、created_at | 通过 |
| 敏感字段脱敏 | `maskSensitive` 递归脱敏 | 通过 |
| 租户隔离 | 后端按 `user.TenantID` 查询和导出 | 通过 |
| 导出权限 | `audit:export` 路由权限 + 前端按钮控制 | 通过 |
| 导出写审计 | `TestExportAuditsWritesAuditAndWatermark` | 通过 |
| 后端测试 | `go test ./...` | 通过 |
| 后端构建 | `go build ...cmd/api-gateway` | 通过 |
| 前端类型检查 | `npm.cmd run typecheck` | 通过 |
| 前端构建 | `npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright viewport checks + screenshots | 通过 |

## 运行命令与结果

```powershell
gofmt -w services\api-gateway\internal\auth\types.go services\api-gateway\internal\auth\handlers.go services\api-gateway\internal\auth\store_memory.go services\api-gateway\internal\auth\store_postgres.go services\api-gateway\internal\auth\handlers_test.go services\api-gateway\internal\server\server.go
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
/#/audit after shell login, no edugrade.access_token -> passed
No-token state -> explicit error, empty list, export disabled, no mock audit rows
Desktop 1280 width -> scrollWidth equals viewport width
Mobile 390x844 -> scrollWidth equals viewport width
Console -> 0 errors, 0 warnings
Screenshots:
- output/playwright/audit-desktop.png
- output/playwright/audit-mobile.png
```

## 剩余风险

- 本 Story 不实现审计日志不可篡改链、归档或跨租户监管。
- 本 Story 导出真实 CSV，不实现 PDF/XLSX。
- 既有历史事件未写入 before_value/after_value 时页面显示“未记录”。
- Vite 构建仍有既有大 chunk warning。

## 下一步

进入 `STORY-033 Windows EXE 客户端骨架`。
