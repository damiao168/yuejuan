# STORY-037 Docker Compose 私有化部署

## Plan

### 目标

实现 EduGrade Enterprise 的 Docker Compose 私有化部署配置，默认适合本地私有化演示环境。交付内容包括 backend API、web-admin、postgres、redis、minio、qdrant、ai-services 占位、nginx，以及可选 prometheus/grafana；同时提供 `.env.example`、初始化脚本、数据库迁移命令、MinIO bucket 初始化、健康检查和部署 README。

### 范围

- 完善 `infra/docker-compose/docker-compose.yml`：
  - `api-gateway`
  - `web-admin`
  - `postgres`
  - `redis`
  - `minio`
  - `qdrant`
  - `ai-services-placeholder`
  - `nginx`
  - `prometheus`（profile 可选）
  - `grafana`（profile 可选）
  - `db-migrate` 一次性迁移服务
  - `minio-init` 一次性 bucket 初始化服务
- 新增容器构建文件：
  - 后端 API Dockerfile。
  - Web 管理后台 Dockerfile。
  - AI services 占位 Dockerfile 和健康服务。
- 新增部署配置：
  - `infra/docker-compose/.env.example`
  - Nginx 配置。
  - Prometheus 配置。
  - 初始化脚本。
  - 备份/恢复脚本或文档命令。
  - Docker Compose README。
- 所有密码从 `.env` 读取，不在 compose 或代码中硬编码。
- 所有服务名称清晰。
- 提供健康检查。
- 运行 `docker compose config` 校验。

### 非范围

- 不实现生产级 Kubernetes/Helm。
- 不实现真实 AI worker runtime；`ai-services-placeholder` 必须明确是占位健康服务，不提供假 AI 能力。
- 不实现 Prometheus 业务指标 endpoint；可选 Prometheus/Grafana 只作为观测栈骨架。
- 不启动完整服务并跑端到端数据流；本 Story 验收重点是 Compose 配置、构建入口、初始化/迁移命令和配置校验。

### 修改文件

预计新增：

- `services/api-gateway/Dockerfile`
- `apps/web-admin/Dockerfile`
- `apps/web-admin/nginx.conf`
- `ai-services/Dockerfile`
- `ai-services/placeholder_server.py`
- `infra/docker-compose/.env.example`
- `infra/docker-compose/nginx/nginx.conf`
- `infra/docker-compose/prometheus/prometheus.yml`
- `infra/docker-compose/scripts/init.ps1`
- `infra/docker-compose/scripts/backup.ps1`
- `infra/docker-compose/scripts/restore.ps1`
- `infra/docker-compose/README.md`
- `docs/stories/STORY-037-approval.md`

预计修改：

- `.gitignore`
- `infra/README.md`
- `infra/docker-compose/docker-compose.yml`
- `docs/stories/README.md`
- `docs/stories/STORY-037-docker-compose-private-deployment.md`

### 验收标准

- `infra/docker-compose/docker-compose.yml` 包含要求的全部服务。
- `infra/docker-compose/.env.example` 存在，并列出所有密码、端口、bucket、DSN 等配置。
- Compose 文件不硬编码密码，使用环境变量。
- 数据库迁移命令可通过 `db-migrate` 服务执行。
- MinIO bucket 初始化可通过 `minio-init` 服务执行。
- 初始化脚本可执行迁移和 bucket 初始化。
- 每个核心长期运行服务都有健康检查。
- README 覆盖启动、停止、重置、日志、备份、恢复。
- AI services 明确标记为占位，不冒充真实能力。
- `docker compose config` 校验通过。

## Plan Review

### 是否越界

未越界。计划只补 Docker Compose 私有化部署所需配置、构建入口、初始化脚本和文档，不实现生产 Kubernetes、真实 AI worker 或新的业务能力。

### 是否遗漏显式需求

未遗漏。计划覆盖 backend API、web-admin、postgres、redis、minio、qdrant、ai-services 占位、nginx、可选 prometheus/grafana、docker-compose.yml、.env.example、初始化脚本、迁移命令、MinIO bucket 初始化、健康检查、README、服务命名、密码不硬编码和本地私有化演示环境。

