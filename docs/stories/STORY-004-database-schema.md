# STORY-004 数据库模型设计

## 状态

Approved

## 目标

设计 EduGrade Enterprise 的企业级数据库模型，保存到 `docs/database/schema-design.md`。

## Plan

本 Story 只产出数据库模型设计文档，围绕 35 个核心实体逐表给出职责、关键字段、状态、分数类型、版本追溯、软删除和索引建议。

## Plan Review

- 计划未越界：不实现后端、不连接数据库、不创建 API。
- 当前仓库没有迁移目录，因此只说明不生成 migration 的原因。
- 原始需求要求支持多租户、双评仲裁、申诉、AI 版本和审计字段，计划已覆盖。
- 为了让后续实现更稳，设计文档还需要包含关系图、状态目录、约束建议和 RBAC 支撑表说明。

## Implementation

- 新增 `docs/database/schema-design.md`。
- 覆盖 35 个核心实体，补充 `role_permission` 作为 RBAC 支撑表。
- 说明通用字段、关系图、状态目录、关键约束、敏感字段、索引和数据隔离。
- 明确当前没有 migration 目录，因此不生成 SQL migration。

## Implementation Review

实现审阅命令已验证：

- 35 个核心表均有 `CREATE TABLE` 设计。
- 包含 `tenant_id`、时间戳、软删除、numeric 分数、AI 版本字段、审计字段。
- 包含状态目录、关键约束、数据隔离说明和 migration 生成说明。

## Fixes

审阅后补充：

- 实体关系概览。
- `role_permission` 支撑表。
- 分数和置信度 check constraint 建议。
- 敏感字段脱敏边界。

## 自审审批

审批文件：`docs/stories/STORY-004-approval.md`

## 范围

必须包含：

- tenant、school、campus、grade、class。
- user、role、permission、user_role。
- student、course。
- exam、exam_class、exam_paper、question、question_rubric、question_answer_key、grading_policy。
- submission、submission_page、answer_segment、ocr_result。
- ai_grade、human_grade、final_grade、review_task、arbitration_task。
- appeal、report、audit_log。
- model_version、prompt_version、rubric_version、file_asset、notification。

## 设计要求

- 所有核心表支持 tenant_id。
- 所有关键表包含 created_at、updated_at。
- 分数使用 decimal/numeric，不使用 float。
- AI 阅卷结果保留 model_version、prompt_version、rubric_version。
- 改分可追溯。
- 审计日志记录 actor、action、target、before、after、reason、ip、user_agent。
- 支持软删除。
- 支持成绩发布前后状态流转。
- 支持双评和仲裁。
- 支持学生申诉。
- 支持文件对象存储路径。
- 给出关键索引建议。
- 给出数据隔离说明。

## 非范围

- 不实现后端代码。
- 不实现数据库连接。
- 只有在项目已有后端数据库迁移目录时才生成 SQL migration。

## 验收标准

- 35 个核心实体全部覆盖。
- tenant_id、timestamps、soft delete、numeric score、AI 版本字段和审计字段均有明确设计。
- 明确当前是否生成 migration，以及原因。
