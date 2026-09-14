# EduGrade 当前能力与验证状态

- 状态更新时间：2026-09-14（补充本地 MATH-14；原 main 审阅快照保留）
- 审阅基线：`56cf04466b163b1194400340165f338969fe0e99`（本轮修改前核验的 GitHub `main` HEAD，用于界定审阅起点，不是“本文档所在提交”）
- 适用范围：原 GitHub `main` 审阅快照，以及下节明确标记的当前工作分支本地 STORY-069A/069B/069C/069D 实现与验证证据。

本文件只记录代码、自动验证和明确的外部边界。它不把 Mock、协议模拟器、静态检查或本机构建写成真实设备、真实模型质量或学校现场验收。

本文件不代表学校生产环境验收、真实教师现场效果、真实模型准确率、真实扫描设备兼容性或外部 Provider SLA。

## 证据分层

### 2026-09-14 本地数学阅卷工作台证据闭环 MATH-14

当前本地工作树已把 MATH-11～13 的 verified artifact、服务端 Rubric 决策、建议区间、step/bbox/SymPy 验证和版本化建议历史接入教师阅卷工作台。当前建议必须精确匹配 segment、artifact/version、effective correction lineage、scorer、Rubric 和服务端总分；校正未保存、等待重验、待确认分、crop 漂移或任一版本不一致都会使旧建议不可采纳。采纳前再次读取服务器证据，模型建议只能由具备 `grading:manage` 的管理端显式请求；R3 默认保持教师独立评分。

Go 全包 test/vet、工作台 49 项 Vitest、全工作区 TypeScript、web-admin production build、OpenAPI/SDK/route/contract/schema 门禁通过。隔离 PostgreSQL 17 应用当前全历史迁移至 schema `000150`，生产绑定 E2E 验证建议历史及错误 version/segment、缺 artifact、不完整绑定拒绝。Playwright CLI 的 UI-only 合成夹具检查当前/未决/crop drift/校正 pending/R3/管理端显式新建议，以及 390×844 无横向溢出；稳定态只有既有 Ant Design 弃用提示。定向 Biome 无 error、9 条 hook dependency warning；隔离数据库和临时浏览器夹具已在验收后清理。

这些证据不代表真实 VLM 准确率、真实学校答卷、教师现场、预生产升级、容量或自动最终评分获批。数学 v2 功能旗标保持关闭，MATH-15 的真实错答、替代解、风险召回 benchmark 与 promotion gate 仍未完成。详见 [MATH-14](stories/STORY-MATH-14-grading-workbench-evidence-loop.md)。

### 2026-09-14 本地 Grading Agent v2 数学生产接线 MATH-13

当前本地工作树已把 verified effective math evidence、score-free v2 模型候选和 MATH-12 服务器确定性计分接入 direct/batch/worker 主观题路径。新链路仅在默认关闭的 feature flag、数学 Assessment Snapshot、已验证数学工件三项同时满足时启用；模型不能返回或持有总分，所有建议仍固定进入教师复核。run/grade 和幂等身份绑定 artifact/version/correction/scorer，推理期间版本变化会 conflict；任何未决 rubric 点只返回 nullable total 与区间，不写成功建议。

最终 Go 全包 `go test -p 2 ./...` 与 `go vet -p 2 ./...`、Python v2 contract/application/HTTP 定向（`17 passed, 15 subtests passed`）和 AI services 全套（`125 passed, 45 subtests passed`）、定向 Ruff、全工作区 TypeScript、OpenAPI/SDK/route/schema contract gates 通过。入口覆盖 verified crop、幂等版本绑定、未决项无 grade、worker execute/complete 重算与伪造分数/非法模式拒绝；crop 变更阻断旧缓存重放，推理期间版本/crop 漂移会 conflict，worker 失效路径同时终结 task。Go 全包运行未配置数据库 E2E 环境；Docker PostgreSQL 18 先前独立隔离库全历史迁移至 `000148` 的核心工作流通过，本轮另一隔离库迁移至最新 `000149` 的绑定回读、batch 冻结恢复与精确 FK/check 四组断言通过，隔离库已清理。使用的是合成工件与 Fake/协议 HTTP 模型，不代表真实 VLM、脱敏学校答案、教师现场、容量或生产升级已验证；开关保持关闭，MATH-14 工作台闭环和 MATH-15 shadow/promotion 尚未实施。全套 Ruff 仍有其他既有测试文件的 10 条 import-sort 诊断。

