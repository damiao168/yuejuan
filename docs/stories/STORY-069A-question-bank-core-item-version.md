# STORY-069A：题库核心与 Item/Version

状态：Approved（2026-09-13，Codex 自审；仅本切片）。优先级：P0。父 Story：[069](STORY-069-versioned-question-bank-rubric-library.md)。

## 问题与目标

建立上游题目身份和草稿内容存储，让教师无需创建考试就能维护题目。此切片不开放发布与正式选题。

## 依赖与范围

复用现有 auth/AccessScope、数据库租户上下文、files、命令回执、服务路由与生成 SDK。

- QuestionBank：tenant、学校归属范围、名称、说明、生命周期、创建者；归属从服务端授权范围校验，不信任请求自报范围。
- QuestionBankItem：稳定 UUID、bank_id、item_code、subject_code、grade_scope、current_published_version_id（预留、此切片为空）、active/retired/archived、审计字段；不保存题干。
- ItemVersion：UUID、item_id、version_no、schema_version、draft revision、question_type、assessment_archetype、stem、options、default_score、knowledge_points、初版受控系统 metadata（包含 subject_code、education_stage、grade_scope 的版本事实）、author、content_hash、created_at。Item 的学科/年级索引只作身份分类，不能用其后续修改改变旧版本组卷输入。评分 bundle 后续扩展，不允许当前不完整 hash 冒充发布 hash。
- draft 修改使用 expected_revision；从旧 draft 派生新版本也保留 source_version_id。版本号由数据库在 Item 行锁下分配，不用无锁 max+1。
- 基础 bank ACL 首版支持创建者的显式内容 read/edit 与结构管理绑定，复用独立权限动作；没有内容绑定的同租户账号默认拒绝。完整组授权与角色矩阵归 069C，基础隔离不能推迟。
- 最小 UI：题库列表、题目列表、预览、草稿编辑与版本历史；未实现 publish、统计和组卷入口不进入生产菜单。

## 数据与 API 规划

首批仅建 question_bank、question_bank_acl（最小绑定）、question_bank_item、question_bank_item_version；表带 tenant 复合外键、版本唯一键 `(tenant,item,version_no)`、编号唯一键与状态约束。current 指针必须指向同 Item 版本，未来发布还需状态校验。

拟定 API：

- `GET|POST /api/v1/question-banks`
- `GET|PATCH /api/v1/question-banks/{bankId}`
- `GET|POST /api/v1/question-banks/{bankId}/items`
- `GET /api/v1/question-bank/items/{itemId}`
- `GET|POST /api/v1/question-bank/items/{itemId}/versions`
- `GET|PATCH /api/v1/question-bank/versions/{versionId}`

错误区分 invalid_input、revision_conflict、resource_not_found、access_denied；资源存在性按现有保护策略统一，不泄露未授权 bank 信息。创建/派生命令重复请求返回原回执，复用 idempotency 与持久化 command receipt。

## 非范围

不建统计、导入、metadata 全套扩展表；不实现答案/Rubric 发布、materialize、组卷或题目市场；不修改现有 paper.Question 的领域职责。

## 预计修改文件

- 新增 `services/api-gateway/internal/questionbank/{types,service,store_memory,store_postgres,handlers}.go` 与定向测试；真实 Postgres 是耐久事实，Memory 仅为明确标记测试实现。
- 新增动态顺延的 Core 迁移与 `internal/server/e2e_question_bank_test.go`，修改 auth 权限迁移/策略、server 路由与数据库租户接线。
- 新增 `apps/web-admin/src/features/question-bank/` 与 `docs/api/question-bank.md`；修改生产导航、OpenAPI 和生成 SDK。
- 迁移变更同步 Compose schema version，执行现有门禁。

## 测试方式

实施时运行 Go questionbank/auth/server 定向测试；真实 PostgreSQL 跑迁移、跨租户 FK、同租户 ACL、并发版本派生、revision 冲突、事务与回执重放。运行 SDK 生成/breaking gate、Web 类型检查/构建/生产路由检查；有限 UI 场景覆盖创建、编辑冲突与未授权直达。新命令以实际登记名称留记录，数据库 skip 不作通过。

## 验收标准

