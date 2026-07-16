# EduGrade Enterprise 成熟产品差距调研与产品路线建议

> 调研日期：2026-07-12  
> 调研对象：`D:\project\yuejuan` 当前工作区  
> 文档性质：产品与技术决策输入，不代表后续 Story 已获批准或已经实现  
> 结论置信边界：代码事实来自当前仓库；竞品能力仅采用公开官方资料；模型效果只引用论文或官方说明，不能外推为本项目效果；法律部分仅用于识别工程要求，不替代正式法律意见。

## 1. 执行结论

EduGrade 当前已经不是“从零开始的原型”：考试、试卷、答卷、对象存储、图像质量、页面配准、切题、OCR、客观题规则、人工复核、双评仲裁、成绩发布、申诉、报告、审计等主要对象和 API 已经出现，Go/PostgreSQL/MinIO/Python Worker/Web 的工程骨架也基本成立。

但它仍不能客观地称为成熟产品，更不能称为高风险考试可用产品。最准确的判断是：

- 可以进行受控演示和集成联调。
- 接近“内部真实数据试验”的准备阶段，但 STORY-056 尚未批准，不能宣称客观题闭环完成。
- 尚不具备有限学校试点所需的完整证据。
- 距离正式、高风险或大规模考试还有明显差距。

核心问题不是“还少几个页面”，而是**横向模块多、纵向闭环浅，代码存在不等于考试结果可被信任**。当前最缺的不是再接一个大模型，而是以下五类行业壁垒：

1. 授权真实答卷数据集、双人裁定真值和持续评测平台。
2. 从打印、扫描到学生/页码/题目归属的零静默丢失链路。
3. 按题型路由的识别系统，而非一个通用 OCR 处理所有内容。
4. 阅卷员标准化、暗桩、漂移监控、抽检、双评和主管干预组成的质量运营系统。
5. 能量化风险并选择性自动化的人机协同评分，而非让模型自报置信度后直接写最终分。

因此，当前规划中“STORY-057 直接进入真实主观题 AI”顺序不合理。建议先完成 STORY-056，再把真实数据基线、采集完整性、识别路由、人工阅卷效率和质量中心放在主观题 AI 前面。否则仍会重复过去多轮对话的问题：模块继续增加，但没有形成可用于产品决策的真实证据。

## 2. 调研方法与证据规则

### 2.1 本次实际核查范围

本次调研不是只读 PRD，而是对以下内容做了交叉核查：

- `docs/stories/README.md` 与 STORY-045、STORY-054、STORY-055、STORY-056 的状态和验收记录。
- API Gateway 中 capture、grading、subjective、evidence、review、score、appeal、report 的实现。
- OCR Worker、页面处理 Worker、OMR 和配准实现。
- Web Admin 的主要业务页面与 API 调用边界。
- 数据库迁移、Worker Runtime、部署与验收文档。
- Gradescope、Crowdmark、RM Assessor、Cambridge、ETS 的公开工作流或质量标准。
- PaddleOCR、UniMERNet、OpenCV、ZXing-C++、NAPS2、STACK、SymPy、Qdrant、BGE、Qwen3-VL、OneRoster 等候选技术的官方资料。
- NIST、UNESCO、教育部和国内网络数据/生成式 AI 规则中的治理要求。

### 2.2 成熟度分级

本报告不使用虚假的“完成百分比”，使用可审计的五级证据：

| 等级 | 含义 | 可以宣称什么 |
| --- | --- | --- |
| L0 | 不存在或只有概念 | 不能演示真实能力 |
| L1 | 有 Schema、API、UI 或占位实现 | 只能宣称接口/结构存在 |
| L2 | 真实组件可运行，单元/集成/合成数据通过 | 可以受控演示，不能外推真实考试效果 |
| L3 | 授权真实数据闭环试点，失败路径、人工接管和指标达标 | 可以进行限定范围学校试点 |
| L4 | 多批次生产证据、SLA、运维、质量与治理持续稳定 | 才能称为成熟生产能力 |

任何能力从 L2 到 L3 都必须经过真实授权样本、明确适用范围、冻结版本和可复现报告。公开榜单、单张图片演示和开发者手工操作都不能替代这一门槛。

### 2.3 第一性原理

阅卷产品的最终价值不是“输出一个分数”，而是在规定时间内，以可解释、可复核、可纠错的方式，为每一名学生交付正确归属的考试结果。由此得到七条不可破坏的产品不变量：

1. **完整性**：每个预期学生、答卷、页和题都必须被对账；缺失不能静默通过。
2. **归属正确**：页、学生、试卷版本和题目不能串联；不确定归属必须升级人工。
3. **评分可重放**：任何分数都能追溯到原图、裁剪、识别候选、规则/Rubric、模型及人工操作版本。
4. **不确定性安全**：机器不知道时必须停下并交给人，不能用默认零分或高置信文字掩盖失败。
5. **阅卷一致性**：同一 Rubric 在人员、时间和批次间保持可测量的一致性。
6. **发布可证明**：只有完整性、评分和质量门禁全部通过，成绩才可发布。
7. **结果可纠错**：学生、教师和管理员有合适的查看、复核、申诉和重新发布路径。

这七条比“用了哪个模型”优先级更高，也是后续 Story 排序的依据。

## 3. 目标产品闭环

成熟产品的最小可信工作流应是：

```text
考试与题目/Rubric 定稿
  -> 生成不可变打印包（模板版本、每页签名码、定位锚点、可选预匹配学生）
  -> 扫描/上传批次（设备配置、原件留存、批次交接记录）
  -> 预期清单与实收页面对账（缺页、重页、外来页、旧模板、错学生）
  -> 图像质量、方向、配准和按题切分
  -> 题型识别路由（OMR / 印刷与手写文本 / 数学公式 / 图形人工）
  -> 确定性规则评分或主观题建议
  -> 不确定性、异常和证据门禁
  -> 按题人工阅卷、答案分组、双评、仲裁、抽检、暗桩和主管监控
  -> 成绩完整性与质量发布门禁
  -> 学生查看、反馈、申诉、改分版本与重新发布
  -> 分析、导出、LMS/SIS 回写、审计和留存
```

