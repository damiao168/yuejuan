# STORY-025 自审审批记录

## Story

STORY-025 Web 试卷与 Rubric 配置页面

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 上传试卷文件 | `POST /api/v1/files` + `file_asset_id` 登记 | 通过 |
| 展示试卷文件列表 | `GET /api/v1/exams/{examId}/papers` | 通过 |
| 题目列表 | `GET /api/v1/exams/{examId}/questions` | 通过 |
| 新建/编辑题目 | `POST /questions`、`PATCH /questions/{id}` | 通过 |
| 答题区域 JSON、标准答案、知识点 | Question Form | 通过 |
| Rubric 可编辑表格 | Rubric points Table | 通过 |
| 分值不匹配明显提示 | Alert + submit disabled | 通过 |
| locked 不可编辑 | `selectedLocked` | 通过 |
| 修改后提示新版本 | save message | 通过 |
| 完整性检查 | `validatePaperConfig` | 通过 |
| 真实 API 对接 | `api/papers.ts` | 通过 |
| 不显示 mock 数据 | 页面不导入 `src/data.ts` | 通过 |
| 后端测试 | `go test ./...` | 通过 |
| 后端构建 | `go build ...cmd/api-gateway` | 通过 |
| 前端 typecheck/build | `npm.cmd run typecheck`、`npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright | 通过 |

## 运行命令与结果

```powershell
Push-Location .\services\api-gateway
gofmt -w internal\paper
go test ./...
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
npm.cmd run typecheck
npm.cmd run build
```

```text
gofmt -> passed
go test ./... -> passed
go build -> passed
npm run typecheck -> passed
npm run build -> passed
```

## 浏览器检查

```text
/#/papers after shell login -> passed
No token state -> explicit error, write actions disabled
Console -> 0 errors, 0 warnings
Mobile 390x844 -> no page horizontal scroll
```

## 剩余风险

- PDF 预览、可视化答题区域框选、自动解析试卷未实现。
- 真实登录/token 获取仍在后续 Story。
- Rubric 历史版本对比和复杂审批流未实现。
- 构建包体仍有 Vite chunk size warning。

## 下一步

进入 `STORY-026 Web 答卷采集页面`。
