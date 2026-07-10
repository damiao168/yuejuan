# STORY-047 数据库级多租户约束硬化规格

## 目标

关闭 STORY-045 与生产化路线图中“多租户数据库级隔离不足”的 P0 风险。当前 API 层已经在关键路径按 `tenant_id` 做查询和权限判断，但大量 PostgreSQL 外键仍只引用父表 `id`，数据库无法阻止“子记录 tenant_id 属于 A、父记录 id 属于 B”的错误写入。

本 Story 的目标是在不大规模重构业务代码的前提下，为核心业务链路增加数据库级复合租户约束，使绕过 API 或 store bug 产生的跨租户父子引用在数据库层失败。

## 背景与依据

- PostgreSQL 支持外键引用被唯一约束覆盖的列组合。因此本轮采用 `(tenant_id, id)` 复合唯一约束加复合外键的路线。
- Row-Level Security 能提供读写纵深防护，但需要处理连接池 session 变量、迁移权限、管理员绕过、测试夹具和性能影响。当前首要风险是跨租户父子引用写入，因此 RLS 不作为本轮默认实现。
- STORY-044 已修复部分关键写入路径的 SQL 校验，但 SQL 校验不能替代数据库约束。
- `lab/` 目录仍是智能体训练实验，不接入生产链路，本 Story 不修改也不引用 `lab/`。

## 范围

本轮处理：

- 新增迁移 `services/api-gateway/migrations/000020_story047_tenant_constraints.sql`。
- 为核心父表补充 `(tenant_id, id)` 唯一约束。
- 为核心子表补充 `(tenant_id, child_fk)` 复合外键，保证子记录的 `tenant_id` 与父记录所属租户一致。
- 在迁移中加入上线前数据一致性检查。若现有数据存在跨租户引用，迁移必须失败并给出明确错误，不静默修复。
- 为迁移增加 Postgres 级测试，直接插入跨租户坏数据，验证数据库拒绝。
- 保留现有单列外键，不在本轮删除旧约束，降低迁移风险。
- 更新 Story 文档和生产化路线图状态。

本轮不处理：

- 不启用全量 PostgreSQL RLS。
- 不重构所有查询为 RLS 驱动。
- 不修改 Web 页面。
- 不接真实 OCR、真实主观题 AI 或 Agent Worker Runtime。
- 不处理完整 SSO/MFA/账号生命周期，那属于 STORY-056。
- 不接入 `lab/` 实验目录。

## 约束覆盖清单

首轮必须覆盖评分主链路、文件链路、组织链路和认证授权链路中最容易造成跨租户污染的父子关系。

### 父表唯一约束

以下表必须具备 `UNIQUE (tenant_id, id)`：

| 表 | 原因 |
| --- | --- |
| `tenant` | 所有租户数据根 |
| `app_user` | 创建人、审核人、阅卷员、会话等大量引用 |
| `role` | `user_role`、权限关系引用 |
| `permission` | `role_permission` 引用 |
| `school` | 年级、班级、学生、考试、文件引用 |
| `grade` | 班级引用 |
| `school_class` | 学生、考试班级、教师班级引用 |
| `student` | 答卷、成绩、申诉引用 |
| `exam` | 试卷、题目、答卷、评分、报告主链路引用 |
| `file_asset` | 答卷页、试卷文件、OCR 来源文件引用 |
| `exam_paper` | 题目引用 |
| `question` | 答题区域、评分、Rubric、仲裁等引用 |
| `rubric_version` | AI grade 和 Rubric 引用 |
| `answer_segment` | AI、人工阅卷、成绩主链路引用 |
| `submission` | OCR、答题区域、成绩、申诉引用 |
| `submission_page` | OCR result、answer segment 引用 |
| `ocr_task` | OCR result 引用 |
| `review_task` | human grade、double mark 引用 |
| `double_mark_session` | arbitration、final grade 引用 |
| `arbitration_task` | final grade 引用 |
| `final_grade` | appeal、score adjustment 引用 |
| `submission_grade` | appeal、score adjustment 引用 |
| `appeal` | score adjustment 引用 |

### 子表复合外键

本轮必须新增以下复合外键。命名统一为 `fk_<child>_<parent>_tenant`。

