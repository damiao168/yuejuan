# EduGrade Enterprise 生产化差距清单与上线路线图

本文档记录截至 2026-07-10 的生产化状态，并在 STORY-051 完成及 STORY-001～STORY-051 已完成范围系统性优化后重新校准生产可用边界。`lab/` 目录属于智能体训练实验约束，尚未接入生产链路，不纳入本路线图的上线能力评估；除非后续有明确 Story 将其产品化，否则不能把 `lab/` 能力写入生产验收材料。

本版路线图不只列缺口，而是为每个关键 Story 给出：成熟产品参考、开源技术候选、推荐技术路线、选择理由、验收门槛。外部资料主要参考了 Gradescope、RM Assessor、STACK 等成熟阅卷/评测产品，以及 PaddleOCR、Surya、Temporal、River、vLLM、Qdrant、Keycloak、OpenTelemetry 等开源项目。公开榜单和开源项目只能用于候选筛选，最终上线选择必须以本项目真实试卷样本集、隐私要求、私有化部署约束和人工复核流程为准。

## 当前结论

EduGrade Enterprise 已具备企业级智能阅卷系统的主体工程骨架、核心 API 边界、Web 工作流页面、私有化部署雏形和企业验收清单，但尚不能按“生产级智能阅卷应用”交付正式上线。

当前可支持受控演示、集成联调和内部试点准备。OCR Worker、图像质量 Worker 和 Agent Worker Runtime 已有真实实现与自动化测试，但仍缺少本项目授权试卷样本的正式效果报告、本轮新迁移的真实 PostgreSQL 容器演练以及完整容器链路验收；主观题 AI、扫描仪、部分 Web 页面和完整离线同步仍是接口边界、占位或待联调能力。对外不能把“代码和单元测试已完成”等同于“真实 OCR 效果已达标”或“系统已可正式生产上线”。所有演示与评测只能使用 synthetic 或已授权脱敏数据。

2026-07-10 已完成范围优化补充：API 层已加强 Worker 读写权限分离、平台租户管理边界、文件 owner/submission 关系校验、试卷/答卷文件关联、成绩状态机防回退和生产配置校验；Worker 已加强陈旧租约、重复完成、认证过期、引擎异常、损坏/超大图片处理；Web 已移除生产可见的硬编码租户和无效工具控件，并完成页面级懒加载。详细证据、未执行项和剩余风险见 [`docs/reviews/completed-system-optimization-report.md`](../reviews/completed-system-optimization-report.md)。本次优化没有授权或实现 STORY-052 及之后的任何能力，路线图中的后续 Story 仅是既有规划，不代表已开始、已完成或已获准实施。

生产目标应定义为“人机协同智能阅卷系统”，不是“完全自动替代阅卷员”。AI 与 OCR 提供建议、证据、分组、风险提示和效率提升；最终成绩必须经过规则门禁、人工复核、双评仲裁或发布审批确认。

## 成熟产品参考模型

成熟智能阅卷产品的共同点不是单一模型能力，而是质量控制流程。

| 参考对象 | 公开能力特征 | 对 EduGrade 的启发 |
| --- | --- | --- |
| Gradescope | 固定模板 PDF、Answer Groups、AI-assisted grading、Rubric 批量应用、Bubble Sheet、Autograder | 先做“相似答案分组 + Rubric 一次评分多份应用 + 人工确认”，比直接让 AI 给最终分更适合生产；STORY-054 应补答案分组与阅卷一致性能力 |
| RM Assessor | 高风险考试电子阅卷、扫描手写答卷、质量控制、安全阅卷、跨材料类型评分 | 正式上线必须有分配、锁定、抽检、种子卷、异常件、仲裁、审计和阅卷员资格控制；STORY-052 与 STORY-054 要覆盖质量运营 |
| STACK | 面向数学和 STEM 的开源自动评测，支持代数表达式等结构化答案 | 客观题、数值题、表达式题优先走确定性规则和专用评测引擎；LLM 只做解释、建议和异常提示 |

本项目的产品形态应是：扫描/上传 -> 图像预处理 -> 版面和答题区域识别 -> OCR/公式/结构化答案提取 -> 客观题规则判分和主观题 AI 建议 -> 证据校验和质量门禁 -> 人工阅卷/双评/仲裁 -> 成绩发布 -> 申诉/报告/审计。

## 当前项目资产与约束

已具备资产：

