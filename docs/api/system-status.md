# System Status API

## GET `/health`

轻量存活检查。用于负载均衡或容器健康检查确认 API Gateway 进程可响应。

示例响应：

```json
{
  "status": "ok",
  "health": "healthy",
  "service": "api-gateway"
}
```

## GET `/ready`

就绪检查。会检查当前注册的依赖项，任一依赖异常或未配置时返回 `503`。

依赖状态值：

| 状态 | 含义 |
| --- | --- |
| `ok` | 已配置且检查通过 |
| `error` | 已配置但检查失败 |
| `not_configured` | 未配置/待接入，不会伪装为健康 |

## GET `/api/v1/system/status`

系统诊断接口。用于 Web 管理后台系统状态页和 Windows EXE 客户端诊断页。该接口不返回密钥、学生答案、学生身份信息或成绩明细。

响应字段：

| 字段 | 说明 |
| --- | --- |
| `status` | `healthy` 或 `degraded` |
| `service` | 服务名 |
| `environment` | 运行环境 |
| `version` | 当前服务版本 |
| `started_at` | 服务启动时间 |
| `generated_at` | 状态生成时间 |
| `uptime_sec` | 已运行秒数 |
| `dependencies` | Postgres、Redis、MinIO、Qdrant、AI 服务等依赖状态 |
| `observability` | 日志格式、系统日志/审计日志边界、请求追踪头、慢请求阈值 |

## 日志边界

系统日志用于排障，默认写 JSON 到标准输出，并附带 `request_id` 和 `trace_id`。普通系统日志会脱敏 `answer`、`student`、`score`、`token`、`secret` 等字段。

审计日志用于业务追溯，保留在审计 API 和数据库记录中，不由普通系统日志替代。导出、改分、发布、申诉等业务动作仍应走审计链路。

## 慢查询预留

当前 Story 实现慢请求日志和慢查询预留标识，配置项为：

```text
EDUGRADE_SLOW_REQUEST_THRESHOLD=2s
```

真实 SQL 级慢查询捕获需要数据库驱动或 APM 插桩支持，后续 Story 可继续接入。