### 是否符合当前仓库实际

符合。当前 `infra/docker-compose/docker-compose.yml` 只有 postgres/redis/minio/qdrant 草案；仓库没有 Dockerfile。后端 Go 服务支持环境变量配置和 `/health`、`/ready`；Web 管理后台可用 Vite 构建静态产物；AI services 当前只有边界 README，因此本 Story 将提供明确占位服务。

## Implementation

- 新增后端 API Dockerfile：
  - 多阶段构建 Go API Gateway。
  - 运行镜像包含 `wget` 以支持容器内 healthcheck。
- 新增 Web Admin Dockerfile 和 Nginx 静态配置：
  - 使用 Node 构建 Vite 产物。
  - 使用 Nginx 提供静态文件与 SPA fallback。
  - `/health` 返回 web-admin 健康状态。
- 新增 AI services 占位镜像：
  - `ai-services/placeholder_server.py` 只提供 `/health`、`/ready`。
  - 响应明确 `status=placeholder`，不提供假 AI/OCR 能力。
- 重写 `infra/docker-compose/docker-compose.yml`：
  - 长期运行服务：postgres、redis、minio、qdrant、api-gateway、web-admin、ai-services-placeholder、nginx。
  - 工具 profile：db-migrate、minio-init。
  - 可观测性 profile：prometheus、grafana。
  - 核心服务均配置 healthcheck。
  - 密码和端口全部从 `.env` 读取。
- 新增 `infra/docker-compose/.env.example`：
  - 覆盖 PostgreSQL、Redis、MinIO、Qdrant、API、Nginx、AI 占位、Prometheus、Grafana 配置。
- 新增 Nginx 反向代理：
  - `/api/` 代理 API Gateway。
  - `/ready` 和 `/backend-health` 代理后端。
  - `/ai-placeholder/` 代理 AI 占位服务。
  - `/` 代理 Web Admin。
- 新增 Prometheus 配置：
  - 当前只抓取 Prometheus 自身。
  - 明确 API metrics 由后续 STORY-038 实现，不抓 `/health` 伪装 metrics。
- 新增运维脚本：
  - `init.ps1`：启动依赖、运行迁移、初始化 bucket、启动应用。
  - `backup.ps1`：备份 PostgreSQL dump 并 mirror MinIO bucket 到宿主备份目录。
  - `restore.ps1`：恢复 PostgreSQL dump。
- 新增 `infra/docker-compose/README.md`：
  - 启动、停止、重置、日志、备份、恢复、迁移、bucket 初始化、健康检查。
- 更新 `.gitignore`，允许子目录 `.env.example` 入库。
- 更新根 README 和 `infra/README.md`。

## Implementation Review

### 验收检查

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| backend API | `api-gateway` service + `services/api-gateway/Dockerfile` | 通过 |
| web-admin | `web-admin` service + `apps/web-admin/Dockerfile` | 通过 |
| postgres | `postgres` service | 通过 |
| redis | `redis` service | 通过 |
| minio | `minio` service | 通过 |
| qdrant | `qdrant` service | 通过 |
| ai-services 占位 | `ai-services-placeholder` service | 通过 |
| nginx | `nginx` service + nginx config | 通过 |
| prometheus/grafana 可选 | `observability` profile | 通过 |
| docker-compose.yml | `infra/docker-compose/docker-compose.yml` | 通过 |
| .env.example | `infra/docker-compose/.env.example` | 通过 |
| 初始化脚本 | `scripts/init.ps1` | 通过 |
| 数据库迁移命令 | `db-migrate` service + README 命令 | 通过 |
| MinIO bucket 初始化 | `minio-init` service + README 命令 | 通过 |
| 健康检查 | 核心服务 healthcheck | 通过 |
| README 覆盖启动/停止/重置/日志/备份/恢复 | `infra/docker-compose/README.md` | 通过 |
| 服务名称清晰 | `edugrade-*` container names and service names | 通过 |
| 密码不硬编码在 compose | Compose 只引用 `${...}` 变量 | 通过 |
| 本地私有化演示默认 | `.env.example` + README | 通过 |
| Compose config 校验 | `docker compose ... config` | 通过 |