- Go API Gateway、PostgreSQL、Redis、MinIO、Qdrant、Docker Compose、Web Admin、Tauri 桌面端骨架。
- 考试、试卷、答卷、上传、OCR task、answer segment、AI grade、evidence check、review task、double mark、arbitration、final grade、appeal、report、audit 等 API 边界。
- STORY-040 的 AI evaluation 框架，STORY-041 的 E2E 思路和验收清单，STORY-046 的管理员 bootstrap 与 HttpOnly cookie 会话。
- `docs/deployment/enterprise-acceptance-checklist.md` 已作为企业验收总清单。

关键约束：

- 面向学校、考试机构或企业培训的私有化部署，不能默认依赖外部云 OCR 或云大模型。
- 中文试卷、手写答案、公式、表格、扫描质量差异会直接决定 OCR 和 AI 方案，必须有本项目样本集评测。
- 学生答案、成绩、身份信息属于敏感数据，日志、模型输入、评测材料必须脱敏。
- Web、API、文档不得把 mock、placeholder、stub、not implemented 说成生产能力。
- `lab/` 当前只算训练实验约束，不影响生产上线路线，也不能作为生产闭环证据。

## 目标生产架构

目标架构按“可靠工作流 + 可验证 AI + 人工最终确认”设计：

```text
扫描仪/文件上传
  -> MinIO 原始件存储
  -> 图像预处理 worker
  -> 模板校准与答题区域切分
  -> OCR/Layout worker
  -> answer_segment 归属
  -> 客观题规则判分 / 主观题 AI 建议
  -> 证据校验、置信度门禁、异常检测
  -> 人工阅卷、双评、仲裁、抽检
  -> final_grade 与发布审批
  -> 申诉、报告、审计、导出
```

服务边界：

- API Gateway 继续使用 Go，承载认证、RBAC、多租户隔离、核心业务 API、审计和任务控制面。
- AI/OCR 服务使用 Python FastAPI 或 worker 进程承载，便于接 PaddleOCR、Surya、vLLM、评测脚本和图像处理库。
- 后台任务优先使用 PostgreSQL 事务一致的 River 队列；当流程跨服务、长时间运行和补偿逻辑显著增加时，再引入 Temporal。
- 对象存储继续使用 MinIO，所有原始件、裁图片、OCR 中间结果、评测证据都必须有 tenant、exam、submission、engine_version 和 hash。
- Qdrant 只用于语义检索、相似答案聚类和 Rubric/evidence 辅助，不允许绕过 Rubric 或人工确认。

## 技术路线决策原则

- 先闭环真实可用能力，再优化模型效果。没有真实 worker、回写、失败隔离和人工复核，模型分数再高也不能上线。
- 公开榜单只作为候选入口。OCRBench、OmniDocBench 等能帮助筛选 OCR/文档解析方向，但必须用本项目试卷、扫描仪、科目、题型做验收集。
- 确定性优先。选择题、填空题、数值题、公式题尽量用规则、答案规范化、数学表达式引擎或专用评测，LLM 作为解释和异常提示。
- AI 不写最终成绩。AI 输出必须包含证据、置信度、风险标记、模型版本和 prompt 版本，并进入人工复核或质量门禁。
- 生产链路必须可重放。OCR、AI、worker、人工操作都要记录输入 hash、版本、耗时、状态、错误和审计事件。
- 私有化部署优先。默认方案必须支持内网部署、离线升级、数据不出域；云服务只能作为可选适配器。

## Story 技术路线矩阵

