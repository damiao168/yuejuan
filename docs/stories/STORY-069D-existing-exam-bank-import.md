# STORY-069D：精选历史考试题导入题库

状态：Approved（仅本切片）。优先级：P0。父 Story：[069](STORY-069-versioned-question-bank-rubric-library.md)。依赖：069B/C。

## 问题与范围

教师应能挑选优质历史题沉淀为资产，不重新录入，也不把全部旧题自动搬入题库。

- 单题与有限批量选择，先预览来源冻结题目、答案、解析、Rubric、Profile/archetype、版权与来源附件，再生成 Draft。
- 优先读取来源 exam 的 readiness 内容快照与对应 Assessment Snapshot，两者绑定一致的题目事实；若没有冻结来源则明确 unavailable/阻断此入口，不把 live Question 查询称为冻结导入。
- 如果涉及重评后评分标准，必须明确选择可验证的冻结评分版本，并保留选择来源；不把当前 Rubric 表自动覆盖原考试快照。无法取得一致评分事实时保留问题并阻断发布。
- 丢弃考试题号的身份含义和历史 AnswerArea 布局，来源题号仅作 provenance；知识点映射按目标 schema 校验，版权缺失默认 unknown。
- 查重先在获授权目标库范围展示题干内容摘要与完整 bundle hash 匹配；分值/答案/Rubric 不同提示不同版本或相似内容，由教师决定新 Item 或关联已有 Item 新 draft，禁止自动合并。
- 批量采用逐题回执与结果清单，明确部分成功、失败原因与可重试项；每题固定来源 snapshot/hash、目标 Bank/schema、mapping 和 command key。无全批事务证据不能宣称一次全量原子提交。

## API 规划

保留需求中的 `POST /api/v1/question-bank/items/import-from-question/{questionId}`，请求固定 target_bank_id、source_snapshot_id/评分事实选择、dedup_decision、expected_target_revision 与 command ID；另提供 batch 预览/确认接口。服务端校验来源考试 read 和目标题库 create，正文不可自报来源 hash 绕过核验。

结果包含 item/version、draft 状态、duplicate/similar 提示、映射问题和来源记录。重复相同命令返回原结果；若同命令 payload 不同则 conflict。

## 非范围

不自动导入全库、不自动 publish、不携带学生身份/作答证据、不导入历史页面坐标用于新考试、不以相似题干自动认定同一测量对象。

## 预计修改文件

扩展 `internal/questionbank/` source adapter/import command 与来源/回执迁移，修改 `internal/paper/` 和 `internal/assessment/` 的冻结事实读取接口、授权校验、考试配置页精选入库与题库导入结果、OpenAPI/SDK。

## 测试方式

Go 验证内容转换、来源一致性与查重权限；真实 PostgreSQL 验证快照导入、live 表后改不影响预览/确认、重复命令、部分失败恢复、跨范围拒绝。UI 验证单/批精选、重复决策和 draft 审核入口；夹具只含无身份内容。

## 验收标准

1. 来源题干、答案、解析、Rubric 与 hash 均来自显式冻结事实，缺快照明确拒绝，不静默读取 live 替代。
2. 目标仅生成 draft，未补齐 mapping/版权/评分问题不能发布；不导入学生答案和考试坐标。
3. 来源与目标双向校验权限，查重不泄露未授权库题干或命中数量。
4. 同命令重放不重复 Item/Version，不同 payload 同 key 冲突；批次失败项可独立恢复并显示成功项。
5. 完全重复、内容相似但评分不同分别提示，教师选择关联版本的结果可查；未自动合并。
6. 导入后来源考试不被修改，目标发布再用于新考试可追溯至原 snapshot。

## 规划审阅与实施记录

### Plan / Plan Review

核对当前代码后确认，既有 readiness 只持久化配置 hash 与 checks，没有题目正文；Assessment Snapshot 也不包含完整题干/答案/解析。因此不回填或伪造旧历史内容。本切片从新的 readiness 确认开始，增加只含作者内容的不可变 companion，并在同一考试锁与事务中绑定 ready 触发器冻结的精确 Assessment Snapshot ID/hash。旧记录保持明确 unavailable，配置 hash 算法保持原样。

评分来源只提供 original_exam，核验 companion 与 Assessment 的 Rubric 事实一致。当前无法验证重评后完整评分 bundle，因此不开放任意重评版本选择；不回查当前 Rubric 替换原快照。传统手工 Question 没有选项字段，客观题需要人工补齐选项，发布门禁继续阻断缺失内容。

### Implementation

新增迁移 000144：readiness 内容 companion 与保留/不可变 guard、question_bank_import 来源记录、源/目标复合约束、来源与目标事实一致性 trigger、RLS 和 append-only provenance。新增 questionbank/import.go、import_postgres.go、import_memory.go 以及单位/真实 PostgreSQL 测试。Memory Store 明确 unavailable，不模拟不存在的冻结来源。

修改 paper/readiness_snapshot.go、store_postgres_configuration.go、types.go：新确认保存内容、答案、解析、Rubric、题库来源附件与精确 Assessment 身份；不含 candidate/student/evidence/AnswerArea。readiness 查询返回 snapshot_id/import_snapshot_available。