产品首页和考试工作区应始终回答三个问题：现在处于哪一步、什么问题阻断下一步、用户此刻唯一的主动作是什么。成熟产品的效率来自流程编排和批量工作方式，不来自把 Worker 的每个技术按钮暴露给教师。

### 3.1 角色与必须完成的任务

| 角色 | 核心任务 | 产品成功标准 |
| --- | --- | --- |
| 考试负责人 | 配置并锁定考试、批准打印包、监控阶段、处理阻断、发布成绩 | 不需要理解内部任务名；任何阶段都能看到下一步和发布风险 |
| 扫描操作员 | 选择设备配置、创建批次、导入/扫描、处理缺页/重页/错页 | 不手工触发 OCR/切题；批次结束时实收与预期可对账 |
| 阅卷教师 | 领取同一道题、看原图和 Rubric、批注/评分、提交下一份 | 核心信息同屏；草稿不丢；不接触学生身份和无关技术字段 |
| 主阅卷长/质检员 | 维护标准卷、批准阅卷员、监控偏差、抽检/回看、处理仲裁 | 能在问题影响发布前发现偏离，并暂停/纠正相关人员或题目 |
| 学生 | 查看已发布结果、题级反馈、提交申诉、接收处理结果 | 只见本人数据；能理解评分依据、状态和改分版本 |
| 学校 IT/系统管理员 | 安装、身份接入、备份恢复、监控容量、升级回滚 | 不依赖开发者现场操作；所有敏感操作可审计 |

页面、API 和模型能力只有进入上述角色闭环后才算产品能力。例如，后端存在条码签发 API，但考试负责人不能生成正式打印包时，仍只能评为 L1。

## 4. 当前能力成熟度审计

### 4.1 总表

| 能力域 | 当前证据 | 等级 | 距离成熟产品的关键缺口 |
| --- | --- | --- | --- |
| 机构、用户、考试、试卷、题目、Rubric | 已有真实数据库/API/Web 工作流和状态约束 | L2 | 大规模权限生命周期、SSO、跨系统同步、真实用户可用性证据 |
| 不可变打印包 | 后端存在每页 HMAC 签名码签发 API | L1 | Web 未调用；未生成可打印 PDF；缺定位锚点、学生预匹配、版本校验和打印验收 |
| 批次采集与页面处理 | 批次、页、质量检测、条码解码、配准、人工修正、拆分/合并已有实现 | L2 | 多型号真实扫描仪、断点续传、实收对账、缺/重/外来页闭环和批量压力证据 |
| OCR | PaddleOCR 真实 Worker 可执行，保存文本、框、置信度与版本 | L2（组件） | 无本项目真实答卷基准；无题型路由；无公式专用链路；置信度未校准 |
| OMR/客观题 | STORY-056 已有真实 OpenCV OMR、选项证据、overlay、版本化规则和评分运行 | L2（进行中） | STORY-056 四个审批阻断项未完成；100 份混合题型和误判矩阵未交付 |
| 数字/单位/代数表达式 | 已有部分规则框架 | L1-L2 | 缺隔离的安全解析/CAS、等价与形式约束、单位和边界用例库 |
| 人工阅卷台 | 匿名任务、领取/租约、裁剪图、Rubric、草稿、提交下一份已实现 | L2（进行中） | 浏览器离线草稿尚缺；答案分组、图像锚定批注、评语库、真实教师效率试验缺失 |
| 双评与仲裁 | 对象、API 和页面存在 | L1-L2 | 不是完整质量运营：缺阅卷员准入、标准化、暗桩、漂移、回看、暂停/再培训 |
| 主观题 AI | 生产路由仍绑定 `NewMockLLMAdapter()`，固定低置信输出 | L0 | 无真实模型、无项目评测集、无影子运行、无风险校准、无模型发布门禁 |
| 证据校验 | 校验分数范围、Rubric 点、OCR 文本包含关系和 bbox 边界 | L1 | 仅结构校验，不等于语义/视觉证据正确，也不能证明评分合理 |
| 成绩发布门禁 | 可阻断未完成复核/仲裁、OCR 失败、异常和缺失最终分 | L2 | 未纳入阅卷员资格、暗桩/漂移、抽检、模型评测版本、完整性对账和批次批准 |
| 报告 | 后端可从最终成绩聚合基本指标 | L2 | 需要真实数据可用性、题目分析/质量分析边界、统计定义和异常数据规则 |
| 学生端与申诉 | 申诉/成绩相关 API 存在，管理端有申诉中心 | L1 | 没有正式学生端、通知、题级反馈、申诉对话和改分后重新发布体验 |
| LMS/SIS/SSO | 未发现 OneRoster、LTI 或完整 OIDC 产品闭环 | L0-L1 | 花名册同步、成绩回写、身份生命周期和学校集成 |
| Windows 扫描/离线 | Tauri/离线结构和部分约束存在 | L1 | 真实设备驱动、安装签名、离线队列冲突、升级和现场运维验收 |
| 生产运维与安全 | Compose、迁移、备份恢复、审计和部分可观测性已有 | L1-L2 | 清洁环境全链路、容量基线、灾备演练、密钥/证书、漏洞门禁、数据生命周期 |

### 4.2 关键代码事实

以下事实决定当前不能提前宣称成熟：