| Story | 缺口 | 成熟产品或开源参考 | 推荐技术路线 | 关闭门槛 |
| --- | --- | --- | --- | --- |
| STORY-046 | 生产管理员 bootstrap 与会话安全 | OWASP ASVS、Keycloak 账号生命周期实践 | 已完成首个管理员 bootstrap、HttpOnly cookie、Web localStorage token 退场；后续并入 STORY-056 做完整账号生命周期 | 保持 Done；后续回归必须证明无默认活跃账号、无固定密码、cookie 安全属性可配置 |
| STORY-047 | 数据库级多租户硬化 | PostgreSQL 外键约束、Row-Level Security | 第一阶段 `(tenant_id, id)` 唯一约束和复合外键已完成；第二阶段对最高风险表评估 RLS | Done；后续继续补读路径权限回归和 RLS 纵深防护评估 |
| STORY-048 | Web mock 页面退场 | Gradescope/RM Assessor 的生产工作台边界 | 已完成：Dashboard 切真实 API；暂未生产化页面用 feature flag 从生产菜单隐藏；静态检查禁止生产路由暴露未收敛 mock 页面 | Done；后续 Playwright 全流程覆盖并入 STORY-057 |
| STORY-049 | 真实 OCR Worker | PaddleOCR、Surya、docTR、Tesseract、OCRBench、OmniDocBench、OpenCV | PaddleOCR 作为首选生产候选；Surya/docTR 做对比评测；Tesseract 只作轻量 fallback；OpenCV 做预处理；以本项目试卷集评估决定上线版本 | OCR worker 可消费任务、回写 text/bbox/confidence/engine_version/model_version/config_hash；低置信进人工复核；有 CER/WER、bbox、吞吐、失败率报告 |
| STORY-050 | 答卷图像质量检测与页面标准化 | OpenCV、NumPy、Pillow、pypdfium2、PaddleOCR orientation 候选 | 用独立 Python image-quality worker 检测模糊、曝光、倾斜、缺边、空白页和透视风险；新增不可变 quality run、claim + lease、标准化 RGB PNG 资产上传和 transform 记录；主观题模型适配器后移 | 低质量页不能静默进入 OCR；每页有 latest_quality_run_id、quality_report、quality_issues、normalized_file_asset_id；worker 错误不等于图像 failed；阈值后续由真实样本校准 |
| STORY-051 | Agent Worker Runtime | River、Temporal、Asynq、NATS JetStream | Approved；已实现 PostgreSQL runtime、语言无关 HTTP worker protocol、OCR/image-quality source adapter；本轮补强读写权限分离、陈旧租约拒绝和并发终态保护；River/Temporal 均未引入 | 已支持 lease/续租、幂等键、重试、超时恢复、死信、审计、指标、取消、人工重投；自动化测试通过，`000023` 已在 PostgreSQL 16 容器实际应用 |
| STORY-052 | 生产部署 Runbook 与预生产验收 | Docker Compose、PostgreSQL pg_dump/pg_restore、MinIO Client、PowerShell | Approved；已实现 preflight、migration checksum tracking、旧库显式 baseline、可重复 init、登录 smoke、备份 manifest/hash、隔离 PostgreSQL 恢复和 MinIO 对象恢复 | 8 个核心服务 healthy，`000001`～`000024`、登录和恢复已真实验证；TLS、正式密钥、生产数据规模、OCR/quality 业务 E2E、k6/ZAP/Trivy 仍是上线门禁 |
| STORY-053 | Windows EXE 真实采集与离线同步 | NAPS2、TWAIN/WIA/SANE/ESCL、OpenCV、SQLite WAL、Tauri updater | 桌面端首版不强行承诺所有扫描仪；优先支持文件夹/图片/PDF 导入 + NAPS2 SDK 或外部扫描流程；离线草稿用 SQLite WAL；Tauri 签名更新 | 至少 2 类真实扫描/导入路径验收；断网草稿可恢复；冲突可检测；安装包签名和更新签名可验证 |
| STORY-054 | 阅卷质量控制与答案分组 | Gradescope Answer Groups、RM Assessor quality-controlled marking | 新增相似答案分组、种子卷/校准卷、抽检、阅卷员一致性指标、异常分布告警 | 同类答案可分组批量应用 Rubric；抽检和种子卷失败可暂停阅卷员；质量指标进入发布门禁 |
| STORY-055 | 语义证据与相似答案检索 | Qdrant、pgvector、RAG evaluation | 继续使用已在栈内的 Qdrant 做相似答案、Rubric 证据和知识点检索；pgvector 作为简化部署备选；检索结果只作辅助证据 | tenant 级向量隔离；检索命中有来源和版本；不能跨租户召回；不能直接改 final_grade |
| STORY-056 | 完整用户、角色、SSO 与密码治理 | Keycloak、OIDC、OPA、OWASP ASVS | 内置账号生命周期先补齐；企业版/私有化可选接 Keycloak OIDC；复杂授权规则再评估 OPA | 用户启停、重置、密码策略、会话撤销、审计完整；OIDC 可选集成有验收；权限变更可追溯 |
| STORY-057 | 生产级测试与安全质量门禁 | Playwright、k6、ZAP、Trivy、OWASP ASVS、OWASP LLM Top 10 | 建立发布前质量门禁：E2E、跨租户、负载、安全、容器、LLM 风险用例全部自动化或半自动化留证 | 发布包必须附测试报告；P0 安全问题为零；已知风险有豁免单和过期时间 |

## OCR 技术路线细化