### 发现的问题

- 初版 Web Dockerfile 只复制了 web-admin workspace package 元数据，monorepo npm install 在 Docker build 中可能找不到其他 workspace package 元数据。
- 初版 MinIO 备份脚本把 bucket mirror 写到容器临时目录，容器退出后不可用。
- 当前 Docker CLI 可用，但 Docker daemon 未运行，无法执行镜像构建 smoke test。

## Fixes

- Web Dockerfile 增加 `apps/desktop-client/package.json` 与 `packages/shared-types/package.json` 复制，避免 workspace 元数据缺失。
- 备份脚本改为把宿主 `backups` 目录挂载到 `minio-init` 容器，并 mirror 到 `/backup/minio-时间戳`。
- restore 脚本改为使用容器内 `POSTGRES_USER` 和 `POSTGRES_DB`，减少硬编码。
- 用 PowerShell `ScriptBlock.Create` 校验 init/backup/restore 脚本语法。

## Approval

### 审批结论

Approved

### 运行命令与结果

```powershell
docker --version
docker compose version
docker compose --env-file .\infra\docker-compose\.env.example -f .\infra\docker-compose\docker-compose.yml config
docker compose --env-file .\infra\docker-compose\.env.example -f .\infra\docker-compose\docker-compose.yml config --services
docker compose --env-file .\infra\docker-compose\.env.example -f .\infra\docker-compose\docker-compose.yml --profile tools --profile observability config --services
$files = @('.\infra\docker-compose\scripts\init.ps1', '.\infra\docker-compose\scripts\backup.ps1', '.\infra\docker-compose\scripts\restore.ps1'); foreach ($file in $files) { $null = [scriptblock]::Create((Get-Content -Path $file -Raw -Encoding UTF8)); "OK $file" }
docker compose --env-file .\infra\docker-compose\.env.example -f .\infra\docker-compose\docker-compose.yml build ai-services-placeholder api-gateway web-admin
```

```text
docker --version -> Docker version 29.6.1
docker compose version -> Docker Compose version v5.3.0
docker compose config -> passed
docker compose config --services -> minio, ai-services-placeholder, redis, postgres, qdrant, api-gateway, web-admin, nginx
docker compose --profile tools --profile observability config --services -> prometheus, grafana, minio, postgres, qdrant, redis, api-gateway, web-admin, ai-services-placeholder, nginx, minio-init, db-migrate
PowerShell script parse -> OK init.ps1 / backup.ps1 / restore.ps1
docker compose build -> not run; Docker daemon unavailable: dockerDesktopLinuxEngine pipe not found
```

### 新增文件

- `services/api-gateway/Dockerfile`
- `apps/web-admin/Dockerfile`
- `apps/web-admin/nginx.conf`
- `ai-services/Dockerfile`
- `ai-services/placeholder_server.py`
- `infra/docker-compose/.env.example`
- `infra/docker-compose/nginx/nginx.conf`
- `infra/docker-compose/prometheus/prometheus.yml`
- `infra/docker-compose/scripts/init.ps1`
- `infra/docker-compose/scripts/backup.ps1`
- `infra/docker-compose/scripts/restore.ps1`
- `infra/docker-compose/README.md`
- `docs/stories/STORY-037-approval.md`

### 修改文件

- `.gitignore`
- `README.md`
- `infra/README.md`
- `infra/docker-compose/docker-compose.yml`
- `docs/stories/README.md`
- `docs/stories/STORY-037-docker-compose-private-deployment.md`

### 剩余风险

- Docker daemon 未运行，未能实际 build/start 镜像；当前已完成 `docker compose config` 校验。
- MinIO、Qdrant、Prometheus、Grafana 运行期健康检查仍需在 Docker daemon 可用后实测。
- Prometheus 当前不抓 API 业务指标；业务 metrics 属于 `STORY-038 可观测性与系统诊断`。
- `.env.example` 是演示值，上线前必须替换所有 `change_me_*` 密码，并保持 `EDUGRADE_POSTGRES_DSN` 与数据库密码一致。

### 下一步

进入 `STORY-038 可观测性与系统诊断`。
