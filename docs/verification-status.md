# EduGrade 当前能力与验证状态

更新日期：2026-08-14
适用范围：当前工作树（含尚未提交的 A01–A26 实现）；不是对 GitHub `main` 或学校生产环境的声明。

本文件只记录代码、自动验证和明确的外部边界。它不把 Mock、协议模拟器、静态检查或本机构建写成真实设备、真实模型质量或学校现场验收。

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

## 最近一次本机验证快照

2026-08-13 在 Windows 工作区执行的定向验证：

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

- 新增“已实现”必须同时列出代码接线、验证入口和未覆盖边界。
- Mock、stub、合成数据和外部模型协议模拟器必须显式标识，不得写成真实模型或现场结果。
- 发布到 GitHub、合并到 `main` 或远端 CI 通过是独立事实；本工作树状态不会自动同步到其中任何一个。