详见 [MATH-13](stories/STORY-MATH-13-grading-agent-v2-production.md)。

### 2026-09-13 本地数学评分基线 MATH-12

当前本地工作树新增冻结考试 Rubric 的只读数学评分预览；不宣称合入远端 main、部署学校或获准自动评分。服务端逐点生成 CriterionDecision，以整数分值聚合 verified/unresolved 与建议区间；未解决分值不成为零分或旧 missing_points，有待确认分时建议总分为 null。候选无分值权，精确目标和已绑定的步骤/公式/验证来源才可确定判定；当前语义与扣分策略仍要求人工判断。

详情和定向单测、契约、隔离 PostgreSQL 验证范围见 [MATH-12](stories/STORY-MATH-12-deterministic-rubric-scorer.md)。本轮不写 AI/教师/最终成绩，也未完成生产模型接线、follow-through、自我纠正、工作台建议区间展示、真实答卷门禁或完整教师提交 E2E。

### 2026-09-13 本地 STORY-069A 批准时快照

当前工作分支 `fix/postgres-integration-regressions`，修改前 HEAD `5ec969389c30fd07851135e901775b88d160872b`。以下为本地工作树实现，未宣称已合入远端 main 或部署到学校。

- 私有题库核心已接线：Bank、稳定 Item、draft Content Version、显式用户动作 ACL、草稿 revision、派生历史、审计/outbox/持久命令回执和管理页面。迁移 000140 开启四表 RLS 与 tenant 复合外键。
- 真实独立 PostgreSQL 18：全历史迁移、生产 Store/Router、HTTP 认证与同租户 ACL、8 个并发/锁定/回滚/RLS/模板/重放子场景通过；已有考试→阅卷→发布→仲裁/报表恢复 PostgreSQL 回归通过。
- Go auth/idempotency/questionbank/server 定向测试、go vet、SDK 一致性、OpenAPI breaking gate、schema version、Web 类型/构建/生产导航、24 项既有 Web 授权与路由相关测试通过。全库 lint 40 条既有 warning；新增题库文件无诊断。
- Playwright CLI 的网络 fixture 明确为 UI-only Mock：创建、预览、换行选项、409 保留编辑、显式加载后保存/派生、撤权隐藏旧缓存、后台刷新保持编辑、1440px 桌面与 390px 移动布局均已检查；移动页面无横向溢出。
- 069A 批准时边界：CGO=0 且无本机 C 编译器，race 检查未成功运行；数据库并发用例已通过。未验收学校现场、容量或预生产升级。当时答案/Rubric、发布 scoring bundle、用于考试和组卷尚未实现，当前 draft hash 不代表发布 hash；后续 069B 证据见下一节。

详见 [069A 实施/审阅/审批记录](stories/STORY-069A-question-bank-core-item-version.md)与[题库 API](api/question-bank.md)。后续章节保留原有证据范围。

### 2026-09-13 本地 STORY-069B 补充

在同一工作分支上继续完成 069B；以下为未合入远端 main、未部署学校的本地工作树证据。

