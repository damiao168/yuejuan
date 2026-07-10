# STORY-005 自审审批记录

## Story

STORY-005 后端基础服务骨架

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 配置加载 | `internal/config` 支持环境变量和 `.env`，并有测试 | 通过 |
| 日志 | `internal/logger` 输出 JSON 结构化日志并携带 request_id | 通过 |
| 健康检查 | 实际启动验证 `GET /health` 返回 `{"service":"api-gateway","status":"ok"}` | 通过 |
| PostgreSQL 连接 | `/ready` 使用 pgx driver 执行 `PingContext` | 通过 |
| Redis 连接 | `/ready` 使用 go-redis 执行 `Ping` | 通过 |
| MinIO/S3 抽象 | `/ready` 使用 MinIO SDK 执行 `ListBuckets` 检查 | 通过 |
| 统一错误响应 | `TestNotFoundUsesUnifiedError` 覆盖 404 JSON 错误结构 | 通过 |
| 请求 ID | `TestHealth` 检查 `X-Request-ID` 响应头 | 通过 |
| 基础中间件 | 已实现 request_id、access log、recover | 通过 |
| API 路由骨架 | 已实现 `/health`、`/ready`、`/api/v1/system/info` | 通过 |
| Docker Compose 基础依赖 | `docker compose config` 校验通过，包含 postgres、redis、minio、qdrant | 通过 |
| `.env.example` | 根目录 `.env.example` 已存在 | 通过 |
| README 运行说明 | `services/api-gateway/README.md` 已存在 | 通过 |
| 基础测试 | `go test ./...` 通过 | 通过 |

## 运行命令与结果

Go 测试：

```powershell
$env:GOPROXY='https://goproxy.cn,direct'
go mod tidy
go test ./...
```

结果：

```text
ok  	edugrade-enterprise/services/api-gateway/internal/config
ok  	edugrade-enterprise/services/api-gateway/internal/server
```

Docker Compose 校验：

```powershell
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
```

结果：通过，输出 postgres、redis、minio、qdrant 配置。

启动验证：

```powershell
go build -o .\bin\api-gateway.exe .\cmd\api-gateway
curl http://127.0.0.1:18084/health
curl http://127.0.0.1:18084/api/v1/system/info
curl -i http://127.0.0.1:18084/ready
```

结果：

```text
/health -> 200 ok
/api/v1/system/info -> 200
/ready -> 503 not_ready with postgres/redis/minio dependency details
```

`/ready` 返回 503 是预期结果，因为本地依赖容器未启动；它证明服务没有假装 ready。

## 剩余风险

- 还没有认证、权限、租户隔离和审计业务写入，这些属于后续 Story。
- Docker Compose 仍未包含 backend/web/nginx 容器，这属于私有化部署 Story。
- 当前没有数据库 migration，数据库表落地属于后续 Story。
- MinIO 当前只检查连接，不实现文件上传和签名 URL。

## 下一步

进入 `STORY-006 认证与权限系统`，实现多租户 RBAC 基础、登录/登出/me、密码 hash、权限中间件和登录审计。
