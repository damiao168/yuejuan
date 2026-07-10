# STORY-027 自审审批记录

## Story

STORY-027 Web 阅卷工作台

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/grading` 真实 API 页面 | `GradingWorkbenchPage.tsx`、`routes.tsx mock=false` | 通过 |
| 缺 token 明确错误，不回退 mock | Playwright `/grading` no-token snapshot | 通过 |
| 获取 review_task | `api/review.ts listReviewTasks/getReviewTask` | 通过 |
| 展示 answer_segment | `listAnswerSegments` 按 `answer_segment_id` 匹配 | 通过 |
| 展示 OCR | `listOcrTasks` + `getOcrTask` results | 通过 |
| 展示 AI grade | `listAiGrades` + AI 证据面板 | 通过 |
| 展示 Rubric | `listQuestions(exam_id)` 读取 question/rubric | 通过 |
| 原始答卷查看器 | 鉴权 blob 下载 + 原图/OCR切换/缩放/旋转/拖拽 | 通过 |
| 提交 human_grade | `POST /api/v1/review-tasks/{id}/submit` | 通过 |
| 快捷键 | `1-9`、`A`、`R`、`Ctrl+Enter` | 通过 |
| 分数校验 | 前端 0 到满分校验 + 后端校验 | 通过 |
| 提交后自动下一份 | `goNext` | 通过 |
| 无权限不可提交 | `canManage && hasToken` 控制提交按钮 | 通过 |
| mock 明确标记 | `grade.mock` 显示 `MOCK AI` | 通过 |
| 不伪造同类答案/历史样例 | 底部明确“待后续 API 支持” | 通过 |
| 后端测试 | `go test ./...` | 通过 |
| 后端构建 | `go build ...cmd/api-gateway` | 通过 |
| 前端类型检查 | `npm.cmd run typecheck` | 通过 |
| 前端构建 | `npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright viewport checks | 通过 |

## 运行命令与结果

```powershell
Push-Location .\services\api-gateway
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
/#/grading after shell login, no token -> passed
No-token state -> explicit error, submit disabled, no mock review tasks
Desktop 1280x? -> scrollWidth equals viewport width
Mobile 390x844 -> scrollWidth equals viewport width
Console -> 0 errors, 0 warnings
```

## 剩余风险

- 主观题 AI 评分接口当前默认是 mock adapter，页面明确显示 `MOCK AI`；真实模型接入不属于本 Story。
- 同类答案检索和历史样例库没有当前 API，页面明确显示待后续支持。
- 当前没有后端聚合上下文接口，前端通过多个真实 API 拼装工作台；后续可在性能优化 Story 中做聚合端点。
- Vite 构建仍有既有大 chunk warning。

## 下一步

进入 `STORY-028 Web 双评仲裁页面`。