- 答案、解析、Rubric、附件摘要和模板来源成为完整评分 bundle；`draft → reviewing → approved → published` 审核链绑定 revision/hash，作者默认不能自审，退回修改使旧审批失效。
- 真实独立 PostgreSQL 18 执行全部历史迁移至 000141：published 主表/子表/附件/来源直接 SQL 修改拒绝，approve/publish 并发各仅一次成功，故障注入 materialize 零残留、原 key 恢复、重启重放同 ID，跨范围与 ready 考试拒绝。
- 指定 published 版本以单事务复制 Question、AnswerKey、Solution、Rubric、默认 Assessment config 和来源追溯。发布模板/题目 v2 及退役 Item 后，考试 JSON、考试内评分事实和 schema v2 snapshot hash 保持不变；缺 AnswerArea 继续被既有 ValidateConfig 判为 not_ready。
- OpenAPI/SDK/route/schema 门禁、Go 定向测试与 vet、全 workspace TypeScript、Web 构建、lint/copy/diff 检查通过。Playwright CLI 使用显式 UI-only Mock 串通评分编辑、独立审核、发布和考试复制；390px 无横向溢出，控制台 0 error/0 warning。
- 边界：本地 PostgreSQL 与协议 fixture 不代表预生产升级、容量或教师现场体验；069B 仅有最小用户级 Reviewer/Publisher 绑定。组 ACL、受控 metadata 与组合搜索归 069C。

详见 [069B 实施/审阅/审批记录](stories/STORY-069B-answer-rubric-publish-workflow.md)与[题库 API](api/question-bank.md)。

### 2026-09-13 本地 STORY-069C 补充

在同一工作分支上继续完成 069C；以下为未合入远端 main、未部署学校的本地工作树证据。

- Bank metadata schema 与 ItemVersion values 分别版本化；enum/string/number/boolean/taxonomy、required、范围、稳定 ID 与受控 knowledge_points 在写入、送审和发布时校验。schema 升级后旧 Version 继续按旧定义读取且 hash 不变。
- ACL 扩展到 read/create/edit/review/publish/retire/statistics/manage，并提供五个命名预设、用户与题库组 membership。Manager 只有结构权；用户/组撤权后，列表、内容、搜索和题库附件通用读取入口都重新校验当前 read。
- 组合搜索先应用 Bank 授权，再在 repeatable-read 内执行 total/page 与确定排序；支持题干/编号、学科、知识点、题型/archetype、工作流、难度、认知、版权、用途、custom metadata 和统计可用性筛选。069F 前统计明确为 unavailable。
- 独立 PostgreSQL 18 执行包含 069C 迁移 000142 的当前全历史迁移（至 000143），069A/B/C 三套 E2E 同时通过，覆盖字段错误、历史 schema/hash、派生新 schema、Manager 内容拒绝、组撤权、附件撤权、组合分页、退役与 audit。Go 定向测试、OpenAPI/SDK/route/schema 门禁、Web 类型和构建通过。
- 浏览器以明确 UI-only fixture 验证受控编辑、schema、ACL、组合检索、统计标记与退役入口；桌面和 390×844 响应式布局通过且无横向溢出。修正 Form.List key 后控制台 0 error/0 warning。
- 边界：本地数据库与 UI fixture 不代表预生产升级、学校现场或容量验收；版本级真实统计仍归 069F。

详见 [069C 实施/审阅/审批记录](stories/STORY-069C-metadata-acl-search.md)与[题库 API](api/question-bank.md)。

### 2026-09-13 本地 STORY-069D 补充

在同一工作分支上继续完成 069D；以下为未合入远端 main、未部署学校的本地工作树证据。

