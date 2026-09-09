# 关键写操作幂等与恢复台账

更新日期：2026-09-08。表内操作均已关闭。`protected` 只表示 HTTP 层要求 `Idempotency-Key`；只有同时具备业务唯一约束、请求指纹和恢复入口的操作，才允许 stale takeover。未来新增操作仍须先进入本台账，未标为 `closed` 的行不得加入 `recoverableRoutes`。

| 操作 | 操作 ID 来源 | 业务唯一/结果关联 | 恢复入口 | 行为证据 | 状态/后续批次 |
|---|---|---|---|---|---|
| 创建考试场次 `POST /exam-sessions` | Web 提交前生成并持久化 `command_id`；header/body 相同 | `exam_session(tenant_id,created_by,command_id)` 唯一；同事务保存 request hash、完成状态及子考试 | `GET /exam-sessions/commands/{commandId}` | CMD-01..08、真实 PostgreSQL middleware 故障注入、Web 快照/刷新/重复点击测试 | **已完成 / closed**；允许该精确路由 stale takeover |
| 创建/变更/重跑试卷导入 | Web 每次明确操作生成 ID；网络重试复用 | `paper_import_run(tenant_id,actor_id,command_id)` 唯一；关联 import/run/generation/source revision | 当前 import/run 查询；repair dry-run | IMP-01..14、MIG-01..03 | **closed for import protocol**；后续可补独立 command 查询资源 |
| 创建采集批次 | Web 按 tenant/actor/exam 持久化原 payload 与 idempotency_key，header 同 ID | 000126：actor/command 唯一约束、请求 SHA-256、业务同事务提交；包括软删除后恢复 | GET /exams/{examId}/capture-batches/commands/{commandId} | audit-capture-command-postgres.jsonl：并发、异请求冲突、软删除、新命令、503 后恢复；最终 PostgreSQL 回归补证 503 后精确 stale takeover 不重复创建；audit-capture-browser.log：刷新/重复点击 | **已完成 / closed**；仅开放该精确创建路由 stale takeover |
| 分块上传 init/chunk/complete | 桌面 SQLite 保存稳定 asset/context key 与 remote upload ID | upload session、内容 hash、confirmed offset；complete 关联 file/capture file | GET /capture/uploads/{id} 返回 command_id、processing/succeeded/failed、原会话及结果；init 响应丢失时复用原输入 init | audit-upload-recovery.jsonl：查询只读/偏移/完成/权限；audit-desktop-final.log：19 项通过，已完成恢复不初始化不传输，错输入拒绝；既有 500 页恢复 | **closed for upload recovery contract**；复用传输状态，不新增第二套工作流，不开放通用 stale takeover |
| 创建评分运行 | Web 按 tenant/actor/exam 保存 command ID；header/body 相同 | 000127：actor/command 唯一、请求指纹与运行同事务；软删除保留结果身份 | GET /exams/{examId}/scoring-runs/commands/{commandId}；创建成功与评分生命周期分别表达 | audit-scoring-command-postgres-final.jsonl：并发、异目标冲突、actor 隔离、软删除、503 后精确 takeover；audit-scoring-browser.log：刷新与双击；客户端 3 项及真实 Handler/schema 通过 | **已完成 / closed**；仅开放该精确创建路由 stale takeover |
| 主观题批次/入队 | Web 保存原 command/segment 列表；enqueue:{batchId} 是稳定入队命令 | 000128：创建指纹与 actor 校验；首次入队持久化版本快照和 run request IDs，重试不重新计算；中断可补齐任务 | GET /subjective-grading-batch-commands/{commandId}；GET /subjective-grading-batches/{batchId}/enqueue-command | audit-subjective-command-final.jsonl：PostgreSQL 并发/冲突/软删除、run/task 中断重试；部分失败后版本变更回归；audit-subjective-browser.log 刷新/双击；客户端 2 项、类型检查、Handler/schema 通过 | **已完成 / closed**；精确创建/入队路由可 stale takeover，部分入队仍只按原冻结计划补齐 |
| 人工评分提交/仲裁 | Web 在提交前按 tenant/actor/target 保存 command ID、原 payload 与 expected revision | 000129 `business_command_receipt(tenant,actor,command_id)` 唯一；请求指纹、任务/成绩结果与业务事务一起提交；目标软删除后收据仍保留 | GET /review-commands/{commandId}，区分 not_accepted/processing/takeover_ready/succeeded/rejected/unknown | 20260908 复审复现覆盖层及真实 HTTP 中断回归：占位提交后立即中断、过期后原命令并发接管、仅一份业务结果、跨 tenant/actor 隔离；浏览器刷新以冻结请求恢复 | **已完成 / closed**；仅开放 submit/arbitrate 精确路由 stale takeover |
| 确认成绩/发布 | Web 按 tenant/actor/exam 保存 command ID 与原 reason | 000129：命令收据与确认/发布业务事务一起提交，actor/command 唯一且请求指纹覆盖操作、目标和输入 | GET /score-commands/{commandId}，保留终态结果及错误状态 | 真实 Handler 重放、异请求冲突与跨 tenant/actor 恢复隔离；Web 120 项及浏览器恢复通过 | **已完成 / closed**；仅开放 confirm/publish 精确路由 stale takeover |
| 报表导出 | Web 按 tenant/actor/exam 保存稳定 command ID | 000129：命令收据与 report/artifact 结果同事务；导出读取显式复用同一 repeatable-read 事务连接和快照；相同命令并发返回同一 report，保存不可变 CSV 结果供过期后恢复 | GET /report-commands/{commandId}；不依赖临时下载 URL | 20260908 复审复现覆盖层、单连接池、并发同命令、占位提交后中断恢复、软删除 artifact 后恢复及新命令新 report 均通过 | **已完成 / closed**；仅开放 export 精确路由 stale takeover |

## 推广门禁

1. 先定义业务事实与 `(tenant, actor, command_id)` 唯一约束，并把请求指纹和结果关联放进同一事务。
2. 再提供可区分 `not_accepted/processing/takeover_ready/succeeded/rejected/unknown` 的恢复查询及同请求/异请求并发测试；只有 `takeover_ready` 允许客户端用冻结的原 ID、原请求重发，实时 `processing` 和 `unknown` 仍只读等待。
3. 最后才把该精确路由加入 `recoverableRoutes`。禁止按整个 `protectedRoutes` 批量开启 stale takeover。
4. 公共新增命令必须进入 OpenAPI；已有缺口只能使用 `registered-route-exceptions.json` 中冻结的精确例外。
