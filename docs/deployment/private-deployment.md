# 私有化部署说明

当前可执行的 Compose 预生产操作以 [预生产部署 Runbook](preproduction-runbook.md) 为准；本文只保留目标拓扑和产品边界。

## 推荐拓扑

```text
Web Admin / Desktop Client
 -> API Gateway
 -> Auth / Tenant / Exam / Paper / Submission / Grading / Report / Appeal / Audit Services
 -> PostgreSQL / Redis / MinIO / Qdrant / OpenSearch / ClickHouse
 -> FastAPI AI Services
```

## 起步依赖

- PostgreSQL：核心业务数据。
- Redis：缓存、锁、任务状态。
- MinIO：答卷图片、PDF、附件。
- Qdrant：样例库、相似答案检索。
- OpenSearch 或 Loki：日志查询。

## 企业要求

- 支持内网部署、离线许可和本地模型。
- 支持 Docker Compose 起步，Kubernetes/Helm 企业化。
- 支持备份恢复、灰度更新、版本回滚和健康检查。
- EXE 更新包必须签名并校验。

## 首个管理员初始化

迁移完成后，生产环境不再启用固定密码默认账号。运维需要在受控终端设置强密码并执行一次 bootstrap：

```powershell
$env:EDUGRADE_BOOTSTRAP_USERNAME="platform_admin"
$env:EDUGRADE_BOOTSTRAP_DISPLAY_NAME="Platform Admin"
$env:EDUGRADE_BOOTSTRAP_PASSWORD="<set-a-strong-secret>"
go run ./cmd/api-gateway bootstrap-admin
```

Docker Compose 环境可使用同一组变量：

```powershell
docker compose --env-file infra/docker-compose/.env.example -f infra/docker-compose/docker-compose.yml run --rm api-gateway bootstrap-admin
```

bootstrap 默认目标是 `platform` 租户和 `platform_admin` 角色；如果已经存在 active 平台管理员，命令会拒绝覆盖。
