# 000123–000130 可靠性整改验收矩阵

基线 `58d4bb4`；最终复核日期 2026-09-08。A 层、B-ARCH、B-CONTRACT、B-UI 与关键命令业务幂等推广均已完成，逐项状态以 remediation-requirements.md 的实施状态表为准。数据库用例使用生产 Postgres Store、真实约束、事务及租约；对象解析器使用合成服务。

| 编号 | 代码入口 | 测试与断言 | 结果 |
|---|---|---|---|
| IMP-01 | `paper/store_postgres.go`, `paper_import_run_postgres.go` | `TestReliabilityCommandRecoveryWithPostgresTestDatabase`：重放仅一条 import/run/intent | PASS |
| IMP-02 | durable dispatch reconciler | 同上及 paper apply E2E：processing 有 v2 task 或持久意图 | PASS |
| IMP-03 | `ReplacePaperImportSources` 单事务 | reliability E2E 的 run INSERT 故障注入：旧 generation/status/source 保持 | PASS |
| IMP-04 | versioned cancel/rerun | reliability E2E；`TestPaperImportCancelThenSameSourceRerunCreatesGeneration` | PASS |
| IMP-05 | decode/OCR/parse binding | reliability E2E：旧代、协议 v1 回调不污染当前代 | PASS |
| IMP-06 | 终态后 rerun | reliability E2E：failed 后建立 generation 3；同命令仍为同 run | PASS |
| IMP-07 | runtime lease/attempt | `TestWorkerRuntimeReliabilityWithPostgresTestDatabase` 的 lease 接管 | PASS |
| IMP-08 | `CompletePaperImportParseTask` | paper apply E2E 发布故障注入：task/job/candidate 全回滚并恢复 | PASS |
| IMP-09 | `FailPaperImportRuntime` | reliability E2E 业务失败故障注入：task/job 同时回滚 | PASS |
| IMP-10 | job/run → task 锁序 | reliability E2E channel 同步 cancel/complete：cancelled、无活跃任务/失效结果 | PASS |
| IMP-11 | result receipt/hash | paper import PostgreSQL E2E：当前代/历史代相同结果重放成功，不同结果冲突且不改当前代 | PASS |
| IMP-12 | restart-safe executor/intent | paper apply E2E 用新 service/executor 实例接续 OCR 后 parse | PASS |
| IMP-13 | attempt/retry/dead-letter | `TestWorkerRuntimeReliabilityWithPostgresTestDatabase` | PASS |
| IMP-14 | review expected generation + job/run row lock | paper import PostgreSQL E2E 用 channel 并发人工保存与换源：保存成功则新代保留人工内容，否则明确冲突 | PASS |
| IMP-15 | preparation dispatch lease/reconciler | reliability E2E 连续模拟执行器直接退出；前三次领取后回收，第三次原子终结 job/run/sources，第四次不能领取；终结故障注入整体回滚 | PASS |
| CMD-01 | exam command fact/recovery GET | reliability E2E：重放/查询同一 session/exams | PASS |
| CMD-02 | immutable Web snapshot + business fact | Web 真实 request 序列保持同一 ID/body；PostgreSQL E2E 在业务提交后注入 replay Complete 故障，503 后真实 middleware 接管仍仅一场次 | PASS |
| CMD-03 | `commandAfterFailure` | Web 参数化 5xx/429/in-progress：ID、payload 不变 | PASS |
| CMD-04 | localStorage + recovery GET | Web persistence/reload；`TestExamSessionCommandReplayAndRecovery` | PASS |
| CMD-05 | transactional request hash | reliability E2E；`TestExamSessionCommandRejectsChangedRequest` | PASS |
| CMD-06 | command unique constraint | reliability E2E channel 同步两个创建者：相同资源 | PASS |
| CMD-07 | explicit new command | reliability E2E：相同业务请求的新 ID 分别创建 | PASS |
| CMD-08 | explicit pre-write rejection | Web invalid_exam_session：释放旧快照，修正后使用新 ID | PASS |
| PROJ-01 | read-only snapshot summary | `TestSummaryProjectsOperationalIssueWithoutChangingSourceFacts` 及 Postgres cursor 前后断言 | PASS |
| PROJ-02 | invalid-set cleanup | reliability E2E：单页/整份答卷删除清 state 并关闭异常 | PASS |
| PROJ-03 | current valid page set | reliability E2E：跨考试移动及 submission 重分组，无重复 | PASS |
| PROJ-04 | exception lifecycle | reliability E2E：重复刷新保留 assigned 与 operator-confirmed resolved；源删除以 source_deleted 关闭 | PASS |
| PROJ-05 | exact claimed version | reliability E2E：处理中新增版本保留 pending 并再次领取 | PASS |
| PROJ-06 | projection + cursor transaction | `TestProjectorRetainsFailedVersionForRetryWithBoundedBackoff` 及真实库重放 | PASS |
| PROJ-07 | owner/lease CAS | reliability E2E：他人 owner 被拒，最终追平 | PASS |
| MIG-01 | `000123_reliability_command_runs.sql` | `TestReliabilityMigrationFrom000122PostgresTestDatabase`：历史 JSON 保持，旧 processing 安全失败 | PASS |
| MIG-02 | `cmd/reliability-repair` | 同上：dry-run/apply/apply，第二次无重复更新时间 | PASS |
| MIG-03 | protocol v2 condition | reliability E2E：v1 task 无法发布 | PASS |
| CMD-09 | transport receipt stale takeover | 真实 HTTP 在幂等占位提交后立即中断；恢复先为 processing、过期后为 takeover_ready；原 ID/原请求并发 POST 仅产生一份业务结果 | PASS |
| CMD-10 | recovery resource boundary | 四类恢复路由按 tenant + actor 分类；人工评分/仲裁/确认/发布/导出及主观题批次的跨 tenant、跨 actor 查询均拒绝 | PASS |
| RPT-01 | export transaction connection/snapshot | MaxOpenConns=1 导出、等并发数连接池并发导出、同命令重放；所有数据读取复用当前 repeatable-read 事务 | PASS |

