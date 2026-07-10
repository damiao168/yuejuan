# STORY-033 自审审批记录

## Story

STORY-033 Windows EXE 客户端骨架

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| Tauri 2 + React + TypeScript 工程 | `apps/desktop-client/package.json`、`src-tauri/tauri.conf.json` | 通过 |
| 开发/构建/打包脚本 | `dev`、`build`、`typecheck`、`tauri:dev`、`tauri:build` | 通过 |
| Windows bundle 目标 | `targets: ["nsis", "msi"]` | 通过 |
| 登录调用真实 API | `POST /api/v1/auth/login` | 通过 |
| session 校验调用真实 API | `GET /api/v1/auth/me` | 通过 |
| 任务列表调用真实 API | `GET /api/v1/review-tasks` | 通过 |
| 扫描不伪造扫描仪 | 页面显示“真实扫描仪：未配置/待接入” | 通过 |
| 文件选择上传调用真实 API | `POST /api/v1/files` | 通过 |
| 缺上下文禁用上传 | Playwright scan page `uploadDisabled=true` | 通过 |
| 未实现能力明确标注 | 诊断页显示安全存储、SQLite、设备绑定、自动更新未配置/待接入 | 通过 |
| 本地日志不冒充审计 | 页面说明“这不是后端 audit_log” | 通过 |
| 前端 typecheck | `npm.cmd run typecheck` | 通过 |
| 前端构建 | `npm.cmd run build` | 通过，存在 large chunk warning |
| Tauri 打包命令 | `npm.cmd --workspace apps/desktop-client run tauri:build` | 脚本存在；当前机器缺 Cargo，无法实际打包 |
| 桌面/移动布局 | Playwright desktop/mobile/scan/diagnostics checks | 通过 |

## 运行命令与结果

```powershell
npm.cmd install
npm.cmd run typecheck
npm.cmd run build
npm.cmd --workspace apps/desktop-client run build
npm.cmd --workspace apps/desktop-client run tauri:build
```

```text
npm.cmd install -> passed
npm.cmd run typecheck -> passed
npm.cmd run build -> passed, with Vite large chunk warnings
npm.cmd --workspace apps/desktop-client run build -> passed, with Vite large chunk warning
npm.cmd --workspace apps/desktop-client run tauri:build -> failed because Cargo is not installed
```

## 浏览器检查

```text
Dev server: http://127.0.0.1:5180/
Desktop 1280x720 -> scrollWidth = viewport width
Mobile 390x844 -> scrollWidth = viewport width
Console -> 0 errors, 0 warnings after favicon fix
Scan page -> no fake scanner; upload disabled without submission/token context
Diagnostics page -> not-configured local capabilities visible
Screenshots:
- output/playwright/desktop-client-desktop.png
- output/playwright/desktop-client-mobile.png
- output/playwright/desktop-client-scan-mobile.png
- output/playwright/desktop-client-diagnostics-mobile.png
```

## 剩余风险

- 当前机器缺少 Rust/Cargo，未能实际生成 Windows 安装包。
- 安全存储、SQLite 加密缓存、设备绑定、自动更新、完整离线阅卷仍是待接入能力。
- Vite 构建仍有 large chunk warning。

## 下一步

进入 `STORY-034 EXE 扫描工作站功能`。
