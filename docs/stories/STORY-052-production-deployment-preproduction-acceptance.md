# STORY-052 生产部署 Runbook 与预生产验收规格

## Plan

### 目标

把 STORY-037 已有的 Docker Compose 私有化演示配置和 STORY-046～051 已完成的生产边界，收敛为一套可重复执行、失败可诊断、数据可备份恢复的预生产部署闭环。

本 Story 完成后，运维人员应能在一台满足条件的 Windows 主机上按 Runbook 完成：配置预检、依赖启动、数据库迁移、MinIO 初始化、首个管理员 bootstrap、应用构建与启动、健康检查、登录冒烟、备份、恢复验证和故障定位。

本 Story 关闭的是“部署过程不可复验、失败原因不清晰、迁移和初始化缺少真实证据”的缺口，不关闭真实主观题 AI、扫描仪、答案分组、完整可观测性或生产安全门禁等后续能力。

### 当前事实

2026-07-10 已在本机真实执行部署，确认：

- PostgreSQL、Redis、MinIO、Qdrant 可由 Compose 启动。
- `000001`～`000024` migration 已在 PostgreSQL 16 容器中完整执行成功。
- MinIO bucket 可创建并保持 private。
- API Gateway 可连接容器依赖，管理员 bootstrap、登录和 `/auth/me` 已真实通过。
- Qdrant 镜像不包含 `wget`，原 healthcheck 会把正常服务误判为 unhealthy。
- `minio-init` 的 entrypoint/command 组合被 Compose 解析为 `mc alias ...` 参数，而不是 shell 脚本。
- `.env.example` 使用 `EDUGRADE_ENV=private-demo`，会被生产配置门禁视为 production-like，但同时仍使用 insecure cookie、localhost CORS 和 `sslmode=disable`，导致 API 正确地拒绝启动。
- Docker Hub 拉取 Go、Node、Python 基础镜像时可能因网络/IPv6 路由失败，当前脚本没有预检、重试说明或离线镜像路线。

### 技术路线

- 继续使用 Docker Compose 作为单机私有化和受控试点部署方式，不引入 Kubernetes。
- 使用 PowerShell 作为 Windows 运维入口，脚本必须 `ErrorActionPreference=Stop`、检查退出码、输出阶段化状态且不打印密钥。
- 使用 PostgreSQL migration 容器执行前向迁移；没有 down migration 时，回滚依赖升级前备份恢复，不伪造自动 SQL 回滚。
- 使用 MinIO Client 容器初始化和备份对象存储，脚本以 shell 单参数形式传入，避免 Compose command 拆词。
- 本地演示配置使用 `EDUGRADE_ENV=local`；production-like 配置继续触发 STORY-051 前完成的安全启动门禁。
- 正式 TLS 由企业反向代理或受控 Nginx TLS 配置承载；本 Story 提供必须项和验证步骤，但不提交真实证书或密钥。
- 镜像构建失败时提供明确的 registry/base-image 预检和离线导入说明，不把网络故障误报为代码故障。

## Scope

本 Story 实现：

- 修复 Qdrant healthcheck，使其不依赖镜像中不存在的工具。
- 修复 `minio-init` 的 shell command 结构。
- 校正 `.env.example` 为明确的 local 配置，保留强制修改的示例密钥提示。
- 新增部署预检脚本，检查 Docker daemon、Compose 配置、环境变量、端口和必要文件。
- 加固初始化脚本：等待依赖健康、执行 migration、初始化 bucket、可选 bootstrap、构建/启动应用并执行 smoke test。
- 新增部署状态/冒烟脚本，检查 API、Web、Nginx、依赖健康和可选登录会话。
- 加固备份脚本，避免二进制 dump 经文本管道损坏，并生成 manifest/hash。
- 加固恢复脚本，默认要求显式确认目标，支持恢复前校验 dump，避免误覆盖。
- 新增预生产 Runbook，记录配置、启动、升级、回滚、备份、恢复、日志、常见故障和验收证据。
- 新增 STORY-052 静态检查脚本和 workspace command。
- 更新 Story 索引、Docker Compose README、生产路线图和系统优化报告中的真实验证状态。

本 Story 不实现：

- 不实现 Kubernetes、Helm、Temporal、River、SSO、OPA。
- 不实现真实主观题 AI、语义/视觉证据、答案分组或新业务页面。
- 不实现完整 OpenTelemetry/Loki、k6、ZAP、Trivy 自动门禁；这些仍属于后续专门验收范围。
- 不提交真实生产密码、token、证书或私钥。
- 不把 `lab/` 接入生产链路。
- 不承诺 Docker Compose 等同于高可用集群或灾备架构。