1. 无考试也能创建 Bank、Item 与 draft，重启后从 PostgreSQL 读取；Item 与内容表明确分离。
2. 两个并发版本派生得到不同递增版本号，同命令重放只有一个版本；陈旧 revision 更新返回冲突。
3. 跨租户引用被数据库拒绝，同租户未授权 bank 的列表、总数、详情与草稿更新均拒绝或不可见。
4. 发布指针为空，未完成答案/Rubric 的草稿不能被正式考试或组卷选择。
5. 分值为正、题型/archetype 使用既有领域枚举，未知值拒绝；对象哈希稳定，语义数组顺序不被错误排序。
6. UI 用生成 SDK 与后端授权，未实现能力不出生产菜单；不影响已有考试和阅卷流程。

## 规划审阅与实施记录

规划已限定四张核心表与基础 ACL，避免推迟安全边界或建立巨型 Story。[总路线图](../prd/assessment-platform-roadmap.md)定义 hash 与权限不变量。本切片批准后进入 069B。

### Plan / Plan Review（2026-09-13）

按当前工作分支审阅而非假定远端 main。首版只存草稿；评分 bundle、发布、导入与组卷不纳入本切片。确认现有 AccessScope、业务回执与 outbox 可复用，但题库没有考试 ancestry，必须在自身 Store 中检查 tenant + school + bank ACL，且不能通过 HTTP 响应缓存绕过撤权。

### Implementation

新增文件：

- `services/api-gateway/internal/questionbank/types.go`、`service.go`、`store_postgres.go`、`store_memory.go`、`handlers.go`、`questionbank_test.go`。
- `services/api-gateway/internal/idempotency/command_identity.go`：沿用 key 校验，仅绑定业务命令标识；不缓存私有内容响应。
- `services/api-gateway/migrations/000140_question_bank_core.sql`：四张核心表、复合外键、草稿状态/身份保护、RLS 与动作许可。
- `services/api-gateway/internal/server/e2e_question_bank_test.go`：独立真实数据库，生产服务接线与 8 个子场景。
- `apps/web-admin/src/api/questionBank.ts`、`src/features/question-bank/QuestionBankPage.tsx`、`question-bank.css`。
- [题库 API 文档](../api/question-bank.md)。

修改文件：

- API `internal/server/{modules,server}.go` 与 `internal/auth/resource_scope_test.go`：Memory/生产 Postgres 接线、11 个受认证路由、明确独立内容范围分类。
- OpenAPI、route-coverage、SDK generated client/types、authorization role-matrix：接口契约、分页、命令 header 与许可。
- Web `AppShell.tsx`、router/routes、types、api/userError：生产入口、延迟加载与中文错误。页面用生成 SDK，查询按 tenant/actor 分区。
- Compose 和 `.env.example`：schema version 同步 000140。
- Story 索引、父 Story、总路线图与验证状态：记录本切片证据，后续切片保持 Planned。

### Implementation Review / Fixes

1. 补齐新租户权限模板：租户创建复制 platform 租户模板，迁移必须同时写入该模板，不能只回填已有学校租户。平台角色没有新增动作绑定，平台 actor 仍拒绝题库。
2. 内容 create/edit 与回执重放都同时检查 read；撤销 read 后保留 edit 也不能读取或重放内容。同租户管理员没有隐含内容授权。
3. 版本分配在 Item 行锁下计算；修改使用 expected_revision。补查同 Item source FK、草稿发布禁令、归档锁定和 JSON null/未知 metadata 的直接数据库写入。
4. outbox 故障注入证明 Item、Version、audit、outbox 和 receipt 整体回滚；同一失败命令解除故障后可重试。
5. 修正选项输入立即过滤换行的问题，只在提交时移除空行；修正后台 refetch 覆盖未保存编辑的问题，版本详情独立查询，表单 revision 仅在显式切换/保存/加载最新版本时更新。
6. source_version_id 用 OpenAPI anyOf 表达 string/null，保证生成 SDK 为明确联合类型；保留原有契约格式，减少无关 diff。

### 运行命令与结果

真实数据库：独立 Docker PostgreSQL 18，loopback 55439；使用现有 E2E helper 创建并清理唯一临时数据库，全部历史迁移至 000140。没有将数据库 skip 计为通过。