- STORY-056 状态仍为 `Implementation In Progress - Review Fixes Underway`，其验收框全部未勾选；审批阻断项明确包括评分运行取消/重试/局部重处理、浏览器草稿兜底、至少 100 份混合题型评测和最终清洁数据库 Compose E2E。
- `services/api-gateway/internal/server/server.go:187` 仍将生产主观题 Handler 绑定到 `subjective.NewMockLLMAdapter()`。
- `services/api-gateway/internal/subjective/adapter.go` 明确返回固定 0 分、0.5 置信度和 `mock_llm_output` 风险标志。主观题 AI 因此是 L0，而不是“已有接口所以完成”。
- `services/ocr-worker/ocr_worker/engine.py` 调用通用 `PaddleOCR`，当前没有按公式、手写文本、印刷体和图表进行专门路由。
- `services/api-gateway/internal/evidence/engine.go` 主要做 OCR 子串、Rubric ID 和 bbox 包含关系校验；这是有价值的结构门禁，但不是语义证据验证器。
- 后端已有 `POST /api/v1/answer-sheet-templates/{id}/page-barcodes`，但 Web Admin 中没有对应调用，尚未形成教师可使用的打印包工作流。
- 当前发布门禁检查任务完成度、OCR 失败、评分异常和最终分缺失，但没有阅卷员标准化、暗桩表现、漂移或模型版本门禁。
- 学生成绩和申诉相关后端对象不等于学生产品；当前 Web 主要面向管理员/教师。

### 4.3 为什么已有 56 个 Story 仍未形成成熟产品

早期 Story 很多完成的是“领域对象、API 边界、页面骨架和单元测试”。这些工作是必要的，但成熟度证据被 Story 数量掩盖了：

- 接口完成不等于一线教师能完成任务。
- Worker 能跑不等于真实答卷识别准确。
- 一张公开手写图成功不等于中国学校答卷可用。
- 双评/仲裁表存在不等于阅卷质量受控。
- 置信度字段存在不等于模型不确定性被校准。
- 发布按钮受控不等于所有学生、页、题和质量指标已被对账。

后续必须把交付单位从“新增模块”改为“真实考试不变量 + 可复现实验证据”。

## 5. 成熟产品公开能力给出的启示

### 5.1 Gradescope：人机协同和按题效率

Gradescope 的公开文档显示，成熟工作流并不是 AI 直接给最终分：固定模板答卷可以先形成 Answer Groups，AI 只协助建议分组，教师确认分组后用同一 Rubric 批量评分；阅卷台提供动态 Rubric、互斥/分组项、快捷键、图像锚定批注、复用评语和“下一份未阅”。它还提供匿名阅卷和题目级 regrade request。

对本项目的直接含义：

- 在主观题自动评分前，答案分组和高效率人工阅卷的收益更确定、风险更低。
- Rubric 不只是一个分数输入框，而是版本化的评分操作系统。
- 学生申诉应是题目级证据与对话，不是后台技术记录列表。

### 5.2 Crowdmark：打印、页面身份和扫描作业

Crowdmark 的官方流程使用每页二维码组织乱序扫描页，支持预匹配、OCR 辅助和人工学生匹配；扫描指南对灰度/彩色、DPI、铅笔深浅、页面边界和 PDF 批次给出明确操作要求。系统还把 LMS 花名册、团队和成绩导出作为正式闭环。

对本项目的直接含义：

- “生成答题卡模板”必须延伸成真正可打印、可校验、可回收对账的考试包。
- 页身份应优先由签名码确定，OCR 姓名只能辅助，不能成为唯一真值。
- 扫描参数和设备配置是产品功能，不应只写在部署说明中。

### 5.3 RM Assessor 与 Cambridge：高风险阅卷质量控制

RM Assessor 公开描述了 seeding、随机双评、back marking、blind peer review、分配策略和细粒度进度监控。Cambridge 的公开标准要求阅卷员先完成标准化并证明达到要求，正式阅卷期间使用隐藏 seed scripts 监控，发现偏离后采取纠正行动，并在关键边界附近复核。

对本项目的直接含义：

- 双评和仲裁只是质量中心的一部分，不是全部。
- 阅卷员必须经过“练习 -> 标准化 -> 准入 -> 实时监控 -> 暂停/再培训 -> 恢复”的生命周期。
- 发布门禁必须读取质量中心状态，而不是只看任务是否完成。

### 5.4 Cambridge 当前 AI 实践：不确定样本必须人工

Cambridge 公开介绍的 AI-assisted marking 是 Digital Mocks 场景；其资料说明使用大量 AI/考官分数进行训练，并把每个不确定分数交给人工。该公开能力不应被误读为正式 IGCSE/A Level 已允许无人主观题评分。

对本项目的直接含义：主观题 AI 首先只能影子运行和建议评分，适用范围必须按题型逐项批准，不能用“模型整体准确率”一次性放行所有科目。

### 5.5 竞品能力对照不是照搬清单

| 产品/标准 | 公开优势 | EduGrade 当前对应状态 | 应吸收的产品原则 |
| --- | --- | --- | --- |
| Gradescope | 按题阅卷、Answer Groups、Rubric、批注、匿名和 regrade | 按题工作台正在形成；分组、锚定批注和学生 regrade 体验不足 | 先提高人工一致性与吞吐，再扩大 AI 自动作业 |
| Crowdmark | 每页二维码、学生匹配、扫描作业、团队/LMS 闭环 | 有签名码/配准后端，无正式打印包和实收对账 | 打印与采集必须是同一产品链路 |
| RM Assessor | 阅卷员分配、seeding、双评、back marking、质量监控 | 有双评/仲裁对象，缺完整阅卷员质量生命周期 | 质量控制是核心业务域，不是报表附属项 |
| Cambridge/ETS | 标准化、持续监控、隐藏标准卷、校准与复核 | 尚无生产级标准化/漂移/暗桩门禁 | 高风险分数必须由可度量的人和流程负责 |
| STACK | 数学答案测试和多种“正确”语义 | 有一般规则框架，缺安全 CAS 和题型用例库 | 数学评分优先确定性语义，不交给模糊语言判断 |

这些公开产品面向的地区、部署方式和考试风险并不完全相同。本项目应吸收经过验证的流程原则，不应复制界面、专有术语或假定其公开能力就是本项目的验收结果。

## 6. 真正的行业壁垒

### 6.1 壁垒一：授权真实数据与评测飞轮

这是当前第一缺口，也是所有模型选型前置条件。公开研究已经显示，中国 K-12 真实答卷同时包含潦草字迹、公式、图、跨行推理和扫描噪声，当前多模态模型仍显著低于人类表现。PaddleOCR 对复杂手写的官方能力说明只能证明它是候选，不能证明它在本项目答卷上达到要求。

