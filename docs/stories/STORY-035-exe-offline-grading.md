# STORY-035 EXE 离线阅卷基础功能

## Plan

### 目标

在 Windows EXE 客户端中实现离线阅卷基础功能。教师登录后获取分配给自己的 `review_task`，可下载任务包，查看 answer_segment、答案图片、OCR 文本、Rubric、AI 建议分，在本地阅卷工作台输入分数、Rubric 得分和评语，使用本地离线密钥加密保存草稿，并在联网后同步提交。同步必须可重试，避免重复提交造成脏数据，冲突结果必须在 UI 中可见。

### 视觉与交互模型

- visual thesis：离线阅卷工作台以答题证据和评分面板为核心，状态条持续提示本地密钥、缓存、联网和同步风险。
- content plan：我的任务；任务包下载；答案/证据；Rubric 与 AI 建议；草稿编辑；加密保存；同步队列；冲突提示；缓存清理。
- interaction thesis：下载包前先确认本地密钥；草稿保存只写加密内容；同步前重新读取服务端任务做冲突检测；同步成功后锁定本地草稿。

### 范围

- 扩展桌面端离线阅卷工作区：
  - 获取当前登录教师 `assigned_to=current_user.id` 的真实 review_task。
  - 下载任务包：
    - `GET /api/v1/review-tasks/{id}`
    - `GET /api/v1/submissions/{id}/answer-segments`
    - `GET /api/v1/submissions/{id}/pages`
    - `GET /api/v1/submissions/{id}/ocr-tasks` + `GET /api/v1/ocr-tasks/{id}`
    - `GET /api/v1/exams/{examId}/questions`
    - `GET /api/v1/answer-segments/{id}/ai-grades`
    - 当前会话按需下载答案图片 blob 预览，不持久化图片二进制。
  - 任务包内容展示：
    - review_task。
    - answer_segment。
    - 答案图片预览或 file_asset 状态。
    - OCR 文本。
    - Rubric。
    - AI 建议分。
  - 本地阅卷工作台：
    - 输入最终分。
    - 选择/填写 Rubric point 得分。
    - 写教师评语、学生反馈、私密备注、提交原因。
  - 离线保存草稿：
    - 要求用户输入本地离线密钥。
    - 使用 Web Crypto AES-GCM + PBKDF2 对任务包摘要和草稿 JSON 加密后写入 localStorage。
    - 未输入密钥时禁止保存草稿。
  - 联网后同步提交：
    - 提交前重新 `GET /api/v1/review-tasks/{id}` 做冲突检测。
    - 调用真实 `POST /api/v1/review-tasks/{id}/submit`。
    - 同步结果在 UI 中显示。
    - 失败可重试。
  - 冲突检测：
    - 服务端任务已提交/完成。
    - Rubric 版本变化。
    - 任务被撤回/退回/不再可提交。
  - 本地缓存过期清理：
    - 默认清理 7 天前的离线包/草稿。
  - 本地日志记录下载包、保存草稿、同步、冲突和清理动作。

### 非范围

- 不新增后端离线包专用 API；本 Story 聚合现有真实 API。
- 不持久化答案图片二进制；图片只在当前会话作为 blob URL 预览。
- 不实现 Stronghold、SQLite、Windows DPAPI 或系统密钥链；当前用 Web Crypto 加密 localStorage 内容，并继续标注企业安全存储待接入。
- 不实现复杂冲突合并 UI；冲突时提示用户刷新任务包或放弃本地草稿。
- 不实现批量离线提交、快捷键、完整图片批注或全量 Web 阅卷工作台能力。

### 修改文件

预计新增：

- `apps/desktop-client/src/lib/offlineStore.ts`
- `docs/stories/STORY-035-approval.md`

预计修改：

- `apps/desktop-client/src/App.tsx`
- `apps/desktop-client/src/api/files.ts`
- `apps/desktop-client/src/api/review.ts`
- `apps/desktop-client/src/api/submissions.ts`
- `apps/desktop-client/src/api/papers.ts`
- `apps/desktop-client/src/types.ts`
- `apps/desktop-client/src/styles.css`
- `apps/desktop-client/README.md`
- `README.md`
- `docs/stories/README.md`
- `docs/stories/STORY-035-exe-offline-grading.md`

