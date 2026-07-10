# STORY-034 EXE 扫描工作站功能

## Plan

### 目标

在 Windows EXE 客户端中实现扫描工作站功能。基于 `STORY-033` 已建立的 Tauri + React 客户端骨架，面向教务处批量采集答卷的真实工作流，支持选择考试、批量选择 PDF/图片、上传前预览、本地质量检查、可恢复上传队列、断网检测、联网后继续上传、失败重试、上传进度和服务端处理状态展示。所有上传必须走后端鉴权 API，不直接访问对象存储，不实现假扫描仪。

### 视觉与交互模型

- visual thesis：高密度、低干扰的教务批量上传工作台，核心是“考试上下文、文件清单、队列状态、服务端回执”四件事。
- content plan：考试选择；submission/page 上下文；批量文件预览；本地质量检查；上传队列；网络状态；失败重试；服务端文件与 submission 处理状态；本地日志。
- interaction thesis：文件进入队列前先做本地检查；断网时只入队不上传；联网恢复后自动续传可上传项；失败项可单独重试，不批量伪造成功。

### 范围

- 扩展桌面端扫描上传工作区：
  - 选择真实考试：调用 `GET /api/v1/exams`。
  - 输入/确认 `submission_id`。
  - 设置起始页码，批量文件按顺序生成待关联页码。
  - 批量选择 PDF/图片文件。
  - 上传前预览文件名、类型、大小、页码、质量检查结果。
  - 图片文件读取真实分辨率；PDF 页数和复杂分辨率检测显示“未配置/待接入”。
- 本地质量检查：
  - 文件类型：仅允许 PDF、PNG、JPG/JPEG。
  - 文件大小：按后端默认上限 100MB 检查。
  - 页数：预留，显示“未配置/待接入”。
  - 分辨率：图片读取真实宽高；PDF/不可读取时明确预留。
- 上传队列：
  - 状态覆盖 `pending`、`uploading`、`succeeded`、`failed`。
  - 展示上传进度。
  - 失败项支持重试。
  - 队列元数据持久化到本地浏览器/Tauri WebView storage；刷新后可恢复队列记录。
  - 文件二进制/文件句柄在浏览器模式无法安全持久化，恢复后需重新选择同名文件才能继续，界面必须明确提示。
- 断网与恢复：
  - 监听 `navigator.onLine`、`online`、`offline`。
  - 离线时禁止实际上传，只保留 pending 队列。
  - 联网后自动继续上传当前会话仍持有文件对象的 pending/failed 项。
- 真实后端处理：
  - 文件上传走 `POST /api/v1/files`，携带 Bearer token。
  - 上传成功后显示服务端返回的 `file_asset.id`、hash、大小等。
  - 若提供 `submission_id` 和页码，则调用真实 `POST /api/v1/submissions/{id}/pages` 关联答卷页。
  - 批次完成后可调用真实 `POST /api/v1/submissions/{id}/quality-check` 展示服务端处理/质量门禁结果。
- 本地日志：
  - 记录选择文件、质量检查、断网/联网、上传成功、失败、重试、服务端质量门禁结果。
- 更新 README、Story 索引和审批记录。

### 非范围

- 不接入真实扫描仪驱动、TWAIN/WIA、扫描仪发现或原生扫描命令。
- 不直接访问 MinIO/S3 或任何对象存储。
- 不新增后端 API、数据库表、权限或审计逻辑。
- 不实现 PDF 页数解析、PDF 渲染预览、OCR、图像裁剪或自动版面切分。
- 不实现跨应用重启后的文件二进制自动续传；当前 Story 恢复本地队列元数据，文件内容需用户重新选择。
- 不实现断点续传协议；失败重试重新调用后端文件上传 API。

### 修改文件

预计新增：

- `apps/desktop-client/src/api/exams.ts`
- `apps/desktop-client/src/api/submissions.ts`
- `docs/stories/STORY-034-approval.md`

预计修改：

- `apps/desktop-client/src/App.tsx`
- `apps/desktop-client/src/api/files.ts`
- `apps/desktop-client/src/types.ts`
- `apps/desktop-client/src/styles.css`
- `apps/desktop-client/README.md`
- `README.md`
- `docs/stories/README.md`
- `docs/stories/STORY-034-exe-scan-workstation.md`

### 验收标准