应建立不可混用的三套数据：

- **开发集**：可用于规则、预处理和模型调试。
- **校准集**：用于置信度校准、路由阈值和人工召回策略。
- **冻结验收集**：开发者不可反复查看，只在版本门禁运行。

每个样本至少记录：授权依据、脱敏状态、年级、学科、题型、模板版本、扫描设备与参数、质量缺陷、原图、配准图、题块、人工转写、标准答案/Rubric、两名阅卷员分数、仲裁真值和难例标签。真值分歧必须仲裁，不能用一名教师的分数直接当金标准。

数据飞轮应只吸收经人工确认的纠错，并保存来源、版本和用途授权；不能把学生原始数据默认送到外部服务或用于训练。

### 6.2 壁垒二：从纸到题的零静默丢失链路

成熟系统首先要证明“这是谁的第几页、属于哪个模板版本、哪些页还没回来”。建议建立不可变 `exam_package_manifest`：

- 每份/每页唯一标识，包含 tenant、exam、template version、booklet、page number、key id 和签名。
- 四角定位锚点或稳定基准标记，用于快速检测与单应性配准。
- 可选预匹配学生包，同时保留匿名阅卷映射。
- 人眼可读的短码，二维码损坏时可人工录入。
- 生成 PDF 的 hash、页数、尺寸、打印配置和批准人不可变记录。

采集后必须做 `expected vs received` 对账：缺页、重复页、外来考试页、旧模板页、未知码、学生重复答卷、额外空白册都进入明确队列。条码失败可尝试图像/版式匹配，但任何低置信 fallback 都不能自动归属。

这条链路比提升 1 个点 OCR 准确率更重要，因为误归属和静默缺页会直接造成系统性错误。

### 6.3 壁垒三：按题型识别和确定性评分

首选路线不是“一页交给一个大模型”，而是先用模板把问题缩小，再按题型选择最可靠工具：

| 输入 | 首选处理 | 机器输出用途 | 自动评分边界 |
| --- | --- | --- | --- |
| 单选/判断/规范多选 | OpenCV OMR + 选项级像素证据 | 选项候选、填涂强度、overlay | 清晰且上游门禁通过时可规则评分 |
| 数字/短填空 | 裁剪 + 文本/数字 OCR + 规范化 | 多候选转写 | 只有确定性等价、误差和单位规则通过才自动评分 |
| 代数表达式 | 公式识别 + 安全 AST/CAS | LaTeX/AST 候选 | 通过特定 answer test；不能仅字符串相似 |
| 手写中文/英文 | PP-OCRv5 等候选模型对比 | 转写和定位 | 默认辅助阅卷，达到题型门禁后再扩大自动化 |
| 复杂公式 | PP-FormulaNet/UniMERNet 候选对比 | LaTeX/结构候选 | 必须经规范化、语法和等价校验 |
| 图、证明过程、作文 | 原图 + OCR/公式辅助 + 多模态建议 | 证据、分组、风险提示 | 首版全部人工确认 |

数学等价服务应独立隔离：使用允许列表语法解析为 AST，再调用 SymPy 或 Maxima；限制符号、函数、执行时间、内存和进程，禁止将未清洗输入直接交给 `parse_expr`。评分语义参考 STACK Answer Tests，但没有必要把整个 Moodle/STACK 产品嵌入本项目。

### 6.4 壁垒四：阅卷员质量运营

质量中心需要的不是更多仪表盘卡片，而是可执行状态机：

```text
未授权
  -> 练习卷
  -> 标准化测试
  -> 按题准入
  -> 正式阅卷
  -> 暗桩/抽检/双评持续监控
  -> 正常 / 警告 / 暂停
  -> 复训与复核
  -> 恢复或取消资格
```

至少应支持：

- 主阅卷长维护 gold/seed scripts，并版本化标准分和解释。
- 暗桩混入真实队列，阅卷员不可识别。
- 随机抽检、随机双评、定向回看和边界分复核。
- 主管查看题目/阅卷员维度的偏差、速度异常、严重分歧和趋势，并可暂停分配。
- Rubric 修改后识别受影响答案，执行回溯或批量 regrade。
- 发布门禁要求所有阅卷员资格有效、严重偏差已处理、抽检完成。

指标不能只看“完全一致率”。应同时使用 exact agreement、邻近一致、加权 Cohen's kappa、平均偏差、MAE、严重分差率、零分误判、暗桩通过率、复核改分率和时间异常。阈值应按考试风险、题目分值和历史试点确定，不应在没有数据时凭空写死。

### 6.5 壁垒五：相似答案分组与高效率人工阅卷

在多数学校场景中，先让教师更快、更一致地阅卷，比追求无人主观题评分更有价值。建议分三层实现：

1. 确定性分组：空白、相同规范化短答案、相同 OMR 组合、完全相同公式 AST。
2. 嵌入建议分组：对同一 tenant/exam/question 内的 OCR 文本和题块视觉向量生成候选簇。
3. 人工确认：教师拆分、合并、命名分组，再把 Rubric 结果应用到组内；个体覆盖保留版本和原因。

Qdrant 可继续作为候选索引，但必须在 payload 中强制 tenant、exam、question、template/model version 过滤。BGE-M3、BGE-VL、DINOv2 等只能进入本项目 bake-off；它们不是直接选定的生产模型。衡量指标应是 pairwise precision/recall、误合并率、教师拆分率、每题耗时和最终改分率，而不是只看聚类图是否“像”。

### 6.6 壁垒六：选择性自动化的主观题 AI

主观题 AI 的正确顺序：

```text
真实人工基线
  -> 影子运行（AI 不影响成绩）
  -> 逐 Rubric 点输出建议和原图证据坐标
  -> 与双人/仲裁真值比较
  -> 校准风险和人工召回
  -> 仅对批准题型做“建议优先”
  -> 在持续暗桩和漂移监控下逐步扩大
```

