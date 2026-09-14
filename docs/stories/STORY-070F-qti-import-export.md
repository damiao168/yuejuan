# STORY-070F：QTI 边界导入导出

状态：Planned（后续阶段）。优先级：P1。父 Story：[070](STORY-070-assessment-blueprint-smart-assembly.md)。依赖：069B/C/E；Test 导出依赖 070B，不依赖 070D/E。

## 问题与范围

用标准交换题目与组卷结果，内部继续采用 QuestionBankItem/Version、评分 bundle 和考试快照。声明首版 QTI 版本/profile 与支持矩阵，不以 XML 能读写宣称完整兼容。

- 首切片支持 QTI 3 的选择题和已选定简单文本响应映射，保留 identifier、interaction、response declaration、基础 scoring、metadata 与 package assets；具体 profile 在官方 schema 审阅后固定。
- 导入先验证包/manifest/schema，再生成候选和字段差异，版权未知、unsupported scoring 与无法表达的 Rubric 进入 Issues；人工对账后生成 draft，不能自动发布或丢弃评分语义。
- 内部 UUID 独立分配，外部 identifier 仅作为来源映射；同包重试按 package hash/import command 防重，不能把第三方 ID 当主键。
- 导出只取指定 published ItemVersion 或固定 assembly candidate，写明 unsupported 内部字段的处理（显式 extension 或阻断），不能悄悄降级 evidence requirements/部分分规则。
- 包资源读写限制路径、大小与数量，XML 禁用外部实体，媒体受生命周期与权限管理；保密题导出需内容/export 动作与用途策略。
- 往返测试核对题干、选项顺序、分值、标准/等价答案与声明的 scoring 子集，保存 loss report；复杂主观题和 Test 导出独立扩展。

## 非范围

不替换内部领域模型，不承诺 QTI 全部交互或认证，不无声简化 Rubric、不自动导入公开发布，不把此切片作为组卷第一阶段前置。

## 预计修改文件

拟新增 `internal/assessmentexchange/qti/` adapter、package import/export 映射与回执、`docs/api/qti-exchange.md` 和 profile 支持矩阵；复用 069E 候选/069B publish，修改管理端导入差异/导出入口、OpenAPI/SDK 与附件生命周期。

## 测试方式与验收标准

官方 schema 校验与合法/非法包夹具验证 manifest、response/scoring 映射、资产路径与 XXE 防护；真实 PostgreSQL 验证 draft、防重与权限；声明子集执行 import→publish→export→reimport 的语义往返。真实外部系统互操作另留证据。

验收要求支持矩阵和 loss report 明确，未支持 scoring 阻断/问题可见，分值与选项顺序往返不变，导入仅 draft，identifier 不覆盖内部主键，越权导出与危险资源拒绝。标准依据 [QTI 官方文档](https://www.1edtech.org/standards/qti/index)；实施时核验具体 schema/profile。

## 规划审阅与实施记录

规划已固定“边界 adapter + 支持子集 + 人工审核”的范围。实现、实现审阅、修正和 Approval 待证据；与 070E 无技术串行依赖。
