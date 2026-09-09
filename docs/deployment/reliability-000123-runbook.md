# 000123–000126 可靠性协议上线与修复手册

适用版本：顺序执行 `000123_reliability_command_runs.sql`、`000124_import_input_integrity.sql`、`000125_projection_lifecycle_identity.sql`、`000126_capture_batch_command.sql` 及其配套 API、Web、Worker。所有命令都应先在预生产按一个明确租户演练。修复工具默认 dry-run，不会修改数据。

## 上线不变量

- `paper_import_job.status = 'processing'` 时，当前 `paper_import_run` 必须有协议 v2 活跃任务，或有 `pending/failed` 的持久调度意图。
- 导入回调必须同时匹配 tenant、import、run、generation、source revision、input hash、task 和有效 lease；协议 v1 任务不能发布。
- 业务结果、runtime task 终态和下游任务必须在同一事务提交。
- 考试创建的 `command_id`、请求指纹和资源关联与业务对象同事务保存；恢复不依赖短期 replay 缓存。
- 投影数据和本次认领的 cursor version 在同一事务提交，只确认认领版本。

## 上线顺序

1. 暂停试卷导入的新建、素材修改、重跑和取消入口；停止旧 API 实例继续接收相关写请求。
2. 停止旧版 page-processing、OCR、paper-parse Worker 的新领取，等待短任务结束；超时任务由数据库租约和迁移撤销，不允许旧 Worker 在迁移后继续消费。
3. 备份数据库并记录下方基线观测 SQL 的结果。执行到 `000126`，确认 schema version 为 `000126`。
4. 部署只产生 `task_protocol_version = 2` 的 API/调度器，再部署带 run/generation/input/lease 校验的新 Worker。
5. 对每个目标租户先执行修复 dry-run，人工核对范围，再按需 `--apply`。需要清理历史投影时同时指定 `--exam-id`。
6. 启动 durable dispatch reconciler 和投影器，确认积压下降；最后恢复导入写入口。
7. 部署 Web。浏览器中尚未完成的考试创建命令会以原 `command_id` 和不可变请求快照恢复。

迁移命令使用现有发布系统执行。不得单独部署新 Worker 到旧库，也不得迁移数据库后重新启动旧 Worker。

仓库 Docker Compose 部署可在项目根目录使用以下维护命令。执行前在该部署的 `.env` 中固定经过验证的新镜像标签及 schema `000126`，先由入口维护页拒绝新业务写入。此处会暂停全部 API 和 Worker；不要对仍需承接流量的其他部署直接执行。

```powershell
$composeFile = 'infra/docker-compose/docker-compose.yml'
docker compose -f $composeFile stop api-gateway grading-agent ocr-worker math-recognition-worker image-quality-worker subjective-grading-worker math-verification-worker page-processing-worker
if ($LASTEXITCODE -ne 0) { throw '停止写入进程失败，不能继续迁移' }
# 使用该部署的备份流程完成数据库备份并验证可恢复后，再运行迁移服务。
docker compose -f $composeFile run --rm db-migrate
if ($LASTEXITCODE -ne 0) { throw '迁移失败，保持维护状态' }
```

保持写入进程停止，执行下述租户级检查及两次修复。确认修复输出和 schema 后再恢复已固定版本的进程：

```powershell
docker compose -f $composeFile up -d api-gateway grading-agent ocr-worker math-recognition-worker image-quality-worker subjective-grading-worker math-verification-worker page-processing-worker web-admin
if ($LASTEXITCODE -ne 0) { throw '启动失败，保持入口维护状态' }
docker compose -f $composeFile ps
```

核对 readiness、任务积压及错误指标后再退出维护状态。API 内的解析执行器和投影器随 API 启停；不能只停止独立 Python Worker 而保留旧 API 后台执行器。

## 存量检查与修复

构建工具：

```powershell
go -C services/api-gateway build -o ../../bin/reliability-repair.exe ./cmd/reliability-repair
```

租户级 dry-run：

```powershell
./bin/reliability-repair.exe --database-url $env:EDUGRADE_DATABASE_URL --tenant-id <tenant-uuid>
```

限定导入或考试：

```powershell
./bin/reliability-repair.exe --database-url $env:EDUGRADE_DATABASE_URL --tenant-id <tenant-uuid> --import-id <import-uuid>
./bin/reliability-repair.exe --database-url $env:EDUGRADE_DATABASE_URL --tenant-id <tenant-uuid> --exam-id <exam-uuid>
```