模型输入应同时包含原始题块、OCR/公式候选、题干、Rubric、标准/锚定答案和上下文边界；输出必须包含逐 Rubric 点结论、分值、原图坐标/片段证据、冲突、风险和完整版本链。模型自报 `confidence` 不能直接用于自动放行，必须用冻结校准集上的经验误差、模型间分歧、重复一致性、规则冲突和输入质量共同形成风险分。

Qwen3-VL、PaddleOCR-VL 或其他私有化 VLM 可进入候选池，但在本项目授权答卷上完成相同硬件、相同提示、相同输出约束的盲测之前，不选定“最适合模型”。原始 LLM 预测与人类评分不完全对齐、需要校准，已有同行评议研究支持这一风险判断。

## 7. 技术路线决策

### 7.1 保留当前架构，不先做大规模微服务重构

建议继续使用 Go 模块化单体承载认证、权限、考试、评分事实、审计和任务控制，PostgreSQL 作为事务真值，MinIO 保存原件和派生资产，Python Worker 承载 OpenCV/Paddle/公式/模型推理。当前阶段不需要为了“企业级”引入更多服务或 Temporal；只有跨服务长流程、补偿逻辑和并发规模证明现有 Worker Runtime 不足时再评估。

### 7.2 候选技术矩阵

| 能力 | 推荐/候选 | 决策 | 说明 |
| --- | --- | --- | --- |
| 页码/试卷码 | ZXing-C++ + 已有 HMAC token | 继续采用 | 支持 QR/DataMatrix；签名和 key rotation 保持本项目控制 |
| 配准/质量/OMR | OpenCV | 继续采用 | 当前实现方向正确；补真实扫描条件评测和 overlay 复核 |
| Windows 扫描 | NAPS2 SDK/CLI | 首选候选 | 可覆盖 WIA/TWAIN/SANE/eSCL，避免自研驱动层；分发前复核 LGPL 义务 |
| 常规中英文 OCR | PaddleOCR PP-OCRv5 | 生产候选，不是最终结论 | 已部署、中文生态好；必须通过本项目答卷基准 |
| 数学公式 | PP-FormulaNet、UniMERNet | 并行候选 | 用公式 exact/normalized match 和人工召回评估，不能只看公开榜单 |
| 数学判等 | 允许列表 Parser + 隔离 SymPy/Maxima | 推荐 | 借鉴 STACK Answer Tests；禁止 `eval` 风格未清洗解析 |
| 文本向量 | BGE-M3 | 候选 | 适合中文/多语检索候选；需和字符/规则基线同场对比 |
| 图文向量 | BGE-VL、DINOv2 | 候选 | 仅用于同题答案分组建议，不直接评分 |
| 向量存储 | Qdrant | 继续采用 | 强制多租户 payload 过滤、版本和删除策略 |
| 主观题 VLM | Qwen3-VL 等可私有化模型 | 只进影子评测 | 权重许可证按具体 checkpoint 复核；不承诺自动评分 |
| 花名册/成绩集成 | OneRoster 1.2 | 推荐标准 | 先 CSV/REST，再按客户系统实现 |
| LMS 启动/回写 | LTI 1.3 | 推荐标准 | 与 OneRoster 分工，不自创学校专有协议作为唯一接口 |
| 企业身份 | OIDC/SAML 适配器 | 推荐 | 内置账号保留，学校版按 IdP 接入；权限仍由本系统审计 |

### 7.3 明确不采用的捷径

- 不用通用 VLM 直接替代页面身份、配准、切题、OCR 和规则评分全链路。
- 不用 OCR 姓名作为页归属唯一依据。
- 不用模糊字符串相似度自动判填空正确。
- 不用模型自报置信度直接决定是否写最终分。
- 不把 Qdrant/RAG 检索结果当作评分真值。
- 不因公开榜单领先就跳过本项目数据评测。
- 不在真实链路未闭环前继续堆仪表盘、营销页或新微服务。

## 8. 重排后的产品路线

下表是对后续 Story 的规划建议，不是当前状态变更。建议在确认后更新 `docs/stories/README.md` 和生产路线图，避免旧的 057-061 大 Story 继续同时承载过多目标。

### 8.1 优先级判断

| 优先级 | 内容 | 原因 |
| --- | --- | --- |
| P0：没有就不能可信试点 | STORY-056 收口、打印/采集完整性、领域评测与识别路由、人工阅卷生产力、阅卷质量中心、发布/学生纠错闭环 | 直接保护页不丢、分不错、问题可发现、结果可纠正 |
| P1：形成差异化和学校规模化 | 主观题 AI 影子模式、答案多模态分组、LMS/SIS/SSO、真实扫描设备和离线运维 | 在 P0 真值和流程上提高效率、降低接入成本 |
| P2：在试点证据后扩展 | 更多模型、更多任意版式、更多云适配器、跨区域大规模调度、高级预测分析 | 价值依赖真实客户范围，过早建设会分散核心闭环 |

主观题 AI 属于长期行业壁垒，但其**模型实现顺序**仍是 P1；它所依赖的数据、评测、证据和质量门禁属于 P0。把这两件事区分开，才能既不回避 AI，也不让 AI 绕过产品责任。

### STORY-056R：完成客观题与人工阅卷台审批

用户结果：一场混合题型考试可以从当前 segment 自动路由到客观题规则或人工队列，失败可恢复，教师刷新/短暂断网不会丢草稿。

必须完成：

- scoring run detail/cancel、失败项 retry、segment reprocess 与 Worker Runtime Cancel/Requeue 联调。
- 有边界的 per-user 浏览器草稿兜底、提交/退出清理和 revision 冲突测试。
- 至少 100 份授权脱敏或合成混合题型验收，交付 OMR confusion matrix、模糊召回、人工路由、吞吐和 false-zero 复核。
- 清洁数据库 Compose E2E 和最终实现审阅。

退出门槛：STORY-056 文档逐项有证据并批准；未批准前不开始下一 Story 实现。

### STORY-057：考试打印包与采集完整性

用户结果：考试负责人能生成正式打印包；扫描后系统能明确告诉他哪些学生/册/页缺失、重复、错误或需要人工确认。

