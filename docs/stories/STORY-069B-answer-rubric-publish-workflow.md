# STORY-069B：答案/Rubric 版本与审核发布

状态：Approved（2026-09-13，Codex 自审；仅本切片）。优先级：P0。父 Story：[069](STORY-069-versioned-question-bank-rubric-library.md)。依赖：069A。

## 问题与目标

题目只有题干不能作为完整评分资产。把答案、解析与 Rubric 固定为版本事实，建立内容绑定的审核发布，并实现手工指定版本用于草稿考试。

## 范围与状态机

- ItemVersion 绑定明确 AnswerVersion、Solution 事实、RubricVersion；答案包含标准/等价答案/容差，Rubric 保留评分点、required、evidence_requirements、deductions/examples/max_score。
- Rubric Library 模板有稳定 template_id 与 template_version，独立审核/发布；选用模板固定版本并生成题目自己的评分 bundle。模板新版本、评分点修改不得改变已有 ItemVersion。
- 默认 workflow 为 draft → reviewing → approved → published。reviewing/approved/published 不得编辑 bundle，包含子表与附件绑定；reviewing 可退回 draft，approved 可撤回 draft，发布只能新建 draft。退回/撤回写记录并使旧审批失效。
- review 记录 reviewer、decision、comment、content_revision、bundle_hash；发布验证审批与当前 revision/hash 相符。默认作者不能批准自己的发布；机构若允许例外，要求显式策略与审计，不能靠角色名隐式绕过。
- 发布校验按 archetype 判断标准答案与 Rubric 的必要性，沿用现有 ready/评分契约；默认分值、评分点总分与 max_score 一致，容差/选项响应合法，版权与用途策略通过。
- 本切片提供最小 Reviewer/Publisher 用户绑定与动作权限；组授权和完整 ACL 管理归 069C。结构 Manager 不能自动查看或发布内容。

## 数据与 API 规划

新增 question_bank_answer_version、question_bank_rubric_version、question_bank_review、question_bank_item_asset 与模板身份/版本表；Solution 首版可作为 ItemVersion 内版本化 JSON，不为单一字段强行建表。保留内容和评分子摘要，正式 bundle hash 覆盖完整发布事实。

拟定 `POST /api/v1/question-bank/versions/{versionId}/submit-review|approve|return-to-draft|publish`，具体路径分别登记 OpenAPI；`GET|POST /api/v1/question-bank/rubric-templates` 与模板版本动作；`POST /api/v1/exams/{examId}/questions/materialize-from-bank` 固定 bank version 清单、question_no/sort_order、target revision 和 command ID。

materialize 必须单事务复制 Question、AnswerKey、Solution、Rubric 和必要 Assessment config，并写 provenance、审计与命令回执。仅允许已发布且 active、用途/版权可正式投放的版本；目标考试必须可配置、分值不隐式缩放。未配置答题卡区域或学科策略时保持 draft/not_ready，不能自动宣布 ready。新考试布局单独配置，不复制旧坐标。

question 来源字段 source_type/source_bank_item_id/source_bank_item_version_id/source_content_hash 在复制后固定；score_release_question.source_type 继续表示评分来源，不能复用为 question_bank 内容来源。快照扩展用新 schema，旧 readiness/Assessment hash 继续按旧版本解释。附件按受控事实复制/绑定保留，题库归档不能触发已使用考试附件清理。

## 非范围

不自动审核、不自动 ready、不缩放 Rubric、不批量导入历史题、不在阅卷时动态读取题库最新内容、不因改模板刷新旧题。

## 预计修改文件

扩展 `internal/questionbank/` 与发布/模板/附件迁移；修改 `internal/paper/` 的复制事务和 provenance、`internal/assessment/` 配置与快照版本、auth/路由、files 引用生命周期、OpenAPI/SDK；添加管理端评分标准、审核历史、发布动作和考试内“从题库选题”。

## 测试方式

