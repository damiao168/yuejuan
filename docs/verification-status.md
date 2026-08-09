# EduGrade 当前能力与验证状态

更新日期：2026-08-09
适用范围：当前 `main` 代码及本文件列出的可复现验证入口

本文件是项目当前能力、验证强度和剩余风险的事实源。README、Story、评审报告记录设计与实施过程；当它们与本文件的当前状态冲突时，以代码、自动化脚本和本文件最近一次核验结果为准。

## 状态口径

| 状态 | 含义 | 不能推导出的结论 |
| --- | --- | --- |
| 已实现且有自动验证 | 生产代码已存在，并有 CI、单元测试、集成测试或静态门禁覆盖关键不变量 | 不等于真实设备、真实数据、真实模型质量或学校现场已验收 |
| 已实现，需本地或人工验证 | 代码和可复现入口已存在，但需要 Docker、模型运行时、物理设备、受治理数据或人工检查 | 不能仅凭代码审查标记为 Production Ready |
| 尚未验证或规划中 | 只有局部实现、开发中工作树、方案或历史 Story 描述，尚无足够证据闭环 | 不能出现在对外“已交付能力”清单中 |

“自动验证”只表示仓库中存在并执行对应门禁。除非同时附有本次 GitHub Actions 运行链接或归档产物，否则不表述为“当前远端 CI 已通过”。

## 当前能力矩阵

| 能力 | 当前状态 | 证据与边界 |
| --- | --- | --- |
| 认证、RBAC、租户/学校数据范围 | 已实现且有自动验证 | Go 测试、PostgreSQL E2E 和前端角色入口检查覆盖主要边界；前端路由权限只提供用户体验，后端授权仍是安全边界 |
| 考试、试卷、题目、Rubric、答卷与文件流程 | 已实现且有自动验证 | Go 全量测试、Story 静态门禁及 PostgreSQL 工作流覆盖核心状态；不代表真实扫描现场已验收 |
| OCR、图像质量、页面处理 Worker | 已实现且有自动验证 | Python CI 包含 Ruff、依赖审计和各 Worker 测试，Compose profile 可静态解析；真实打印、扫描、复杂版面和大规模吞吐仍需现场验证 |
| 客观题规则评分、OMR 校准与人工回退 | 已实现且有自动验证 | STORY-056/060 的 Go/PostgreSQL 测试覆盖规则评分、模板差异、租约恢复和完整性阻断；当前外部样本规模不构成生产准确率证明 |
| 主观题单题建议、证据校验与人工复核边界 | 已实现且有自动验证 | API Gateway、`grading-agent`、Lab 契约和测试覆盖“建议而非最终成绩”；作文/论述题保持 `shadow_only`，置信度尚未完成生产校准 |
| 主观题批次 Worker 运行时 | 已实现且有自动验证 | Python CI 已纳入安装、Ruff、pytest、Compose profile、镜像构建和配置烟测；Worker 有健康标记、断线重登和本地 HTTP 协议烟测 |
| 主观题批次端到端运行 | 已实现且有自动验证 | 隔离 Compose 已验证批次、专用 Worker 服务账号、租约、受治理 Agent、AI 建议、审计和数据库收敛；Worker 健康检查通过。它仍不证明真实模型效果或学校现场能力 |
| 主观题批次可靠性 | 已实现且有自动验证 | 管理页按用户意图轮换幂等键，轮询避免重叠且错误可见；后端批量加载片段上下文并返回结构化部分成功结果，隔离 PostgreSQL 批次链路已通过；尚无大批量性能基线 |
| 人工阅卷、双评、仲裁、发布与申诉 | 已实现且有自动验证 | 领域测试覆盖主要状态和权限；最终成绩必须由人工流程确认，AI 没有发布权限 |
| Web 角色入口与关键页面回归 | 已实现且有自动验证 | Vitest 覆盖角色体验和路由注册关键逻辑；当前 `npm run test:e2e` 会拦截 `/api/v1/**` 并使用 `tests/e2e/fixtures/apiMocks.ts`，它只证明 UI、导航、权限展示和前端接口契约，不是跨服务生产证明 |
| 生产 Mock 页面退场 | 已实现且有自动验证 | 生产模式从注册表构造阶段排除 `productionReady: false` 页面，不能用环境开关重新启用；Vitest 和生产路由脚本覆盖该边界 |
| 不拦截 API 的浏览器跨服务黄金路径 | 已实现且有自动验证 | 隔离 Compose 启动真实 Web、反向代理、API 和 PostgreSQL；Playwright 未注册 API 路由拦截，已验证学校管理员登录、工作台和已持久化考试列表。范围尚不包含真实扫描仪或整场考试浏览器操作 |
| STORY-060 外部模型协议模拟器 | 测试夹具，自动执行 | 固定响应模拟器只存在于隔离测试边界，用于验证真实 `grading-agent` 适配器、API、PostgreSQL、审计及“AI 不写 `final_grade`”约束；它不是生产模型，也不证明真实模型的准确率、鲁棒性、校准、公平性或成本 |
| 本地真实模型适配器 | 已实现，需本地或人工验证 | 需要显式启动 llama.cpp 与受治理模型，再运行 opt-in 测试；模型文件和密钥不进入仓库，模拟器通过不能替代该验证 |
| 多厂商模型治理 | 已实现，需本地或人工验证 | Provider、Deployment、Secret 引用状态、租户策略和 fail-closed 门禁已有实现；真实外部厂商调用、数据出境审批、效果评测和灰度证据尚未完成 |
| 核心 API 契约与前端查询一致性 | 已实现且有自动验证 | OpenAPI 3.1 明确标记为 `core-pilot-partial`，覆盖核心列表和主观题部分入队语义；共享查询构造器已迁移高频列表并有前后端契约测试。它不是全量 API 描述，也尚未生成客户端或做破坏性差异门禁 |
| Windows/Tauri 客户端 | 已实现，需本地或人工验证 | 自动登录凭据使用 Windows Credential Manager，失败时不降级到 WebView 存储；远程 API 强制 HTTPS，日志在前端与 Rust 双层脱敏，CI 覆盖前端/Rust/安全配置。真实凭据库写入、离线草稿加密 SQLite、设备绑定、扫描仪与安装升级仍需专项验收 |
| 私有化部署、备份、恢复与可观测性 | 已实现，需本地或人工验证 | Compose、预检和运维脚本存在并有静态检查；恢复演练、容量基线、告警闭环和长时间运行必须在预生产环境留存证据 |
| PostgreSQL 容量保护 | 已实现且有自动验证 | 连接池上限、连接生命周期、空闲回收、语句超时和锁等待超时可配置且有安全范围校验；这只防止资源无界占用，不是容量基线或 SLO 证明 |
| 数据库 RLS、容量基线、AI Pilot 效果治理 | 尚未验证或规划中 | RLS 仍需受控应用角色和事务级 tenant context 试点；真实数据评测、教师一致性和容量报告完成前，不宣称学校生产就绪 |

