# STORY-026 Web 答卷采集页面

## Plan

### 目标

实现 Web 管理后台的答卷采集页面，对接真实后端 API，支持按考试查看答卷采集状态、批量上传答卷文件、查看页面文件、重传某页、执行质量检查、触发 OCR、触发答案切分，并查看 OCR 任务与结果。

### 范围

- 新增 Web 答卷采集页面并接入 `/capture` 路由。
- 新增前端 submissions API 封装，复用真实文件上传、submission、OCR task、answer segment 接口。
- 新增通用 files API 封装，支持上传进度和带 Bearer token 的 blob 下载。
- 后端补充真实“重新上传某页”契约，不改变新增页面的重复页冲突语义。
- 答卷列表展示：
  - `anonymous_code` 需求以当前真实字段 `candidate_no` 显示，并明确标注“后端当前未返回 anonymous_code 独立字段”。
  - 学生姓名仅在用户具备组织数据权限且真实学生接口可映射 `student_id` 时显示；否则显示“无权限或未关联”。
  - 页数、OCR 状态、切分状态、阅卷状态、质量问题。
- 支持质量问题筛选：
  - 模糊
  - 缺页
  - 重复页
  - OCR 失败
  - 需要人工处理
- UI 使用高信息密度表格、清晰状态标签、批量上传进度和失败原因。

### 非范围

- 不实现真实 OCR 推理、图片预处理、模糊检测、自动缺页识别以外的图像质量算法。
- 不实现 PDF 自动拆页；本 Story 的批量上传按“一个文件创建一份 submission + 第 1 页关联文件”的真实后端能力处理，并在 UI 中明确说明。
- 不实现评分工作台和真实阅卷状态 API；阅卷状态显示为“待后续 API 支持”。
- 不实现 OCR 人工校正、answer segment 可视化框选和图片裁剪。
- 不把 mock/stub 数据显示成真实业务记录。

### 修改文件

预计新增：

- `apps/web-admin/src/api/files.ts`
- `apps/web-admin/src/api/submissions.ts`
- `apps/web-admin/src/pages/SubmissionCapturePage.tsx`
- `docs/stories/STORY-026-approval.md`

预计修改：

- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/api/client.ts`
- `apps/web-admin/src/api/papers.ts`
- `apps/web-admin/src/api/org.ts`
- `apps/web-admin/src/auth/session.ts`
- `apps/web-admin/src/router/routes.tsx`
- `apps/web-admin/src/styles.css`
- `services/api-gateway/internal/submission/types.go`
- `services/api-gateway/internal/submission/handlers.go`
- `services/api-gateway/internal/submission/store_memory.go`
- `services/api-gateway/internal/submission/store_postgres.go`
- `services/api-gateway/internal/submission/handlers_test.go`
- `services/api-gateway/internal/server/server.go`
- `docs/api/submissions.md`
- `docs/stories/README.md`
- `README.md`
- `apps/web-admin/README.md`

### 验收标准

- `/capture` 不再是 mock 占位页，路由标记为真实 API。
- 缺少 `edugrade.access_token` 时页面显示明确错误/禁用写操作，不回退 mock 数据。
- 可选择真实考试并读取真实 submission 列表。
- 可批量选择文件上传，逐文件展示真实上传/创建/关联进度与失败原因。
- 每条 submission 可查看真实页面文件列表，并通过鉴权 blob 下载预览/打开。
- 可通过真实接口重传指定页，质量门禁被重置。
- 可通过真实接口执行质量检查、推进 `ready_for_ocr`、创建 OCR 任务、生成 answer segments。
- 可查看 OCR 任务详情和已回写结果；没有结果时显示真实空状态。
- 质量筛选能基于后端 quality issue、OCR task failed/manual review、segment manual review 状态过滤。
- 后端新增重传接口有测试覆盖，重复新增页仍返回冲突。
- 后端测试、后端构建、前端 typecheck、前端 build 通过。
- Playwright 桌面和移动视口检查 `/capture` 无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只覆盖 Story 026 的答卷采集页面和支撑它必须存在的“重传某页”后端契约，不进入 Story 027 的阅卷工作台、AI 证据评分、人工复核或成绩发布。

### 是否遗漏显式需求

已覆盖 10 条功能要求与 5 条 UI 要求。对当前后端缺失的独立 `anonymous_code`、真实学生姓名字段、阅卷状态、真实 OCR 推理、PDF 自动拆页做了明确边界说明，避免用假数据冒充真实能力。

### 是否符合当前仓库实际

符合。仓库已有真实文件上传、submission、quality-check、OCR task 和 answer segment API；当前缺口是页面重传某页的服务端契约、前端 submissions API 封装、带鉴权 blob 下载能力和 `/capture` 页面本身。实现路径与已有考试管理、试卷管理页面的 React + Ant Design 模式一致。

## Implementation

- 后端新增 `PUT /api/v1/submissions/{id}/pages/{pageNo}`，用于替换已有答卷页文件。
- Memory/Postgres submission store 新增 `ReplacePage`，替换成功后保持页数不变，并把 submission 状态重置为 `pages_uploaded`、`quality_status=unchecked`。
- 后端测试新增“替换页重置质量门禁且不创建重复页”覆盖；`POST /pages` 仍保持重复页冲突语义。
- 前端新增通用 `api/files.ts`，提供真实 XHR 上传进度和带 Bearer token 的 blob 下载。
- 前端新增 `api/submissions.ts`，封装 submission、page、quality-check、status、OCR task、answer segment API。
- 新增 `SubmissionCapturePage.tsx` 并接入 `/capture` 真实 API 路由。
- 页面实现考试选择、质量筛选、批量上传队列、学生答卷高密表格、页面 Drawer、页文件重传、OCR 任务 Drawer、质量检查、标记 OCR、触发 OCR 和触发切分。
- 学生姓名只通过真实 `GET /api/v1/students` 映射；缺权限或未关联时不伪造姓名。
- `anonymous_code` 当前后端未返回独立字段，页面以 `candidate_no` 展示并明确提示。
- 阅卷状态没有 submission API 支撑，页面明确显示“待后续 API 支持”。

## Implementation Review

- `/capture` 已由 mock 占位改为真实 API 页面，路由 `mock=false`。
- 缺 token 时页面显示明确 warning/error，批量上传禁用，不显示 mock submission。
- 批量上传链路使用真实 `POST /files`、`POST /submissions`、`POST /pages`，并展示真实 XHR 上传进度与失败原因。
- 页文件查看使用带鉴权 blob 下载，不使用无 Bearer token 的新窗口直连。
- 重新上传某页使用新增真实后端接口，服务端测试验证质量门禁重置。
- OCR 触发只创建真实 OCR task，页面说明等待真实 worker 回写结果，不伪造 OCR 文本。
- 切分触发使用真实 answer segment API，不做图片裁剪或自动版面识别伪实现。
- 高密度表格、状态标签、上传进度、失败原因均已覆盖。
- 桌面与移动 Playwright 检查无整页横向滚动、无控制台 error/warning。

## Fixes

- 修复 `StatusTag` 子节点类型问题，把 `+N` 质量问题数量渲染为明确字符串，保证 TypeScript strict 模式通过。
- Playwright wrapper 在当前 Windows/bash 环境下受 CRLF shebang 影响无法直接执行，按技能 fallback 使用 `npx.cmd --package @playwright/cli playwright-cli` 完成同等浏览器验证。

## Approval

Approved。

本 Story 已按计划完成，并通过后端测试、后端构建、前端类型检查、前端构建和浏览器检查。剩余未实现能力均已明确标注为后续 Story 或真实 worker/模型接入范围，没有用 mock/stub 冒充真实业务能力。
