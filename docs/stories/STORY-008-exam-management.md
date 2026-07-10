# STORY-008 考试管理模块

## 状态

Approved

## 目标

实现 EduGrade Enterprise 的考试/作业管理基础模块，支持创建考试、选择班级、设置学科/类型/总分/阅卷模式/申诉策略/发布策略、查询、修改、归档和受控状态流转。

## Plan

- 新增考试管理 migration：
  - `exam`
  - `exam_class`
  - 考试状态、阅卷模式和权限索引。
- 新增 `internal/exam`：
  - types。
  - Store interface。
  - PostgresStore。
  - MemoryStore。
  - handlers。
- 接口：
  - `POST /api/v1/exams`
  - `GET /api/v1/exams`
  - `GET /api/v1/exams/{id}`
  - `PATCH /api/v1/exams/{id}`
  - `POST /api/v1/exams/{id}/archive`
  - `POST /api/v1/exams/{id}/status`
- 权限：
  - 所有考试管理接口需要登录。
  - 修改类接口需要 `exam:manage`。
  - 查询接口本 Story 暂用 `exam:manage`，后续教师/阅卷任务细分后再开放只读权限。
- 审计：
  - 创建、修改、状态变化、归档都写 audit。
- 测试：
  - 创建考试。
  - 列表和详情按 tenant 隔离。
  - 非法状态流转失败。
  - published 后不能修改核心配置。
  - 归档写状态。
  - 无权限返回 403。

## Plan Review

- 不越界：不实现试卷上传、题目配置、答卷采集、阅卷或成绩。
- 状态流转必须服务端校验，不能只靠前端。
- 目前已有 school_class 表，考试班级选择通过 `exam_class` 绑定。
- 目前没有完整 school data scope，本 Story 先强制 tenant 隔离，后续在教师/阅卷分配 Story 加强班级/考试级数据范围。
- 发布后不能修改名称、学科、总分、阅卷模式、班级等核心配置。

## Implementation

- 新增 `000003_exam_management.sql`，包含 `exam` 和 `exam_class`。
- 新增 `internal/exam`，包含类型、状态机、MemoryStore、PostgresStore、handlers 和测试。
- 接入 API Gateway 路由，并用 `exam:manage` 权限保护。
- 新增 `docs/api/exams.md`。
- 更新 README 和系统能力声明。

## Implementation Review

审阅发现并修正：

- PostgresStore 动态 SQL 占位符使用字符拼接，已改为 `fmt.Sprintf`。
- 系统信息仍把 `exam_management` 列为未实现，已改为 capability。
- README 未反映考试管理 API，已修正。
- 补充 tenant 隔离测试和发布后锁定测试。

## Fixes

- 修正 Postgres SQL 占位符生成。
- 更新能力声明与 API 文档。
- 补充测试覆盖：状态流转、发布后锁定、权限拒绝、归档、tenant 隔离。

## 自审审批

审批文件：`docs/stories/STORY-008-approval.md`

## 非范围

- 不实现试卷/题目/Rubric。
- 不实现成绩发布实际动作。
- 不实现考试质量检查。
- 不实现学生端考试。

## 验收标准

- 状态流转有白名单校验。
- 发布后核心配置不可修改。
- 所有操作校验权限。
- 所有关键操作写审计。
- API 文档更新。
- 测试通过。
