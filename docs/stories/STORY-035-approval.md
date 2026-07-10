# STORY-035 自审审批记录

## Story

STORY-035 EXE 离线阅卷基础功能

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 当前教师任务 | `GET /api/v1/review-tasks?assigned_to=current_user.id` | 通过 |
| 未登录不读任务/无假任务 | UI 禁用与错误逻辑 | 通过 |
| 任务包聚合 | review/submission/OCR/questions/AI API 封装 | 通过 |
| answer_segment/OCR/Rubric/AI 展示 | `OfflineWorkbench` package view | 通过 |
| 答案图片当前会话预览 | `downloadFileBlob` + blob URL | 通过 |
| 草稿编辑 | 分数、Rubric、评语、反馈、私密备注、原因 | 通过 |
| 本地离线密钥 | 少于 8 位禁止保存 | 通过 |
| 加密保存 | PBKDF2 + AES-GCM | 通过 |
| 同步提交 | `POST /api/v1/review-tasks/{id}/submit` | 通过 |
| 冲突检测 | 状态、分配人、Rubric 版本 | 通过 |
| 防重复提交 | synced 状态阻止重复提交 | 通过 |
| 同步重试 | failed/conflict 状态可见，failed 可再次同步 | 通过 |
| 过期缓存清理 | `purgeExpiredOfflineDrafts` | 通过 |
| 本地日志 | 下载/保存/同步/冲突/清理 | 通过 |
| typecheck | `npm.cmd run typecheck` | 通过 |
| build | `npm.cmd run build` | 通过，存在 large chunk warning |
| Playwright | 桌面/移动离线页检查 | 通过 |

## 运行命令与结果

```powershell
npm.cmd run typecheck
npm.cmd run build
```

```text
npm.cmd run typecheck -> passed
npm.cmd run build -> passed, with Vite large chunk warnings
```

## 浏览器检查

```text
Dev server: http://127.0.0.1:5180/
Desktop 1280x720 offline page -> no whole-page horizontal overflow
Mobile 390x844 offline page -> no whole-page horizontal overflow
Console -> 0 errors, 0 warnings
Key UI text -> offline workbench, local key, Web Crypto, local drafts, package empty state, sync submit
Screenshots:
- output/playwright/desktop-client-offline-035-desktop.png
- output/playwright/desktop-client-offline-035-mobile.png
```

## 剩余风险

- 未在真实 token/review_task 环境中做端到端同步。
- 无专用离线包 API。
- 图片二进制不做长期离线缓存。
- Web Crypto localStorage 加密仍需升级为企业安全存储。
- 复杂冲突合并未实现。
- Vite large chunk warning 仍存在。

## 下一步

进入 `STORY-036 UI 设计规范文档`。