| 子表 | 字段 | 父表 |
| --- | --- | --- |
| `app_user` | `(tenant_id, school_id)` | `school(tenant_id, id)`，仅当 `school_id` 非空 |
| `user_role` | `(tenant_id, user_id)` | `app_user(tenant_id, id)` |
| `user_role` | `(tenant_id, role_id)` | `role(tenant_id, id)` |
| `role_permission` | `(tenant_id, role_id)` | `role(tenant_id, id)` |
| `role_permission` | `(tenant_id, permission_id)` | `permission(tenant_id, id)` |
| `auth_session` | `(tenant_id, user_id)` | `app_user(tenant_id, id)` |
| `audit_log` | `(tenant_id, actor_id)` | `app_user(tenant_id, id)`，仅当 `actor_id` 非空 |
| `grade` | `(tenant_id, school_id)` | `school(tenant_id, id)` |
| `school_class` | `(tenant_id, school_id)` | `school(tenant_id, id)` |
| `school_class` | `(tenant_id, grade_id)` | `grade(tenant_id, id)` |
| `school_class` | `(tenant_id, homeroom_teacher_id)` | `app_user(tenant_id, id)`，仅当非空 |
| `student` | `(tenant_id, school_id)` | `school(tenant_id, id)` |
| `student` | `(tenant_id, class_id)` | `school_class(tenant_id, id)` |
| `teacher_class` | `(tenant_id, teacher_id)` | `app_user(tenant_id, id)` |
| `teacher_class` | `(tenant_id, class_id)` | `school_class(tenant_id, id)` |
| `exam` | `(tenant_id, school_id)` | `school(tenant_id, id)` |
| `exam` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `exam_class` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `exam_class` | `(tenant_id, class_id)` | `school_class(tenant_id, id)` |
| `file_asset` | `(tenant_id, school_id)` | `school(tenant_id, id)`，仅当非空 |
| `file_asset` | `(tenant_id, exam_id)` | `exam(tenant_id, id)`，仅当非空 |
| `file_asset` | `(tenant_id, uploaded_by)` | `app_user(tenant_id, id)` |
| `exam_paper` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `exam_paper` | `(tenant_id, file_asset_id)` | `file_asset(tenant_id, id)` |
| `exam_paper` | `(tenant_id, uploaded_by)` | `app_user(tenant_id, id)` |
| `question` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `question` | `(tenant_id, exam_paper_id)` | `exam_paper(tenant_id, id)`，仅当非空 |
| `question_answer_key` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `question_answer_key` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `rubric_version` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `rubric_version` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `rubric_version` | `(tenant_id, approved_by)` | `app_user(tenant_id, id)`，仅当非空 |
| `question_rubric` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `question_rubric` | `(tenant_id, rubric_version_id)` | `rubric_version(tenant_id, id)` |
| `question_rubric` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `question_rubric` | `(tenant_id, approved_by)` | `app_user(tenant_id, id)`，仅当非空 |
| `submission` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `submission` | `(tenant_id, student_id)` | `student(tenant_id, id)`，仅当非空 |
| `submission` | `(tenant_id, collected_by)` | `app_user(tenant_id, id)` |
| `submission_page` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `submission_page` | `(tenant_id, file_asset_id)` | `file_asset(tenant_id, id)` |
| `ocr_task` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `ocr_task` | `(tenant_id, requested_by)` | `app_user(tenant_id, id)` |
| `ocr_result` | `(tenant_id, ocr_task_id)` | `ocr_task(tenant_id, id)` |
| `ocr_result` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `ocr_result` | `(tenant_id, submission_page_id)` | `submission_page(tenant_id, id)` |
| `ocr_result` | `(tenant_id, source_image_file_id)` | `file_asset(tenant_id, id)`，仅当非空 |
| `answer_segment` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `answer_segment` | `(tenant_id, submission_page_id)` | `submission_page(tenant_id, id)` |
| `answer_segment` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `answer_segment` | `(tenant_id, reviewed_by)` | `app_user(tenant_id, id)`，仅当非空 |
| `answer_segment_answer` | `(tenant_id, answer_segment_id)` | `answer_segment(tenant_id, id)` |
| `answer_segment_answer` | `(tenant_id, recorded_by)` | `app_user(tenant_id, id)` |
| `ai_grade` | `(tenant_id, answer_segment_id)` | `answer_segment(tenant_id, id)` |
| `ai_grade` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `ai_grade` | `(tenant_id, rubric_version_id)` | `rubric_version(tenant_id, id)`，仅当非空 |
| `ai_grade` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `review_task` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `review_task` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `review_task` | `(tenant_id, answer_segment_id)` | `answer_segment(tenant_id, id)` |
| `review_task` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `review_task` | `(tenant_id, assigned_to)` | `app_user(tenant_id, id)`，仅当非空 |
| `review_task` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `human_grade` | `(tenant_id, review_task_id)` | `review_task(tenant_id, id)` |
| `human_grade` | `(tenant_id, answer_segment_id)` | `answer_segment(tenant_id, id)` |
| `human_grade` | `(tenant_id, reviewer_id)` | `app_user(tenant_id, id)` |
| `double_mark_policy` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `double_mark_policy` | `(tenant_id, question_id)` | `question(tenant_id, id)`，仅当非空 |
| `double_mark_policy` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, answer_segment_id)` | `answer_segment(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, first_review_task_id)` | `review_task(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, second_review_task_id)` | `review_task(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, first_reviewer_id)` | `app_user(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, second_reviewer_id)` | `app_user(tenant_id, id)` |
| `double_mark_session` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, double_mark_session_id)` | `double_mark_session(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, answer_segment_id)` | `answer_segment(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, first_reviewer_id)` | `app_user(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, second_reviewer_id)` | `app_user(tenant_id, id)` |
| `arbitration_task` | `(tenant_id, assigned_to)` | `app_user(tenant_id, id)`，仅当非空 |
| `arbitration_task` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `final_grade` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `final_grade` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `final_grade` | `(tenant_id, answer_segment_id)` | `answer_segment(tenant_id, id)` |
| `final_grade` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `final_grade` | `(tenant_id, double_mark_session_id)` | `double_mark_session(tenant_id, id)`，仅当非空 |
| `final_grade` | `(tenant_id, arbitration_task_id)` | `arbitration_task(tenant_id, id)`，仅当非空 |
| `final_grade` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `submission_grade` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `submission_grade` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `submission_grade` | `(tenant_id, student_id)` | `student(tenant_id, id)`，仅当非空 |
| `submission_grade` | `(tenant_id, confirmed_by)` | `app_user(tenant_id, id)`，仅当非空 |
| `submission_grade` | `(tenant_id, published_by)` | `app_user(tenant_id, id)`，仅当非空 |
| `submission_grade` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `appeal` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `appeal` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `appeal` | `(tenant_id, submission_grade_id)` | `submission_grade(tenant_id, id)` |
| `appeal` | `(tenant_id, student_id)` | `student(tenant_id, id)` |
| `appeal` | `(tenant_id, final_grade_id)` | `final_grade(tenant_id, id)`，仅当非空 |
| `appeal` | `(tenant_id, question_id)` | `question(tenant_id, id)`，仅当非空 |
| `appeal` | `(tenant_id, assigned_to)` | `app_user(tenant_id, id)`，仅当非空 |
| `appeal` | `(tenant_id, reviewed_by)` | `app_user(tenant_id, id)`，仅当非空 |
| `appeal` | `(tenant_id, closed_by)` | `app_user(tenant_id, id)`，仅当非空 |
| `appeal` | `(tenant_id, created_by)` | `app_user(tenant_id, id)` |
| `score_adjustment` | `(tenant_id, appeal_id)` | `appeal(tenant_id, id)` |
| `score_adjustment` | `(tenant_id, exam_id)` | `exam(tenant_id, id)` |
| `score_adjustment` | `(tenant_id, submission_id)` | `submission(tenant_id, id)` |
| `score_adjustment` | `(tenant_id, submission_grade_id)` | `submission_grade(tenant_id, id)` |
| `score_adjustment` | `(tenant_id, final_grade_id)` | `final_grade(tenant_id, id)` |
| `score_adjustment` | `(tenant_id, question_id)` | `question(tenant_id, id)` |
| `score_adjustment` | `(tenant_id, adjusted_by)` | `app_user(tenant_id, id)` |