核对 JSON 输出后追加 `--apply`。工具会幂等地补回持久调度意图、把不可恢复的旧 processing 标成明确失败、标记无法证明归属的旧结果待复核，并为指定考试请求投影重建。已有终态任务的孤立运行会创建新 generation，原取消/失败历史保持不变。输出包含执行前 findings 和执行后的 import/command 状态。执行后再次运行相同 `--apply`，不得产生重复任务或新的业务变化。保存两次输出作为审计证据。指定 `--exam-id` 时每次调用会请求一次投影刷新，游标版本增加不计为新业务变化。

## 运行观测

以下 SQL 必须带明确租户参数。不要在日志或工单中记录 lease token。

```sql
-- processing 但没有活跃任务或可恢复调度意图
SELECT j.id, j.current_generation, r.id AS run_id, r.dispatch_status
FROM paper_import_job j
LEFT JOIN paper_import_run r
  ON r.tenant_id=j.tenant_id AND r.paper_import_id=j.id AND r.generation=j.current_generation
WHERE j.tenant_id=$1 AND j.status='processing'
  AND NOT EXISTS (
    SELECT 1 FROM agent_worker_task t
    WHERE t.tenant_id=j.tenant_id AND t.paper_import_run_id=r.id
      AND t.task_protocol_version=2 AND t.status IN ('queued','leased','running')
  )
  AND COALESCE(r.dispatch_status,'missing') NOT IN ('pending','failed');

-- 停滞、失败、耗尽任务及尝试次数
SELECT id, source_type, source_id, status, attempt_count, max_attempts,
       available_at, lease_expires_at, error_code
FROM agent_worker_task
WHERE tenant_id=$1 AND status IN ('queued','leased','running','failed','dead_letter')
ORDER BY updated_at;

-- 命令恢复事实；业务事实优先于有 TTL 的 HTTP replay 记录
SELECT command_id, command_status, id AS exam_session_id, command_completed_at
FROM exam_session
WHERE tenant_id=$1 AND command_id IS NOT NULL
ORDER BY command_completed_at DESC;

-- 投影积压和最近完成时间
SELECT exam_id, requested_version, projected_version,
       requested_version-projected_version AS backlog,
       projected_at, lease_owner, lease_expires_at, last_error
FROM processing_projection_cursor
WHERE tenant_id=$1
ORDER BY backlog DESC, updated_at;
```

应用日志应关联 `command_id/import_id/generation/task_id/attempt/request_id/error_code`，并记录校验结果（stale generation、protocol rejected、lease lost），而不是原始 lease token。

Prometheus 可直接查询提交拒绝和命令恢复结果；标签只有注册路由及稳定枚举，不含 command/import/task 等高基数标识：

```promql
sum by (route, error_code) (increase(edugrade_http_errors_total{error_code=~"worker_task_lease_.*|paper_import_.*|configuration_conflict"}[15m]))
sum by (route, outcome) (increase(edugrade_operation_outcomes_total[15m]))
```

API 访问日志从标准响应元数据记录 `error_code`、`command_id` 和 `operation_outcome`。后台解析失败日志记录 `import_id`、`run_id`、`generation`、`task_id`、`attempt`、`error_code` 和 `lease_validation=accepted|rejected`；实现不会记录原始 lease token。

## 暂停与回滚

出现 orphan processing、协议 v1 发布成功、候选题部分提交或投影 cursor 越过 requested version 时，立即重新暂停导入写入口和 Worker 领取，保留数据库与日志现场。

`000123` 是前向兼容、保留历史的迁移，不提供破坏性 down migration。应用回滚不能回到忽略 generation 的旧写路径继续消费新任务。安全回退方式是：保持数据库在 `000123`，停用导入相关命令和 Worker，回滚不涉及导入写回的其他服务；修复或重新部署 v2 API/Worker 后再恢复。已有 v2 任务不得降级成 v1。

考试创建和投影读接口可继续提供；若对应新应用也需回退，应先停止写入，并确认旧版本能够容忍新增列。任何人工 SQL 修复都必须先 dry-run 查询、限定 tenant/import/exam，并记录前后状态。

## 上线完成条件

- migration、真实 PostgreSQL 可靠性用例、Web 单测/类型检查、SDK/OpenAPI/schema gate 全部通过。
- orphan processing 查询为 0；协议 v1 活跃导入任务为 0。
- repair 同范围第二次执行无重复变化。
- 投影 backlog 可归零，重复刷新不改变人工 assigned/resolved 决定。
- 旧 Worker 已确认下线，且新日志能按上述关联键检索。

000126 不改写已有采集批次；新命令保存 actor 范围的指纹与唯一身份。旧记录没有指纹时按原创建字段核对；发现多个同 actor/command 的旧记录返回冲突，不猜测归属。软删除保留命令关联，不能复用旧 ID 创建新资源。