| 命令（相应 workspace） | 结果 |
| --- | --- |
| `go test ./internal/auth ./internal/idempotency ./internal/questionbank ./internal/server -skip 'TestE2EPostgres\|TestPostgres' -count=1` | 四包通过；不作为真实 Postgres 证据，数据库用例另跑 |
| `go test ./internal/server -run '^TestE2EPostgresQuestionBank$' -count=1 -v`（设 EDUGRADE_E2E_DATABASE_URL） | 通过，8 个子场景；HTTP 认证/ACL/重放、并发、外键、锁定、事务、RLS、新租户模板 |
| `go test ./internal/server -run '^TestCoreWorkflowE2EWithPostgresTestDatabase$' -count=1 -v`（同上） | 已有真实考试→阅卷→发布→仲裁/报表恢复流程通过 |
| `go test ./internal/questionbank -count=1`、`go vet ./internal/questionbank ./internal/idempotency ./internal/server` | 通过；分值、枚举、哈希顺序与 Memory 回执/ACL 并发验证 |
| `npm run generate:sdk`、`npm run generate:route-coverage`、`npm run sync:schema-version`、`npm run ci:contracts` | 通过；449 条登记路由（176 OpenAPI，273 既有 reviewed gaps），SDK 与 000140 同步 |
| `npm --workspace @edugrade/web-admin run typecheck`、`run build`、`run check:production-routes` | 通过；生产构建有既有大 chunk 提示 |
| `npm --workspace @edugrade/web-admin test -- src/router/routes.test.tsx src/auth src/api/userError.test.ts` | 4 个文件、24 项通过 |
| `npm run lint`、新题库文件 Biome lint、`npm run check:user-facing-copy`、`git diff --check` | 通过；全库 lint 有 40 条既有 warning，新题库无诊断 |
| Playwright CLI，1440×1000 / 390×844 | 明确 UI-only Mock：创建题库/题目、选项换行、草稿预览、409 保留编辑、加载后保存、派生与版本历史、撤权后隐藏缓存、响应式布局；修正后后台刷新保留编辑且仍以原 revision=1 提交，显式加载后以最新 revision=3 保存并派生 v2，复验通过 |
| `go test -race ./internal/questionbank -count=1` | 未运行成功：本机 CGO=0 且无 C 编译器。不能声明 race 检查通过；真实 DB 并发与 Memory 并发用例已通过 |

浏览器截图位于本地 `output/playwright/question-bank-desktop.png` 与 `question-bank-mobile.png`，不冒充跨服务/学校现场证据。

### Approval 与剩余边界

审批结论：**批准 STORY-069A 私有题库核心草稿切片**。审批人：Codex（按仓库规则自审自批），日期：2026-09-13。最终浏览器修正复验与最后真实 PostgreSQL 补查均通过；教师可创建私有 Bank/Item，租户管理员仍不能读取该私库，创建者 ACL 和重放均不能越过学校范围。

| 验收项 | 最终证据 | 结论 |
| --- | --- | --- |
| 1：考试外创建与耐久存储、身份/内容分离 | 真实教师与管理员 HTTP 创建、生产 Store、重新构建 Router 后耐久回执读取，独立 Item/Version 表 | 通过 |
| 2：并发派生、同命令单版本、revision 冲突 | 8 个不同命令并发产生 v2～v9，4 个相同命令只产生 v10，竞争编辑一个成功/一个冲突 | 通过 |
| 3：跨租户 FK、同租户私库、学校范围 | tenant 复合 school/bank/source FK、四表 RLS、私库列表/计数/详情/编辑拒绝、撤权与学校范围重放拒绝 | 通过 |
| 4：草稿不得正式选题 | current 指针 null、workflow draft 的数据库约束，直接改 published/指针被拒绝，无考试选题或组卷接线 | 通过 |
| 5：枚举、分值与哈希 | 正数/两位小数、既有题型/archetype、metadata 校验；稳定哈希且选项顺序改变哈希，draft hash 明确分域 | 通过 |
| 6：生产 UI/SDK 与既有回归 | 11 个契约接口与生成 SDK、生产菜单、浏览器明确 Mock 交互、后台刷新修正复验；已有真实 PostgreSQL 考试/阅卷/发布/报表回归 | 通过 |

- 真实学校体验、容量与预生产升级未验收；本地测试库不代表生产数据库已升级。
- 首版 ACL 只开放创建者绑定；组授权与授权管理 UI 归 069C。
- 不具备完整评分 bundle、审核发布或用于考试的能力；draft hash 不代表发布 hash。
- 下一切片为 [069B](STORY-069B-answer-rubric-publish-workflow.md)，父 069 不因本切片完成而整体批准。