## 执行记录（复核后）

日志位于 `output/architecture-review-58d4bb4/`，均为专用本地 PostgreSQL 或本地客户端测试，不代表线上迁移已执行。

| 证据 | 结果与范围 |
|---|---|
| `audit-postgres-final.jsonl` | 000124 版本完整 PostgreSQL 筛选回归通过，431.617 秒；不能替代后续改动验证。 |
| `audit-projection-lifecycle.jsonl` | 000125 版本命令与投影回归通过，14.928 秒；真实采集删除/恢复、目标考试先刷新、纯时间变化保留人工状态、租约过期和同 owner 重领均覆盖。 |
| `audit-repair-construction.jsonl` | 000122 升级、修复 dry-run/apply/apply、终态历史保留及新代恢复通过；必需依赖缺失、typed nil、执行超时安全余量通过。 |
| `audit-command-client-postgres.jsonl` | 实际 Web API/命令快照逻辑连接真实 Handler 和 PostgreSQL；提交后 503、原命令恢复、新命令分别创建通过。 |
| `audit-browser-command.log` | 最终代码两项浏览器提交/刷新恢复/重复点击/清理失败场景通过。 |
| `audit-contracts-final.log` 及最终门禁复跑 | 完整 `ci:contracts` 通过，schema 000130；424 routes = 162 OpenAPI + 262 classified reviewed gaps。缺失必填属性、移除状态枚举、新增未声明路由、缺口账本缺少必需元数据均有失败证明。 |
| `audit-web-final.log` 及最终复跑 | 旧日志为 Web 119 项；20260908 后续复跑为 31 个文件 / 120 项，新增 takeover_ready 以冻结请求恢复；processing/unknown 不重发，rejected 清理。 |
| `audit-business-command-postgres-000130.jsonl`、`audit-business-command-recovery-final.json` | 人工评分/仲裁、确认/发布、报表导出的真实 Handler 重放、冲突、处理中、拒绝、未知、跨域隔离、软删除与并发恢复通过。 |
| `output/architecture-review-20260908/remediation-final.json` | 本次 P1 复审的三个原始复现均转为 PASS；记录进程中断接管、准备任务失联耗尽、单连接导出、全量 Go（20 分钟包级上限）、151 项 workspace 单测、测试门禁及客户端回归。 |
| `audit-typecheck-final.log` | 所有 workspace 类型检查通过。 |
| `audit-go-unit.jsonl` | 前次 Go 单测通过，数据库测试在该无 DSN 运行中跳过；不作为集成证据。 |
| `audit-postgres-000125.jsonl` | 完整筛选除迁移准备超时一项外通过；不称整批通过。唯一失败项在 audit-migration-retry.jsonl 单独复跑通过（58.087 秒）。 |

早期 `audit-go-tests.jsonl`、`audit-postgres-all.jsonl` 中存在迁移准备超时；迁移准备限时从 30 秒改为 2 分钟后复跑。旧失败日志保留，不算通过证据。Python 两个 Worker 目录不能在同一个 pytest 进程以默认导入模式一起收集，已分别运行；独立日志为 `audit-python-page.log` 和 `audit-python-ocr.log`。

