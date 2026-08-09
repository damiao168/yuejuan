# Desktop Client

EduGrade Enterprise Windows EXE 客户端骨架。

## 技术路线

- Tauri 2
- React
- TypeScript
- Vite
- Ant Design

## 当前能力

- 服务端地址配置：地址可保存到本次会话；远程服务必须使用 HTTPS，只有 `localhost`/回环地址允许 HTTP 开发连接。
- 登录：调用桌面客户端专用的 `POST /api/v1/auth/token`，访问令牌只保存在进程内存中。
- 保存登录与自动登录：仅 Windows Tauri 运行时可用。勾选后，服务地址、租户、账号和密码由当前 Windows 用户的 Credential Manager 保护；启动时读取并自动换取新的短期访问令牌。系统凭据库不可用时明确失败，不降级写入 `localStorage`、配置文件或日志。
- Session 校验：调用真实 `GET /api/v1/auth/me`。
- 任务列表：调用真实 `GET /api/v1/review-tasks`，无 token 或无权限时显示真实错误。
- 扫描工作站：不实现假扫描仪；支持选择真实考试、批量选择 PDF/图片、上传前预览、本地质量检查、可恢复队列元数据、断网检测、联网后继续当前会话可上传项、失败重试、真实 `POST /api/v1/files`、submission page 关联和服务端质量门禁入口。
- 离线阅卷：获取当前教师 review_task，聚合 answer_segment、submission page、OCR、Rubric、AI 建议分和当前会话答案图片预览；支持本地离线密钥、Web Crypto 加密草稿、联网同步提交、冲突检测、同步重试和过期缓存清理。
- 同步队列：展示当前客户端产生的上传任务；离线阅卷同步未配置/待接入。
- 系统诊断：显示 Tauri/浏览器运行时、本地能力状态和后端 `/health` 检查入口。
- 本地日志：记录客户端侧关键操作并在前端、原生命令两层脱敏认证字段；Tauri 运行时不再向 WebView `localStorage` 写日志副本，并支持删除当前/轮转日志。这不是后端 `audit_log`。

## 未实现能力

以下能力在界面中必须显示“未配置/待接入”，不得冒充真实能力：

- 真实扫描仪驱动、扫描仪发现和 TWAIN/WIA 接入。
- 设备绑定后端接口、设备吊销和绑定校验。
- Stronghold、加密 SQLite 缓存。登录凭据已接 Windows Credential Manager，但离线草稿仍不属于企业安全存储。
- 专用离线包后端 API、Stronghold/SQLite/DPAPI 持久安全存储、复杂冲突合并、图片二进制长期离线缓存。
- 自动更新源、签名校验和灰度发布。
- 跨应用重启后的文件二进制自动续传；当前只恢复队列元数据，文件句柄需要重新选择。

## 开发命令

```powershell
npm.cmd --workspace apps/desktop-client run dev
npm.cmd --workspace apps/desktop-client run test
npm.cmd --workspace apps/desktop-client run typecheck
npm.cmd --workspace apps/desktop-client run build
npm.cmd --workspace apps/desktop-client run check:security-config
```

## Tauri 命令

```powershell
npm.cmd --workspace apps/desktop-client run tauri:dev
npm.cmd --workspace apps/desktop-client run tauri:build
```

`tauri:build` 生成 Windows 安装包需要 Rust/Cargo 和 Windows 打包依赖。当前 Story 已提供脚本与配置；若本机缺少 Cargo，会失败于 `cargo metadata`。

## 安全存储运行边界

- 保存密码只在打包后的 Windows Tauri 客户端启用；浏览器开发模式会显示不可用，便于发现错误环境。
- 清除“保存的登录”会删除 Credential Manager 中 `com.edugrade.enterprise.desktop` 对应的当前用户凭据。
- 自动登录失败不会把密码复制到其他存储，也不会删除凭据；用户可修正服务地址/密码后重新保存，或手动清除。
- 访问令牌不会进入系统凭据库。每次应用启动都通过保存的凭据重新调用登录接口，因此服务端禁用账号或修改密码会立即阻止自动登录。
