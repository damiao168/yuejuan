# System Status API

## GET `/health/live`

轻量存活检查。用于负载均衡或容器健康检查确认 API Gateway 进程可响应。

示例响应：

```json
{
  "status": "ok",
  "health": "healthy",
  "service": "api-gateway"
}
```

旧地址 `/health` 保留兼容，但新探针使用 `/health/live`。

## GET `/health/ready`

匿名、脱敏的就绪检查。会检查当前注册的必需依赖项，任一必需依赖异常时返回 `503`。响应只包含依赖名称、状态和是否必需，不返回错误详情、地址或凭据。

依赖状态值：

| 状态 | 含义 |
| --- | --- |
| `ok` | 已配置且检查通过 |
| `error` | 已配置但检查失败 |
| `not_configured` | 未配置/待接入，不会伪装为健康 |

旧地址 `/ready` 需要 `system:read` 权限，仅为兼容保留；容器与负载均衡探针应使用 `/health/ready`。

## GET `/api/v1/system/status`

系统诊断接口。用于 Web 管理后台系统状态页和 Windows EXE 客户端诊断页。该接口不返回密钥、学生答案、学生身份信息或成绩明细。

响应字段：

| 字段 | 说明 |
| --- | --- |
| `status` | `healthy` 或 `degraded` |
| `service` | 服务名 |
| `environment` | 运行环境 |
| `version` | 当前服务版本 |
| `git_sha` / `build_time` | 构建提交与时间 |
| `image_digest` / `release_id` | 镜像与发布标识 |
| `schema_version` | 数据库 Schema 版本 |
| `started_at` | 服务启动时间 |
| `generated_at` | 状态生成时间 |
| `uptime_sec` | 已运行秒数 |
| `dependencies` | Postgres、Redis、MinIO、Qdrant、AI 服务等依赖状态 |
| `observability` | 日志格式、系统日志/审计日志边界、请求追踪头、慢请求阈值 |

## 日志边界

系统日志用于排障，默认写 JSON 到标准输出，并附带 `request_id` 和 `trace_id`。普通系统日志会脱敏 `answer`、`student`、`score`、`token`、`secret` 等字段。

审计日志用于业务追溯，保留在审计 API 和数据库记录中，不由普通系统日志替代。导出、改分、发布、申诉等业务动作仍应走审计链路。

## 请求与 SQL 指标

当前 Story 实现慢请求日志和慢查询预留标识，配置项为：

```text
EDUGRADE_SLOW_REQUEST_THRESHOLD=2s
```

API Gateway 已在数据库驱动层按操作类型记录 SQL 耗时和慢查询计数；指标不包含 SQL 文本或参数。Prometheus 从 `/metrics` 抓取请求、连接池和数据库指标。