## 测试证据分层

### 每次 PR / `main` 的自动门禁

以 [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) 为准：

- Web：依赖审计、TypeScript、生产构建、生产路由检查、Story 门禁、Mock Playwright 回归、Lab 测试/合成评测/开发门禁。
- Go：全量测试、并发安全 race 子集、`go vet`、`staticcheck`、`govulncheck`、PostgreSQL 工作流及 Lab/主项目边界检查。
- Python：依赖一致性与审计、Ruff、OCR/图像质量/页面处理/主观题 Worker 测试、AI 服务和评测测试。
- Desktop：Rust 格式、编译、测试、Clippy 和 PowerShell 语法检查。
- Compose / 供应链：配置解析、主观题 Worker 镜像构建与配置烟测、隔离 E2E Compose 静态检查、SBOM 和高危漏洞门禁。
- 真实系统 E2E：隔离启动 STORY-060 服务，先验证评分代理、模型协议、数据库和审计，再运行不拦截 API 的 Playwright 用例；失败时保留容器日志和浏览器产物。

这些门禁覆盖代码与协议不变量，但不会自动产生真实打印机、真实模型效果或学校现场结果。

### 显式的本地/预生产验证

| 目标 | 入口 | 前置条件 |
| --- | --- | --- |
| 主观题 Worker 静态与协议门禁 | `npm.cmd run check:story063` | Node.js 与仓库依赖 |
| 主观题 Worker 单测 | `python -m pytest -q services/subjective-grading-worker/tests` | 先安装对应 editable package，或仅开发时设置该服务目录到 `PYTHONPATH` |
| 主观题批次真实服务闭环 | `infra/docker-compose/scripts/story063-subjective-smoke-test.ps1` | Compose 全栈、迁移、测试账号、真实答题片段、Worker 与 Agent 就绪 |
| 当前 Mock 浏览器回归 | `npm.cmd run test:e2e` | Chromium；接口由 fixture 拦截 |
| 真实系统浏览器回归 | `playwright.real.config.ts` 与 CI `real-system-e2e` job | 隔离 STORY-060 Compose；使用外部模型协议模拟器，不使用真实模型 |
| 本地真实模型适配器 | README 中的 `TestHTTPAdapterRealLocalAgent` opt-in 命令 | llama.cpp、受治理模型、Agent 和本地服务令牌 |
| 部署/恢复验收 | `infra/docker-compose/scripts` 与 `docs/deployment/preproduction-runbook.md` | 独立预生产环境、备份介质、监控和验收责任人 |