- 扫描工作站可刷新并选择真实考试，调用 `GET /api/v1/exams`。
- 无 token 或无权限时考试选择、上传、队列继续均显示真实错误，不展示假考试。
- 支持批量选择 PDF/PNG/JPG/JPEG。
- 上传前预览显示文件名、类型、大小、页码、质量检查结果。
- 文件类型与大小检查真实执行；图片分辨率尽量读取真实宽高；PDF 页数/复杂分辨率明确显示“未配置/待接入”。
- 上传队列状态覆盖 `pending`、`uploading`、`succeeded`、`failed`。
- 上传进度真实来自 XHR progress。
- 失败项可重试。
- 离线状态下不发起上传；联网后自动继续当前会话可继续的 pending/failed 项。
- 本地队列元数据可刷新恢复；无法恢复的文件句柄必须提示重新选择，不冒充可自动续传。
- 上传调用真实 `POST /api/v1/files`，带 Bearer token，不直接访问对象存储。
- 上传成功后显示服务端 `file_asset` 状态。
- 提供 submission page 关联和 quality-check 入口，均调用真实 submission API；失败显示真实错误。
- 本地日志记录关键动作。
- 前端 `npm.cmd run typecheck` 和 `npm.cmd run build` 通过。
- Playwright 检查扫描工作站桌面/移动无明显重叠、无整页横向滚动、无控制台错误，且关键文案/按钮状态符合验收。

## Plan Review

### 是否越界

未越界。计划只增强 EXE 客户端扫描工作站页面与前端 API 封装，复用已有考试、文件和 submission 后端 API；不新增后端、不接入对象存储、不实现扫描仪驱动、PDF 解析、OCR 或断点续传协议。

### 是否遗漏显式需求

显式需求已覆盖：选择考试、批量选择 PDF/图片、上传前预览、本地质量检查占位、上传队列四种状态、上传进度、失败重试、断网检测、联网后继续上传、上传完成后服务端处理状态、本地日志、真实文件上传 API、后端鉴权、本地队列可恢复和适合教务处批量操作。

### 是否符合当前仓库实际

符合。`STORY-033` 已建立桌面客户端骨架和文件上传基础。后端已有 `GET /api/v1/exams`、`POST /api/v1/files`、`POST /api/v1/submissions/{id}/pages`、`POST /api/v1/submissions/{id}/quality-check`，可支撑本 Story 的真实 API 调用。当前桌面端没有持久文件句柄或 SQLite，因此本 Story 将恢复队列元数据，并明确标注跨刷新/重启后需要重新选择文件才能继续上传。

## Implementation

- 新增桌面端考试 API：
  - `apps/desktop-client/src/api/exams.ts`
  - 调用真实 `GET /api/v1/exams`。
- 新增桌面端 submission API：
  - `apps/desktop-client/src/api/submissions.ts`
  - 调用真实 `POST /api/v1/submissions/{id}/pages`。
  - 调用真实 `POST /api/v1/submissions/{id}/quality-check`。
- 扩展桌面端类型：
  - `Exam`。
  - `SubmissionPage`。
  - `SubmissionQualityResult`。
  - `ScanQualityCheck`。
  - 扩展 `SyncQueueItem`，支持 exam、submission、page、file_asset、serverStatus、qualityChecks、requiresReselect。
- 重构扫描工作站 UI：
  - 真实考试选择与刷新。
  - submission_id 和起始页码。
  - 批量选择 PDF/PNG/JPG/JPEG。
  - 上传前队列表格预览。
  - 本地质量检查标签。
  - 网络状态标签。
  - pending 上传、failed 重试、清空 succeeded。
  - 服务端质量门禁入口和结果展示。
- 实现本地质量检查：
  - 文件类型真实检查。
  - 文件大小按 100MB 上限真实检查。
  - 图片分辨率通过浏览器 Image 读取。
  - PDF 页数和 PDF/复杂分辨率显示“未配置/待接入”。
- 实现本地队列：
  - 队列状态覆盖 `pending`、`uploading`、`succeeded`、`failed`。
  - XHR upload progress 更新上传进度。
  - 队列元数据持久化到 `localStorage`。
  - 同上下文刷新后恢复队列记录，并提示“需重新选择文件”，不假装文件句柄仍可用。
- 实现断网/恢复：
  - 监听 `online` / `offline`。
  - 离线时不发起上传。
  - 联网后自动重试当前会话仍可上传的 pending/failed 项。
- 实现真实服务端处理状态：
  - 上传成功记录 `file_asset.id`。
  - 上传成功后尝试关联 submission page。
  - 页面展示 submission page 状态和服务端质量门禁结果。
- 更新 README 和 Story 索引。

## Implementation Review