推荐结论：STORY-049 首选 PaddleOCR 作为生产 OCR 候选，原因是它对中文、多语言、文档解析生态和工程化使用更贴近本项目；Surya、docTR 用作评测对照；Tesseract 只作为简单印刷体和低资源环境 fallback。最终是否上线 PaddleOCR，不由榜单直接决定，而由本项目“试卷验收集”决定。

候选对比：

| 候选 | 适用点 | 风险 | 本项目定位 |
| --- | --- | --- | --- |
| PaddleOCR | 中文、英文、日文、多场景 OCR，PP-OCRv5、PP-Structure、文档解析生态 | 手写复杂公式、低质量扫描仍需实测 | 首选生产候选 |
| Surya | OCR、layout、reading order、table、LaTeX OCR，文档结构能力强 | 中文考试场景、部署资源和许可证边界需复核 | 对照模型和结构化解析候选 |
| docTR | PyTorch 文档 OCR，便于研究和替换模型 | 中文与手写能力需实测 | 评测基线候选 |
| Tesseract | 开源稳定、CPU 友好、命令行/API 简单 | 中文手写、复杂版面、公式能力弱 | fallback 和扫描工作站本地轻量能力 |
| 云 OCR | 商业成熟、上线快 | 私有数据出域、成本、合规、断网不可用 | 仅作为可选适配器，不做默认路线 |
| 多模态大模型 OCR | 对复杂图文理解强 | 成本、可控性、幻觉、批量吞吐和隐私风险 | 只做疑难件辅助，不替代 OCR 基础管线 |

STORY-049 应拆成可验收步骤：

1. 建立 OCR 验收集：至少覆盖印刷体、手写中文、数字、英文、公式、表格、涂改、低清扫描、倾斜、阴影、双面/多页。
2. 定义指标：字符错误率 CER、词错误率 WER、答题区域 bbox IoU、页级成功率、低置信召回率、平均耗时、P95 耗时、GPU/CPU 资源、失败重试率。
3. 实现预处理：OpenCV 灰度、去噪、纠偏、裁边、透视校正、DPI 标准化、空白页检测。
4. 实现 worker adapter：从 OCR task 拉取输入，从 MinIO 读取原件，回写结构化结果，记录 engine、engine_version、model_version、config_hash、input_hash。
5. 接人工复核：低置信、空文本、区域异常、公式失败、bbox 越界必须进入 review_task 或 OCR correction queue。
6. 留版本证据：任何 OCR 结果都要可追溯到模型、配置、输入文件和运行日志。

首版生产建议目标值需要按真实样本校准，建议先设为“门禁型指标”而不是营销指标：关键字段漏识别率低于业务可接受阈值；低置信样本召回要宁可偏高，不可漏掉高风险错识别；吞吐要满足学校批量扫描窗口。

## 主观题 AI 技术路线细化

路线调整：原“真实主观题模型适配器与评估门禁”后移。新的 STORY-050 先补答卷图像质量检测与页面标准化，因为公开手写答卷测试已经证明：在没有稳定原卷质量、标准化页面、按题切图和人工校正数据前，直接接主观题模型会缺少可信输入和评估基础。主观题模型适配器应放在真实答卷数字化闭环之后，再用 vLLM 提供私有化 OpenAI-compatible 推理服务，并以 Qwen、DeepSeek、GLM 等候选模型做项目样本集评估。

AI 输出必须是结构化对象，至少包含：

```json
{
  "suggested_score": 3.5,
  "max_score": 5,
  "rubric_points": [
    {
      "id": "point_1",
      "matched": true,
      "score": 2,
      "evidence": ["学生答案中的证据片段或坐标引用"]
    }
  ],
  "confidence": 0.78,
  "risk_flags": ["low_confidence", "needs_human_review"],
  "model": "qwen-candidate",
  "model_version": "version",
  "prompt_version": "rubric-v1",
  "mock": false
}
```

技术门禁：

- JSON Schema 或 Guardrails 校验输出格式、分数范围、Rubric id、证据字段和风险字段。
- 所有 prompt、model、temperature、输入 hash 和输出 hash 入库。
- 学生答案中的提示注入内容必须当作答案文本，不得改变系统评分规则。
- AI 分数不得直接写入 final_grade，只能写入 `ai_grade` 并触发证据校验和人工复核。
- 离线评测至少覆盖 MAE、RMSE、相邻分一致率、越界率、漏判采分点、不同班级/阅卷员/题型偏差、低置信召回率。
- 对作文、开放简答、价值判断强的题型，首版只能作为“建议 + 风险提示”，不得自动通过。

确定性题型路线：