修改 questionbank Store/类型/Handler：获授权预览、单题/最多 50 题确认、target revision/schema 固定、查重摘要、显式新 Item 或关联新 Draft、版本 provenance 读取、审核/发布前缺失年级与版权阻断。每题独立事务提交内容、audit/outbox 与持久回执，不宣称全批原子事务。

新增 HistoryQuestionImporter.tsx/CSS，接入考试资料页；目标题库、版权/年级、选项补齐、来源预览、查重摘要、关联已有题目与逐题结果可见。题库版本详情展示 provenance。未确认结果的原请求先持久化到当前用户/考试 localStorage，重试复用逐题命令；部分成功后只保留失败项。

修改 server 路由、OpenAPI、生成 SDK、路由覆盖与 schema 配置。需求中的单题 URL 与既有 items/{itemId}/versions 在 Go ServeMux 中歧义，专用 import 子 mux 精确分发 POST 路径并共用认证/数据范围中间件，既有版本路由继续保持。路由覆盖扫描支持子 mux。SDK 生成器补传 JSON 请求的声明请求头，使单题 command_id 与 Idempotency-Key 可一致传输。

### Implementation Review / Fixes

1. 移除导入时任取最新 Assessment 的行为，来源 companion 固定确认时的具体 ID/hash，并由数据库再次验证源题/readiness/Assessment/目标版本匹配。
2. 把响应中断视为结果未确认，保留原逐题 payload/command，禁止此时重新预览生成新命令。收到部分失败回执后，成功项移出待处理列表，失败项才可原命令重试或重新映射。
3. 查重增加标准化题干相同但分值/metadata 不同的 similar_content，与 exact_bundle 和 same_content_different_scoring 分开。UI 展示摘要并对关联 Item 选项去重。
4. 新增 grade_scope=unmapped 的发布阻断；版权 unknown 与目标 schema/评分门禁继续生效。
5. 真实 PostgreSQL 夹具先验证冻结题目普通写入被拒，再仅在隔离测试事务中模拟旧维护写入，证明预览和确认仍读取冻结正文；该维护绕过不属于产品功能。
6. 审阅发现 SDK JSON 方法丢弃声明的幂等请求头；已修复生成器，并用实际生成客户端与捕获 transport 验证正文/请求头身份一致。

### 验证证据

| 命令/场景 | 结果 |
| --- | --- |
| `go test -p=1 ./internal/questionbank ./internal/paper ./internal/server`、`go vet ./internal/questionbank ./internal/paper ./internal/server` | 通过；包含冻结转换、附件摘要、发布门禁以及 paper/server 回归 |
| `EDUGRADE_E2E_DATABASE_URL=... go test -p=1 ./internal/server -run '^TestE2EPostgresQuestionBankFrozenImport$' -count=1 -v` | 独立 PostgreSQL 16、当前全历史迁移至 000144，通过；覆盖普通冻结写拒绝、旧维护写后仍读快照、不可用旧记录、不可变来源、单题重放/冲突、查重、关联版本、逐题故障恢复、权限撤销、发布后用于新考试和原 snapshot 追溯 |
| `EDUGRADE_E2E_DATABASE_URL=... go test -p=1 ./internal/server -run '^TestE2EPostgresQuestionBank($|Publication$|MetadataACLSearch$|FrozenImport$)' -count=1 -v` | 069A～069D 四套真实数据库 E2E 全部通过 |
| `npm run ci:contracts`、生成 SDK transport 捕获检查、`npm --workspace apps/web-admin run check:production-routes` | 通过；478 条注册路由中 205 条 OpenAPI、273 条 reviewed gaps，breaking/schema/generated gate 通过；单题 JSON 请求保留显式 Idempotency-Key |
| `npm run typecheck`、`npm --workspace apps/web-admin run build`、新增 importer 定向 Biome lint、`npm --workspace apps/web-admin run check:production-routes` | 通过；全 workspace 类型、生产构建与路由检查通过，构建仅保留既有大 chunk 提示 |
| `npm --workspace apps/web-admin run test -- --run src/auth/loginSecurity.test.ts src/router/experience.test.ts src/router/navigation.test.ts` | 3 个文件、11 项通过；为并行身份安全改动新增的 `SessionUser.publicComputer` 补齐两个旧路由测试夹具 |
| Playwright CLI，显式 UI-only 网络夹具，桌面与 390×844 | 冻结来源、两题批量、客观题选项、相似题摘要、显式关联、网络中断原请求恢复可见；长预览在弹窗内容区滚动，移动端不横向溢出。故障场景控制台只有预期的合成网络中断 |

### Approval

审批结论：**批准 STORY-069D 精选历史考试冻结题目入库切片**。审批人：Codex（按仓库规则自审自批），日期：2026-09-13。

批准范围是当前本地工作分支的软件实现与自动验证；没有宣称预生产升级、容量或学校教师现场验收。旧 hash-only readiness 明确不可导入；当前只接受与 Assessment Snapshot 一致的 `original_exam` 评分事实，重评分版本选择仍未实现。父 [069](STORY-069-versioned-question-bank-rubric-library.md) 仍为 In Progress；下一切片为 [069E](STORY-069E-paper-import-bank.md)。