### 验收检查

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 选择真实考试 | `api/exams.ts` + 扫描页刷新考试按钮 | 通过 |
| 无 token 不展示假考试/不上传 | 扫描页禁用刷新/上传并显示未登录提示 | 通过 |
| 批量选择 PDF/图片 | 文件 input `multiple` + accept `.pdf,.png,.jpg,.jpeg` | 通过 |
| 上传前预览 | `ScanQueueTable` 展示文件名、类型、大小、页码、预览/占位 | 通过 |
| 本地质量检查 | `inspectScanFile` 类型/大小/分辨率/PDF 页数预留 | 通过 |
| 队列状态覆盖 | `pending`、`uploading`、`succeeded`、`failed` | 通过 |
| 上传进度 | `uploadFileWithProgress` XHR progress | 通过 |
| 失败重试 | 单项重试 + failed 批量重试 | 通过 |
| 断网检测 | `navigator.onLine` + online/offline listeners | 通过 |
| 联网后继续上传 | online handler 调用 `uploadQueueItems("all")` | 通过 |
| 本地队列可恢复 | `localStorage` 队列元数据 + Playwright 同上下文 reload 检查 | 通过，有文件句柄边界 |
| 真实文件上传 API | `POST /api/v1/files`，使用 Bearer token | 通过 |
| 不直接访问对象存储 | 前端只调用 API Gateway | 通过 |
| 上传完成后服务端状态 | `file_asset.id`、submission page 状态、quality-check 结果 | 通过 |
| 本地日志记录 | 选择、上传、失败、联网/断网、质量门禁均写本地日志 | 通过 |
| typecheck/build | `npm.cmd run typecheck`、`npm.cmd run build` | 通过 |
| Playwright 可视检查 | 扫描页桌面/移动/队列/恢复检查 | 通过 |

### 发现的问题

- 浏览器/Tauri WebView 无法跨刷新保留 `File` 二进制句柄；只能恢复队列元数据。实现中已明确显示“需重新选择文件”，未冒充自动续传。
- 当前未连接真实后端 session，因此 Playwright 验证了无 token 状态、队列预览、质量检查、本地持久化和 UI；真实上传链路由代码路径与既有 API 封装保证，需在有后端 token 的环境做联调。
- Vite 构建仍有 large chunk warning。

## Fixes

- 将简单扫描上传替换为可恢复队列模型，避免刷新后丢失队列记录。
- 为恢复后的队列项增加 `requiresReselect` 提示，避免把浏览器无法恢复的文件句柄伪装成可继续上传。
- 在移动布局中让扫描上下文表单和操作区纵向排列，Playwright 复测无整页横向滚动。

## Approval

### 审批结论

Approved

### 运行命令与结果

```powershell
npm.cmd run typecheck
npm.cmd run build
```

```text
npm.cmd run typecheck -> passed，desktop-client 与 web-admin 均通过
npm.cmd run build -> passed，desktop-client 与 web-admin 均通过，存在 Vite large chunk warning
```

### 浏览器检查

```text
Dev server: http://127.0.0.1:5180/
Desktop scan page 1280x720 -> scrollWidth 等于 viewport width，console 0 errors / 0 warnings
Mobile scan page 390x844 -> scrollWidth 等于 viewport width，console 0 errors / 0 warnings
Scan page text -> 选择考试、上传前预览与本地质量检查、上传 pending、真实扫描仪未配置、服务端质量门禁均存在
Local quality check fixture -> scan-sample.png 入队后显示文件名、文件类型、文件大小、分辨率、pending 状态
Queue persistence -> localStorage 存在 edugrade.desktop.scan_queue；同上下文 reload 后恢复 scan-sample.png 并提示需重新选择文件
Screenshots:
- output/playwright/desktop-client-scan-034-desktop.png
- output/playwright/desktop-client-scan-034-mobile.png
- output/playwright/desktop-client-scan-034-queued.png
- output/playwright/desktop-client-scan-034-recovered.png
```

### 新增文件

- `apps/desktop-client/src/api/exams.ts`
- `apps/desktop-client/src/api/submissions.ts`
- `docs/stories/STORY-034-approval.md`

### 修改文件

- `README.md`
- `apps/desktop-client/README.md`
- `apps/desktop-client/src/App.tsx`
- `apps/desktop-client/src/styles.css`
- `apps/desktop-client/src/types.ts`
- `docs/stories/README.md`
- `docs/stories/STORY-034-exe-scan-workstation.md`

### 剩余风险

- 当前未在有真实 token 的运行环境中实际上传文件；代码路径已对接真实 API，后续扫描工作站联调需使用真实后端 session。
- 队列跨刷新只能恢复元数据，文件二进制/文件句柄需要用户重新选择；完整跨重启续传需后续接入 Tauri 文件路径授权、SQLite/安全存储或原生文件缓存。
- PDF 页数解析、PDF 缩略预览、复杂分辨率检测仍为未配置/待接入。
- 不支持断点续传协议；失败重试会重新调用后端上传。
- Vite 构建仍有 large chunk warning。

### 下一步

进入 `STORY-035 EXE 离线阅卷功能`。
