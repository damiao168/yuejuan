# STORY-026 自审审批记录

## Story

STORY-026 Web 答卷采集页面

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/capture` 真实 API 页面 | `SubmissionCapturePage.tsx`、`routes.tsx mock=false` | 通过 |
| 缺 token 明确错误，不回退 mock | Playwright `/capture` no-token snapshot | 通过 |
| 按考试查看答卷 | `listExams` + `listSubmissions` | 通过 |
| 批量上传答卷文件 | `uploadFileWithProgress` + `createSubmission` + `addSubmissionPage` | 通过 |
| 学生答卷列表 | Ant Design small table columns | 通过 |
| anonymous_code 处理 | 以 `candidate_no` 展示并标注独立字段未返回 | 通过 |
| 学生姓名按权限显示 | `canReadStudentNames` + `listStudents` 映射 | 通过 |
| 页数、OCR、切分、阅卷、质量问题 | 表格字段与真实 API 派生状态 | 通过 |
| 查看答卷页面文件 | `downloadFileBlob` 带 Bearer token | 通过 |
| 重新上传某页 | `PUT /api/v1/submissions/{id}/pages/{pageNo}` | 通过 |
| 触发 OCR | `POST /api/v1/submissions/{id}/ocr-tasks` | 通过 |
| 触发切分 | `POST /api/v1/submissions/{id}/segment-answers` | 通过 |
| 查看 OCR 结果 | OCR Drawer + `GET /api/v1/ocr-tasks/{id}` | 通过 |
| 质量问题筛选 | `derivedIssues` + filter options | 通过 |
| 上传进度与失败原因 | XHR progress + upload queue | 通过 |
| 后端测试 | `go test ./...` | 通过 |
| 后端构建 | `go build ...cmd/api-gateway` | 通过 |
| 前端类型检查 | `npm.cmd run typecheck` | 通过 |
| 前端构建 | `npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright viewport checks | 通过 |

## 运行命令与结果

```powershell
Push-Location .\services\api-gateway
gofmt -w internal\submission internal\server
go test .\internal\submission
go test ./...
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
npm.cmd run typecheck
npm.cmd run build
```

```text
gofmt -> passed
go test ./internal/submission -> passed
go test ./... -> passed
go build -> passed
npm run typecheck -> passed
npm run build -> passed
```

## 浏览器检查

```text
/#/capture after shell login, no token -> passed
No-token state -> explicit error, write actions disabled, no mock submissions
Desktop 1280x? -> scrollWidth equals viewport width
Mobile 390x844 -> scrollWidth equals viewport width
Console -> 0 errors, 0 warnings
```

## 剩余风险

- 后端 submission 当前没有独立 `anonymous_code` 字段，页面以 `candidate_no` 承接并明确标注。
- 真实 OCR worker、真实 OCR 结果回写、图像模糊检测、自动 PDF 拆页和图片裁剪不属于本 Story。
- 阅卷状态尚无 submission API 支撑，页面显示“待后续 API 支持”。
- Vite 构建仍有既有的大 chunk warning。

## 下一步

进入 `STORY-027 Web 阅卷工作台`。