- 新 readiness 确认在考试锁与单事务内保存不含学生身份/作答/AnswerArea 的题目内容 companion，并绑定 ready 触发器生成的具体 Assessment Snapshot ID/hash。旧 hash-only readiness 保持 unavailable；导入没有 live Question/Rubric 回退。
- 单题与最多 50 题批量先预览后生成 Draft。每题固定来源、目标题库/schema/revision、mapping 和 command；批量按逐题事务返回 HTTP 207。命令回执、audit/outbox 与 Item/Version 同事务，响应中断可复用原请求，部分成功后只重试失败项。
- 服务端同时检查来源考试范围、目标题库 read/create 和关联 Item edit；回执重放复查当前授权。查重只在获授权目标库返回摘要，区分 exact bundle、同内容不同评分与相同标准化题干，教师显式选择新 Item 或关联新版本。
- 导入 provenance 记录 readiness/Assessment 身份与 hash、Profile/archetype/scoring policy、目标 schema、mapping、去重决定与 command。真实 PostgreSQL 16 E2E 串通 Draft 补齐、独立审核发布、用于新考试，并从目标版本追溯到原 snapshot；来源考试保持不变。
- 069A～069D 四套真实数据库 E2E、questionbank/paper/server Go 回归与 vet、OpenAPI/SDK/route/schema 门禁、全 workspace TypeScript/Web 构建、生产路由、相关身份兼容测试及浏览器 UI-only fixture 通过。构建仅保留既有大 chunk 提示。
- 边界：旧 readiness 无内容时需要重新确认；当前只支持和 Assessment Snapshot 一致的原考试评分事实，不提供重评分版本选择。手工历史客观题没有冻结选项字段时需教师补齐。未完成预生产升级、容量或学校教师现场验收。

详见 [069D 实施/审阅/审批记录](stories/STORY-069D-existing-exam-bank-import.md)与[题库 API](api/question-bank.md)。

### 原有证据分类

| 证据类型 | 本文件可以声明 | 本文件不能据此声明 |
| --- | --- | --- |
| Repository implementation | 代码、配置、路由、数据结构或运行入口已经存在 | 对应能力已在生产环境有效运行 |
| Automated verification | 仓库存在单元测试、集成测试、PostgreSQL E2E、Vitest、Playwright、类型检查、构建、安全检查或 CI 门禁 | 真实学校、教师、设备、模型、网络或容量已经验收 |
| External evidence | 已明确留存的学校试点、教师评审、物理设备、真实模型或预生产运行证据 | 可以用 Mock、synthetic 数据或代码存在替代外部验证 |

## 状态口径

| 状态 | 含义 | 不代表 |
| --- | --- | --- |
| 已实现且有自动验证 | 代码、路由/界面或耐久存储接线已存在，且有定向测试、类型检查、构建或契约门禁 | 已在真实学校生产运行 |
| 已实现，待外部验证 | 产品代码已闭环，但依赖设备、受治理数据、第三方模型或预生产环境 | 真实效果、容量或现场可用性已证明 |
| 外部验证边界 | 必须由人工或预生产留存证据的条件 | 可以通过放宽门禁或文档声明跳过 |

## A01–A26 实施总览