- 选择题、判断题、多选题继续走规则判分。
- 填空题走标准答案、同义词、数值容差、单位归一化。
- 数学表达式和 STEM 题参考 STACK 思路，优先引入表达式解析、等价变换或 CAS 类校验，再让 LLM 解释步骤。
- LLM 负责“可能相似答案分组、采分点建议、异常提示”，不负责最终裁决。

## Agent Worker Runtime 技术路线细化

推荐结论：STORY-051 首版采用 Go + PostgreSQL 的 Agent Worker Runtime，并对 worker 暴露语言无关 HTTP 协议。River 作为 Go 侧队列内核首选方向，但不把 River 表结构或 Go 类型暴露给 Python OCR/image-quality worker。当前项目已经以 PostgreSQL 作为事实源，OCR/AI/阅卷任务需要和业务状态、审计、租户隔离同事务落库；Temporal 很强，但引入服务、概念和运维成本更高，适合后续长流程、多服务补偿、复杂暂停恢复。

候选对比：

| 候选 | 优点 | 风险 | 本项目决策 |
| --- | --- | --- | --- |
| River | Go 原生、PostgreSQL、事务一致、部署轻 | 长流程编排能力不如 Temporal；Python worker 不应直接依赖 River 内部表 | Go 侧队列内核首选，外部协议仍用本项目 HTTP worker protocol |
| Temporal | Durable Execution、重试、恢复、长流程强 | 新服务和学习成本高，部署更重 | M2 或复杂流程引入 |
| Asynq | Go、Redis、易上手、重试和监控生态 | Redis 与业务数据库事务一致性弱 | 非核心异步任务可用，不作评分主队列 |
| NATS JetStream | 高性能消息、持久化、边缘和事件流强 | 对业务任务状态和幂等仍需额外设计 | 后续事件总线或边缘同步候选 |

STORY-051 必须实现：

- 任务类型：ocr、layout、preprocess、image_quality、ai_grade、evidence_verify、report_generate、export、desktop_sync。
- 状态：queued、leased、running、succeeded、failed、dead_letter、cancelled。
- 幂等：每个任务有 tenant_id、source_type、source_id、idempotency_key。
- 失败隔离：可重试错误与不可重试错误分开，达到上限进入 dead letter。
- 恢复：worker 崩溃后 lease 过期可重新领取，重复执行不产生重复结果。
- 审计：任务创建、领取、完成、失败、重试、取消都写 audit log。
- 指标：队列长度、失败率、重试次数、P95 耗时、worker 心跳、死信数量。

## 多租户、安全和账号路线

STORY-047 推荐先做数据库约束硬化，而不是一开始全量 RLS。原因是当前 Go 服务已经按 `tenant_id` 做了大量业务查询和权限判断，最紧急风险是跨租户父子引用被错误写入。复合唯一键和复合外键可以用数据库强约束兜底，改造路径清晰，回归测试可直接验证。RLS 可用于高风险读表或后续多租户安全纵深，但需要谨慎处理连接池、迁移、管理员绕过和性能。

STORY-056 推荐补完整账号生命周期：

- 内置账号：创建、启停、锁定、重置密码、密码轮换、会话撤销、登录失败限流、审计。
- 企业 SSO：用 Keycloak OIDC 作为首选开源集成路线，满足学校或机构统一身份源。
- 授权策略：现有 RBAC 继续保留；当权限规则复杂到跨租户、跨组织、跨考试策略组合时，再引入 OPA 做 policy-as-code。
- 安全基线：按 OWASP ASVS 对认证、会话、访问控制、日志、文件、配置做逐项验收；按 OWASP LLM Top 10 覆盖 prompt 注入、数据泄露、过度代理和不安全输出处理。

## Web、验收和真实产品边界

STORY-048 不只是“删 mock”，而是把 Web 后台变成生产工作台：

- 未接真实 API 的页面，生产构建默认隐藏；演示环境可通过 feature flag 打开并明确标注。
- 空状态必须来自真实 API 空结果，不得用硬编码数据撑页面。
- Dashboard 只能展示真实统计或明确的“未接入”状态。
- Review、Quality、Permissions、Settings 页面逐个绑定 API；绑定前不得作为验收通过依据。
- Playwright 覆盖登录、菜单、列表、详情、提交、错误、空状态、低权限、跨租户阻断。

STORY-054 应补成熟阅卷系统的质量能力：