## Deployment Profiles

### Local / Controlled Trial

- `EDUGRADE_ENV=local`。
- 允许 HTTP、localhost CORS 和内部 PostgreSQL `sslmode=disable`。
- 只能用于本机、隔离网络或受控试点。
- 示例凭据必须在首次启动前修改；bootstrap 密码只通过进程环境或安全提示输入。

### Production-like

- `EDUGRADE_ENV=production` 或组织定义的非 local/test/dev 名称。
- session cookie 必须 secure。
- PostgreSQL DSN 不得使用开发凭据或 `sslmode=disable`。
- MinIO 不得使用示例 access key/secret。
- CORS 不得包含 localhost。
- 必须由 TLS 反向代理暴露 Web/API，并验证证书链、Host、Secure Cookie 和同源 API。
- 任何一项不满足时 API 必须拒绝启动。

## Required Workflow

```text
preflight
-> dependency images available
-> postgres/redis/minio/qdrant healthy
-> migration
-> minio bucket private
-> optional bootstrap-admin
-> build/start api/web/nginx
-> API/Web/backend-health smoke
-> optional authenticated login smoke
-> record compose status and evidence
```

升级流程：

```text
preflight
-> backup PostgreSQL + MinIO + manifest/hash
-> pull/build target image
-> run migration
-> start application
-> smoke test
-> failure: stop target + restore backup + start previous image tag
```

## Security And Privacy

- 脚本不得输出数据库、Redis、MinIO、Grafana、bootstrap 或 worker 密码。
- `.env`、备份、证书和日志不得提交仓库。
- bootstrap 密码不得写入 committed env；完成后应从当前 shell 清除。
- 备份目录包含学生答卷和成绩敏感数据，必须限制 ACL，并在转移时加密。
- smoke test 不输出 session cookie。
- 恢复属于破坏性操作，必须显式指定 dump、目标和确认参数。
- 日志采集不得包含完整答案、密码、token、密钥或 session。

## Acceptance Criteria

### 配置与启动

- Compose 在默认 local env 下可通过 `config --quiet`。
- Qdrant 能进入 healthy，不依赖 `wget/curl`。
- `minio-init` 能一次执行完成 alias、bucket 创建、private policy 和 list。
- `init.ps1` 在依赖已存在时可重复执行，不删除 volume、不重复创建管理员、不破坏数据。
- 生产配置使用 insecure cookie、localhost CORS、开发凭据或 `sslmode=disable` 时，预检或 API 启动必须失败并说明原因。
- Docker Hub/registry 不可达时，脚本必须指出镜像拉取问题和离线导入路线，不能显示为数据库或业务故障。

### 数据与恢复

- `000001`～当前最新 migration 在干净 PostgreSQL 16 上执行成功。
- 重复执行 migration 不破坏数据。
- MinIO bucket 存在且为 private。
- PostgreSQL backup 是非空 custom dump，`pg_restore --list` 可校验。
- 备份生成 manifest，记录时间、文件名、SHA-256 和镜像/项目版本线索。
- 恢复脚本必须有显式确认，不允许无参数覆盖当前数据库。
- 至少在隔离目标数据库或干净环境完成一次 restore verification。

### 应用与登录

- `/health` 返回 200。
- `/ready` 或受保护 readiness 的预期语义在 Runbook 中明确。
- Nginx `/health`、`/backend-health` 返回 200。
- Web 入口可加载，不出现静态资源 404。
- bootstrap 在没有 active platform admin 时成功；已有管理员时拒绝覆盖。
- 使用 bootstrap 管理员登录返回 200，`/auth/me` 返回 200，session cookie 为 HttpOnly。

### 回归

- `go test -count=1 ./...` 和 `go vet ./...` 通过。
- Web/Desktop typecheck 和 build 通过。
- OCR/image-quality Python tests 通过。
- STORY-049～052 静态检查通过。
- Compose local、ocr、quality、tools profiles 解析通过。

## Implementation Plan Draft

1. 修复 Compose Qdrant 和 MinIO 初始化定义。
2. 校正 local env，新增部署预检和状态检查。
3. 重构 init，使每个阶段可重入、可诊断。
4. 加固 backup/restore，增加 manifest/hash 和恢复确认。
5. 新增 Runbook 与静态检查。
6. 在当前 Docker 环境执行真实 migration、bucket、API、登录、backup 和隔离 restore 验证。
7. 执行全项目回归并记录未执行项。