| 范围 | 当前实现 | 自动验证与边界 |
| --- | --- | --- |
| A01–A06：学科领域、工作区、SDK、专业工作台、批注 | 版本化 Assessment Snapshot、五阶段考试工作区、Feature/Router/Query 架构、按学科的阅卷上下文、键盘操作、任务预取、批注和评语模板均已接线 | Go 领域/路由测试、Web 类型检查、Vitest、生产构建与路由门禁；20 份语文/数学/物理/历史夹具的键盘浏览器流程及双窗口草稿 revision conflict 均通过；真实教师连续阅卷体验仍需现场观察 |
| A07–A13：人工质量体系 | 相似答案分组、Gold Papers、阅卷员校准、Seed、漂移检测、回标和质量总控台均有持久化模型、服务、管理入口和质量门禁接线；回标严重差异只能显式转入 A19 待审批题目复评，不改当前分或发布 | 对应 Go 包定向测试通过；样本分布、校准阈值和真实阅卷员一致性待试点数据验证 |
| A14–A17：AI 准入与评测 | Eligibility 先于外部 AI 调用并保守拒绝；模型置信度校准、分学科切片评测、难度投影、AI/人工分歧分类均已接线，并在既有“模型治理”的“评分保障”工作区实际可管理 | 对应 Go 包定向测试通过；真实外部模型、数据出境审批、有效校准曲线和公平性结论尚未验证 |
| A18–A22：发布、重评、学生端与申诉 | 不可变 Score Release、题级 Regrade、发布门禁、独立 Student Portal、发布版本锚定的题目申诉均已实现；题级重评包含分派、盲评候选、冻结 Rubric 证据、管理端逐项复核和后继版本草稿；学生可圈选本题答题图，学校端在同一申诉上下文中处理原图、冻结 Rubric 与版本事实 | Go 领域/处理器测试、OpenAPI/SDK 门禁及学生端/管理端类型构建通过；真实发布流程和学校申诉运营需预生产演练 |
| A23–A26：桌面扫描站与处理异常 | Tauri 使用 SQLite、本地受控文件 Spool、AES-GCM 信封和 Windows Credential Manager；断点续传、扫描 Profile/预检、统一 Processing State 与异常处理中心已接线；工作区可直达阻断异常，导入页可定位相应异常 | Desktop Vitest、Rust 测试/Clippy、Web 构建与 Go 定向测试通过；真实扫描仪、设备驱动及大批量断电恢复仍待验证 |

详细的逐项范围、接线、验证和外部边界见 [`docs/stories/README.md`](stories/README.md) 的 A 系列索引及各 Story 事实记录。

## 关键能力矩阵

| 能力 | 当前状态 | 证据与边界 |
| --- | --- | --- |
| 多租户考试、试卷、题目、Rubric 与冻结评分事实 | 已实现且有自动验证 | A01 Assessment、考试工作区和评分链路使用冻结快照；真实数据迁移与学校配置仍需人工核验 |
| 人工阅卷、双评、仲裁与质量控制 | 已实现且有自动验证 | 人工评分仍是最终成绩来源；AI 建议不拥有发布或最终分权限 |
| OCR、图像质量、页面处理与异常运营 | 已实现且有自动验证 | A26 只投影既有处理事实并通过异常中心处理；复杂扫描版面和吞吐未由本轮证明 |
| 多厂商模型治理与主观题 AI 建议 | 已实现，待外部验证 | 受治理的 Provider/Deployment/策略、Eligibility、评测和校准事实已存在；未接入或未批准的模型必须 fail closed |
| OpenAPI 与 Generated SDK | 已实现且有自动验证 | `npm run generate:sdk`、`npm run check:openapi-breaking` 和 SDK 类型检查已通过；契约仍只覆盖已登记的核心 API，不等于全仓 API 已覆盖 |
| Web 管理端生产路由 | 已实现且有自动验证 | Web Vitest、类型检查、生产构建及生产路由门禁通过；Mock 浏览器回归仅证明前端交互，不冒充跨服务生产证明 |
| 学生端成绩与题目申诉 | 已实现且有自动验证 | 独立 Student DTO 仅读取已发布版本；真实学生身份接入、通知与申诉处理时效需要学校侧验证 |
| Windows 扫描工作站 | 已实现，待外部验证 | Tauri SQLite/AES-GCM/系统凭据与本地 Spool 已实现；WIA 设备发现/Profile/预检可用，直接采集能力不对未经验证的设备作保证 |
| 私有化部署与恢复 | 已实现，待外部验证 | Compose 与运维入口存在；恢复演练、容量基线、告警闭环和长时间运行必须在预生产留存证据 |

## 当前自动化验证入口

审阅基线中的 `.github/workflows/ci.yml` 为 Pull Request 和 `main` push 配置了以下入口：

