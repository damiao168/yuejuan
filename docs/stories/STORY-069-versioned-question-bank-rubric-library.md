# STORY-069：版本化题库与 Rubric 库

状态：In Progress（069A 核心草稿、069B 发布与考试复制、069C metadata/ACL/搜索、069D 历史题精选入库 Approved；069E～069F Planned）。优先级：P0。所属阶段：考试内容资产。

## 问题与目标

现有 paper.Question 是考试实例，历史题目与评分标准不能作为独立资产审核和复用。建立上游 Question Bank，以稳定 Item 身份承载不可变版本，指定版本复制进入考试，继续使用既有 ready、阅卷和发布链路。

## 依赖与范围

复用 paper、assessment、auth、files、Score Release 与命令回执。按以下顺序分别实施和验收：

| 子 Story | 范围 |
| --- | --- |
| [069A](STORY-069A-question-bank-core-item-version.md) | Bank、Item、draft Version、基础权限与草稿界面 |
| [069B](STORY-069B-answer-rubric-publish-workflow.md) | Answer/Solution/Rubric 版本、审核发布、指定版本用于考试 |
| [069C](STORY-069C-metadata-acl-search.md) | 受控 metadata、完整题库 ACL、搜索 |
| [069D](STORY-069D-existing-exam-bank-import.md) | 精选历史考试冻结事实入库 |
| [069E](STORY-069E-paper-import-bank.md) | 已有解析候选人工对账后入库 |
| [069F](STORY-069F-usage-psychometric-feedback.md) | 版本级 usage、发布版本统计回流 |

Item 不保存内容；ItemVersion 固定题干、选项、默认分值、archetype、知识点、metadata 与评分/附件版本。通用 Rubric 模板独立版本化，题目绑定精确模板版本并生成自己的评分事实。任何子表改动同样受发布锁约束。

## 非范围

- 不替换现有 Question、AnswerKey、Rubric、Assessment Snapshot 或发布门禁。
- 不自动迁移所有历史考试题，不自动审核发布 AI 候选。
- 不包括组卷 solver、QTI、IRT、跨租户题库交易或公开题库市场。

## 预计修改文件

拟新增 `services/api-gateway/internal/questionbank/` 与分切片迁移、`apps/web-admin/src/features/question-bank/`、`docs/api/question-bank.md`。按子 Story 修改现有 `internal/paper/`、`internal/assessment/`、`internal/auth/`、`internal/server/`、OpenAPI 与生成 SDK。表按切片增加，不能在 069A 一次建全套未来表。

## 测试方式与验收标准

以子 Story 的定向 Go、真实 PostgreSQL、OpenAPI/SDK、Web 类型构建及有限浏览器场景为证据。必须验收：完整 bundle 不可变、权限隔离、指定版本 materialize 全事务、旧 ready 考试不随题库改变、精选导入幂等、重发布统计不重复计样本。

主 Story 完成要求 069A～069F 各有实现审阅、修正、批准记录，且串通“历史题入库 → 发布 → 用于新考试 → ready → 已发布统计回流”。没有真实模型或学校证据时，标明软件验收范围。

## 规划审阅与实施记录

规划已核对考试实例边界与[总路线图](../prd/assessment-platform-roadmap.md)不变量；新增独立解析入口的工作归 069E，不在 Core 偷换 examID。069A～069D 已分别留下实现、实现审阅、修正和 Approval 证据；069D 从新 readiness 的不可变内容 companion 读取，串通历史考试题入库、独立审核发布、用于新考试和原 snapshot 追溯。069E～069F 尚未完成，因此本 Story 仍为 In Progress。