Go 定向验证评分必要性、状态机与 bundle hash；真实 PostgreSQL 直接 UPDATE 子表、并发 approve/publish、当前指针、materialize 注入失败回滚、命令恢复和附件引用；契约生成/门禁与 Web 类型构建；浏览器仅串通 draft→审核→发布→用于考试。既有 paper/assessment/评分消费者做定向回归。

## 验收标准

1. Published 的题干、答案、解析、Rubric、metadata、附件绑定在服务和数据库均不可原地改，模板新版不影响旧 bundle。
2. 审批对应同 revision/hash；退回修改后必须重新审批，旧审批不能发布新内容，默认自审拒绝。
3. materialize 任一步失败不留下半套题目或孤立回执；同命令重放返回相同题目 ID，跨范围或 ready 考试写入拒绝。
4. 考试使用 v1 并 ready 后，题库 v2 发布/Item 退役不改变考试题干、答案、评分与快照 hash。
5. 源 hash、目标考试 hash 职责分开，旧快照可读取；模板/附件来源可追溯，实际评分读取考试冻结事实。
6. 选题后仍通过既有分值、AnswerArea、Profile、风险与 ready 检查；缺配置明确 not_ready，不旁路 AI 准入。

## 规划审阅与实施记录

规划已把“可发布资产”和“指定版本进入考试”放在同一验收切片，补齐 Rubric Library 模板身份与子表锁。以下记录只批准 069B；父 069 仍需 069C～069F。

### Plan / Plan Review（2026-09-13）

按 069A 已批准的工作树继续实施。审阅确认评分事实必须有独立子表和数据库锁，审批必须绑定 revision/hash，模板选用必须复制事实，materialize 必须复用 paper/assessment 的考试实例和 ready 门禁。来源 hash 与考试快照 hash 保持不同职责；题库不保存或复制 AnswerArea 坐标。

### Implementation

- `000141_question_bank_publication.sql` 增加 answer/rubric/review/asset/template 事实、review/publish ACL、状态机、current published 指针、bundle hash、发布/子表/附件/来源追溯保护和题库来源快照 schema v2；新租户权限模板同步包含新增动作。
- `internal/questionbank/` 增加评分规范化、跨 Memory/PostgreSQL 的规范 hash、审核发布、Reviewer/Publisher 绑定、模板和 materialize；所有写命令继续使用同事务 command receipt、audit 与 outbox。
- `internal/paper/materialize_bank.go` 在调用者事务中复制 Question、AnswerKey、Solution、Rubric 与默认 Assessment config；paper/assessment 类型和查询保存不可变来源字段，旧快照仍按 v1 读取。
- files 服务和数据库都阻止仍被题库版本引用的附件进入删除生命周期。
- OpenAPI、生成 SDK、角色矩阵和服务器路由登记评分、审核、模板与考试复制接口。
- 管理端增加答案/解析/Rubric/模板/附件编辑、审核历史与动作、最小用户授权，以及考试资料页的“从题库选题”。按钮按全局许可、Bank ACL、作者和考试状态共同收敛。

### Implementation Review / Fixes

1. 将 SQL/Go canonical bundle 统一为 schema v2，排除数据库行 ID 与附件存储标识，只纳入内容摘要；真实数据库与 Memory 同事实 hash 对比通过。
2. 为内容、评分子表、review、附件绑定和考试来源列补齐直接 SQL UPDATE/DELETE 防线；published 指针只能由合法状态转换维护，引用附件不能删除。
3. materialize 对 exam、Bank、Item 取一致锁并稳定排序；来源需 active + published + exam_allowed，目标仅 draft/configured，整套复制与回执同事务。故障解除后允许原 key 重试，成功重放保持 ID。
4. 退回 draft 增加 revision，使旧批准失效；补充作者自审拒绝和 approve/publish 双并发竞争，数据库行锁保证每轮恰好一次成功。
5. assessment snapshot 对题库来源升级到 v2 并保存来源事实；手工题 v1 不变。评分继续读取考试内复制的冻结事实。
6. 管理端将内容区与评分区的 dirty 状态互锁，reviewing 以后锁定编辑；独立审核者登录后才显示批准，publisher 批准后发布。考试复制后明确提示继续配置 AnswerArea 并执行开考检查。

