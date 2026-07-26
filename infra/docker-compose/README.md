# EduGrade Docker Compose 私有化部署

本目录提供本地私有化与预生产验收环境。完整升级、备份和恢复门禁见 [`docs/deployment/preproduction-runbook.md`](../../docs/deployment/preproduction-runbook.md)。

默认长期服务：

- `api-gateway`、`web-admin`、`nginx`
- `postgres`、`redis`、`minio`、`qdrant`
- 内网 `grading-agent`

工具和可选服务：`db-migrate`、`minio-init`、OCR/图像质量/页面处理 Worker、Prometheus、Grafana。

## 智能阅卷边界

`grading-agent` 只监听 Compose 内网 `8100`，不映射宿主机端口，也不经过 Nginx 暴露。浏览器只能调用 API 网关。服务通过 `host.docker.internal` 访问宿主机 llama.cpp `8087`；`/health` 表示服务存活，`/ready` 表示模型运行时可用。

当前模型仍为影子建议：教师必须复核，作文和论述题仅保存影子结果，不允许自动发布最终成绩。

## 准备配置

```powershell
Set-Location infra\docker-compose
Copy-Item .env.example .env
```

至少替换：PostgreSQL、Redis、MinIO、Grafana 密码，`EDUGRADE_QDRANT_API_KEY`，以及长度不少于 32 字符的 `EDUGRADE_AI_SERVICE_TOKEN`。令牌只放在未提交的 `.env`，API 网关和 `grading-agent` 使用同一值。

### 从既有部署升级

`.env` 不随仓库更新，升级后需要手工补齐两个新增项，否则 `preflight.ps1` 会直接拒绝启动：

- `EDUGRADE_QDRANT_API_KEY`：**必填**。Qdrant 此前无鉴权，现在容器会读取该值；网关侧用同一个值发送 `api-key` 头，两端必须一致。
- `EDUGRADE_REDIS_PASSWORD`：不能再留空。Redis 过去在空密码时会静默降级为无鉴权启动，现在会直接拒绝启动。

以下新增项都有默认值，不填也能启动：`EDUGRADE_INTERNAL_BIND_HOST`（默认 `127.0.0.1`，数据面端口只监听回环，仅 nginx 对外）、各 `EDUGRADE_*_MEM_LIMIT`、`EDUGRADE_PAGE_PROCESSING_HEARTBEAT_*`。若需要从其他机器直连数据库或 MinIO 控制台，显式设置 `EDUGRADE_INTERNAL_BIND_HOST=0.0.0.0`（生产环境 preflight 会拒绝该值）。

模型运行参数：

```text
EDUGRADE_GRADING_MODEL_BASE_URL=http://host.docker.internal:8087/v1
EDUGRADE_AI_MODEL_VERSION=Qwen/Qwen3-4B-GGUF:Q4_K_M
EDUGRADE_AI_PROMPT_VERSION=subjective-local-structured-v2
```

## 校验与初始化

```powershell
.\scripts\preflight.ps1
.\scripts\init.ps1
```

等价手工命令：

```powershell
docker compose --env-file .env -f docker-compose.yml up -d postgres redis minio qdrant
docker compose --env-file .env -f docker-compose.yml --profile tools run --rm db-migrate
docker compose --env-file .env -f docker-compose.yml --profile tools run --rm minio-init
docker compose --env-file .env -f docker-compose.yml up -d --build grading-agent api-gateway web-admin nginx
```

## 访问地址

- Web：`http://127.0.0.1:8088`
- API：`http://127.0.0.1:8080`
- MinIO Console：`http://127.0.0.1:9001`
- Qdrant：`http://127.0.0.1:6333`

没有公开的 grading-agent 地址。需要诊断时使用：

```powershell
docker compose --env-file .env -f docker-compose.yml exec grading-agent wget -q -O - http://127.0.0.1:8100/health
docker compose --env-file .env -f docker-compose.yml exec grading-agent wget -q -O - http://127.0.0.1:8100/ready
```

## 常用运维

```powershell
docker compose --env-file .env -f docker-compose.yml ps
docker compose --env-file .env -f docker-compose.yml logs -f api-gateway grading-agent
.\scripts\smoke-test.ps1
.\scripts\backup.ps1
```

停止服务不会删除数据：

```powershell
  docker compose --env-file .env -f docker-compose.yml down
  ```

  ## Local Lab model integration

  The integrated grading-agent container calls the Lab-pinned llama.cpp runtime
  on the host at `host.docker.internal:8087`. The Lab startup script generates an
  API key for that runtime, so synchronize it into the private Compose `.env`
  before running preflight:

  ```powershell
  powershell -ExecutionPolicy Bypass -File ..\..\lab\scripts\prepare-local-runtime.ps1
  powershell -ExecutionPolicy Bypass -File ..\..\lab\scripts\start-local-server.ps1
  .\scripts\sync-local-grading-model-key.ps1
  .\scripts\preflight.ps1
  .\scripts\init.ps1
  ```

  The key is written only to the ignored `.env` file. Do not commit it or place it
  in frontend configuration. `grading-agent` remains internal and only returns
  teacher-reviewed suggestions; it cannot publish a final grade.

`down -v` 会永久删除当前 Compose 项目的数据库和对象存储卷，只能在确认目标项目后用于一次性环境。日常升级不得执行。

恢复命令（默认恢复到隔离的验证库；覆盖主库需显式加 `-AllowPrimaryDatabase`）：

```powershell
.\scripts\restore.ps1 -PostgresDump .\backups\postgres-YYYYMMDD-HHMMSS.dump -TargetDatabase edugrade_restore_check -CreateTargetDatabase -ConfirmRestore
```

## 模型启动

仓库不提交 GGUF 和 llama.cpp 二进制。实验机已按 `lab/config/local-runtime.json` 准备时，可在仓库根目录运行：

```powershell
powershell -ExecutionPolicy Bypass -File lab\scripts\start-local-server.ps1 -Candidate qwen3_4b
```

日志位于忽略目录 `lab/.runtime/logs`。停止模型：

```powershell
powershell -ExecutionPolicy Bypass -File lab\scripts\stop-local-server.ps1
```
