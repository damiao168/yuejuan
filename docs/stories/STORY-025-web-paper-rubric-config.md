# STORY-025 Web 试卷与 Rubric 配置页面

## 状态

Approved

## 目标

实现 Web 管理后台的试卷与 Rubric 配置页面，对接真实后端 API，支持：

- 上传试卷文件。
- 展示试卷文件版本列表。
- 展示题目列表。
- 新建/编辑题目。
- 编辑标准答案、答题区域 JSON、知识点。
- Rubric 可编辑表格。
- Rubric 分值校验。
- 试卷完整性检查。
- 提交审批/锁定 Rubric。

## Plan

- 修正后端契约缝隙：
  - 当前文件上传 API 返回 `file.id`，不暴露 `storage_bucket/storage_key`。
  - 当前试卷登记 API 要求 `storage_bucket/storage_key`，前端无法真实串联上传结果。
  - 本 Story 不伪造对象存储 key；后端 `POST /api/v1/exams/{examId}/papers` 增加支持 `file_asset_id`。
  - 当传入 `file_asset_id` 时，后端从真实 `file_asset` 读取文件元数据和存储位置，创建 `exam_paper` 并把文件 owner 更新为 `exam_paper`。
  - 兼容原有 metadata 登记方式，避免破坏已有测试和文档。
- API 层：
  - 新增/完善 `apps/web-admin/src/api/papers.ts`。
  - 封装文件上传、试卷登记、试卷列表、题目列表、创建题目、更新题目、删除题目、创建 Rubric、配置校验。
- 路由：
  - `/papers` 从 mock 占位页切换为真实试卷与 Rubric 配置页面。
  - 试卷管理路由 `mock=false`。
- 页面结构：
  - 顶部选择考试，考试数据来自真实 `GET /api/v1/exams`。
  - 左侧题目列表，高信息密度显示题号、题型、分值、Rubric 状态。
  - 右侧题目配置表单，包含题号、题型、分值、知识点、答题区域 JSON、标准答案。
  - 右侧下方 Rubric 编辑表格，包含采分点、分值、是否必需、扣分点、等价答案、样例答案。
- 上传：
  - 使用 Ant Design Upload 自定义上传。
  - 调用 `POST /api/v1/files` 上传真实二进制。
  - 上传成功后调用 `POST /api/v1/exams/{examId}/papers` 携带 `file_asset_id` 登记试卷版本。
  - 展示 `GET /api/v1/exams/{examId}/papers` 返回的真实试卷版本。
- 题目：
  - 新建题目调用 `POST /api/v1/exams/{examId}/questions`。
  - 编辑题目调用 `PATCH /api/v1/questions/{id}`。
  - locked Rubric 状态下禁用题目和 Rubric 编辑。
  - 保存题目修改后提示“将生成/关联新的答案版本”，与后端 answer key v2 行为一致。
- Rubric：
  - 提交 Rubric 调用 `POST /api/v1/questions/{id}/rubric`。
  - 前端先校验 `points.score` 总和必须等于题目分值。
  - `max_score` 必须等于题目分值。
  - status 支持 `draft`、`pending_review`、`approved`、`locked`。
  - 分值不匹配时明显提示，不调用后端。
- 完整性检查：
  - 调用 `POST /api/v1/exams/{examId}/validate-paper-config`。
  - 展示 valid/issue 列表。
- 状态：
  - loading、error、empty 状态完整。
  - 无真实 token 时不请求后端、不显示 mock 数据、禁用写操作。
- 验证：
  - 后端 `go test ./...`。
  - 前端 `npm.cmd run typecheck`。
  - 前端 `npm.cmd run build`。
  - Playwright 检查 `/papers` 无 token 状态、桌面/移动布局和无整页横向滚动。

## Plan Review

- 不越界：
  - 不实现 PDF 在线预览、版面可视化拖拽框选、OCR、自动切题或真实图像裁剪。
  - 不实现复杂 Rubric 审批流；只使用后端支持的 status 提交。
  - 不实现历史 Rubric 版本对比，只展示最新题目返回的 Rubric。
- 不造假：
  - 不使用 mock 试卷、题目或 Rubric 数据。
  - 无 token 或后端不可用时显示错误状态。
  - 对后端暂不支持的 PDF 预览/视觉标注能力不做假控件。
- 后端修正合理性：
  - 文件上传 API 出于安全不暴露 storage key 是正确的。
  - 试卷登记接受 `file_asset_id` 是真实产品链路所需，不需要新增数据库表。
  - 兼容旧 metadata 请求，控制改动范围。
- 风险：
  - 当前真实登录仍不在本 Story，Web 页面依赖 `edugrade.access_token`。
  - 上传 API 要求真实 UUID owner/exam id；开发环境需真实后端数据。

## 非范围

- PDF 在线预览。
- 可视化框选答题区域。
- OCR/切分触发。
- Rubric 历史版本对比。
- 多人审批流。
- 自动生成题目。
- 自动解析试卷。
- E2E 测试文件。

## 验收标准

- `/papers` 使用真实 API Client，不显示 mock 试卷/题目/Rubric 数据。
- 上传试卷文件调用 `POST /api/v1/files`，然后用真实 `file_asset_id` 登记试卷版本。
- 能展示真实试卷文件列表。
- 能展示真实题目列表。
- 能新建/编辑题目字段：题号、题型、分值、知识点、答题区域 JSON、标准答案。
- Rubric 使用可编辑表格。
- Rubric 分值不匹配时有明显提示，并阻止提交。
- 能调用配置完整性检查 API 并展示问题。
- locked 状态不可编辑。
- 修改题目或答案后提示会产生新版本。
- 有 loading、error、empty 状态。
- 无 token 写操作禁用。
- 后端测试通过。
- 前端 typecheck/build 通过。
- 桌面/移动视口无明显布局重叠和整页横向滚动。