## 本次复核新增修复

- 导入阶段同时验证 task type、source type、source ID 和 run/generation，禁止借用其他阶段任务资格。
- 持久化调度认领在同一事务取得来源快照；过期认领不能入队或写失败。HTTP 接受命令后不再执行可失控的来源 IO。
- 最后一次任务租约过期由导入协调器原子结束 task/attempt/job/run，故障注入证明整体回滚。
- 新迁移 000124 固化来源快照、解析输入不可变约束与 run/generation 外键；不能证明的旧来源保持未知。
- 新迁移 000125 统一有效页面集合并将异常历史唯一范围加入 exam，避免先刷新目标考试时覆盖原考试历史。
- 命令快照深复制；鉴权失败不清除未知命令；成功后的本地清理失败不重置业务结果；操作收据按 command ID 归档。
- 修复工具对已终态任务创建新运行而不复用取消历史，限定导入时不修复无关考试命令，并输出修复后的业务状态。
- 生产应用入口校验必需 Store，接口中携带的 typed nil 也会被拒绝。
- 业务命令恢复新增 `takeover_ready`：只有超过执行租约、且仍没有业务收据的占位可由原 ID/原请求通过数据库 CAS 接管；实时 `processing` 与 `unknown` 不重发。
- 资料准备执行超时只取消业务工作，收尾使用独立有界 context；执行器直接退出则由准备阶段租约回收器递增尝试并在上限原子终结 job/run/sources。
- 报表导出的数据集和质量查询复用当前 repeatable-read 事务连接，单连接池和并发同命令均不再等待第二条连接。

## 迁移与修复状态

- schema version 与 Docker 示例已同步到 `000130`，须按顺序执行 000123～000130。
- 旧协议任务只保留审计历史；无法证明归属的历史结果不得臆造来源快照。
- 修复工具必须限定 tenant，可再限定 import/exam；默认 dry-run。未对线上数据执行迁移或修复。
- 上线手册需要与最终版本同步核验；生产暂停写入、备份、旧执行者退出是操作前提。

## 最终验收状态

A 层已完成。首次创建并发、冻结考试限制和历史重放摘要校验在 audit-import-boundaries.jsonl 通过；Go 回归证据见 audit-go-unit-final.jsonl 及各命令 PostgreSQL 日志。已完成项不再重复开发。

B 层状态如下：

1. 关键命令业务幂等推广已完成：命令台账全部为 `closed`；stale takeover 只对完成业务收据闭环的精确路由开放。
2. B-ARCH 已完成：构造入口缺失及 typed nil、包装后事务能力和生产装配回归通过。
3. B-CONTRACT 已完成：262 条精确例外已分为 operational 2、internal-worker 33、legacy-public-read 94、legacy-public-command 133，均有 owner、范围和复核日期；关键命令六态恢复、关联 ID 与 Retry-After 已进入 OpenAPI/SDK，四类命令恢复路由有明确 tenant + actor 资源边界。
4. B-UI 已完成：职责边界、刷新/重复点击恢复、桌面磁盘 Blob 内存测量和新进程摘要恢复均有证据。

本规格的整改台账没有未关闭项；未对线上数据库执行迁移或修复。

## 本次剩余项实施结果

- B-UI 已完成：32/256 MiB 磁盘 Blob 内存测量与新进程恢复摘要通过，证据 audit-desktop-digest-memory.json；桌面 17 项、既有 Rust 12 项恢复测试通过。测量范围是摘要代码，非完整 WebView。
- 导入创建/追加/替换/人工审阅/应用/取消、投影重试/分派/解决与错误响应已通过真实 Handler schema；Web/桌面共享响应消费通过。候选结果缺省列表现在序列化为空数组；不改变解析事务。
- 000126 采集批次新命令指纹和恢复、软删除保留身份、客户端持久化与双击/刷新回归通过；最终故障注入证明 503 后精确 stale takeover 不重复创建。
- 000127 评分运行、000128 主观题批次/入队和 000129 人工评分/仲裁、确认/发布、报表导出均已完成业务唯一约束、请求指纹、同事务结果收据和恢复入口。
- 恢复协议区分 `not_accepted/processing/takeover_ready/succeeded/rejected/unknown`。Web 只在 `not_accepted` 或 `takeover_ready` 时以冻结的原命令重发；`processing/unknown` 保留原命令继续查询。
- 最终契约门禁为 424 条注册路由 = 162 条 OpenAPI + 262 条已分类冻结例外，schema 000130。按用户要求不继续扩大审查范围。