- Web/Node：OpenAPI SDK 生成一致性、breaking-change gate、依赖审计、Biome lint、工作区 typecheck、Web Admin Vitest、工作区构建、生产路由检查、Desktop 前端测试、Story checks、基于 API mock 的 Playwright 流程以及 Lab 测试/开发门禁。
- Go：模块校验、格式检查、全包测试、关键并发原语 race test、`go vet`、staticcheck、govulncheck、PostgreSQL 工作流 E2E 与 Lab/main 边界检查。
- Python：依赖一致性与审计、Ruff，以及 OCR、图像质量、页面处理、主观题评分、数学验证和 MathBench 的测试入口。
- Desktop Rust：格式、编译、测试、Clippy 和恢复脚本语法检查。

这些是 CI 配置事实，不是对某次 GitHub Actions 运行结果的推测；具体成功或失败必须以对应 Actions 记录为准。

Student Portal 在审阅基线中已经有 `vitest run` 测试入口和测试文件，但该基线的 CI 尚未显式执行 `npm --workspace apps/student-portal run test`，因此本文不将其写成已由 CI 强制执行。

## 历史本机验证快照

以下内容是 2026-08-13 在当时 Windows 工作区执行的定向验证，不代表当前 `main` 的全部测试状态：

- Web：`npm --workspace @edugrade/web-admin run test`（`32` 项）、`typecheck`、`build`、`check:production-routes`、`check:story053`、`check:story057`、`check:story061`、`check:story063` 通过；Exam Workspace 在 `1366×768`、`1440×900`、`1920×1080` 的 Playwright 视觉回归与无横向溢出检查通过；构建仅有包体积提示。
- 阅卷 E2E：20 份跨学科夹具以键盘完成领取、草稿保存、刷新恢复、提交并切换下一份；双浏览器窗口草稿 revision conflict 均通过。均为状态化 API mock 浏览器回归，不冒充真实学校数据链路。
- OpenAPI/SDK：`npm run generate:sdk`、`npm run check:openapi-breaking`、`npm --workspace @edugrade/sdk run typecheck` 通过。
- Go：A01–A26 相关领域包、OpenAPI 契约和服务器路由的定向 `go test` 通过；未把一次受本机时限中断的全量命令写成成功。
- Student Portal：`npm --workspace @edugrade/student-portal run typecheck` 与 `build` 通过。
- Desktop：上传续传定向 Vitest、TypeScript、生产构建、`cargo test --lib`（`10` 项）与 `cargo clippy -- -D warnings` 通过；500 页 fixture 的第 173 页中断、断网/5xx、重启续传与幂等完成均有 Rust、桌面客户端及服务端定向测试。
- 基础检查：`git diff --check` 通过。

本机未配置 `EDUGRADE_E2E_DATABASE_URL`，因此依赖真实 PostgreSQL 的 E2E 在本机为跳过状态；Docker Desktop 当前未运行，不能把 Compose/真实系统 E2E 记为本机通过。远端 CI 的结果须以对应 Actions 运行记录为准。

## 必须保留的外部验证边界

1. **物理扫描与设备**：真实 WIA/TWAIN 设备、纸张、双面进纸、TIFF/PDF、样张质量和安装升级需要实际设备验收。
2. **离线大批量恢复**：500 页受控 fixture 已覆盖第 173 页中断、重启、断网/5xx 与确认 offset 续传；真实扫描仪、断电、磁盘压力和现场网络仍需要演练，自动化 fixture 不能替代该演练。
3. **第三方/本地模型效果**：真实 Provider 调用、提示词版本、数据审批、分学科切片、风险覆盖、校准、公平性和成本均需受治理评测证据。
4. **学校现场**：账号接入、考试流程、教师校准、成绩发布、申诉时效、备份恢复、容量与告警必须由预生产/学校现场的责任人确认。

## 维护规则

### 数学答卷理解专项