- 相似答案分组：对固定模板题、短答案、选择/填空、OCR 后文本做分组，人工确认后批量应用 Rubric。
- 种子卷和校准卷：阅卷员上线前必须通过校准；正式阅卷中穿插种子卷，失败触发暂停或复训。
- 抽检：按题目、阅卷员、分数段、AI 低置信、异常时长、仲裁差异抽检。
- 一致性指标：同类答案分差、双评分差、阅卷员偏差、异常高低分。
- 发布门禁：质量指标不达标不得发布成绩。

## 桌面采集和离线同步路线

STORY-053 不建议首版承诺“所有扫描仪直接驱动”。扫描仪生态复杂，Windows 设备驱动差异大。更稳的路线是：

- M1：支持文件夹、图片、PDF、已有扫描软件输出目录导入；给扫描工作站提供批量上传、断点、校验、重试。
- M1/M2：评估 NAPS2 SDK 或与 NAPS2 外部流程集成，覆盖 WIA、TWAIN、SANE、ESCL 常见路径。
- 图像预处理复用 OCR worker 的 OpenCV 逻辑，桌面端只做轻量预览、裁剪确认和上传队列。
- 离线阅卷草稿使用 SQLite WAL，记录本地操作日志、冲突版本、同步状态和重试。
- Tauri 更新必须签名；离线安装包也必须签名和校验 hash。

验收要用真实设备和真实断网场景，不接受只在模拟文件上跑通就宣称扫描仪生产可用。

## 观测、部署和安全门禁路线

STORY-052 和 STORY-057 共同构成上线门禁：

- OpenTelemetry：统一 trace、metric、log correlation，覆盖 API、worker、OCR、AI、MinIO、PostgreSQL 关键路径。
- Prometheus：采集服务健康、请求耗时、错误率、队列、worker、数据库连接、磁盘、对象存储。
- Grafana Loki：集中日志，日志必须脱敏，不能输出学生原文答案、完整成绩、密码、token、密钥。
- k6：API、上传、OCR 队列、阅卷提交、报告生成做负载和容量测试。
- Playwright：Web 全流程、权限和生产菜单验收。
- ZAP：预生产环境动态安全扫描，必须只扫授权环境。
- Trivy：容器镜像、文件系统、依赖和 IaC misconfiguration 扫描。
- 备份恢复：PostgreSQL、MinIO、Qdrant、配置和密钥都要有备份、恢复、校验和演练记录。

上线前必须产出一个可交付包：

- 部署 runbook。
- 密钥和 TLS 配置说明。
- 数据迁移和回滚步骤。
- 备份恢复演练记录。
- 监控仪表盘和告警规则。
- E2E、性能、安全、跨租户、AI/OCR 评测报告。
- 已知风险清单和豁免记录。

## 生产化里程碑

### M0：受控演示

目标是让产品在内部或客户演示中可被诚实展示，不把 mock 或 placeholder 说成真实能力。

完成标准：

- STORY-044 已关闭默认密码、mock 登录、阅卷/仲裁对象级权限和关键父对象租户校验风险。
- STORY-046 已完成管理员 bootstrap、HttpOnly cookie 会话和 Web token 存储退场。
- 所有 mock、placeholder、not implemented 能力在 UI、API 或验收文档中明确标记。
- 演示数据使用 synthetic 或脱敏数据。
- 演示环境可以展示已实现的 OCR/图像质量/Agent Runtime 技术链路，但在真实样本评测和容器端到端验收前，不得承诺其效果、容量或可靠性已达到生产指标；真实主观题 AI 和真实扫描仪能力仍不得宣称已具备。

### M1：受控试点

目标是在非正式生产场景中跑通真实数据形态的有限流程，仍保留人工复核和上线豁免边界。

必须关闭：

- STORY-047 数据库级多租户约束硬化。
- STORY-048 Web mock 页面退场与生产菜单收敛。
- STORY-049 真实 OCR Worker 接入。
- STORY-050 答卷图像质量检测与页面标准化。
- STORY-051 Agent Worker Runtime。
- STORY-052 生产部署 Runbook 与预生产验收的核心项。

完成标准：

- 接入至少一个真实 OCR 服务，并通过样本集评估。
- 接入至少一个真实主观题模型适配器，所有 AI 结果只能作为建议分。
- Agent worker runtime 支持任务领取、幂等、重试、失败隔离和审计。
- Web 生产菜单不再展示未接 API 的 mock 页面，或显式关闭相关入口。
- 完成跨租户、低权限、文件下载、成绩发布、申诉和审计的试点验收。
- 有可执行备份、恢复和回滚演练记录。

### M2：正式生产

目标是满足学校、考试机构或企业培训场景的正式上线要求。

必须补齐：

