# STORY-033 Windows EXE 客户端骨架

## Plan

### 目标

创建 EduGrade Enterprise 的 Windows EXE 客户端骨架，采用 Tauri 2 + React + TypeScript。客户端定位为扫描工作站、教师离线阅卷端、本地任务缓存与同步端。本 Story 只建立可运行、可构建、边界清晰的客户端基础，不实现假扫描仪、假离线阅卷或假自动更新。

### 视觉与交互模型

- visual thesis：克制、密集、面向一线工作站的桌面操作台，突出连接状态、任务状态和本地能力边界。
- content plan：登录与服务端配置；设备绑定状态；任务列表；文件选择上传；离线阅卷占位；同步队列；系统诊断；本地日志；自动更新预留。
- interaction thesis：服务端配置和登录走真实 API；切换工作区保持桌面端稳定布局；未实现能力统一显示“未配置/待接入”，不让按钮文案承诺不存在的能力。

### 范围

- 在 `apps/desktop-client` 建立 Tauri 2 + React + TypeScript 工程骨架。
- 新增桌面端开发、前端构建、Tauri 开发和 Tauri 打包脚本。
- 新增 Tauri 配置，目标为 Windows 安装包构建。
- 新增桌面端页面结构：
  - 登录页。
  - 服务端地址配置。
  - 设备绑定状态。
  - 任务列表。
  - 扫描上传页面：只支持文件选择上传，不接入真实扫描仪。
  - 离线阅卷页面：占位并明确“未配置/待接入”。
  - 同步队列页面。
  - 系统诊断页面。
  - 自动更新预留。
  - 本地日志。
- 新增桌面端 API Client：
  - 支持配置后端服务端地址。
  - 调用真实 `POST /api/v1/auth/login` 登录。
  - 调用真实 `GET /api/v1/auth/me` 校验 session。
  - 调用真实 `GET /api/v1/review-tasks` 获取任务列表。
  - 扫描文件上传调用真实 `POST /api/v1/files`，要求用户填写后端所需 owner/submission/exam 上下文。
- 新增本地能力接口层：
  - 安全配置存储接口。
  - 本地缓存接口。
  - 本地日志接口。
  - 系统诊断接口。
  - 当前未接入 SQLite、Stronghold 或企业更新源的能力必须返回/展示“未配置/待接入”。
- 更新 `README.md`、`apps/desktop-client/README.md` 和 Story 索引。

### 非范围

- 不实现真实扫描仪驱动、TWAIN/WIA 接入或扫描仪发现。
- 不实现完整离线阅卷任务包下载、离线草稿加密保存、冲突合并或同步提交。
- 不实现 SQLite schema、Stronghold/系统密钥链集成或真实加密存储落盘；本 Story 只搭接口并在 UI 明确标注未配置。
- 不实现自动更新服务器、签名校验或灰度升级。
- 不新增后端设备绑定、客户端诊断上报或离线包 API。
- 不新增后端权限、审计或业务模型。

### 修改文件

预计新增：

- `apps/desktop-client/package.json`
- `apps/desktop-client/index.html`
- `apps/desktop-client/tsconfig.json`
- `apps/desktop-client/vite.config.ts`
- `apps/desktop-client/src/main.tsx`
- `apps/desktop-client/src/App.tsx`
- `apps/desktop-client/src/styles.css`
- `apps/desktop-client/src/api/client.ts`
- `apps/desktop-client/src/api/auth.ts`
- `apps/desktop-client/src/api/files.ts`
- `apps/desktop-client/src/api/review.ts`
- `apps/desktop-client/src/lib/localRuntime.ts`
- `apps/desktop-client/src/types.ts`
- `apps/desktop-client/src/vite-env.d.ts`
- `apps/desktop-client/src-tauri/Cargo.toml`
- `apps/desktop-client/src-tauri/build.rs`
- `apps/desktop-client/src-tauri/tauri.conf.json`
- `apps/desktop-client/src-tauri/capabilities/default.json`
- `apps/desktop-client/src-tauri/src/main.rs`
- `apps/desktop-client/src-tauri/src/lib.rs`
- `docs/stories/STORY-033-approval.md`

预计修改：

- `apps/desktop-client/README.md`
- `package-lock.json`
- `README.md`
- `docs/stories/README.md`
- `docs/stories/STORY-033-windows-exe-client-skeleton.md`

### 验收标准