## 迁移策略

迁移必须按以下顺序执行：

1. 新增父表复合唯一约束。约束名统一为 `uq_<table>_tenant_id_id`。
2. 对每个拟新增复合外键先执行一致性检查。检查语句使用 `NOT EXISTS` 或 `LEFT JOIN` 找到跨租户引用，发现任何记录即 `RAISE EXCEPTION`。
3. 新增复合外键。约束名统一为 `fk_<child>_<parent>_tenant`。
4. 保留旧单列外键，避免影响既有 ORM/store 假设；后续若要清理旧约束，单独开 Story。

迁移必须可重复运行：

- 使用 `ADD CONSTRAINT` 前检查 `pg_constraint`，避免重复添加。
- 不依赖手工数据修复。
- 对 nullable 外键，检查和约束都允许字段为空。

## 测试要求

新增或修改测试：

- `services/api-gateway/internal/auth/migrations_test.go` 或新增等价迁移测试文件，验证 `000020` 能在空库和现有迁移链上成功执行。
- 新增跨租户负例测试，至少覆盖以下链路：
  - `user_role` 不能引用其他租户的 `app_user` 或 `role`。
  - `exam` 不能引用其他租户的 `school` 或 `created_by`。
  - `submission` 不能引用其他租户的 `exam` 或 `student`。
  - `submission_page` 不能引用其他租户的 `submission` 或 `file_asset`。
  - `ocr_result` 不能引用其他租户的 `ocr_task`、`submission_page` 或 `file_asset`。
  - `answer_segment` 不能引用其他租户的 `submission`、`submission_page` 或 `question`。
  - `ai_grade` 不能引用其他租户的 `answer_segment`、`question` 或 `rubric_version`。
  - `review_task` 不能引用其他租户的 `answer_segment`、`submission` 或 `assigned_to`。
  - `human_grade` 不能引用其他租户的 `review_task` 或 `reviewer_id`。
  - `double_mark_session` 不能引用其他租户的 review task、reviewer 或 answer segment。
  - `arbitration_task` 不能引用其他租户的 double mark session 或 assigned user。
  - `final_grade` 不能引用其他租户的 answer segment、submission、double mark session 或 arbitration task。
  - `submission_grade` 不能引用其他租户的 submission、student 或 confirmer/publisher。
  - `appeal` 不能引用其他租户的 submission grade、student、final grade 或 reviewer。
  - `score_adjustment` 不能引用其他租户的 appeal、final grade 或 adjusted_by。
