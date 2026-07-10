# EduGrade Enterprise 数据库模型设计

## 1. 当前阶段

本文件属于 `STORY-004 数据库模型设计`。当前仓库尚未实现后端服务，也没有数据库迁移目录，因此本 Story 只交付数据库设计文档，不生成 migration 文件。

目标数据库：PostgreSQL。

## 2. 全局设计原则

- 所有核心业务表必须支持 `tenant_id`。
- 关键表必须包含 `created_at`、`updated_at`、`deleted_at`。
- 分数使用 `NUMERIC(8,2)` 或更明确的 `NUMERIC(p,s)`，禁止使用 float。
- 状态使用受控枚举值或受控文本，状态流转由服务层校验。
- 学生身份、成绩、答卷、申诉和私密备注属于敏感数据。
- 文件只保存元数据和对象存储 key，不把二进制文件直接存入数据库。
- AI 输出必须保存 `model_version_id`、`prompt_version_id`、`rubric_version_id`。
- 改分、发布、导出、Rubric 修改和权限变化必须写 `audit_log`。
- 删除默认软删除，使用 `deleted_at`。
- 数据库层建议增加关键 check constraint，服务层仍必须做业务校验。
- 审计日志是例外表，不提供普通业务意义上的 `updated_at` 和 `deleted_at`。

## 3. 通用字段

除特别说明外，核心表包含：

```sql
id UUID PRIMARY KEY,
tenant_id UUID NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
deleted_at TIMESTAMPTZ
```

组织类表还可包含：

```sql
created_by UUID,
updated_by UUID
```

## 4. 实体关系概览

```text
tenant
  -> school -> campus
  -> school -> grade -> school_class -> student
  -> app_user -> user_role -> role -> role_permission -> permission
  -> course
  -> exam -> exam_class -> school_class
  -> exam -> exam_paper -> file_asset
  -> exam -> question -> question_rubric -> rubric_version
  -> exam -> question -> question_answer_key
  -> exam -> grading_policy
  -> exam -> submission -> submission_page -> file_asset
  -> submission -> answer_segment -> ocr_result
  -> answer_segment -> ai_grade
  -> answer_segment -> human_grade
  -> answer_segment -> final_grade
  -> answer_segment -> review_task
  -> answer_segment -> arbitration_task
  -> final_grade -> appeal
  -> exam -> report -> file_asset
  -> audit_log
```

`role_permission` 不在 35 个核心实体清单中，但 RBAC 落地必须有角色与权限的绑定表；本设计将它作为支撑表列出，后续 migration 应一并实现。

## 5. 核心实体总览

| 序号 | 表名 | 职责 |
| --- | --- | --- |
| 1 | tenant | 租户/教育集团 |
| 2 | school | 学校 |
| 3 | campus | 校区 |
| 4 | grade | 年级 |
| 5 | class | 班级 |
| 6 | user | 系统用户 |
| 7 | role | 角色 |
| 8 | permission | 权限 |
| 9 | user_role | 用户角色绑定 |
| 10 | student | 学生 |
| 11 | course | 学科/课程 |
| 12 | exam | 考试/作业 |
| 13 | exam_class | 考试适用班级 |
| 14 | exam_paper | 试卷文件与版本 |
| 15 | question | 题目 |
| 16 | question_rubric | Rubric 配置 |
| 17 | question_answer_key | 标准答案 |
| 18 | grading_policy | 阅卷策略 |
| 19 | submission | 答卷 |
| 20 | submission_page | 答卷页 |
| 21 | answer_segment | 单题答案段 |
| 22 | ocr_result | OCR 结果 |
| 23 | ai_grade | AI 建议分 |
| 24 | human_grade | 人工评分 |
| 25 | final_grade | 最终分 |
| 26 | review_task | 人工复核任务 |
| 27 | arbitration_task | 仲裁任务 |
| 28 | appeal | 申诉 |
| 29 | report | 报告 |
| 30 | audit_log | 审计日志 |
| 31 | model_version | 模型版本 |
| 32 | prompt_version | Prompt 版本 |
| 33 | rubric_version | Rubric 版本 |
| 34 | file_asset | 文件元数据 |
| 35 | notification | 通知 |