## Plan Review

规格审阅结论：需要修正后进入实现。

审阅发现：

- 原路线图把 OpenTelemetry、Loki、k6、ZAP、Trivy 全部放入 STORY-052，会使单个 Story 过大，并与生产测试/安全门禁 Story 重叠。
- “一键部署成功”不能只验证容器处于 running，必须验证 migration、bucket private、API、Web、登录和 session。
- 原备份脚本把 PostgreSQL custom dump 经过 PowerShell 文本管道，存在二进制损坏风险。
- 原恢复脚本直接对当前数据库执行 `--clean`，缺少确认和隔离验证，误操作风险过高。
- `.env.example` 的 `private-demo` 与 production-like 安全校验冲突，必须明确 local 和 production 两套语义。
- 镜像仓库网络不可达属于真实部署风险，但不应通过关闭安全校验或提交固定镜像凭据解决。
- Runbook 必须记录 Compose 单机部署不提供高可用和跨机灾备，避免验收边界失真。

## Spec Fixes

已根据规格审阅修正：

- 将本 Story 收敛为 Compose 单机预生产部署核心闭环，完整性能/安全扫描留给后续门禁，不提前实现。
- 把真实登录、HttpOnly session、bucket private 和 migration 作为功能验收，不只看容器状态。
- 增加二进制安全备份、manifest/hash、显式恢复确认和隔离恢复验证。
- 明确 `local` 与 `production-like` 配置边界，保留生产配置拒绝不安全启动的行为。
- 增加 registry 网络/离线镜像诊断，但不引入私有镜像仓库产品。
- 明确 Compose 不等于高可用生产集群。

规格修正后结论：Spec Ready，可以进入实现。

## Implementation

已完成 Compose 单机预生产部署核心闭环。

新增：

- `infra/docker-compose/scripts/preflight.ps1`
- `infra/docker-compose/scripts/smoke-test.ps1`
- `docs/deployment/preproduction-runbook.md`
- `scripts/check-story052-production-deployment.mjs`

修改：

- `infra/docker-compose/docker-compose.yml`
- `infra/docker-compose/.env.example`
- `infra/docker-compose/scripts/init.ps1`
- `infra/docker-compose/scripts/backup.ps1`
- `infra/docker-compose/scripts/restore.ps1`
- API、Web、AI placeholder、OCR、image-quality Dockerfile
- `infra/docker-compose/README.md`
- 根 `package.json`、`.gitignore` 和相关项目状态文档

主要实现：

- Qdrant healthcheck 改为检查容器内 6333 监听，不再依赖镜像中不存在的 `wget`。
- `minio-init` command 改为单元素 shell script，Compose 不再错误拆词为 `mc` 参数。
- `.env.example` 明确为 `EDUGRADE_ENV=local`，与 production-like 安全启动门禁保持一致。
- Dockerfile 基础镜像支持通过 `EDUGRADE_*_IMAGE` 切换企业 mirror 或离线镜像。
- 新增 migration tracking：记录 filename、SHA-256、applied_at；已应用文件校验后跳过，校验和变化拒绝部署。
- 旧数据库没有 migration history 时默认拒绝自动 baseline；仅支持显式 `-MigrationBaselineVersion 000020`，且必须先通过 000020 Schema 指纹校验。
- `init.ps1` 增加预检、健康等待、可重复 migration、bucket 初始化、可选 bootstrap、可选 worker profile 和 smoke。
- `smoke-test.ps1` 验证 API、Nginx、backend proxy、Web、登录、HttpOnly Cookie、`/auth/me` 和受保护 readiness。
- PostgreSQL backup 在容器内生成并用 `pg_restore --list` 校验，再以 `docker cp` 复制，避免文本管道损坏。
- 每次备份使用独立目录，manifest 记录文件 hash、24 条 migration 和 8 个运行镜像 ID。
- restore 默认禁止覆盖 primary database，必须显式 `-ConfirmRestore`；支持隔离验证库和 MinIO mirror 恢复。
- 备份目录加入 `.gitignore`，避免敏感数据进入代码资产。

真实部署结果：

