# STORY-069C：受控 Metadata、ACL 与搜索

状态：Approved（仅本切片）。优先级：P0。父 Story：[069](STORY-069-versioned-question-bank-rubric-library.md)。依赖：069A/B。

## 问题与范围

自由 tags 和仅租户级权限无法支撑学校保密题库与稳定组卷。提供 bank 级受控字段、内容授权和一致搜索。

- 系统字段复用学科、学段、年级、question_type、archetype、knowledge_points，增加 cognitive_level、difficulty_band、suggested_time、source_year、copyright、language、intended_use；不能另建与 Assessment 相冲突的题型枚举。
- 每个 Bank 绑定 metadata_schema_version；自定义字段声明 enum/string/number/boolean/taxonomy、required、范围与选项。string 有长度约束，知识点使用受控 taxonomy ID，展示标签不当稳定身份。
- ItemVersion 固定 schema 与 values，修改组卷用 metadata 产生新 draft。schema/taxonomy 更新创建新定义，旧发布内容按原定义读取；不可删除使旧值失去解释的历史选项。
- ACL 动作为 read/create/edit/review/publish/retire/statistics/manage。Viewer、Author、Reviewer、Publisher、Manager 为题库授权预设，独立于可登录后端角色。
- Author 只能修改获授权 draft；Reviewer/Publisher 需独立动作，Manager 默认管理结构/授权，不自动获得内容 read。用户/组绑定复用现有组织身份，组 membership 从服务端解析；不得预设教师都能访问同学科题库。
- 搜索按授权 Bank 先过滤，再按题干、学科、知识点、题型、版本状态、metadata、版权、用途、统计可用性组合；稳定分页/排序，默认显示当前发布版与本人可见 draft 的明确模式。
- 权限撤销后列表缓存、重复提示、附件签名链接、统计与历史均重新按当前范围验证，开放临时下载 URL 必须有受控有效期。

## 数据与 API 规划

新增 question_bank_metadata_schema、question_bank_item_metadata（绑定 item_version，不放可变 Item）、受控 taxonomy 定义；扩展既有最小 ACL。拟定 `GET|PUT /api/v1/question-banks/{bankId}/metadata-schema|acl`（路径分别登记）、组合 items query 与 schema validation 响应；值、定义和数据行均有租户复合约束。

## 非范围

不开发任意脚本字段或无限自由文本、不强制部署搜索引擎、不跨租户共享，不通过 Manager 单一角色获得所有内容，不修改旧发布版本 metadata。

## 预计修改文件

扩展 `internal/questionbank/` schema/ACL/search 与相应迁移、auth 范围解析、附件读授权；管理端题库授权/schema 配置/组合筛选，OpenAPI/SDK 与 API 文档。先用 PostgreSQL 查询和索引，性能证据需要时再引入额外搜索设施。

## 测试方式

Go 字段类型、schema 兼容、权限预设测试；真实 PostgreSQL 验证同租户 bank 隔离、用户/组变更、统计总数与分页、schema 版本保留、发布锁。Web/SDK 门禁及有限浏览器验证筛选和直达授权；性能验证用标明规模的授权合成库，不称学校容量已验证。

## 验收标准

1. enum/number/taxonomy 类型与 required 校验覆盖 API 和发布；非法值明确报字段级错误。
2. 旧版 schema 更新后旧发布题目仍可解释且 hash 不变；新字段值编辑只能进入新 draft。
3. 同租户 A/B 题库隔离，未授权题干、列表总数、查重和附件不泄露；平台或 Manager 身份不能隐式获得内容访问。
4. 搜索只返回合法范围，筛选结果与分页总数一致，稳定排序不漏/重项；权限撤销后失去内容读取。
5. 审核/发布/退役/结构管理分别验证动作，后端直接请求不能绕过 UI；全部变更有审计。
6. 受控字段为 070 提供确定的过滤与约束值，教师标签难度与观测统计字段明确分开。

## 规划审阅与实施记录

### Plan / Plan Review

