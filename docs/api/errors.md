# API 错误模型

失败响应使用统一结构：

```json
{
  "error": {
    "code": "operation_in_progress",
    "message": "the operation is still processing"
  },
  "request_id": "...",
  "trace_id": "..."
}
```

客户端应根据稳定的 `error.code` 处理，不解析 message。message 只提供安全的人类可读说明。

| HTTP | 常用 code | 客户端动作 |
| --- | --- | --- |
| 400 | `invalid_request`、`invalid_pagination`、`idempotency_key_required` | 修正输入，不自动重试 |
| 401 | `unauthenticated`、`invalid_credentials` | 清理本地会话并重新登录 |
| 403 | `forbidden`、`access_scope_missing`、`worker_scope_forbidden`、`csrf_validation_failed` | 停止操作；浏览器写请求缺少 CSRF 头时刷新页面后重试，不得通过更换 tenant 参数绕过 |
| 404 | `*_not_found` | 刷新列表；响应不泄露其他 tenant 是否存在该资源 |
| 409 | `revision_conflict`、`invalid_*_transition`、`operation_in_progress`、`idempotency_key_reused_with_different_request` | 刷新最新 revision；仅对同请求和同 Key 安全重试 |
| 429 | `login_rate_limited` | 遵守 `Retry-After`，不要轮换用户名规避 |
| 503 | `idempotency_unavailable`、`dependency_unavailable` | 保留用户输入，退避后重试或转人工流程 |

未知 5xx 只显示通用失败提示和 request ID。数据库错误、SQL、堆栈、内部 URL、Token、密码和 API Key 不进入响应。
