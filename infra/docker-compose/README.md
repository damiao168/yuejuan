# EduGrade Docker Compose 私有化部署

本目录提供 EduGrade Enterprise 本地私有化与预生产验收环境。完整操作、升级和恢复门禁见 [`docs/deployment/preproduction-runbook.md`](../../docs/deployment/preproduction-runbook.md)。默认包含：

- `api-gateway`
- `web-admin`
- `postgres`
- `redis`
- `minio`
- `qdrant`
- `ai-services-placeholder`
- `nginx`
- `db-migrate`
- `minio-init`
- `prometheus`（可选 profile）
- `grafana`（可选 profile）

`ai-services-placeholder` 只提供健康检查与“未配置”状态，不提供 OCR、模型推理或 Agent worker 能力。

## 准备配置

```powershell
Set-Location infra\docker-compose
Copy-Item .env.example .env
```

编辑 `.env`，至少修改：

- `EDUGRADE_POSTGRES_PASSWORD`
- `EDUGRADE_POSTGRES_DSN` 中的密码
- `EDUGRADE_REDIS_PASSWORD`
- `EDUGRADE_MINIO_SECRET_KEY`
- `EDUGRADE_GRAFANA_ADMIN_PASSWORD`

密码只从 `.env` 注入，`docker-compose.yml` 不硬编码密码。

## 校验配置

```powershell
docker compose --env-file .env -f docker-compose.yml config
```

只输出服务名：

```powershell
docker compose --env-file .env -f docker-compose.yml config --services
```

## 初始化

初始化会启动依赖、执行数据库迁移、初始化 MinIO bucket，再启动应用服务。

```powershell
.\scripts\init.ps1
```

首次管理员、安全 baseline、离线镜像、备份和隔离恢复参数请按预生产 Runbook 执行。初始化脚本支持重复运行，migration 会按文件名和 SHA-256 记录并跳过已应用文件。

等价手工命令：

```powershell
docker compose --env-file .env -f docker-compose.yml up -d postgres redis minio qdrant
docker compose --env-file .env -f docker-compose.yml --profile tools run --rm db-migrate
docker compose --env-file .env -f docker-compose.yml --profile tools run --rm minio-init
docker compose --env-file .env -f docker-compose.yml up -d api-gateway web-admin ai-services-placeholder nginx
```

## 启动

```powershell
docker compose --env-file .env -f docker-compose.yml up -d --build
```

访问：

- Web 管理后台入口：`http://127.0.0.1:8088`
- API Gateway：`http://127.0.0.1:8080`
- MinIO Console：`http://127.0.0.1:9001`
- Qdrant：`http://127.0.0.1:6333`

启用可观测性 profile：

```powershell
docker compose --env-file .env -f docker-compose.yml --profile observability up -d prometheus grafana
```

## 停止

```powershell
docker compose --env-file .env -f docker-compose.yml down
```

## 重置

重置会删除 Docker volume，所有数据库和对象存储数据都会丢失。

```powershell
docker compose --env-file .env -f docker-compose.yml down -v
```

之后重新执行：

```powershell
.\scripts\init.ps1
```

## 查看日志

全部服务：

```powershell
docker compose --env-file .env -f docker-compose.yml logs -f
```

单个服务：

```powershell
docker compose --env-file .env -f docker-compose.yml logs -f api-gateway
docker compose --env-file .env -f docker-compose.yml logs -f nginx
```

## 数据库迁移

```powershell
docker compose --env-file .env -f docker-compose.yml --profile tools run --rm db-migrate
```

迁移文件来自：

```text
services/api-gateway/migrations
```

## MinIO bucket 初始化

```powershell
docker compose --env-file .env -f docker-compose.yml --profile tools run --rm minio-init
```

默认 bucket：

```text
EDUGRADE_FILE_BUCKET=edugrade-files
```

## 健康检查

核心服务均配置 Docker healthcheck。查看状态：

```powershell
docker compose --env-file .env -f docker-compose.yml ps
```

HTTP 检查：

```powershell
Invoke-WebRequest http://127.0.0.1:8088/health
Invoke-WebRequest http://127.0.0.1:8088/backend-health
.\scripts\smoke-test.ps1
```

`/ready` 返回依赖详情并受 `system:read` 保护；需要登录会话。匿名容器健康检查使用 `/health`。

## 备份

```powershell
.\scripts\backup.ps1
```

输出目录：

```text
infra/docker-compose/backups
```

备份内容：

- PostgreSQL custom dump：`postgres-YYYYMMDD-HHMMSS.dump`
- MinIO bucket mirror：`minio-YYYYMMDD-HHMMSS/`

## 恢复

恢复 PostgreSQL：

```powershell
.\scripts\restore.ps1 -PostgresDump .\backups\postgres-YYYYMMDD-HHMMSS.dump
```

恢复 MinIO 对象：

```powershell
docker compose --env-file .env -f docker-compose.yml run --rm -v "${PWD}\backups:/backup" --entrypoint "" minio-init sh -ec 'mc alias set edugrade http://minio:9000 "$EDUGRADE_MINIO_ACCESS_KEY" "$EDUGRADE_MINIO_SECRET_KEY"; mc mirror --overwrite /backup/minio-YYYYMMDD-HHMMSS edugrade/$EDUGRADE_FILE_BUCKET'
```

## 常见问题

- `api-gateway` 不健康：先看 `postgres`、`redis`、`minio` 是否 healthy，再看 `api-gateway` 日志。
- Web 登录后 API 请求失败：确认从 `nginx` 访问时 `EDUGRADE_WEB_API_BASE_URL` 留空，让前端走同源 `/api`。
- MinIO bucket 不存在：重新运行 `minio-init`。
- Prometheus 没有业务指标：业务 metrics 在后续可观测性 Story 中实现，本 Story 不伪造指标。