- STORY-053 若桌面端纳入交付范围，完成真实采集和离线同步验收。
- STORY-054 阅卷质量控制与答案分组。
- STORY-055 语义证据与相似答案检索。
- STORY-056 完整用户、角色、SSO 与密码治理。
- STORY-057 生产级测试与安全质量门禁。

完成标准：

- OCR、AI、Agent、人工复核、双评仲裁、成绩发布、申诉、报告、审计全链路可复验。
- 核心数据模型完成数据库级租户隔离硬化。
- 认证、会话、密钥、TLS、CORS、请求限制、敏感日志、审计不可篡改满足安全验收。
- 完成负载测试、容量计划、监控告警、备份恢复和灾备演练。
- Windows EXE 若进入交付范围，必须完成真实采集、离线同步和签名更新验收。

## 推荐后续 Story 队列

| Story | 名称 | 技术路线摘要 | 建议优先级 |
| --- | --- | --- | --- |
| STORY-046 | 生产管理员 Bootstrap 与会话安全加固 | 已完成；后续持续回归 | Done |
| STORY-047 | 数据库级多租户约束硬化 | PostgreSQL 复合租户约束已完成首轮硬化，RLS 作为后续纵深防护 | Done |
| STORY-048 | Web mock 页面退场与生产菜单收敛 | Dashboard 已接真实 API，mock route 生产默认隐藏，静态检查和 typecheck 已通过 | Done |
| STORY-049 | 真实 OCR Worker 接入 | Done；已实现独立 Python OCR worker + PaddleOCR 首选候选 + 本项目样本集评估规范；Surya/docTR/Tesseract 保留为对照评估路线 | Done |
| STORY-050 | 答卷图像质量检测与页面标准化 | Approved；OpenCV/Python worker + immutable quality_run + claim/lease + normalized RGB PNG 资产；主观题模型适配器后移 | Done |
| STORY-051 | Agent Worker Runtime | Approved；PostgreSQL runtime + HTTP worker protocol + OCR/image-quality adapter；River/Temporal 保留后续升级 | Done |
| STORY-052 | 生产部署 Runbook 与预生产验收 | Compose 核心部署、迁移、登录、备份和隔离恢复已完成；完整性能/安全门禁后续执行 | Done |
| STORY-053 | Windows EXE 生产采集与离线同步验收 | 文件导入优先，NAPS2/TWAIN/WIA 逐步接入，SQLite WAL | P1 |
| STORY-054 | 阅卷质量控制与答案分组 | Answer Groups、种子卷、抽检、一致性指标、发布门禁 | P0 |
| STORY-055 | 语义证据与相似答案检索 | Qdrant 首选，pgvector 备选，tenant 隔离 | P1 |
| STORY-056 | 完整用户、角色、SSO 与密码治理 | 内置账号生命周期 + Keycloak OIDC 可选 + OPA 后续 | P0 |
| STORY-057 | 生产级测试与安全质量门禁 | Playwright、k6、ZAP、Trivy、OWASP ASVS/LLM Top 10 | P0 |

建议执行顺序：

1. STORY-047：先把数据底座硬化，避免后续真实 OCR/AI 数据写入后再补约束成本更高。
2. STORY-048：清掉生产 UI 的假能力，保证后续验收界面可信。
3. STORY-049：OCR 是答卷数字化入口，优先打通真实 worker。
4. STORY-050：先完成答卷图像质量检测与页面标准化，让真实 OCR 只消费通过质量门禁的 normalized 页面。
5. STORY-051：再把 STORY-049/050 的 worker 领取、lease、幂等、重试和审计抽象为通用 Agent Worker Runtime。
6. STORY-052/057：预生产验收、观测、安全和性能门禁贯穿执行，不应留到最后。
7. STORY-054/056：正式试点前补质量运营和完整账号治理。
8. STORY-053/055：按交付范围决定是否提前。

## 每个 Story 的证据要求

每个后续 P0 Story 必须留下以下证据：

- 规格、规格审阅、规格修改、实现、实现审阅、修改实现、审批记录。
- 成熟产品参考和开源候选分析，说明为什么选当前技术路线，为什么暂不选其他路线。
- 能证明真实能力的自动化测试、接口响应、日志片段、截图、导出文件或操作录像。
- 对 mock、placeholder、stub、not implemented 的清理记录或生产隐藏记录。
- 对模型、OCR、worker 的版本、配置、输入 hash、输出 hash 和评测数据说明。
- 对 `lab/` 的影响说明；如果继续不接入生产链路，需要明确“不影响生产上线能力”。

