# STORY-070：考试蓝图与智能组卷

状态：Planned。优先级：P0。所属阶段：考试内容资产。

## 问题与目标

用显式考试规格决定题量、分值、覆盖和难度，输出可解释、可重放的候选。教师确认后复制固定题目版本进入草稿考试，最终题目有效性仍由现有 ready 门禁判断。

## 依赖与范围

依赖 069B 的已发布评分 bundle 与 materialize、069C 的 metadata/ACL、069F 的 usage；观测质量缺失时允许显式使用教师难度标签。

| 子 Story | 范围 | 阶段 |
| --- | --- | --- |
| [070A](STORY-070A-blueprint-dsl.md) | 版本化 Typed Constraint DSL、校验与编辑 | 第一阶段 |
| [070B](STORY-070B-deterministic-assembly.md) | Go 确定性组卷、约束报告、教师确认 | 第一阶段 |
| [070C](STORY-070C-exposure-control.md) | 曝光策略、预留和并发确认复验 | 第一阶段 |
| [070D](STORY-070D-parallel-forms.md) | 多份平行卷与重叠控制 | 后续 |
| [070E](STORY-070E-milp-solver.md) | Worker 求解器、MILP/CP-SAT 后端评估 | 后续 |
| [070F](STORY-070F-qti-import-export.md) | QTI 边界导入导出 | 后续，技术上不依赖 070E |

固定 blueprint_version、池快照、seed、solver_version、policy_version、操作预算和 constraint result。LLM 后续可生成蓝图草稿，最终选题和约束检查由类型化服务完成。

## 非范围

- 第一阶段不要求自然语言生成、平行卷、外部求解器或完整 QTI 支持。
- 不隐式改分、缩放 Rubric、放宽硬约束或从未发布 draft 抽题。
- 不对无校准样本的候选宣称 IRT 等值、测量信息一致或真实心理测量有效性。

## 预计修改文件

拟新增 `services/api-gateway/internal/testassembly/`、各切片迁移与 `docs/api/test-assembly.md`、`apps/web-admin/src/features/test-assembly/`。修改 questionbank materialize 接口、服务路由、OpenAPI、生成 SDK。外部 Worker 和 QTI adapter 分别到 070E/F 再增加。

## 测试方式与验收标准

固定池及 seed 重放清单和硬约束结果相同；整数分值无误差；未知 DSL 拒绝；权限和状态过滤先于选题。验证容量不足、搜索耗尽、超时和 stale candidate 的独立语义。确认必须重新校验并幂等，不通过的候选不能写考试题。

第一阶段出口为 070A/B/C 分别验收，完成曝光并发场景与题库到 ready 考试端到端证据。主 Story 后续范围尚未交付时标明“第一阶段软件已验收，070D/E/F 待实施”，不能直接把全部范围标为 Approved。

## 规划审阅与实施记录

规划已区分硬约束与软目标、统计缺失与标签难度、搜索失败与证明无解。按[总路线图](../prd/assessment-platform-roadmap.md)记录各切片 Plan Review、实现、审阅、修正和审批；当前产品实现未开始。