## 最近一次本机核验快照

2026-08-09 在 Windows 工作区执行：

- A1：`check:story063`、Ruff、主观题 Worker `5` 项测试和 Compose 配置检查通过；CI 已配置 Worker 镜像构建与配置烟测。
- A2：Docker Desktop Server `29.6.1` 下隔离 Compose 全栈一次启动成功；主观题批次、专用 Worker 服务账号、租约/执行、真实评分代理、外部模型协议模拟器、API、PostgreSQL 和审计验证通过，Worker healthy，并确认 AI 没有写入 `final_grade`。
- A2：真实 Playwright 未注册 API 路由拦截，学校管理员登录、工作台、考试列表用例 `1/1` 通过。
- A3：`go test ./internal/subjective`、批次测试重复运行、`go vet`、定向 `staticcheck`、前端 TypeScript 和生产构建通过；隔离 PostgreSQL 批次链路也已执行批量上下文加载。
- A4：Web Vitest `3` 个文件、`9` 项测试通过；TypeScript、生产构建、生产路由检查和 STORY-053 门禁通过。
- B：核心 OpenAPI 契约、公共分页约束和共享查询构造器测试通过；桌面 Vitest `5/5`、Rust `2/2`、Clippy、安全配置门禁与依赖审计通过。
- C：PostgreSQL 连接池、连接生命周期、语句/锁超时已实际应用到 pgx，配置、DB 和依赖测试通过；修复 readiness checker 覆盖池配置的问题。

以上是本机证据，不替代 GitHub Actions 远端结果。真实模型效果、真实扫描设备和学校现场仍未由本轮验证覆盖。

## 本轮整改结果

| 项目 | 结论 |
| --- | --- |
| A1 主观题 Worker 门禁 | 已完成代码与自动门禁，并在隔离 STORY-063 业务闭环中验证 Worker 运行健康 |
| A2 真实跨服务路径 | 已完成并本机通过；浏览器请求未被 Mock，外部模型仍是明确标识的协议模拟器 |
| A3 批次可靠性 | 已完成幂等、轮询、错误语义、部分成功和批量加载整改并通过定向检查；PostgreSQL 批量路径仍需集成用例 |
| A4 前端最小测试与 Mock 退场 | 已完成；Mock 页面不进入生产注册表，Mock Playwright 保留为独立 UI 回归 |
| A5 说明与证据 | 由本文件统一维护状态口径、能力矩阵、验证入口和未覆盖边界 |
| B API 与桌面安全 | 核心 OpenAPI/查询统一、Windows 系统凭据库、HTTPS 门禁和日志脱敏已完成；全量契约和加密离线存储仍待后续 |
| C 容量与治理核对 | PostgreSQL 容量保护已完成；RLS 与真实数据 AI 效果治理保留为明确的预生产工作，不以静态配置冒充完成 |

## 当前整改优先级

1. 远端门禁：合入后确认 GitHub Actions 的 Python、Web、Go、Compose、真实系统 E2E 和供应链 job 全部通过。
2. 数据库纵深防御：设计非表所有者应用角色和事务级 tenant context，先对高风险表做 RLS 试点，不直接给现有连接池套用未验证策略。
3. 预生产：用非生产受治理数据完成恢复演练、容量基线、告警闭环和持续运行验证。
4. P1/P2：扩展 OpenAPI 全量覆盖与破坏性差异门禁，拆分超大文件，迁移离线草稿到加密 SQLite，并推进真实数据 AI Pilot 治理。

## 维护规则

- 新增“已完成”声明时，必须同时写明证据入口、运行环境和未覆盖边界。
- Mock、stub、合成数据和外部模型协议模拟器必须显式标识，不能写成“真实模型效果已验证”。
- 物理扫描、真实考试数据、第三方厂商和容量结论必须附环境、数据版本、模型/提示词版本、日期和结果归档位置。
- Story 的 `Approved` 表示该 Story 当时的验收结论，不自动升级为当前版本 Production Ready。
- 一次验证失败时保留失败证据并更新本文件，不通过删除测试、放宽门禁或改写状态绕过。