## 上线判断规则

满足以下条件前，不应宣称系统已达到生产级智能阅卷应用：

- OCR、主观题 AI 和 Agent runtime 均由真实服务闭环驱动，并通过样本集评估。
- 任一角色无法越权访问他人任务、跨租户资源、敏感文件或未授权成绩。
- 成绩发布前质量门禁可阻止未完成阅卷、仲裁、OCR 失败、AI 低置信或异常分。
- 关键操作都有审计记录，并可按验收要求导出。
- 备份恢复和回滚在干净环境完成演练。
- 生产环境不依赖默认账号、固定密码、演示密钥或未标记 mock 数据。
- Web 生产菜单不展示未生产化能力。
- AI 不直接写最终分，所有建议分有证据、版本和人工复核路径。
- OCR 结果有低置信复核路径，不能把无法识别或错位识别静默流入最终成绩。

## 外部参考链接

成熟产品与评测模式：

- [Gradescope AI-assisted grading and answer groups](https://guides.gradescope.com/hc/en-us/articles/24838908062093-AI-assisted-grading-and-answer-groups)
- [Gradescope product overview](https://www.gradescope.com/)
- [RM Assessor e-marking](https://www.rm.com/assessment/services/e-marking)
- [STACK open-source online assessment](https://stack-assessment.org/)
- [STACK Moodle question type GitHub](https://github.com/maths/moodle-qtype_stack)

OCR 与文档解析：

- [PaddleOCR GitHub](https://github.com/PaddlePaddle/PaddleOCR)
- [PP-OCRv5 multilingual recognition documentation](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.en.md)
- [Surya OCR GitHub](https://github.com/datalab-to/surya)
- [docTR GitHub](https://github.com/mindee/doctr)
- [Tesseract OCR GitHub](https://github.com/tesseract-ocr/tesseract)
- [OCRBench repository](https://github.com/Yuliang-Liu/MultimodalOCR)
- [OCRBench v2](https://99franklin.github.io/ocrbench_v2/)
- [OmniDocBench GitHub](https://github.com/opendatalab/OmniDocBench)
- [OpenCV documentation](https://docs.opencv.org/)

LLM、评估和安全：

- [vLLM OpenAI-compatible server](https://docs.vllm.ai/en/stable/serving/online_serving/)
- [Qwen3 GitHub](https://github.com/QwenLM/Qwen3)
- [Guardrails AI](https://guardrailsai.com/guardrails/docs/api_reference_markdown/guards)
- [Ragas documentation](https://docs.ragas.io/en/stable/)
- [DeepEval documentation](https://deepeval.com/docs/getting-started)
- [OWASP Top 10 for Large Language Model Applications](https://owasp.org/www-project-top-10-for-large-language-model-applications/)

Worker、队列和事件：

- [River GitHub](https://github.com/riverqueue/river)
- [River documentation](https://riverqueue.com/docs)
- [Temporal documentation](https://docs.temporal.io/)
- [Asynq GitHub](https://github.com/hibiken/asynq)
- [NATS JetStream documentation](https://docs.nats.io/nats-concepts/jetstream)

数据隔离、账号和策略：

- [PostgreSQL Row-Level Security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [PostgreSQL constraints and foreign keys](https://www.postgresql.org/docs/current/ddl-constraints.html)
- [Keycloak](https://www.keycloak.org/)
- [Keycloak OIDC documentation](https://www.keycloak.org/securing-apps/oidc-layers)
- [Open Policy Agent](https://openpolicyagent.org/docs)
- [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/)

观测、测试、安全扫描和桌面端：

- [OpenTelemetry documentation](https://opentelemetry.io/docs/)
- [Prometheus overview](https://prometheus.io/docs/introduction/overview/)
- [Grafana Loki documentation](https://grafana.com/docs/loki/latest/)
- [Playwright](https://playwright.dev/)
- [Grafana k6 documentation](https://grafana.com/docs/k6/latest/)
- [OWASP ZAP documentation](https://www.zaproxy.org/docs/)
- [Trivy](https://trivy.dev/)
- [NAPS2 GitHub](https://github.com/cyanfish/naps2)
- [NAPS2 SDK NuGet](https://www.nuget.org/packages/NAPS2.Sdk/)
- [Tauri updater](https://v2.tauri.app/plugin/updater/)
- [SQLite WAL](https://www.sqlite.org/wal.html)
- [Qdrant documentation](https://qdrant.tech/documentation/)
- [pgvector GitHub](https://github.com/pgvector/pgvector)