范围：签名页码、定位锚点、PDF manifest、预匹配/匿名映射、打印预检、expected-vs-received 对账、异常修复队列、批次交接记录、至少两类真实扫描路径。

退出门槛：验收集内不存在已知静默缺页或静默误归属；所有异常都有业务可理解的修复动作；原始件和每次修正可追溯。

非目标：不在本 Story 接主观题大模型。

### STORY-058：答卷领域评测平台与识别路由

用户结果：系统按题型自动选择识别方式；管理员能看到模型适用范围、失败率和人工复核队列，而不是只看到一个 OCR 状态。

范围：授权数据规范、双人转写/仲裁、开发/校准/冻结集、文本/数字/公式/OMR 路由、人工纠错、版本化 benchmark report、模型候选 bake-off。

退出门槛：每个宣布支持的题型都有冻结集报告；低质量/低置信样本的人工召回可测；模型切换可回滚；未达标题型明确保持人工。

非目标：不设没有数据依据的统一“95% 准确率”，也不把一个 CER 数字替代业务风险。

### STORY-059：专业人工阅卷生产力

用户结果：教师进入后立即继续同一题阅卷，能在不离开原图的情况下使用 Rubric、锚定批注、常用评语、答案分组和快捷操作。

范围：确定性/建议答案分组、人工拆分合并、组级评分和个体覆盖、图像锚定批注、评语库、Rubric 快捷键、下一份预取、真实教师任务测试。

退出门槛：真实教师能完成端到端任务；记录每题耗时、回退、草稿恢复、误合并和评分修改；可用性问题闭环，而非仅通过截图验收。

### STORY-060：阅卷质量中心

用户结果：主阅卷长能知道谁有资格阅哪道题、谁正在偏离标准、哪些答案必须复核，以及为什么当前不能发布。

范围：练习/标准化/准入、gold/seed、隐藏暗桩、抽检、随机双评、back marking、偏差与漂移、暂停/复训、Rubric 变更回溯、质量门禁。

退出门槛：用授权试点数据演练阅卷员偏离、暂停、复训、回看和恢复；发布门禁能真实阻断未解决质量问题；指标定义经考试负责人批准。

### STORY-061：主观题 AI 影子模式与模型门禁

用户结果：AI 为教师提供逐 Rubric 点建议和原图证据，但不在未获批准时改变学生成绩；负责人能比较 AI 与人类真值并决定适用范围。

范围：真实模型 adapter、私有化推理、结构化输出、证据坐标、版本链、影子运行、校准、严重错误/false-zero/fairness slice、题型级 promotion policy。

退出门槛：冻结集和影子试点报告完成；所有 AI 建议 100% 人工可见/可拒绝；高风险和不确定样本召回门禁生效；模型/提示/Rubric 版本可回放。

非目标：不承诺通用无人主观题评分。

### STORY-062：学生结果、申诉与重新发布

用户结果：学生能看到自己被允许查看的题目、分数、批注和反馈，按题提交申诉；教师处理后生成可审计新版本并通知学生。

范围：学生身份与门户、发布可见性、题级反馈、申诉对话、证据、改分审批、成绩版本、重新发布、通知和导出。

退出门槛：从发布到学生查看、申诉、改分、通知和审计的真实角色 E2E 通过；私有备注和他人数据不泄露。

### STORY-063：学校集成、离线采集与现场运维

用户结果：学校能同步花名册、回写成绩，在网络不稳定和常见扫描设备环境中持续采集，管理员能安装、升级、备份和恢复。

范围：OneRoster/LTI/OIDC 适配器、NAPS2 设备路径、离线 spool 与冲突、签名安装包、升级回滚、容量/监控/备份恢复。

退出门槛：至少一个目标学校系统和实际设备矩阵验收；断网/重启/重复上传演练无数据丢失；安装和恢复由非开发人员按 Runbook 完成。

### STORY-064：有限学校试点与正式发布门禁

用户结果：产品团队能基于证据决定“哪些学校、科目、题型、规模和硬件配置可上线”，而不是给出笼统 Production Ready 声明。

范围：多批次试点、SLA/SLO、隐私与安全评估、容量、灾备、支持流程、已知限制、模型卡、数据卡、发布/回滚批准。

退出门槛：每个支持声明都能指向试点证据；P0 缺陷清零；剩余风险有负责人和到期日；正式版本包可复现。

### 8.2 首个真实可用版本的边界

建议把首个有限学校试点明确限定为：

- 固定模板、系统生成打印包的纸质考试，不承诺任意历史试卷拍照即用。
- 选择题/判断题走 OMR；数字/短填空只在已发布确定性规则内自动评分。
- 主观题由教师按题人工阅卷，OCR/公式/分组/AI 仅辅助，所有机器建议可拒绝。
- 所有低质量、归属不确定、识别不确定和评分冲突均进入人工队列。
- 单校私有化或受控内网部署；首个试点可用 CSV 完成花名册/成绩交换，标准集成随后验证。
- 明确支持的年级、学科、题型、模板、扫描设备和规模写入版本说明，范围外自动降级或拒绝。

首个试点明确不承诺：任意版式通用识别、作文/证明题无人自动评分、手机随拍无人工处理、高风险统考正式放行、无网络冲突的全自动跨校同步。这个边界不是降低质量，而是让质量声明可被验证。

## 9. 评测与验收体系

### 9.1 不能只测“准确率”

每次真实验收至少需要五类指标：

| 层级 | 必测指标示例 | 主要防止什么 |
| --- | --- | --- |
| 采集完整性 | 预期/实收页、缺页、重复、误归属、人工匹配率、未解释额外页 | 丢卷、串卷、漏页 |
| 识别 | CER/WER、bbox/segment IoU、公式 exact/normalized match、OMR confusion、低置信召回 | 错识别被当成真值 |
| 评分 | exact/邻近一致、weighted kappa、bias、MAE、严重分差、false-zero | 分数系统性偏差 |
| 人机流程 | 人工路由率、建议采纳/修改、每题耗时、草稿恢复、误分组/拆分 | 自动化没有实际效率或增加风险 |
| 运行 | 吞吐、P50/P95、队列年龄、失败/重试、GPU/CPU/内存、恢复时间 | 实验能跑但考试窗口不可用 |

