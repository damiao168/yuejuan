# STORY-A24：Offline Scan Spool 与断点续传

状态：Implemented（当前工作树）

## 范围

- 扫描文件可先进入本地 Spool；恢复网络后按服务端确认的 offset 上传。
- init/chunk/complete 使用哈希、幂等键和最终校验，避免重复页或错误续传。

## 接线

- `internal/captureupload` 注册上传生命周期接口，桌面队列保存远端会话和确认进度。
- 完成后由既有采集/登记链路创建一次业务事实，不把上传重试当成多次导入。

## 验证

- Capture upload 服务/存储的定向 Go 测试与 Desktop 队列测试已执行。

## 外部边界

- 500 页、断网/杀进程/重启、弱网与磁盘空间场景需要使用大样本专项演练。