### 验收标准

- 离线阅卷页可获取当前教师分配任务，调用真实 `GET /api/v1/review-tasks?assigned_to=current_user.id`。
- 未登录时不读取任务、不展示假任务。
- 任务包下载聚合真实 answer_segment、submission pages、OCR、Rubric、AI 建议分 API。
- 缺失的任务包部分以 warning 显示，不补假数据。
- 当前会话可预览答案图片；图片下载失败显示真实错误。
- 可输入分数、Rubric point 得分、教师评语、学生反馈、私密备注和提交原因。
- 未配置本地离线密钥时禁止保存草稿。
- 草稿/任务包摘要以 Web Crypto 加密后保存到 localStorage。
- 联网后同步提交调用真实 `POST /api/v1/review-tasks/{id}/submit`。
- 同步前检测任务已提交、Rubric 版本变化、任务撤回/不可提交，并在 UI 显示冲突。
- 同步成功后本地草稿标记为 synced，不允许重复提交。
- 同步失败可重试。
- 可清理过期本地缓存。
- 本地日志记录下载、保存、同步、冲突、清理。
- `npm.cmd run typecheck` 和 `npm.cmd run build` 通过。
- Playwright 检查离线阅卷页桌面/移动无明显重叠、无整页横向滚动、无控制台错误，关键文案与禁用状态符合验收。

## Plan Review

### 是否越界

未越界。计划只在 EXE 客户端聚合既有 review/submission/paper/grading API 实现离线阅卷基础功能，不新增后端 API、不实现专用离线包服务、不接入原生安全存储或复杂冲突合并。

### 是否遗漏显式需求

显式需求已覆盖：教师登录后获取自己的 review_task、下载任务包、任务包内容、阅卷工作台、分数/Rubric/评语、离线草稿、联网同步、冲突检测、冲突提示、缓存过期清理、安全存储抽象/加密、同步重试、防重复提交和同步结果可见。

### 是否符合当前仓库实际

基本符合。当前后端已有 review_task、answer_segment、submission page、OCR task、questions/Rubric、AI grades、human grade submit 等真实 API；Web 阅卷台已证明这些 API 可组合出阅卷上下文。当前桌面端没有 Stronghold/SQLite/DPAPI，因此本 Story 使用 Web Crypto 加密 localStorage 中的草稿和任务包摘要，同时把企业级安全存储继续标为待接入。

## Implementation

- 新增离线存储模块 `apps/desktop-client/src/lib/offlineStore.ts`：
  - 使用 Web Crypto PBKDF2 + AES-GCM 加密任务包摘要和草稿 JSON。
  - localStorage 仅保存加密 payload 与最小 envelope 元数据。
  - 支持读取 envelope、加载解密草稿、更新同步状态、清理过期缓存。
- 新增离线阅卷组件 `apps/desktop-client/src/components/OfflineWorkbench.tsx`：
  - 获取当前用户分配任务：`GET /api/v1/review-tasks?assigned_to=current_user.id`。
  - 下载任务包并聚合 answer_segment、submission pages、OCR tasks、questions/Rubric、AI grades。
  - 当前会话按需下载答案图片 blob 预览。
  - 展示 OCR、Rubric、AI 建议分和缺失 warning。
  - 支持分数、Rubric point 得分、教师评语、学生反馈、私密备注和提交原因。
  - 未输入 8 位以上本地离线密钥时禁用草稿保存。
  - 同步前检测服务端任务状态、任务分配人和 Rubric 版本。
  - 同步调用真实 `POST /api/v1/review-tasks/{id}/submit`。
  - 同步结果显示为 `draft/syncing/synced/failed/conflict`。
- 扩展桌面端 API：
  - `requestBlob`。
  - 文件下载。
  - review task detail、AI grades、human grade submit。
  - submission pages、answer segments、OCR tasks。
  - exam questions/Rubric。
- 扩展桌面端类型：
  - answer_segment、OCR、question、Rubric、AI grade、human grade、offline package、offline draft、sync status。
- 替换原离线阅卷占位页。
- 更新 README 和 Story 索引。

## Implementation Review