- Docker Engine 29.6.1、Compose 5.3.0。
- PostgreSQL、Redis、MinIO、Qdrant、API Gateway、Web Admin、AI placeholder、Nginx 共 8 个服务 healthy。
- Web/Nginx 入口：`http://127.0.0.1:8088`。
- `000001`～`000024` 已在 PostgreSQL 16 容器应用；采用 tracking 后重复执行全部安全 skip。
- MinIO `edugrade-files` bucket 已创建并保持 private。
- 管理员真实登录、HttpOnly session、`/auth/me`、authenticated `/ready` 全部 200。
- PostgreSQL custom dump 已恢复到 `edugrade_restore_verify`，验证 `24 migration / 1 active admin / 2 tenant`。
- synthetic MinIO 对象已完成“写入 -> 备份 -> 删除 -> 恢复 -> 内容校验 -> 清理”闭环。

## Implementation Review

实现审阅结论：发现问题，修正后可批准。

审阅发现：

- Windows PowerShell 把传给函数的 Compose `-d` 识别为自身 `-Debug`，导致初始化进入前台 attach。
- PowerShell 5 在 `docker image inspect` 镜像不存在时会把 stderr 提升为终止异常，预检 warning 变成失败。
- `WebRequestSession` 类型在 PowerShell 5 脚本解析阶段未加载，smoke 无法启动。
- MinIO backup/restore 的 shell script 经过 PowerShell 参数拆分后只执行到 `mc alias`，`mc mirror` 缺少参数。
- 备份路径包含 `..` 时 manifest 相对路径被错误截断。
- PowerShell 5 解析 Compose 顶层 JSON array 时把 8 个镜像聚合成一个对象。
- migration count 的容器 shell 变量传递得到空值，manifest 错记为 0。
- 空 MinIO bucket 只能证明命令成功，不能证明对象实际可恢复。
- 备份目录最初未被 `.gitignore` 排除，存在敏感数据误提交风险。

## Fixes

已完成：

- 所有 Compose 参数改为显式 string array 传递，`-d` 不再被 PowerShell 捕获。
- 预检镜像探测局部使用非终止错误策略，缺失镜像按预期 warning/fail。
- smoke helper 移除解析期类型依赖，兼容 Windows PowerShell 5。
- MinIO shell 操作构造成单一命令参数，backup/restore 实测成功。
- 备份目录在创建后立即规范化为绝对路径，manifest 相对路径正确。
- 镜像证据改为逐容器 `docker inspect`，manifest 正确记录 8 个镜像。
- migration count 改为直接读取容器环境中的数据库名/用户后执行 `psql`，manifest 正确记录 24。
- 加入 synthetic MinIO 对象恢复测试并在验证后清理测试对象。
- `infra/docker-compose/backups/` 已加入 `.gitignore`。

## Approval

结论：Approved。

验证时间：2026-07-10。

已通过：

- `go test -count=1 ./...`
- `go vet ./...`
- OCR worker：11 passed。
- image-quality worker：7 passed。
- workspace TypeScript typecheck。
- Web Admin、Desktop Client production build。
- STORY-049、050、051、052 静态检查。
- PowerShell 全脚本语法检查。
- Compose tools/ocr/quality profile `config --quiet`。
- 完整 Compose core build/up，8 个服务 healthy。
- 初始化脚本第二次执行：24 个 migration skip，bucket private，服务保持 healthy。
- anonymous + authenticated smoke：API、Nginx、Web、login、HttpOnly session、`/auth/me`、`/ready` 全部通过。
- PostgreSQL backup、manifest/hash、隔离 restore 验证通过。
- MinIO synthetic object backup/delete/restore/content verification 通过。

保留风险：

- 当前运行的是 `local` 配置和示例密钥，只适用于本机/受控试点；正式环境必须换真实密钥、TLS、非 localhost CORS 和加密数据库连接。
- migration 已在干净/小数据 PostgreSQL 16 上执行，尚未评估生产规模数据下的锁时间和升级窗口。
- OCR 与 image-quality Python tests 已通过，但本轮没有配置独立 worker 服务账号，因此没有启动两个 worker profile 做真实业务任务 E2E。
- 没有执行 k6、ZAP、Trivy、完整 Playwright 业务流和灾备主库覆盖演练。
- Compose 是单机拓扑，不提供高可用、自动故障转移或跨地域灾备。

## References

- [Docker Compose documentation](https://docs.docker.com/compose/)
- [Docker image save/load](https://docs.docker.com/reference/cli/docker/image/save/)
- [PostgreSQL pg_dump](https://www.postgresql.org/docs/current/app-pgdump.html)
- [PostgreSQL pg_restore](https://www.postgresql.org/docs/current/app-pgrestore.html)
- [MinIO Client documentation](https://min.io/docs/minio/linux/reference/minio-mc.html)
- [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/)
