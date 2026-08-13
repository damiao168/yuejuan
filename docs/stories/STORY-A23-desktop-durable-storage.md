# STORY-A23：Desktop Durable Storage

状态：Implemented（当前工作树）

## 范围

- Windows Tauri 扫描站使用 SQLite 保存队列/草稿元数据，本地受控目录保存扫描文件。
- 主密钥存于 Windows Credential Manager；持久化信封采用 AES-GCM，不把生产扫描队列降级为浏览器 localStorage。

## 接线

- `durable_store.rs`、`durableStore.ts` 提供队列、草稿、归档和受控资产读取边界。
- 浏览器开发模式明确报错或使用显式开发回退，不能伪装为生产耐久存储。

## 验证

- Desktop Vitest、Rust 库测试、Clippy、TypeScript 和生产构建已执行。

## 外部边界

- Credential Manager 的真实写入、安装升级、磁盘配额和崩溃恢复需要在目标 Windows 设备验收。