- `apps/desktop-client` 是真实 Tauri 2 + React + TypeScript 工程，而不是仅 README 预留目录。
- 提供 `dev`、`build`、`tauri:dev`、`tauri:build` 脚本。
- Tauri 配置包含 `beforeDevCommand`、`beforeBuildCommand`、`devUrl`、`frontendDist` 和 Windows bundle 目标。
- 登录页可配置服务端地址，并调用真实 `POST /api/v1/auth/login`，失败时显示真实错误。
- 客户端可调用真实 `GET /api/v1/auth/me` 显示当前用户/session 状态。
- 任务列表调用真实 `GET /api/v1/review-tasks`；无 token 或无权限时显示明确错误，不展示假任务。
- 扫描上传页面不出现假扫描仪；只提供文件选择，调用真实 `POST /api/v1/files`，缺少必要上下文时禁用上传并提示。
- 离线阅卷、安全存储、SQLite/本地缓存、自动更新、设备绑定等未实现能力全部显示“未配置/待接入”。
- 本地日志页面记录客户端侧关键操作，不伪造后端审计。
- 前端 typecheck/build 通过。
- 根级 `npm.cmd run typecheck` 和 `npm.cmd run build` 仍通过或其范围变化被明确记录。
- 若当前机器无 Rust/Cargo，Tauri 打包不作为通过项，只记录阻塞原因；若 Rust/Cargo 可用，则运行 Tauri build。
- Playwright 或浏览器检查桌面客户端页面非空，桌面和移动窄屏无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只创建 Windows EXE 客户端骨架和必要前端/原生配置，不新增后端设备绑定、离线包、自动更新、安全存储落盘或扫描仪驱动能力。

### 是否遗漏显式需求

显式需求已覆盖：Tauri 2 + React + TypeScript、登录页、服务端地址配置、设备绑定、本地加密配置存储接口、任务列表、扫描上传文件选择、离线阅卷占位、同步队列、系统诊断、自动更新预留、本地日志、Windows 打包脚本、真实后端 API 调用、未实现能力显示“未配置/待接入”。

### 是否符合当前仓库实际

符合。当前 `apps/desktop-client` 只有 README，README 和架构文档均指定 Tauri 2 + React + TypeScript。后端已有登录、当前用户、文件上传、复核任务列表等真实 API，可用于骨架阶段验证客户端 API 调用。后端尚无设备绑定、自动更新、离线任务包和安全存储服务端接口，因此本 Story 只搭客户端接口并在界面明确标注未配置。当前机器未安装 Rust/Cargo，Tauri Windows 安装包打包脚本可以提供，但本机无法实际产出安装包，需在验收记录中写明。

## Implementation

- 建立 `apps/desktop-client` Tauri 2 + React + TypeScript 工程：
  - 新增 Vite 入口、React 入口、TypeScript 配置、favicon 和前端样式。
  - 新增 `@edugrade/desktop-client` workspace package。
  - 新增 `dev`、`build`、`typecheck`、`preview`、`tauri:dev`、`tauri:build` 脚本。
- 新增 Tauri 原生骨架：
  - `src-tauri/tauri.conf.json` 配置 `beforeDevCommand`、`beforeBuildCommand`、`devUrl`、`frontendDist` 和 Windows `nsis`/`msi` bundle target。
  - `src-tauri` Rust 入口暴露运行时诊断、本地能力状态、本地日志追加命令。
  - 安全存储、SQLite 本地缓存、设备绑定、自动更新均返回“未配置/待接入”，不冒充真实可用能力。
- 新增桌面端 API Client：
  - `POST /api/v1/auth/login` 登录。
  - `GET /api/v1/auth/me` session 校验。
  - `GET /api/v1/review-tasks` 任务列表。
  - `POST /api/v1/files` 文件选择上传，缺少 token 或 submission 上下文时禁用上传。
- 新增桌面端工作区 UI：
  - 连接登录。
  - 任务列表。
  - 扫描上传。
  - 离线阅卷接口骨架。
  - 同步队列。
  - 系统诊断。
  - 本地日志。
- 更新根 README 和桌面客户端 README：
  - 记录当前能力、未实现能力、开发命令、Tauri 打包命令和 Cargo 依赖。
- 更新根 `package.json`：
  - `typecheck` 和 `build` 改为跑所有 workspace 中声明的对应脚本，避免桌面端绕过验证。
- 更新 `package-lock.json` 以锁定新增 workspace 依赖。

## Implementation Review

### 验收检查

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `apps/desktop-client` 是真实 Tauri 2 + React + TypeScript 工程 | `package.json`、`src-tauri/tauri.conf.json`、`src/main.tsx`、`src/App.tsx` | 通过 |
| 提供开发、构建和 Tauri 打包脚本 | `apps/desktop-client/package.json` | 通过 |
| Tauri 配置包含 dev/build/dist 和 Windows bundle | `src-tauri/tauri.conf.json` | 通过 |
| 登录调用真实后端 API | `src/api/auth.ts`、`App.tsx` | 通过 |
| session 校验调用真实后端 API | `src/api/auth.ts`、`App.tsx` | 通过 |
| 任务列表调用真实 API，不填假任务 | `src/api/review.ts`、`App.tsx`、Playwright 文本检查 | 通过 |
| 扫描页不出现假扫描仪 | Playwright scan page 检查 `真实扫描仪：未配置/待接入` | 通过 |
| 文件上传调用真实 `/api/v1/files` | `src/api/files.ts`、`App.tsx` | 通过 |
| 缺 submission/token 时禁用上传 | Playwright scan page 检查 `uploadDisabled=true` | 通过 |
| 安全存储/SQLite/设备绑定/自动更新明确未配置 | `localRuntime.ts`、`src-tauri/src/lib.rs`、Playwright diagnostics 检查 | 通过 |
| 本地日志不冒充后端审计 | UI 文案和 `appendLocalLog` | 通过 |
| 前端 typecheck/build 通过 | `npm.cmd run typecheck`、`npm.cmd run build` | 通过 |
| Tauri 打包脚本存在并执行到环境依赖检查 | `npm.cmd --workspace apps/desktop-client run tauri:build` | 有环境阻塞：本机无 Cargo |
| 页面桌面/移动非空、无横向滚动、无控制台错误 | Playwright 检查和截图 | 通过 |

