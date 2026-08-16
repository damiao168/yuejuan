# EduGrade Enterprise Stories

本目录用于按 Story 分步交付 EduGrade Enterprise。

> Story 文档是规划和实施记录，不是当前生产能力清单。当前能力、验证强度和未覆盖边界以 [`docs/verification-status.md`](../verification-status.md) 为事实源；`Approved` 只表示对应 Story 当时通过其约定验收，不等于当前版本 Production Ready。

## 执行规则

1. 每次只执行一个 Story。
2. Story 未完成验收前，不进入下一个 Story。
3. 每个 Story 必须按固定闭环推进：规划 Story、审阅规划、实现、审阅实现、修改实现、自审审批。
4. 每个 Story 完成后由 Codex 按验收标准自审自批，并留下审批记录。
5. 每个 Story 必须包含范围、非范围、修改文件、测试方式和验收标准。
6. 不允许用 mock/stub 冒充真实能力；需要 mock 时必须明确标记。
7. 每个 Story 完成后必须输出：
   - 修改摘要
   - 新增文件
   - 修改文件
   - 运行命令
   - 测试结果
   - 剩余风险
   - 下一步建议

## Story 内部闭环

每个 Story 的文档或审批记录必须能回答：

1. `Plan`：本 Story 要做什么，不做什么，验收证据是什么。
2. `Plan Review`：计划是否越界、是否遗漏显式需求、是否符合当前仓库实际。
3. `Implementation`：实际改了哪些文件，为什么这样改。
4. `Implementation Review`：逐条检查验收标准，指出不足。
5. `Fixes`：针对实现审阅发现的问题做修正。
6. `Approval`：修正后再次检查并给出批准/不批准结论。

## Story 顺序

| Story | 名称 | 状态 |
| --- | --- | --- |
| STORY-000 | 总控审阅与实施计划 | Done |
| STORY-001 | 企业级项目骨架 | Approved |
| STORY-002 | 企业级 PRD | Approved |
| STORY-003 | 企业级技术架构文档 | Approved |
| STORY-004 | 数据库模型设计 | Approved |
| STORY-005 | 后端基础服务骨架 | Approved |
| STORY-006 | 认证与权限系统 | Approved |
| STORY-007 | 组织、学校、班级、学生管理 | Approved |
| STORY-008 | 考试管理模块 | Approved |
| STORY-009 | 试卷与题目配置模块 | Approved |
| STORY-010 | 文件上传与对象存储 | Approved |
| STORY-011 | 答卷采集与 Submission | Approved |
| STORY-012 | OCR 服务接口与任务队列 | Approved |
| STORY-013 | 答题区域切分模块 | Approved |
| STORY-014 | 多智能体 Orchestrator | Approved |
| STORY-015 | 客观题与填空题判分 | Approved |
| STORY-016 | 主观题 AI 评分接口 | Approved |
| STORY-017 | 证据校验 Agent | Approved |
| STORY-018 | 人工复核与阅卷工作流 | Approved |
| STORY-019 | 双评与仲裁模块 | Approved |
| STORY-020 | 最终成绩与成绩发布 | Approved |
| STORY-021 | 申诉流程 | Approved |
| STORY-022 | 学情报告 | Approved |
| STORY-023 | Web 管理后台基础框架 | Approved |
| STORY-024 | Web 考试管理页面 | Approved |
| STORY-025 | Web 试卷与 Rubric 配置页面 | Approved |
| STORY-026 | Web 答卷采集页面 | Approved |
| STORY-027 | Web 阅卷工作台 | Approved |
| STORY-028 | Web 双评仲裁页面 | Approved |
| STORY-029 | Web 成绩管理与发布页面 | Approved |
| STORY-030 | Web 学情报告页面 | Approved |
| STORY-031 | Web 申诉中心 | Approved |
| STORY-032 | Web 审计日志页面 | Approved |
| STORY-033 | Windows EXE 客户端骨架 | Approved |
| STORY-034 | EXE 扫描工作站功能 | Approved |
| STORY-035 | EXE 离线阅卷基础功能 | Approved |
| STORY-036 | UI 设计规范文档 | Approved |
| STORY-037 | Docker Compose 私有化部署 | Approved |
| STORY-038 | 可观测性与系统诊断 | Approved |
| STORY-039 | 安全加固 | Approved |
| STORY-040 | AI 评估集与评分质量测试 | Approved |
| STORY-041 | 全链路 E2E 测试 | Approved |
| STORY-042 | 企业级验收文档 | Approved |
| STORY-043 | 上线前代码审查 | Reviewed - blockers found |
| STORY-044 | 严重与高风险问题修复 | Approved |
| STORY-045 | 生产化差距清单与上线路线图 | Approved |
| STORY-046 | 生产管理员 Bootstrap 与会话安全加固 | Approved |
| STORY-047 | 数据库级多租户约束硬化 | Approved |
| STORY-048 | Web mock 页面退场与生产菜单收敛 | Approved |
| STORY-049 | 真实 OCR Worker 接入 | Approved |
| STORY-050 | 答卷图像质量检测与页面标准化 | Approved |
| STORY-051 | Agent Worker Runtime | Approved |
| STORY-052 | 生产部署 Runbook 与预生产验收 | Approved |
| STORY-053 | 产品外壳、角色首页、机构启用和考试工作区 | Approved |
| STORY-054 | 考试配置、试卷模板、题目、Rubric 和开考准备 | Approved |
| STORY-055 | 采集批次、扫描导入、页面处理、模板配准和自动切题 | Approved |
| STORY-056 | 客观题评分引擎和专业人工阅卷工作台 | Approved |
| STORY-057 | Grading Agent 生产契约（lab 智能体接入线） | Approved |
| STORY-058 | Grading Agent 服务实现（lab 智能体接入线） | Approved |
| STORY-059 | 平台接入与评分结果落库（lab 智能体接入线） | Approved |
| STORY-060 | 客观题自动化产能兑现与答卷完整性保障 | Software Accepted / Physical Validation Pending |
| STORY-061 | 多厂商原生模型治理、影子评测与策略中心 | Foundation Complete / Expansion Paused（已完成至 061C2） |
| STORY-062 | 阅卷主链可靠性与教师连续阅卷 | Software Accepted（不等于 Production Ready） |
| STORY-063 | 主观题 AI 影子批处理与人工复核衔接 | In Progress（幂等基础切片已完成） |
| STORY-064 | 阅卷质量中心与评分证据治理 | Planned |
| STORY-065 | 学生成绩、申诉与重发布闭环 | Planned |
| STORY-066 | 学校系统集成、离线采集与现场运维 | Planned |
| STORY-067 | 限量学校试点与发布门禁 | Planned |