规划已区分系统与自定义字段、定义与值的版本、结构管理与内容权。Plan Review 将“未授权缓存与附件”列为服务端验收项，并确认组合查询必须先套授权谓词，再在同一一致性快照中执行 count/page；069F 未完成前，statistics_available 只能明确返回 false，不能用教师难度标签替代。

### Implementation

- `000142_question_bank_metadata_acl_search.sql` 增加不可变 schema、版本 metadata、Bank 组/membership/组 ACL、group-aware 动作函数、RLS、索引和 retire/statistics 权限；部署 schema 版本同步到 000142。
- questionbank Store 增加 schema 版本读写/校验、完整 ACL、组合检索和退役。字段错误保留精确路径；旧 Version 绑定旧 schema/hash，新派生草稿绑定当前 schema；受控 knowledge_points 使用稳定 term ID。
- ACL 预设覆盖 Viewer/Author/Reviewer/Publisher/Manager。Manager 只看 Bank 结构；内容读取、审核、发布、退役、统计分别校验动作。用户/组撤权在每次请求重新解析，files 服务对题库附件复查当前 read。
- PostgreSQL 搜索在内容筛选前应用授权谓词，以 repeatable-read 保证 total/page 一致，并增加确定排序。默认返回当前发布版和调用者草稿；statistics 可用性与难度标签分开。
- OpenAPI、生成 SDK、路由覆盖、角色矩阵和管理端同步增加 schema、ACL、组合筛选、受控字段与退役入口。管理端提供动态 enum/number/boolean/taxonomy 控件，知识点在配置 taxonomy 后改为受控多选。

### Implementation Review / Fixes

实现审阅首先发现 PostgreSQL 数组扫描不能直接接收聚合输出，已将组动作查询改为 JSONB 聚合并由 Store 解码。既有 E2E 的新增权限数量期望从 6 同步到 8。随后补查受控知识点，修正为 schema 定义 `knowledge_points` taxonomy 时拒绝自由 ID。

浏览器审阅发现 Metadata/ACL 的 Form.List 把同一个 React key 展开到多项 Form.Item，动态增删会产生重复 key；已把 key 仅放在列表行容器，复验控制台 0 error/0 warning。390×844 页面与组合检索弹窗无横向溢出。

### 验证证据

| 命令/场景 | 结果 |
| --- | --- |
| `go test -p=1 ./internal/questionbank ./internal/files ./internal/server` | 通过；metadata 类型/范围/required/taxonomy、历史 schema/hash、预设、组撤权、搜索、退役及既有 server 回归 |
| `go test -p=1 ./internal/server -run 'TestE2EPostgresQuestionBank$\|TestE2EPostgresQuestionBankPublication$\|TestE2EPostgresQuestionBankMetadataACLSearch$' -count=1 -v` | PostgreSQL 18、含 069C 的 000142 且当前全历史至 000143，069A/B/C 三套 E2E 通过；覆盖 Manager 内容拒绝、组撤权、附件撤权、组合分页、字段错误、旧 schema/hash 和 audit |
| `npm run ci:contracts` | 通过；OpenAPI/SDK 当前，breaking/coverage/schema gate 通过（193 条 OpenAPI + 273 条 reviewed gaps） |
| `npm run typecheck`、`npm run build`、`npm run lint`、新增页面定向 lint | 通过；全库 lint 保留 40 条既有 warning，新增页面无诊断，构建仅有大 chunk 提示 |
| 本机浏览器，显式 UI-only fixture，桌面与 390×844 | 题目受控字段、退役入口、组合检索/统计可用性、schema 历史提示、用户/组 ACL 可见；移动端无横向溢出，修正后控制台 0 error/0 warning |

### Approval

审批结论：**批准 STORY-069C 受控 Metadata、完整 ACL、组合检索与退役切片**。审批人：Codex（按仓库规则自审自批），日期：2026-09-13。

批准范围是当前本地工作分支的软件实现与自动验证；未宣称已经完成预生产升级、学校教师现场或容量验收。统计数据仍归 [069F](STORY-069F-usage-psychometric-feedback.md)，当前 API 只准确表达不可用。父 [069](STORY-069-versioned-question-bank-rubric-library.md) 仍为 In Progress；下一切片为 [069D](STORY-069D-existing-exam-bank-import.md)。