### 发现的问题

- 首轮 Playwright 检查发现 `/favicon.ico` 404 控制台错误。
- Tauri 打包无法在当前机器完成，错误为 `cargo metadata ... program not found`，说明本机缺 Rust/Cargo。
- Vite 构建中桌面端和 Web 管理后台均有大 chunk warning。

## Fixes

- 新增 `apps/desktop-client/public/favicon.svg` 并在 `index.html` 引入，复测控制台错误为 0。
- 清理 `App.tsx` 中未使用的 UI import。
- 在 README 和审批记录中明确 Tauri 打包需要 Rust/Cargo，当前机器无法实际生成 Windows 安装包。

## Approval

### 审批结论

Approved

### 运行命令与结果

```powershell
npm.cmd install
npm.cmd run typecheck
npm.cmd run build
npm.cmd --workspace apps/desktop-client run build
npm.cmd --workspace apps/desktop-client run tauri:build
```

```text
npm.cmd install -> passed
npm.cmd run typecheck -> passed，desktop-client 与 web-admin 均通过
npm.cmd run build -> passed，desktop-client 与 web-admin 均通过，存在 Vite large chunk warning
npm.cmd --workspace apps/desktop-client run build -> passed，存在 Vite large chunk warning
npm.cmd --workspace apps/desktop-client run tauri:build -> failed，原因是当前机器未安装 Cargo
```

### 浏览器检查

```text
Dev server: http://127.0.0.1:5180/
Desktop 1280x720 -> scrollWidth 等于 viewport width，console 0 errors / 0 warnings
Mobile 390x844 -> scrollWidth 等于 viewport width，console 0 errors / 0 warnings
Scan page -> 显示“真实扫描仪：未配置/待接入”，上传按钮在缺上下文时禁用
Diagnostics page -> 显示本地加密配置存储、SQLite 本地缓存、设备绑定、自动更新均未配置/待接入
Screenshots:
- output/playwright/desktop-client-desktop.png
- output/playwright/desktop-client-mobile.png
- output/playwright/desktop-client-scan-mobile.png
- output/playwright/desktop-client-diagnostics-mobile.png
```

### 新增文件

- `apps/desktop-client/package.json`
- `apps/desktop-client/index.html`
- `apps/desktop-client/tsconfig.json`
- `apps/desktop-client/vite.config.ts`
- `apps/desktop-client/public/favicon.svg`
- `apps/desktop-client/src/main.tsx`
- `apps/desktop-client/src/App.tsx`
- `apps/desktop-client/src/styles.css`
- `apps/desktop-client/src/api/client.ts`
- `apps/desktop-client/src/api/auth.ts`
- `apps/desktop-client/src/api/files.ts`
- `apps/desktop-client/src/api/review.ts`
- `apps/desktop-client/src/lib/localRuntime.ts`
- `apps/desktop-client/src/types.ts`
- `apps/desktop-client/src/vite-env.d.ts`
- `apps/desktop-client/src-tauri/Cargo.toml`
- `apps/desktop-client/src-tauri/build.rs`
- `apps/desktop-client/src-tauri/tauri.conf.json`
- `apps/desktop-client/src-tauri/capabilities/default.json`
- `apps/desktop-client/src-tauri/src/main.rs`
- `apps/desktop-client/src-tauri/src/lib.rs`
- `docs/stories/STORY-033-approval.md`

### 修改文件

- `README.md`
- `package.json`
- `package-lock.json`
- `apps/desktop-client/README.md`
- `docs/stories/README.md`
- `docs/stories/STORY-033-windows-exe-client-skeleton.md`

### 剩余风险

- 当前机器没有 Rust/Cargo，因此未实际产出 Windows 安装包；需安装 Rust/Cargo 后运行 `npm.cmd --workspace apps/desktop-client run tauri:build`。
- 安全配置存储、SQLite 加密缓存、设备绑定、自动更新和完整离线阅卷均为接口骨架，界面已明确显示“未配置/待接入”。
- 桌面端和 Web 管理后台构建均存在 Vite large chunk warning，后续可按性能 Story 做 code splitting。

### 下一步

进入 `STORY-034 EXE 扫描工作站功能`。