## A 系列研究型实施路线

| Story | 名称 | 状态 |
| --- | --- | --- |
| STORY-A01 | [多学科 Assessment Domain Foundation](STORY-A01-assessment-domain.md) | Implemented |
| STORY-A02 | [Design System 与考试生命周期工作区](STORY-A02-design-system-workspace.md) | Implemented |
| STORY-A03 | [OpenAPI、Generated SDK 与契约门禁](STORY-A03-openapi-sdk.md) | Implemented |
| STORY-A04 | [Feature Architecture 与 Exam Workspace 重构](STORY-A04-feature-architecture-workspace.md) | Implemented |
| STORY-A05 | [专业阅卷工作台内核重构](STORY-A05-subject-aware-grading-workbench.md) | Implemented |
| STORY-A06 | [键盘优先、预取、批注与评语模板](STORY-A06-annotations-keyboard-prefetch.md) | Implemented |
| STORY-A07 | [相似答案分组与 Human-amplification Grading](STORY-A07-answer-groups.md) | Implemented |
| STORY-A08 | [Gold Papers / 标准卷体系](STORY-A08-gold-papers.md) | Implemented |
| STORY-A09 | [Grader Calibration / 阅卷员校准](STORY-A09-grader-calibration.md) | Implemented |
| STORY-A10 | [Seed Papers / 暗桩质量样本](STORY-A10-seed-quality.md) | Implemented |
| STORY-A11 | [Reviewer Drift Detection / 阅卷漂移检测](STORY-A11-grader-drift.md) | Implemented |
| STORY-A12 | [Back Marking / 受影响区间回溯重阅](STORY-A12-backmark.md) | Implemented |
| STORY-A13 | [Scoring Quality Dashboard / 评分质量总控台](STORY-A13-quality-dashboard.md) | Implemented |
| STORY-A14 | [AI Eligibility Engine / 题型级自动评分准入](STORY-A14-ai-eligibility.md) | Implemented |
| STORY-A15 | [Confidence Calibration 与 Risk-Coverage 门禁](STORY-A15-model-confidence-calibration.md) | Implemented |
| STORY-A16 | [分学科 Slice Evaluation 与 Response Difficulty](STORY-A16-slice-evaluation.md) | Implemented |
| STORY-A17 | [Human-AI Disagreement 与错误分类闭环](STORY-A17-ai-human-disagreement.md) | Implemented |
| STORY-A18 | [不可变 Score Release Version](STORY-A18-score-releases.md) | Implemented |
| STORY-A19 | [Regrade Workflow / 题目级重评](STORY-A19-question-regrade.md) | Implemented |
| STORY-A20 | [Release Gate / 成绩发布质量门禁](STORY-A20-release-gate.md) | Implemented |
| STORY-A21 | [Student Portal 1.0 / 成绩与反馈](STORY-A21-student-portal.md) | Implemented |
| STORY-A22 | [Question-level Appeal / 题目级申诉闭环](STORY-A22-question-appeal.md) | Implemented |
| STORY-A23 | [Desktop Durable Storage / 本地耐久安全存储](STORY-A23-desktop-durable-storage.md) | Implemented |
| STORY-A24 | [Offline Scan Spool 与断点续传](STORY-A24-offline-scan-spool.md) | Implemented |
| STORY-A25 | [Scanner Integration 与设备 Profile](STORY-A25-scanner-profile.md) | Implemented |
| STORY-A26 | [统一 Processing State 与异常中心](STORY-A26-processing-state.md) | Implemented |