### 9.2 阈值设定方法

本报告不编造一套适用于所有考试的阈值。正确方法是：

1. 由目标考试风险、题目分值、人工基线、批量窗口和可接受人工量定义业务损失函数。
2. 在校准集上选择识别/评分的自动放行阈值，以高风险错误召回优先。
3. 在冻结集上只运行一次正式门禁，报告总体和年级/学科/题型/扫描设备/质量缺陷切片。
4. 由考试负责人、产品和技术共同批准适用范围；任何模型/模板/Rubric 变更触发重新评测。
5. 生产中用 seed、抽检和漂移监控持续验证，超界自动降级到人工。

可以预先写死的是安全不变量，例如“不得存在已知静默丢页、误归属、重复当前分、无来源零分”；不能预先写死的是未经真实基线支持的 OCR/AI 营销准确率。

### 9.3 评测报告必须可复现

每份报告保存：Git commit、数据库迁移版本、镜像 digest、模型/权重/license、prompt、规则/Rubric、配置 hash、硬件、数据集版本、随机种子、运行日志、失败样本索引和人工裁定。只保存一张汇总截图不合格。

## 10. 产品体验的优先原则

当前 UI 优化应服务流程，不应与核心能力分离：

- 管理员首页优先展示会阻断考试的唯一最高风险问题和主动作。
- 考试列表显示阶段、完成比例、阻断项和下一步，不让用户手工推进内部状态机。
- 采集页按批次进度和异常优先，OCR/切分由系统编排，技术操作折叠。
- 阅卷入口默认是“开始/继续阅卷”，阅卷台保持原图、Rubric、当前分和“提交并下一份”同时可见。
- 无仲裁/无申诉时显示完整空状态，不保留禁用工作台占据屏幕。
- 发布页按“检查 -> 确认 -> 发布完成”推进，当前只出现一个主动作。
- 教师报告与阅卷质量分离；AI/OCR 运维指标不混入学情分析。

Gradescope 和 Turnitin 的成熟界面值得参考的是任务布局、答案画布占比、Rubric 操作密度和持续阅卷动作，不是颜色或表面样式。UI 验收除响应式截图外，必须让目标教师完成真实任务并记录时间、错误和停顿原因。

## 11. 合规与治理工程要求

面向学校的学生身份、答卷、成绩和操作记录属于高敏感业务数据。正式试点前至少完成：

- 数据分类分级、最小权限、加密、备份、访问认证和安全审计。
- 明确数据控制者、处理目的、保存期限、删除/导出流程和跨组织访问边界。
- 模型输入、日志、评测样本和人工标注环境脱敏；外部 API 默认关闭并经过单独批准。
- 模型/数据许可证清单，尤其是具体权重 checkpoint、训练数据用途和再分发义务。
- 训练/评测数据的授权、标注规则、质量抽检和人员保密要求。
- 学生成绩改动、发布、下载和管理员敏感操作的不可抵赖审计。

《网络数据安全管理条例》已对分类分级、加密、备份、访问控制和安全认证提出要求；教育部相关标准与学籍管理文件强调学生信息安全；生成式 AI 规则还涉及合法数据来源、个人信息处理和标注质量。项目应在试点学校、部署方式和数据流明确后由合格法务/数据保护负责人做适用性评估，本报告不代替法律结论。

## 12. 当前最应该做什么

严格顺序如下：

1. **不新增 Story，实现并批准 STORY-056 剩余四个阻断项。**
2. **冻结“STORY-057 直接做主观题 AI”的旧顺序。**先审阅本报告并把后续路线拆成可验收 Story。
3. **启动数据与场景发现。**确认目标考试风险、年级/学科/题型、规模、扫描设备、部署硬件和数据授权；这些当前未知，不能靠猜测补齐。
4. **优先规格化 STORY-057 打印包与采集完整性。**它保护所有下游结果，是最早的高风险不变量。
5. **并行准备 STORY-058 的数据治理和标注协议，但在 Story 获批前不实现模型。**

在以下证据出现前，不应对外承诺“手写主观题自动阅卷可用”：

- 本项目授权真实答卷冻结集；
- 双人评分和仲裁真值；
- 分题型、分扫描条件的识别与评分报告；
- 不确定样本人工召回和严重错误/false-zero 分析；
- 影子运行与持续质量监控；
- 学校批准的适用范围和人工兜底流程。

## 13. 需要产品负责人确认但本报告不擅自假定的事实

以下信息会改变技术路线和验收阈值，目前仓库不能给出确定答案：

- 首发是日常作业、校内考试、区域统考，还是高风险正式考试。
- 首发年级、学科、题型和手写/公式/图形占比。
- 单场学生数、总页数、扫描窗口和允许人工复核比例。
- 学校现有扫描仪型号、DPI、网络条件和客户端操作系统。
- 私有化服务器的 CPU/GPU/内存预算与是否允许云适配器。
- 花名册/LMS/SIS/统一身份的实际厂商和协议。
- 学生端是校内账号、家长账号还是通过既有平台访问。
- 数据保存期限、跨校/跨区域部署和模型训练授权范围。

这些不是阻止工程前进的问题。STORY-057 可以先按可配置和可追溯原则设计，但真实试点范围、容量和模型门槛必须在上述事实确认后冻结。

## 14. 参考资料

### 成熟产品与考试流程