## 6. 组织与权限模型

### 6.1 tenant

```sql
CREATE TABLE tenant (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  status TEXT NOT NULL,
  license_plan TEXT,
  settings JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (code)
);
```

说明：`tenant.tenant_id` 等于 `tenant.id`，用于统一租户过滤约定。实际 migration 建议增加 `CHECK (tenant_id = id)`。

### 6.2 school

```sql
CREATE TABLE school (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  status TEXT NOT NULL,
  settings JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code)
);
```

### 6.3 campus

```sql
CREATE TABLE campus (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID NOT NULL,
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, school_id, code)
);
```

### 6.4 grade

```sql
CREATE TABLE grade (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID NOT NULL,
  name TEXT NOT NULL,
  level_no INT,
  academic_year TEXT,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

### 6.5 class

`class` 是 SQL 关键字风险较高，实际 migration 可使用 `school_class`。产品模型中仍称 class。

```sql
CREATE TABLE school_class (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID NOT NULL,
  grade_id UUID NOT NULL,
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  status TEXT NOT NULL,
  homeroom_teacher_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, grade_id, code)
);
```

### 6.6 user

`user` 也是关键字风险较高，实际 migration 可使用 `app_user`。

```sql
CREATE TABLE app_user (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID,
  username TEXT NOT NULL,
  display_name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  email TEXT,
  phone TEXT,
  status TEXT NOT NULL,
  last_login_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, username)
);
```

### 6.7 role

```sql
CREATE TABLE role (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  scope_type TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code)
);
```

### 6.8 permission

```sql
CREATE TABLE permission (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  resource TEXT NOT NULL,
  action TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code)
);
```

### 6.9 role_permission

```sql
CREATE TABLE role_permission (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  role_id UUID NOT NULL,
  permission_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, role_id, permission_id)
);
```

### 6.10 user_role

```sql
CREATE TABLE user_role (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  user_id UUID NOT NULL,
  role_id UUID NOT NULL,
  data_scope JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, user_id, role_id)
);
```

### 6.11 student

```sql
CREATE TABLE student (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID NOT NULL,
  class_id UUID NOT NULL,
  student_no TEXT NOT NULL,
  name TEXT NOT NULL,
  gender TEXT,
  status TEXT NOT NULL,
  sensitive_profile JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, school_id, student_no)
);
```

### 6.12 course

```sql
CREATE TABLE course (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  subject TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code)
);
```

## 7. 考试、试卷与评分标准

### 7.1 exam

```sql
CREATE TABLE exam (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID NOT NULL,
  course_id UUID,
  name TEXT NOT NULL,
  subject TEXT NOT NULL,
  exam_type TEXT NOT NULL,
  total_score NUMERIC(8,2) NOT NULL,
  status TEXT NOT NULL,
  grading_mode TEXT NOT NULL,
  appeal_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  publish_policy TEXT NOT NULL,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

状态建议：

```text
draft -> configured -> collecting -> grading -> reviewing -> finalized -> published -> archived
```

### 7.2 exam_class

```sql
CREATE TABLE exam_class (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  class_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, class_id)
);
```

### 7.3 exam_paper

```sql
CREATE TABLE exam_paper (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  file_asset_id UUID NOT NULL,
  version_no INT NOT NULL,
  status TEXT NOT NULL,
  uploaded_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, version_no)
);
```

### 7.4 question

```sql
CREATE TABLE question (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  exam_paper_id UUID,
  question_no TEXT NOT NULL,
  question_type TEXT NOT NULL,
  score NUMERIC(8,2) NOT NULL,
  stem TEXT,
  knowledge_points JSONB NOT NULL DEFAULT '[]',
  answer_area JSONB,
  sort_order INT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, question_no)
);
```

### 7.5 question_rubric

```sql
CREATE TABLE question_rubric (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  question_id UUID NOT NULL,
  rubric_version_id UUID NOT NULL,
  status TEXT NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  points JSONB NOT NULL DEFAULT '[]',
  deductions JSONB NOT NULL DEFAULT '[]',
  examples JSONB NOT NULL DEFAULT '[]',
  created_by UUID NOT NULL,
  approved_by UUID,
  approved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

状态建议：

```text
draft -> pending_review -> approved -> locked
```

### 7.6 question_answer_key

```sql
CREATE TABLE question_answer_key (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  question_id UUID NOT NULL,
  answer_version TEXT NOT NULL,
  standard_answer JSONB NOT NULL,
  equivalent_answers JSONB NOT NULL DEFAULT '[]',
  tolerance JSONB NOT NULL DEFAULT '{}',
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

### 7.7 grading_policy

```sql
CREATE TABLE grading_policy (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  question_id UUID,
  mode TEXT NOT NULL,
  double_mark_threshold NUMERIC(8,2),
  ai_confidence_threshold NUMERIC(5,4) NOT NULL DEFAULT 0.8000,
  ocr_confidence_threshold NUMERIC(5,4) NOT NULL DEFAULT 0.8500,
  sampling_rate NUMERIC(5,4),
  settings JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

## 8. 答卷、OCR 与切分

### 8.1 submission

```sql
CREATE TABLE submission (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  student_id UUID NOT NULL,
  anonymous_code TEXT NOT NULL,
  status TEXT NOT NULL,
  page_count INT NOT NULL DEFAULT 0,
  quality_flags JSONB NOT NULL DEFAULT '[]',
  submitted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_id, student_id),
  UNIQUE (tenant_id, exam_id, anonymous_code)
);
```

状态建议：

```text
uploaded -> preprocessing -> ocr_pending -> ocr_done -> segmentation_pending -> segmented -> grading_pending -> grading_done -> review_pending -> finalized
```

### 8.2 submission_page

```sql
CREATE TABLE submission_page (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  page_no INT NOT NULL,
  file_asset_id UUID NOT NULL,
  quality_status TEXT NOT NULL,
  quality_flags JSONB NOT NULL DEFAULT '[]',
  image_metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, submission_id, page_no)
);
```

### 8.3 answer_segment

```sql
CREATE TABLE answer_segment (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  question_id UUID NOT NULL,
  page_id UUID,
  image_asset_id UUID,
  segment_area JSONB,
  ocr_text TEXT,
  ocr_confidence NUMERIC(5,4),
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, submission_id, question_id)
);
```

### 8.4 ocr_result

```sql
CREATE TABLE ocr_result (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  submission_page_id UUID,
  answer_segment_id UUID,
  text TEXT NOT NULL,
  bbox JSONB NOT NULL,
  confidence NUMERIC(5,4) NOT NULL,
  engine_name TEXT NOT NULL,
  engine_version TEXT NOT NULL,
  mock BOOLEAN NOT NULL DEFAULT FALSE,
  needs_manual_review BOOLEAN NOT NULL DEFAULT FALSE,
  raw_payload JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

## 9. 阅卷、复核、仲裁与最终分

### 9.1 ai_grade

```sql
CREATE TABLE ai_grade (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  model_version_id UUID NOT NULL,
  prompt_version_id UUID NOT NULL,
  rubric_version_id UUID NOT NULL,
  suggested_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  confidence NUMERIC(5,4) NOT NULL,
  matched_points JSONB NOT NULL DEFAULT '[]',
  missing_points JSONB NOT NULL DEFAULT '[]',
  evidence JSONB NOT NULL DEFAULT '[]',
  risk_flags JSONB NOT NULL DEFAULT '[]',
  needs_human_review BOOLEAN NOT NULL,
  mock BOOLEAN NOT NULL DEFAULT FALSE,
  raw_output JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

### 9.2 human_grade

```sql
CREATE TABLE human_grade (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  reviewer_id UUID NOT NULL,
  review_task_id UUID,
  score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  rubric_marks JSONB NOT NULL DEFAULT '[]',
  comment TEXT,
  private_note TEXT,
  reason TEXT,
  grade_round TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

`grade_round` 示例：`single`、`first_mark`、`second_mark`、`appeal_review`。

### 9.3 final_grade

```sql
CREATE TABLE final_grade (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  student_id UUID NOT NULL,
  question_id UUID NOT NULL,
  final_score NUMERIC(8,2) NOT NULL,
  max_score NUMERIC(8,2) NOT NULL,
  source TEXT NOT NULL,
  source_grade_id UUID,
  status TEXT NOT NULL,
  confirmed_by UUID,
  confirmed_at TIMESTAMPTZ,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, answer_segment_id)
);
```

状态建议：

```text
draft -> confirmed -> published -> appealed -> archived
```

### 9.4 review_task

```sql
CREATE TABLE review_task (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  assigned_to UUID,
  status TEXT NOT NULL,
  priority INT NOT NULL DEFAULT 0,
  due_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

### 9.5 arbitration_task

```sql
CREATE TABLE arbitration_task (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  question_id UUID NOT NULL,
  answer_segment_id UUID NOT NULL,
  first_grade_id UUID NOT NULL,
  second_grade_id UUID NOT NULL,
  score_difference NUMERIC(8,2) NOT NULL,
  threshold NUMERIC(8,2) NOT NULL,
  assigned_to UUID,
  final_score NUMERIC(8,2),
  decision_reason TEXT,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

## 10. 申诉、报告、审计、版本、文件、通知

### 10.1 appeal

```sql
CREATE TABLE appeal (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  student_id UUID NOT NULL,
  question_id UUID,
  final_grade_id UUID,
  reason TEXT NOT NULL,
  status TEXT NOT NULL,
  handler_id UUID,
  resolution TEXT,
  score_adjusted BOOLEAN NOT NULL DEFAULT FALSE,
  old_score NUMERIC(8,2),
  new_score NUMERIC(8,2),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

状态建议：

```text
submitted -> under_review -> need_more_info -> accepted -> rejected -> closed
```

### 10.2 report

```sql
CREATE TABLE report (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  exam_id UUID NOT NULL,
  report_type TEXT NOT NULL,
  scope_type TEXT NOT NULL,
  scope_id UUID,
  status TEXT NOT NULL,
  file_asset_id UUID,
  data JSONB NOT NULL DEFAULT '{}',
  generated_by UUID,
  generated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

### 10.3 audit_log

```sql
CREATE TABLE audit_log (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  actor_id UUID,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id UUID,
  before_value JSONB,
  after_value JSONB,
  reason TEXT,
  ip_address TEXT,
  user_agent TEXT,
  request_id TEXT,
  hash_prev TEXT,
  hash_current TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

审计日志不设计 `updated_at` 和 `deleted_at`，避免普通业务语义上的修改和删除。归档由独立审计策略处理。

### 10.4 model_version

```sql
CREATE TABLE model_version (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  provider TEXT NOT NULL,
  model_name TEXT NOT NULL,
  version TEXT NOT NULL,
  deployment_type TEXT NOT NULL,
  status TEXT NOT NULL,
  config JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, provider, model_name, version)
);
```

### 10.5 prompt_version

```sql
CREATE TABLE prompt_version (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  agent_type TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, name, version)
);
```

### 10.6 rubric_version

```sql
CREATE TABLE rubric_version (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  question_id UUID NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  created_by UUID NOT NULL,
  approved_by UUID,
  approved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, question_id, version)
);
```

### 10.7 file_asset

```sql
CREATE TABLE file_asset (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  school_id UUID,
  exam_id UUID,
  submission_id UUID,
  owner_type TEXT NOT NULL,
  owner_id UUID,
  original_name TEXT NOT NULL,
  content_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  hash_sha256 TEXT NOT NULL,
  storage_bucket TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  visibility TEXT NOT NULL,
  uploaded_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, hash_sha256, owner_type, owner_id)
);
```

### 10.8 notification

```sql
CREATE TABLE notification (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  recipient_id UUID NOT NULL,
  type TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  data JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
```

## 11. 状态目录

状态值应由服务层集中定义，数据库可用 check constraint 或 lookup table 约束。第一版建议先使用受控文本和服务层校验。

| 对象 | 状态 |
| --- | --- |
| tenant/school/user/student | active, disabled, archived |
| exam | draft, configured, collecting, grading, reviewing, finalized, published, archived |
| question_rubric | draft, pending_review, approved, locked |
| submission | uploaded, preprocessing, ocr_pending, ocr_done, segmentation_pending, segmented, grading_pending, grading_done, review_pending, finalized |
| submission_page | ok, blurry, missing, duplicate, barcode_failed, needs_manual_review |
| answer_segment | pending, segmented, needs_manual_adjustment, failed |
| review_task | pending, assigned, in_progress, submitted, cancelled |
| arbitration_task | pending, assigned, submitted, cancelled |
| final_grade | draft, confirmed, published, appealed, archived |
| appeal | submitted, under_review, need_more_info, accepted, rejected, closed |
| report | pending, generating, succeeded, failed |
| notification | unread, read, archived |

## 12. 关键约束建议

```sql
ALTER TABLE exam ADD CONSTRAINT ck_exam_total_score_nonnegative CHECK (total_score >= 0);
ALTER TABLE question ADD CONSTRAINT ck_question_score_nonnegative CHECK (score >= 0);
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_score_range CHECK (suggested_score >= 0 AND suggested_score <= max_score);
ALTER TABLE human_grade ADD CONSTRAINT ck_human_score_range CHECK (score >= 0 AND score <= max_score);
ALTER TABLE final_grade ADD CONSTRAINT ck_final_score_range CHECK (final_score >= 0 AND final_score <= max_score);
ALTER TABLE grading_policy ADD CONSTRAINT ck_ai_confidence_range CHECK (ai_confidence_threshold >= 0 AND ai_confidence_threshold <= 1);
ALTER TABLE grading_policy ADD CONSTRAINT ck_ocr_confidence_range CHECK (ocr_confidence_threshold >= 0 AND ocr_confidence_threshold <= 1);
ALTER TABLE ocr_result ADD CONSTRAINT ck_ocr_confidence_range CHECK (confidence >= 0 AND confidence <= 1);
ALTER TABLE ai_grade ADD CONSTRAINT ck_ai_confidence_range CHECK (confidence >= 0 AND confidence <= 1);
```

外键建议全部带 `tenant_id` 业务校验。PostgreSQL 可使用复合唯一键和复合外键进一步约束，但实现复杂度较高，第一版至少必须在服务层强制验证关联对象属于同一 tenant。

## 13. 敏感字段与脱敏

| 表 | 敏感字段 | 默认展示 |
| --- | --- | --- |
| app_user | email, phone | 按权限展示 |
| student | name, student_no, sensitive_profile | 阅卷员默认隐藏 |
| submission | student_id, anonymous_code | 阅卷员只看 anonymous_code |
| ocr_result | text | 仅授权阅卷/复核角色可见 |
| human_grade | private_note | 仅提交人和授权管理员可见 |
| final_grade | final_score | 学生本人、授权教师和管理员可见 |
| appeal | reason, resolution | 学生本人和授权处理人可见 |
| audit_log | before_value, after_value | 审计员和授权管理员可见 |

## 14. 改分追溯

改分不得直接覆盖历史记录。

建议规则：

- `human_grade` 每次提交产生新记录。
- `final_grade` 保存当前最终分和来源。
- 改分原因保存在 `human_grade.reason`、`appeal.resolution` 或审计日志。
- `audit_log.before_value` 和 `audit_log.after_value` 保存分数变化。
- 发布后改分必须关联申诉、仲裁或管理员审批。

## 15. 成绩发布状态

考试发布前必须检查：

- 所有 `submission` 已完成阅卷或标记例外。
- 所有 `arbitration_task` 已完成。
- 所有 `final_grade` 已生成。
- 无未处理 OCR 失败或切分失败。
- Rubric 已 approved/locked。
- 无分数超过满分。

发布后：

- `exam.status = published`。
- `final_grade.status = published`。
- 核心配置只读。
- 改分必须走申诉或审批流程。

## 16. 双评与仲裁

双评通过 `human_grade.grade_round` 区分一评和二评。

仲裁触发：

```text
ABS(first_mark.score - second_mark.score) > grading_policy.double_mark_threshold
```

触发后创建 `arbitration_task`，仲裁完成后写入 `final_grade`，并写 `audit_log`。

## 17. 学生申诉

申诉必须关联：

- exam_id。
- student_id。
- question_id，可选。
- final_grade_id，可选。
- reason。

处理结果必须写：

- status。
- handler_id。
- resolution。
- old_score/new_score，当涉及改分。
- audit_log。

## 18. 关键索引建议

```sql
CREATE INDEX idx_school_tenant ON school (tenant_id);
CREATE INDEX idx_student_class ON student (tenant_id, class_id);
CREATE INDEX idx_role_permission_role ON role_permission (tenant_id, role_id);
CREATE INDEX idx_exam_school_status ON exam (tenant_id, school_id, status);
CREATE INDEX idx_question_exam ON question (tenant_id, exam_id);
CREATE INDEX idx_submission_exam_status ON submission (tenant_id, exam_id, status);
CREATE INDEX idx_submission_anonymous ON submission (tenant_id, exam_id, anonymous_code);
CREATE INDEX idx_answer_segment_submission ON answer_segment (tenant_id, submission_id);
CREATE INDEX idx_answer_segment_question ON answer_segment (tenant_id, question_id);
CREATE INDEX idx_ocr_segment ON ocr_result (tenant_id, answer_segment_id);
CREATE INDEX idx_ai_grade_segment ON ai_grade (tenant_id, answer_segment_id);
CREATE INDEX idx_review_task_assignee ON review_task (tenant_id, assigned_to, status);
CREATE INDEX idx_arbitration_assignee ON arbitration_task (tenant_id, assigned_to, status);
CREATE INDEX idx_final_grade_exam_student ON final_grade (tenant_id, exam_id, student_id);
CREATE INDEX idx_appeal_exam_status ON appeal (tenant_id, exam_id, status);
CREATE INDEX idx_audit_target ON audit_log (tenant_id, target_type, target_id);
CREATE INDEX idx_audit_actor_time ON audit_log (tenant_id, actor_id, created_at DESC);
CREATE INDEX idx_file_owner ON file_asset (tenant_id, owner_type, owner_id);
```

## 19. 数据隔离说明

- 服务端必须从认证上下文获取 tenant_id，不信任客户端传入的 tenant_id。
- 普通查询必须带 `tenant_id = current_tenant_id`。
- 平台级管理员跨租户查询必须走专门权限和审计。
- 对象存储路径必须包含 tenant_id。
- Qdrant payload 或 collection 必须包含 tenant_id。
- 审计日志必须包含 tenant_id。
- 测试必须覆盖租户逃逸失败场景。

## 20. Migration 生成说明

当前仓库没有后端数据库迁移目录，例如：

- `services/*/migrations`
- `services/*/db/migrations`
- `infra/migrations`

因此本 Story 不生成初始 SQL migration 文件。后续在 `STORY-005 后端基础服务骨架` 或专门迁移 Story 中创建迁移目录后，再把本设计转化为 PostgreSQL migration。