MATH-00～08 的软件底座已接线：版本化数学工件、现有 OCR Worker 的公式路由、受限 AST、空间关系图、内网符号验证、Solution DAG、现有 Rubric Point 数学证据、教师校正工作台及追加式分层 Pilot Gate。公式模型调用范围严格限制为数学、物理、化学；语文、历史、政治、地理等文科和当前未校准的生物不调用公式模型。当前只有合成 smoke fixture，没有真实 HMER/结构模型准确率、分学科门禁或学校试点证据；任何门禁结果最多允许 `teacher_suggestion_only`，不得开启自动最终评分。逐项边界见 [`docs/stories/README.md`](stories/README.md) 的 MATH 专项索引。

2026-08-14 定向验证：数学领域/试点门禁/现有 Rubric 与服务器路由 Go 测试通过；OCR 路由、受限 Parser、符号验证和 MathBench 共 15 项 Python 测试通过；Python Ruff、Web TypeScript、OpenAPI 生成 SDK 与 SDK 类型检查通过。真实 PP-FormulaNet/UniMERNet、真实脱敏答卷与教师现场操作未验证。

2026-08-16 定向验证与入库：Go（mathunderstanding/apicontract/paper/server）、Python（OCR 路由、符号验证、MathBench 契约）、Web 类型检查、MathEvidenceInspector 定向 Vitest、`generate:sdk`（无新增 diff）、`check:openapi-breaking`、SDK 类型检查、Ruff 与 `git diff --check` 全部通过；MATH-00～08 全部改动已随 `9ea52c8` 提交。MathBench 由单条 smoke 扩充为 55 个确定性合成样本、25 个类别全覆盖（`generate_synthetic_fixtures.py` 可幂等再生成），产出 synthetic-v2 基线报告（`reports/synthetic-v2.json`，`dataset: synthetic`）。该组数字仅为 harness 自校验基线，证明各指标路径有区分度，不代表任何真实模型准确率；真实 MathBench 仍需脱敏答卷与真实模型运行。

2026-08-29 MATH-09 定向验证：现有 deterministic `BuildSpatialRelations` / `BuildSolutionGraph` 已接入 math-understanding runtime，Worker 空 relations 会在入库前补全，SolutionGraph 会由服务器 canonical builder 重建并保留 Worker 的人工复核信号。这不代表 learned layout model 已实现，也不代表复杂手写阅读顺序准确率或真实学校数据已完成验证。

2026-08-29 MATH-10 定向验证：`image-quality-worker` 已接入 deterministic skew、page-border、perspective 与 shadow geometry measurement，并仅对满足安全条件的小角度 skew 执行扩大画布、可追踪矩阵的 deskew。未自动 perspective rectify、shadow removal 或 page crop，未实现 Answer Perception，也未验证真实学校图像准确率。

2026-09-13 MATH-08A 有效证据投影：新增统一 current artifact + latest correction resolver，GET 保留 raw artifact，工作台全部数学草稿从 effective contract 加载，并提交 artifact version + correction revision 防止并发覆盖。Go mathunderstanding / apicontract、9 个前端测试、Web / SDK 类型检查、定向 lint、OpenAPI 兼容性和生成契约检查通过。真实 PostgreSQL 最小测试表回归验证生产查询与行锁（含 revision 503 / 500 条历史上限），独立临时库已清理；Chromium stateful mock 检查确认重复校正与刷新保留新公式和步骤，不提交 human grade。不是全量迁移/RLS/E2E/真实学校数学模型验证；Windows CGO 未启用，race detector 未运行。自动重新验证、建议版本失效以及后续 Mixed Perception / Grading Agent v2 仍未接入。编号使用 08A 以保留原 08 Pilot Gate 历史。

2026-09-13 MATH-09A Mixed Perception：学生答题区新增 `mixed` 路由，同图并行执行全文 OCR、共享 Paddle 公式布局检测和 FormulaNet ROI 批识别；OCR/公式重叠区域去重，像素 bbox 统一归一化，FormulaArtifact 保存候选与所选索引。`000145` 只升级 queued 数学任务，运行中与历史证据保持不可变。OCR Worker mixed/math/paper-formula/calibration 定向测试及 Go mathunderstanding/apicontract 测试通过；真实模型、脱敏手写答卷、字符级 overlap 精度和学校 benchmark 尚未验证，因此仍仅供人工复核/教师建议链路。