## 数学答卷理解专项

| Story | 名称 | 状态 |
| --- | --- | --- |
| STORY-MATH-00 | [数学答卷理解契约与 MathBench](STORY-MATH-00-contract-benchmark.md) | Implemented（底座） |
| STORY-MATH-01 | [公式识别路由](STORY-MATH-01-formula-router.md) | Implemented / Model Evidence Pending |
| STORY-MATH-02 | [公式结构与受限 AST](STORY-MATH-02-formula-ast.md) | Implemented |
| STORY-MATH-03 | [空间关系图](STORY-MATH-03-spatial-graph.md) | Implemented / Learned Model Pending |
| STORY-MATH-04 | [符号验证服务](STORY-MATH-04-symbolic-verification.md) | Implemented |
| STORY-MATH-05 | [解题过程图](STORY-MATH-05-solution-graph.md) | Implemented / Learned Model Pending |
| STORY-MATH-06 | [Rubric 数学证据](STORY-MATH-06-rubric-evidence.md) | Implemented |
| STORY-MATH-07 | [教师数学证据工作台](STORY-MATH-07-evidence-workbench.md) | Implemented |
| STORY-MATH-08 | [分层试点门禁](STORY-MATH-08-pilot-gates.md) | Implemented / Real Gate Evidence Pending |

## 编号语义说明（2026-07-26）

`docs/deployment/production-readiness-roadmap.md` 中 V1.0 总控计划的 STORY-057～059（主观题 AI/质量中心/学生端）与实际实施的 STORY-057～059（lab 智能体接入三部曲）存在历史错位。**以本索引为准**：057～059 已被 lab 接入线占用并完成；总控计划中对应的能力（阅卷质量中心、学生端/申诉/报告）顺延至 STORY-061 之后重新编号。规划文档中引用旧编号处以本表为准，不再回改历史文档。

STORY-061 现用于多厂商原生模型治理、影子评测与策略中心；历史文档中曾将“正式发布门禁”或其他能力标为 STORY-061 的引用继续顺延，后续单独编号。

STORY-053 之后按 V1.0 正式产品交付总控任务继续推进，不提前宣称 Production Ready。

### 当前编号映射（2026-08-01）

为避免历史路线图与实际交付编号混淆，当前实施顺序固定为：

- STORY-062：阅卷主链可靠性与教师连续阅卷（软件已验收）；
- STORY-063：主观题 AI 影子批处理与人工复核衔接；
- STORY-064：阅卷质量中心与评分证据治理；
- STORY-065：学生成绩、申诉与重发布闭环；
- STORY-066：学校系统集成、离线采集与现场运维；
- STORY-067：限量学校试点与发布门禁。

旧路线图中将上述能力标作 STORY-059～064 的内容属于历史编号，不代表当前 Story 状态；新增实现、验收和审批记录均以本表为准。