### 运行命令与结果

真实数据库使用独立 Docker PostgreSQL 18、loopback 55439，E2E helper 为每次运行创建并清理唯一临时数据库，执行全部历史迁移至 000141；没有把 skip 计为通过。

| 命令（相应 workspace） | 结果 |
| --- | --- |
| `go test ./internal/server -run '^TestE2EPostgresQuestionBankPublication$' -count=1 -v`（设 EDUGRADE_E2E_DATABASE_URL） | 通过；独立审批/退回失效、approve/publish 并发、直接 SQL 不可变、附件保留、故障全回滚与原 key 恢复、跨范围/ready 拒绝、模板/题库变化不影响考试事实 |
| `go test ./internal/server -run '^TestE2EPostgresQuestionBank(Publication)?$' -count=1 -v`（同上） | 069A 与 069B 两套真实 PostgreSQL 回归通过 |
| `go test ./internal/questionbank ./internal/paper ./internal/assessment ./internal/files ./internal/auth ./internal/server -skip 'TestE2EPostgres\|TestPostgres' -count=1`、相同包 `go vet` | 通过 |
| `npm run generate:sdk`、`npm run generate:route-coverage`、`npm run sync:schema-version`、`npm run ci:contracts` | 通过；459 条登记路由（186 OpenAPI，273 个既有 reviewed gaps），SDK 与 000141 同步 |
| `npm run typecheck`、`npm --workspace @edugrade/web-admin run build`、`npm run lint`、`npm run check:user-facing-copy`、`git diff --check` | 通过；构建仅有既有 chunk 提示，全库 lint 为 40 条既有 warning、0 error |
| Playwright CLI，桌面与 390×844 | 明确 UI-only Mock：编辑完整评分 bundle、提交审核、切换独立审核者、批准、发布、指定版本复制到考试；考试内题干、标准答案和 5 分 Rubric 可见，全部写命令有非空幂等键，移动端无横向溢出，控制台 0 error/0 warning |

浏览器截图位于本地 `output/playwright/069b-exam-copy.png` 与 `069b-question-bank-mobile.png`。fixture 是显式 UI 协议模拟，不代表真实学校现场或跨服务验证。

### Approval 与剩余边界

审批结论：**批准 STORY-069B 答案/Rubric、审核发布与指定版本进入考试切片**。审批人：Codex（按仓库规则自审自批），日期：2026-09-13。

| 验收项 | 最终证据 | 结论 |
| --- | --- | --- |
| 1：published 全 bundle 不可变 | 服务状态锁、子表/附件/来源数据库触发器、模板复制、直接 SQL 破坏测试 | 通过 |
| 2：审批绑定与自审 | revision/hash review 记录、退回修订、旧审批拒绝、默认自审拒绝、并发 approve/publish | 通过 |
| 3：materialize 原子/幂等/范围/状态 | 故障注入零残留、原 key 恢复与重启重放同 ID、跨范围 not found、ready locked | 通过 |
| 4：题库变化不改考试 | 发布模板 v2/题目 v2、退役 Item 后考试完整 JSON 与 snapshot hash 不变 | 通过 |
| 5：来源与冻结职责 | 不可变 source item/version/bundle hash、题库 snapshot v2、评分读取考试 AnswerKey/Rubric | 通过 |
| 6：既有 ready 门禁 | materialize 不复制 AnswerArea，ValidateConfig 明确 invalid，Assessment 默认配置与 AI 风险链未旁路 | 通过 |

- 本地独立数据库与 UI fixture 不代表生产升级、学校现场体验或容量已验收。
- 069B 仅提供最小用户级 Reviewer/Publisher 绑定；组授权、完整 ACL 与组合搜索归 [069C](STORY-069C-metadata-acl-search.md)。
- 父 [069](STORY-069-versioned-question-bank-rubric-library.md) 仍为 In Progress。下一切片为 069C。