2026-09-13 MATH-10A SolutionGraph v2：服务器 canonical builder 增加同一视觉行的 mixed block 分组，step 保存 bbox、类型、识别/结构分层置信度，graph overall confidence 改为关键 step 最小值；同高远距离 block 保持平行流，block 关系投影后不产生 step 自环。Go mathunderstanding/apicontract、OpenAPI breaking、generated contracts、SDK/Web TypeScript 通过；真实多栏、涂改、分类讨论和 learned layout benchmark 尚未验证。

2026-09-13 MATH-11 Symbolic Verification：新增 `math-verification` 第二阶段任务和不可变 verified 派生工件，绑定 parent/version/hash/exact correction revision；recognition/correction 数据库触发器原子排队，验证证据激活与任务完成原子提交，无效租约回滚，新 crop/teacher revision 使旧任务 superseded，不同结果重试冲突。OCR Worker 89 tests + 5 subtests、SymPy 11 tests、Go mathunderstanding/workerruntime/apicontract 通过；真实 PostgreSQL 隔离库验证 projection、invalid lease、successful activation、correction supersession，并实际执行 `000146` 后回滚部署库事务。OpenAPI breaking、generated SDK/route coverage 和 SDK/Web TypeScript 通过。定义域保留约分前分母排除点及根式限制；显式复杂约束、多变量、substitution/unit、真实手写 benchmark 和完整 segment→teacher-submit E2E 仍待验证；解集计算不等于学生答案完整正确，不写 final_grade。

2026-09-14 MATH-13 Grading Agent v2：数学 verified effective artifact 经最小化、无 AST/debug 的 evidence DTO 进入生产 `/grading/grade-v2`；模型结构化响应只有 criterion/evidence semantic candidates，Gateway 以 frozen rubric 和 canonical verification facts 重算建议分。未决、低关键质量、diagram、graph uncertainty、替代解或版本漂移全部失败关闭到人工。run/grade/幂等绑定 artifact/version/effective correction lineage/scorer；OpenAPI/SDK 正式覆盖 public subjective grade 和新字段，schema version 为 `000148`。本地 Go/Python/契约/Ruff 与隔离 PostgreSQL 全迁移核心 E2E 通过，但 feature flag 默认关闭，未做真实模型 shadow、学校现场、MATH-14 工作台或 MATH-15 promotion，不授予自动最终评分权。

2026-09-14 MATH-14 数学阅卷工作台：当前数学证据、MATH-12 服务端评分、评分点×步骤、bbox 高亮和最近 20 条版本建议统一接入阅卷上下文；crop/artifact/correction/scorer/Rubric/分数任一漂移都会使旧建议不可采纳，待确认分显示区间而非零分。校正排队状态显式，管理端只在点击后请求新建议，R3 默认教师独立评分。Go 全包、工作台 49 项测试、TypeScript/build、契约门禁、隔离 PostgreSQL 绑定 E2E 和 UI-only Playwright 边界态/移动布局通过；feature flag、真实模型/答卷/教师现场和 MATH-15 promotion 仍未验证。

- 新增“已实现”必须同时列出代码接线、验证入口和未覆盖边界。
- 必须区分 Repository implementation、Automated verification 和 External evidence，不得用前两类替代外部验收。
- 历史本机快照必须保留日期和当时边界，不得改写成当前 CI 状态。
- Mock、stub、synthetic 数据和外部模型协议模拟器必须显式标识，不得写成真实模型或现场结果。
- 如填写 commit，只能作为明确核验过的审阅/验证基线；不得声称它是随后修改文档所在的提交。
- 远端 CI 是否通过必须以对应 Actions 运行记录为准，不根据 workflow 存在或本机结果推断。