## Implementation

后端契约修正：

- `services/api-gateway/internal/paper/types.go`
  - `CreatePaperInput` 增加 `file_asset_id`。
- `services/api-gateway/internal/paper/handlers.go`
  - `POST /api/v1/exams/{examId}/papers` 支持 `file_asset_id` 模式。
  - 保留旧的 metadata 登记模式。
- `services/api-gateway/internal/paper/store_postgres.go`
  - `file_asset_id` 模式下从真实 `file_asset` 读取文件元数据和对象存储位置。
  - 创建 `exam_paper` 后把文件 owner 更新为 `exam_paper`。
  - 校验上传文件所属考试，不允许跨考试关联。
- `services/api-gateway/internal/paper/store_memory.go`
  - MemoryStore 支持 `file_asset_id`，用于路由测试。
- `services/api-gateway/internal/paper/handlers_test.go`
  - 增加 `TestCreatePaperFromUploadedFileAssetID`。
- `docs/api/paper-question.md`
  - 文档补充推荐链路：先上传文件，再用 `file_asset_id` 登记试卷。

前端实现：

- `apps/web-admin/src/api/client.ts`
  - `ApiClient` 支持 `FormData`，multipart 上传时不强制设置 JSON Content-Type。
- `apps/web-admin/src/api/papers.ts`
  - 封装上传文件、登记试卷、列试卷、列题目、创建题目、更新题目、删除题目、创建 Rubric、校验配置。
- `apps/web-admin/src/pages/PaperRubricPage.tsx`
  - `/papers` 真实 API 页面。
  - 考试选择来自真实 `GET /api/v1/exams`。
  - 上传调用 `POST /api/v1/files`，成功后用 `file_asset_id` 调用 `POST /api/v1/exams/{examId}/papers`。
  - 展示真实试卷版本列表。
  - 左侧真实题目列表。
  - 右侧题目表单：题号、题型、分值、知识点、答题区域 JSON、标准答案、等价答案、容差 JSON。
  - Rubric 可编辑表格：采分点、分值、必需、扣分点 JSON、样例答案 JSON。
  - 前端阻止 Rubric 分值不匹配提交。
  - locked 状态禁用题目和 Rubric 编辑。
  - 完整性检查调用真实后端校验 API。
  - 无 token 时不请求后端、不显示 mock 数据、禁用写操作。
- `apps/web-admin/src/App.tsx`
  - `/papers` 路由接入真实页面。
- `apps/web-admin/src/router/routes.tsx`
  - 试卷管理路由 `mock=false`，权限为 `exam:manage` + `file:manage`。
- `apps/web-admin/src/auth/session.ts`
  - 框架 mock session 增加 `file:manage`，仅用于前端壳层权限展示。
- `apps/web-admin/src/styles.css`
  - 增加试卷配置工作台、题目列表、Rubric 编辑器和移动端样式。

## Implementation Review

验收检查：

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/papers` 使用真实 API Client | `PaperRubricPage.tsx`、`api/papers.ts` | 通过 |
| 上传试卷文件真实链路 | `uploadFile` + `registerPaperFromFile` + 后端 `file_asset_id` | 通过 |
| 展示试卷文件列表 | `listPapers` + Table | 通过 |
| 展示题目列表 | `listQuestions` + 左侧 List | 通过 |
| 新建/编辑题目字段完整 | Question Form | 通过 |
| Rubric 可编辑表格 | Rubric points Table | 通过 |
| Rubric 分值不匹配明显提示 | `scoreMismatch` Alert + 提交禁用 | 通过 |
| 完整性检查 | `validatePaperConfig` | 通过 |
| locked 状态不可编辑 | `selectedLocked` + `questionDisabled` | 通过 |
| 修改后提示新版本 | 保存成功 message | 通过 |
| 不显示 mock 数据 | 页面不导入 `src/data.ts` | 通过 |
| 后端测试 | `go test ./...` | 通过 |
| 后端构建 | `go build ...cmd/api-gateway` | 通过 |
| 前端 typecheck/build | `npm.cmd run typecheck`、`npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright screenshot + scrollWidth check | 通过 |

审阅发现的问题：

- 初始实现中无 token 错误页未渲染 Form，Ant Design 发出 useForm 未连接警告。
- 上传链路如果继续让前端传 storage key，会违反文件 API 的安全边界。

## Fixes

- 在无编辑器可见时挂载 `component={false}` 的 Form，消除 AntD useForm 警告。
- 后端支持 `file_asset_id` 关联真实上传文件，不向前端暴露对象存储 key。
- 无 token 时禁用上传、完整性检查和写操作。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
gofmt -w internal\paper
go test ./...
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
npm.cmd run typecheck
npm.cmd run build
```

结果：

```text
gofmt -w internal\paper -> passed
go test ./... -> passed
go build api-gateway -> passed
npm run typecheck -> passed
npm run build -> passed
```

浏览器验收：

```text
Playwright /#/papers after shell login -> passed
No token state -> explicit error, upload/check/write actions disabled
No mock paper/question/rubric data rendered -> passed
Desktop console -> 0 errors, 0 warnings
Mobile 390x844 scrollWidth -> document 390px at 390px viewport
Mobile screenshot -> no obvious overlap
```

限制记录：

- 当前 Story 不实现 PDF 在线预览或可视化答题区域框选。
- 真实登录/token 获取仍不属于本 Story。
- Rubric 历史版本对比和多人审批流留给后续治理/工作流 Story。
- Vite 仍提示首包超过 500 kB，后续适合做路由级拆包。

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-026 Web 答卷采集页面`。