- 正例测试必须证明同租户引用仍可写入，避免只测失败不测可用。

验证命令：

- `go test ./internal/auth -run TestMigrations`
- `go test ./...`

如果本地没有 PostgreSQL 集成测试环境，必须在文档中明确说明跳过原因；但 Story 不应批准为完成。

## 验收标准

- 新增迁移存在，能在完整迁移链后执行。
- 核心父表具备 `(tenant_id, id)` 唯一约束。
- 核心子表具备本规格列出的复合租户外键。
- 跨租户错误写入在数据库层失败，即使绕过 API/store 直接 SQL 插入也失败。
- 同租户合法写入不受影响。
- 现有 `go test ./...` 通过。
- 生产化路线图中 STORY-047 状态更新为完成或已进入实现闭环。
- `lab/` 未接入生产链路，本 Story 文档说明“不影响生产上线能力”。

## 规格审阅

审阅发现：

- 如果直接启用 RLS，当前 store、迁移测试和连接池都需要额外 session 变量治理，容易把本 Story 变成多模块大改，且短期不能替代外键一致性。
- 如果只给父表加唯一约束而不加子表复合外键，数据库仍不能阻止跨租户引用，不能关闭 P0 风险。
- 如果删除旧单列外键，可能影响已有迁移顺序和错误信息，不利于本轮低风险上线。
- 如果规格覆盖所有 nullable JSON 内部引用，会把范围扩展到不可由数据库外键直接保证的结构化字段，应放到后续业务校验 Story。
- 当前表较多，迁移写法需要 helper DO block 或重复检查函数来降低手写错误，但 SQL 仍必须清晰列出每个约束。

## 规格修改

根据审阅，本 Story 收敛为：

- 采用“父表 `(tenant_id, id)` 唯一约束 + 子表复合外键”的首轮硬化路线。
- 保留旧单列外键，不做破坏性删除。
- 不启用全量 RLS，只在文档中记录后续可作为纵深防护评估。
- 范围限定为真实表字段外键，不处理 JSON 内部 id 引用。
- 测试必须同时覆盖同租户正例和跨租户负例，且负例必须证明数据库层失败。

## 下一步实现要求

本规格通过后，下一步进入实现阶段。实现阶段必须先写失败测试，再写迁移和必要的测试夹具，最后运行全量 Go 测试。

## 实现

新增：

- `services/api-gateway/migrations/000020_story047_tenant_constraints.sql`

修改：

- `services/api-gateway/internal/auth/migrations_test.go`
- `services/api-gateway/internal/server/e2e_postgres_test.go`
- `services/api-gateway/internal/server/migration_split_test.go`
- `services/api-gateway/internal/score/store_postgres.go`

实现摘要：

