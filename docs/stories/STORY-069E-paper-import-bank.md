# STORY-069E：复用 Paper Import 的题库入库

状态：Planned。优先级：P0。父 Story：[069](STORY-069-versioned-question-bank-rubric-library.md)。依赖：069B/C/D 的导入与查重命令。

## 问题与目标

把现有 PDF/OCR/模型解析的题目、答案、解析、Rubric 候选复用于题库，不开发第二套 parser。先完成已有导入候选入库，再决定独立题库文档入口。

## 范围与顺序

1. **首验收边界：已有 Paper Import 候选入库。** 校验来源 exam/import 范围，固定 parse_run 与候选版本，复用 QuestionCandidate、AnswerCandidate、SolutionCandidate、RubricCandidate、SourceRefs、Confidence、Issues。教师明确选题并对账，解决缺题号、错配答案、缺评分点与未知知识点后调用 069D 公共 draft 创建/查重边界。
2. **后续独立文档入口。** 当前 [DocumentImportService.Start](../../services/api-gateway/internal/paper/document_import.go) 依赖 examID 与 paper.Store，不能直接用于无考试 Bank。先抽出共享 parse 输入/输出和任务执行接口，题库拥有独立 import job，上游 document→candidate 处理仍只有一套。不能传空 examID、伪造隐藏考试或直接复用会 commit 进 paper.Question 的事务。

- Parser 结果与人工调整分别版本化保存；标明 parse_run、模型/提示词版本、source refs 和 human_confirmed_fields；原候选不可被人工 edits 覆写。
- 低 confidence 不等于必错、高 confidence 不等于已审核；所有题库创建结果为 draft，发布仍走 069B。
- 来源文件生命周期遵从 files 的引用/保留要求，题干图片和附件进入题目事实时保存摘要及访问范围。用户只看来源位置和问题，不暴露模型凭据。
- 解析取消、失败或候选 revision 变化后不得提交旧预览；批量提交保存每候选 command receipt，重试不重复入库。
- 独立入口是否纳入本阶段扩展在首切片实现审阅后记录；第一阶段必需条件是已存在候选可人工对账入库，不以独立文档入口未做阻塞 070。

## 数据与 API 规划

增加来源 parse_run/candidate 映射与人工确认记录，拟提供 `POST /api/v1/question-bank/imports/from-paper-import/{importId}/preview|commit`（各路径登记），固定候选 IDs、source revision、target schema 与去重决策。独立 Bank document import 的 job 表/API 在共享边界设计审阅后再增加，不提前复制 paper import 全套表。

## 非范围

不另写 OCR/parser、不自动 publish、不把置信度当审核、不把独立解析能力标成已存在、不修改原导入已提交的考试事实。

## 预计修改文件

扩展 `internal/questionbank/` 候选 adapter 与导入记录，复用/按需要抽取 `internal/paper/document_import.go`、`parse_task_executor.go` 和候选契约；新增管理端候选对账到题库入口，OpenAPI/SDK 与定向测试。AI parser 本身只有共享协议确需调整时才修改。

## 测试方式

固定、显式标记的 parser 输出夹具验证候选映射、缺字段和来源；真实 PostgreSQL 验证 parse_run 绑定、旧 revision 拒绝、部分失败和重复 commit；既有 paper import/任务处理定向回归。真实 PDF/模型效果与模拟候选的事务验证分开记录。

## 验收标准

1. 同一现有 parse_run 的候选可进入 Bank draft，SourceRefs、Issues、Confidence 与人工确认记录保留。
2. 题目/答案/解析/Rubric 错配或未解决必需字段不能发布，确认后的字段变更重新审核。
3. stale/cancelled parse run 不得提交；重复候选 command 不重复生成 Item/Version，失败项可独立恢复。
4. 目标与来源授权分别校验，来源附件不泄露，模型凭据不存入候选或浏览器 DTO。
5. 入库不调用 paper commit 创建额外考试 Question；既有 PDF→考试对账流程保持可用。
6. 若交付独立文档入口，证明无需真实/伪造 examID、仅有一套 parse pipeline，并单独留下 API/Worker/持久化验收证据。

## 规划审阅与实施记录

规划已修正原方案“直接调用 DocumentImportService 即可支持独立题库导入”的隐含假设。当前实现、实现审阅、修正和 Approval 待证据；两步交付范围分别声明，不能冒充独立解析已实现。