- Gradescope, [AI-assisted grading and answer groups](https://guides.gradescope.com/hc/en-us/articles/24838908062093-AI-assisted-grading-and-answer-groups)
- Gradescope, [Grading submissions with rubrics](https://guides.gradescope.com/hc/en-us/articles/22249389005709-Grading-submissions-with-rubrics)
- Gradescope, [Anonymous Grading](https://guides.gradescope.com/hc/en-us/articles/22020218026893-Anonymous-Grading)
- Gradescope, [Managing Regrade Requests](https://guides.gradescope.com/hc/en-us/articles/22237994239885-Managing-Regrade-Requests)
- Crowdmark, [Matching booklets to students](https://www.crowdmark.com/help/matching-booklets-to-students/)
- Crowdmark, [Creating an administered assessment](https://www.crowdmark.com/help/creating-an-administered-assessment/)
- Crowdmark, [Scanning assessments](https://www.crowdmark.com/help/scanning-assessments/)
- Crowdmark, [Ready to grade](https://www.crowdmark.com/help/ready-to-grade/)
- Crowdmark, [How Crowdmark works with LTI 1.3](https://www.crowdmark.com/help/how-does-crowdmark-work-with-lti-1-3/)
- RM, [RM Assessor e-marking](https://www.rm.com/assessment/services/e-marking)
- RM, [SEAB e-marking case study](https://www.rm.com/assessment/case-studies/seab-e-marking)
- Cambridge International, [Assessment standards](https://www.cambridgeinternational.org/about-us/our-standards/assessment-standards/)
- Cambridge International, [Code of Practice](https://www.cambridgeinternational.org/Images/416992-code-of-practice.pdf)
- Cambridge International, [AI-assisted marking for Digital Mocks](https://www.cambridgeinternational.org/programmes-and-qualifications/developing-digital-exams/digital-mocks-service/new-development-for-digital-mock-exams/)

### 阅卷质量与心理测量

- ETS, [Best Practices for Constructed-Response Scoring](https://www.ets.org/research/policy_research_reports/publications/report/2022/kgpl.html)
- ETS, [Calibration and Scale Drift in Rater-Mediated Assessment](https://www.ets.org/research/policy_research_reports/publications/report/2019/kaab.html)
- ETS, [Monitoring Human and Automated Scoring](https://www.ets.org/research/policy_research_reports/publications/report/2014/jsek.html)
- ETS, [Agreement Statistics and Cohen's Kappa](https://www.ets.org/research/policy_research_reports/publications/report/2000/iazi.html)
- ETS, [Providing Feedback to Raters](https://www.ets.org/research/policy_research_reports/publications/report/2019/kbfc.html)

### OCR、手写、公式和 AI 评测

- PaddleOCR, [PP-OCRv5](https://www.paddleocr.ai/main/en/version3.x/algorithm/PP-OCRv5/PP-OCRv5.html)
- PaddleOCR, [Formula Recognition Pipeline](https://www.paddleocr.ai/main/en/version3.x/pipeline_usage/formula_recognition.html)
- PaddlePaddle, [PaddleOCR repository](https://github.com/PaddlePaddle/PaddleOCR)
- OpenDataLab, [UniMERNet](https://github.com/opendatalab/UniMERNet)
- Google Research, [MathWriting dataset](https://arxiv.org/abs/2404.10690)
- ACL 2026, [EduMARS: authentic Chinese K-12 handwritten responses](https://aclanthology.org/2026.findings-acl.466/)
- [Human-in-the-loop assessment of handwritten mathematics](https://arxiv.org/abs/2603.13083)
- [EDU-CIRCUIT-HW](https://arxiv.org/abs/2602.00095)
- ACL 2024, [LLM-Rubric](https://aclanthology.org/2024.acl-long.745/)
- QwenLM, [Qwen3-VL](https://github.com/QwenLM/Qwen3-VL)

### 确定性评分、扫描和集成

- STACK, [Answer Tests](https://docs.stack-assessment.org/en/Authoring/Answer_Tests/)
- STACK, [Equivalence](https://docs.stack-assessment.org/en/Authoring/Answer_Tests/Equivalence/)
- SymPy, [Parsing documentation and warning](https://docs.sympy.org/latest/modules/parsing.html)
- OpenCV, [Features2D + Homography](https://docs.opencv.org/master/d7/dff/tutorial_feature_homography.html)
- ZXing-C++, [Repository](https://github.com/zxing-cpp/zxing-cpp)
- NAPS2, [Repository and SDK](https://github.com/cyanfish/naps2)
- 1EdTech, [OneRoster 1.2](https://standards.1edtech.org/oneroster/specifications/standards/v1p2)
- Qdrant, [Multitenancy](https://qdrant.tech/documentation/tutorials/multiple-partitions/)
- FlagEmbedding, [BGE models](https://github.com/FlagOpen/FlagEmbedding)
- Meta, [DINOv2](https://github.com/facebookresearch/dinov2)

### AI 风险与国内数据治理

- NIST, [AI Risk Management Framework](https://airc.nist.gov/airmf-resources/airmf/)
- NIST, [AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- UNESCO, [Guidance for generative AI in education and research](https://www.unesco.org/en/articles/guidance-generative-ai-education-and-research?hub=67098)
- 中国政府网, [网络数据安全管理条例](https://app.www.gov.cn/govdata/gov/202409/30/520076/article.html)
- 教育部, [国家智慧教育平台个人信息保护通用要求相关标准](https://www.moe.gov.cn/srcsite/A16/s3342/202507/t20250731_1200912.html)
- 教育部, [中小学生学籍管理相关通知](https://www.moe.gov.cn/srcsite/A06/jcys_jyzb/202502/t20250207_1177647.html)
- 国家互联网信息办公室, [生成式人工智能服务管理暂行办法](https://www.cac.gov.cn/2023-07/13/c_1690898327029107.htm)

## 15. 最终判断

EduGrade 最有价值的基础已经存在：领域模型、版本化事实、Worker、图像处理、客观题引擎和人工阅卷台正在形成。但“成熟产品”的剩余工作不是收尾，而是从工程原型转向考试质量产品的第二阶段建设。

产品应把护城河建立在**真实数据、完整性对账、题型路由、阅卷质量运营、答案分组和选择性 AI**上。完成这些之前，继续扩展主观题模型、页面数量或宣传性指标都会稀释资源。完成这些之后，即使 AI 只做建议，产品也能先成为真实可用、可审计、能持续改进的阅卷系统；随后每一项自动化提升才有可信的测量基础。