- 新增 `TestStory047MigrationAddsTenantScopedDatabaseConstraints`，先验证 `000020_story047_tenant_constraints.sql` 不存在时失败。
- 新增 STORY-047 迁移，使用 `pg_temp.ensure_tenant_identity(table_name, constraint_name)` 为核心父表补 `(tenant_id, id)` 唯一约束。
- 新增 `pg_temp.ensure_tenant_fk(child_table, child_column, parent_table, constraint_name)`，在添加每个复合外键前先检查现有数据是否存在跨租户引用。
- 如果发现现有跨租户引用，迁移会 `RAISE EXCEPTION 'tenant scoped foreign key violation before adding %'`，避免静默修复或带脏数据上线。
- 新迁移保留旧单列外键，不做破坏性删除。
- 新迁移不启用全量 RLS，RLS 仍保留为后续纵深防护方向。
- 修复 Postgres E2E SQL splitter，使其支持 `DO $$ ... $$;` 和 `$fn$ ... $fn$` dollar-quoted block，避免迁移 helper 被错误拆分。
- 修复 Postgres E2E 学生 scope 种子数据的 UUID 参数类型推断问题。
- 修复 `score.PostgresStore.listGradesWithQueryer` 在同一事务中未关闭父查询 `Rows` 就发起子查询导致真实 PostgreSQL 连接变为 bad connection 的问题。

## 实现审阅

审阅结论：

- RED 测试已确认：新增测试在迁移不存在时失败，失败原因为找不到 `000020_story047_tenant_constraints.sql`。
- GREEN 验证已确认：新增迁移后，目标测试通过。
- 全量 Go 测试通过，说明新增迁移静态检查未破坏现有后端测试。
- 迁移覆盖父表唯一约束与评分主链路、文件链路、组织链路、认证授权链路的复合租户外键。
- 真实 PostgreSQL 16 容器已执行完整迁移链，`000020_story047_tenant_constraints.sql` 成功执行。
- 真实 PostgreSQL E2E 已执行完整核心阅卷流程，覆盖迁移、登录、考试、试卷、题目、答卷、OCR task、segment、AI grade、evidence、review、finalize、confirm、publish、student grade、appeal 和 audit。
- 直接 SQL 负例已验证跨租户写入会被数据库拒绝，覆盖 `user_role`、`exam`、`exam_paper` 三类代表性父子关系。
- `lab/` 未修改，仍不接入生产链路。

## 修改实现

实现审阅后补充并修正：

- 启动 Docker Desktop 后，用临时 `postgres:16-alpine` 容器执行真实迁移链。
- 修复 Go E2E 迁移 splitter 对 dollar-quoted SQL block 的支持。
- 修复 Postgres E2E 种子数据 UUID 参数类型推断。
- 修复 score Postgres store 在同一事务内嵌套查询时未先关闭父 `Rows` 的问题。
- 恢复调试期间临时打开的 HTTP 底层错误文本，生产接口继续隐藏内部 DB 错误。

## 测试结果

- RED：`go test ./internal/auth -run TestStory047MigrationAddsTenantScopedDatabaseConstraints -count=1`
  - 结果：失败，原因为 `000020_story047_tenant_constraints.sql` 不存在。
- GREEN：`go test ./internal/auth -run TestStory047MigrationAddsTenantScopedDatabaseConstraints -count=1`
  - 结果：通过。
- 包级测试：`go test ./internal/auth -count=1`
  - 结果：通过。
- SQL splitter 回归：`go test ./internal/server -run TestE2ESplitSQLStatementsKeepsDollarQuotedBlocksTogether -count=1`
  - 结果：先失败，修复后通过。
- 真实 PostgreSQL 迁移链：临时 `postgres:16-alpine` 容器逐个执行 `services/api-gateway/migrations/*.sql`
  - 结果：通过，`000020_story047_tenant_constraints.sql` 执行结果为 `DO`。
- 真实 PostgreSQL E2E：`EDUGRADE_E2E_DATABASE_URL=postgres://... go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1`
  - 结果：通过。
- 直接 SQL 跨租户负例：临时 `postgres:16-alpine` 容器中执行 `user_role`、`exam`、`exam_paper` 跨租户插入
  - 结果：均被 `foreign_key_violation` 拒绝。
- 全量后端测试：`go test ./...`
  - 结果：通过。

## 审批结论

STORY-047 批准。

理由：规格、规格审阅、规格修改、实现、实现审阅、修改实现和真实 PostgreSQL 验证均已完成。数据库层已具备首轮核心多租户复合约束，能够阻止代表性跨租户父子引用绕过 API 写入。

## 剩余风险

- 本轮不解决所有读路径越权问题，读路径仍依赖 API 层查询条件和权限测试；数据库约束主要防写入污染。
- 本轮不处理 JSONB 字段中可能保存的 id，例如 `raw_output`、`context`、`attachment` 内部引用。
- RLS 仍是后续安全纵深方向，但不应阻塞本轮外键硬化。
