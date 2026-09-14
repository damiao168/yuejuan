# STORY-070E：外部优化求解器

状态：Planned（后续阶段）。优先级：P1。父 Story：[070](STORY-070-assessment-blueprint-smart-assembly.md)。依赖：070B/C；可增强 070D。

## 问题与范围

当复杂约束超出 Go 启发式的稳定求解能力，通过既有 Worker Runtime 增加外部优化后端，Go API 仍持有蓝图、任务与确认事实。

- 先用授权合成题池比较 Go baseline、OR-Tools CP-SAT、HiGHS/scipy MILP 的可行率、硬约束违例、目标偏差、运行时与部署代价，依据结果选择一个首版后端，不同时引入全部依赖。
- 任务输入固定 schema、score_units、blueprint/hash、pool hash、seed、backend/version、线程/操作预算、政策与 objective；任务只包含已授权内容必要字段，不发学生原始数据。
- 任务按 runtime lease/result publication 规则幂等，输出 feasible/optimal/infeasible/timeout/unknown 与 solver metadata；只有后端给出有效证明状态才称 infeasible/optimal，timeout 有可行 incumbent 时仍需逐条验证。
- Go 独立 evaluator 验证所有选题、版本、分值与硬约束，拒绝池外 ID、重复 Item 或篡改 hash；最终曝光/权限/目标 revision 由 Go 确认事务复核。
- 精确重放需固定容器/依赖、单线程/确定性模式与预算；后端不保证时仅承诺输入和约束可审计，标记 reproducibility 状态，不冒充候选清单必然相同。
- 后端失败可显式重新选择 Go solver 创建新 run，保存来源与差异；不覆盖失败 run 或悄悄 fallback。

## 非范围

不替换 API 事实源、不训练 LLM solver、不保证任意规模全局最优，不在没有 IRT 数据时加入伪造信息函数，不提前阻塞第一阶段。

## 预计修改文件

拟新增 `services/test-assembly-worker/`、容器与受控依赖/测试；修改 `internal/testassembly/` backend 契约与 runtime 接线、任务迁移/配置、Compose/部署 Runbook、OpenAPI/SDK。具体后端依赖版本在实施时核验官方文档与许可证。

## 测试方式与验收标准

固定基准池跑 backend 实验，记录求解状态与 objective；真实 Worker/Go/PostgreSQL 跑重放、lease 失效、超时、错误输出和确认复验。模型/solver 协议模拟器仅证明契约，至少有实际选定求解器运行结果。

验收要求两后端共享 DSL/evaluator，任何 hard 违例被 Go 拒绝；infeasible/optimal 语义与后端证据一致；worker 重试不重复结果；输入/依赖/复现状态完整；部署失败不阻断已有 Go 组卷；实际选择后端有比较证据。

## 规划审阅与实施记录

规划已把 CP-SAT 与 MILP 视为不同候选技术，先实验再定后端，保留 Go 验证与事务边界。实现、实验审阅、修正和 Approval 待证据；用户方案中的 ATA 论文需此阶段核对模型适用性。