### 验收检查

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 获取当前教师 review_task | `OfflineWorkbench.loadMyTasks` 使用 `assigned_to=user.id` | 通过 |
| 未登录不读取任务/不展示假任务 | 按钮禁用与未登录逻辑 | 通过 |
| 下载任务包聚合真实 API | `buildTaskPackage` 调用 review/submission/OCR/questions/AI APIs | 通过 |
| 缺失数据 warning | `warnings` 汇总并显示 | 通过 |
| 当前会话答案图片预览 | `downloadFileBlob` + blob URL | 通过 |
| 分数/Rubric/评语输入 | `DraftEditor` | 通过 |
| 未配置离线密钥禁止保存 | 保存按钮/逻辑要求 8 位 key | 通过 |
| 草稿加密保存 | `offlineStore.ts` PBKDF2 + AES-GCM | 通过 |
| 联网同步真实提交 | `submitHumanGrade` | 通过 |
| 冲突检测 | 任务状态、分配人、Rubric 版本 | 通过 |
| 防重复提交 | synced 状态阻止再次同步 | 通过 |
| 同步失败可重试 | failed 状态可再次点击同步 | 通过 |
| 过期缓存清理 | `purgeExpiredOfflineDrafts` | 通过 |
| 本地日志 | 任务加载、包下载、草稿保存、同步、冲突、清理 | 通过 |
| typecheck/build | `npm.cmd run typecheck`、`npm.cmd run build` | 通过 |
| Playwright 布局与控制台 | 离线页桌面/移动检查 | 通过 |

### 发现的问题

- 当前环境没有真实后端 token，Playwright 只能验证未登录状态、关键 UI、禁用状态、布局和控制台；任务包下载/同步提交需在真实 session 下联调。
- 答案图片只在当前会话通过 blob URL 预览，不持久化图片二进制。
- Web Crypto 加密 localStorage 是基础安全措施，不等同 Stronghold/SQLite/DPAPI。

## Fixes

- 用独立 `OfflineWorkbench` 替换占位内容，避免把离线阅卷状态塞入扫描工作站逻辑。
- 修复 Web Crypto `Uint8Array`/`BufferSource` TypeScript 兼容问题，显式复制为 `ArrayBuffer`。
- 移动端将离线任务、草稿、工作台布局改为单列，Playwright 复测无整页横向滚动。

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
Desktop offline page 1280x720 -> scrollWidth 等于 viewport width，console 0 errors / 0 warnings
Mobile offline page 390x844 -> scrollWidth 等于 viewport width，console 0 errors / 0 warnings
Offline page text -> 离线阅卷基础工作台、本地离线密钥、Web Crypto、暂无本地草稿、尚未下载任务包、同步提交均存在
未下载任务包时 -> 加密保存草稿按钮禁用
Screenshots:
- output/playwright/desktop-client-offline-035-desktop.png
- output/playwright/desktop-client-offline-035-mobile.png
```

### 新增文件

- `apps/desktop-client/src/api/papers.ts`
- `apps/desktop-client/src/components/OfflineWorkbench.tsx`
- `apps/desktop-client/src/lib/offlineStore.ts`
- `docs/stories/STORY-035-approval.md`

### 修改文件

- `README.md`
- `apps/desktop-client/README.md`
- `apps/desktop-client/src/App.tsx`
- `apps/desktop-client/src/api/client.ts`
- `apps/desktop-client/src/api/files.ts`
- `apps/desktop-client/src/api/review.ts`
- `apps/desktop-client/src/api/submissions.ts`
- `apps/desktop-client/src/styles.css`
- `apps/desktop-client/src/types.ts`
- `docs/stories/README.md`
- `docs/stories/STORY-035-exe-offline-grading.md`

### 剩余风险

- 需要真实 token 和真实 review_task 数据做端到端联调。
- 当前没有专用离线包后端 API，客户端聚合多个现有 API。
- 图片二进制不做长期离线缓存。
- Web Crypto localStorage 加密不是最终企业安全存储方案；后续仍需 Stronghold/SQLite/DPAPI。
- 复杂冲突合并未实现，只提示冲突并阻止提交。
- Vite 构建仍有 large chunk warning。

### 下一步

进入 `STORY-036 UI 设计规范文档`。
