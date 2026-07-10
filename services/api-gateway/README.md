# API Gateway

Go 后端 API Gateway。

当前已实现能力：

- 配置加载。
- JSON 结构化日志。
- 请求 ID。
- 统一 JSON 错误响应。
- panic recovery。
- `GET /health`。
- `GET /ready`。
- `GET /api/v1/system/info`。
- PostgreSQL、Redis、MinIO/S3 客户端连接抽象。
- `POST /api/v1/auth/login`。
- `POST /api/v1/auth/logout`。
- `GET /api/v1/auth/me`。
- session 认证中间件和 RBAC 权限中间件。
- 登录、登出、失败登录审计写入接口。
- 租户、学校、年级、班级、学生基础管理接口。
- 学生 CSV 导入和教师班级绑定。
- 考试创建、列表、详情、修改、状态流转和归档。
- 试卷文件元数据登记，不包含二进制文件上传。
- 题目配置、受控题型、标准答案配置。
- Rubric 版本创建、分值校验和 locked 状态保护。
- 试卷配置完整性检查，返回可定位的问题列表。
- 私有文件上传到 MinIO/S3。
- 文件 hash、类型、大小和安全文件名校验。
- 文件元数据查询、后端流式下载和软删除。
- 答卷 submission 创建、页面关联、元数据级质量门禁和进入 OCR 前状态流转。
- OCR 任务创建、状态流转、外部 worker 结果回写和低置信度人工复核标记。
- 基于题目 `answer_area` 生成 answer_segment 元数据，并支持人工修正。
- 多智能体 Orchestrator 控制面：orchestration run、agent task、状态流转、失败重试、输出/证据引用和人工复核触发。
- 客观题与填空题规则判分：answer_segment 答案记录、规则型 `ai_grade` 生成、风险标记和人工复核触发。
- 主观题 AI 评分接口层：可替换 LLM Adapter、mock LLM adapter、schema 校验、失败 ai_grade 落库和人工复核触发。
- 规则级证据校验 Agent：校验 ai_grade 证据、Rubric 采分点、分数一致性、OCR 低置信和 bbox 边界，并记录 agent job。
- 人工复核与阅卷任务：review_task 创建、查询、分配、批量分配、human_grade 提交、退回重评和审计。
- 双评与仲裁：double mark policy、双评会话、两条独立 review_task、双盲 review_task 查询、分差比较、自动合分、arbitration_task 和 final_grade 写入。
- 最终成绩与发布：final_grade 状态、submission_grade 汇总、发布前质量检查、成绩确认、成绩发布锁定、学生已发布成绩查询和 CSV 水印导出。
- 申诉流程：学生对已发布锁定成绩提交申诉，教师/仲裁员/管理员处理申诉，改分生成 score_adjustment 并更新锁定成绩，学生查询处理结果，管理员查看统计。
- 学情报告与考试质量分析：学生报告、考试概览、班级报告、题目分析、阅卷质量分析和 CSV 水印导出。

尚未实现：

- 真实 OCR 引擎推理、真实 Agent worker runtime、真实主观题模型推理、语义/视觉证据核验、图片裁剪/自动版面切分、Web 成绩页面、Web 报告页面、Web 申诉中心、学生端页面。

## 本地运行

```powershell
Copy-Item ..\\..\\.env.example .\\.env
go mod tidy
go test ./...
go run ./cmd/api-gateway
```

默认监听：`127.0.0.1:8080`。

## 接口

```text
GET /health
GET /ready
GET /api/v1/system/info
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET /api/v1/auth/me
POST /api/v1/tenants
GET /api/v1/tenants
PATCH /api/v1/tenants/{id}
POST /api/v1/schools
GET /api/v1/schools
POST /api/v1/grades
GET /api/v1/grades
POST /api/v1/classes
GET /api/v1/classes
POST /api/v1/students
GET /api/v1/students
PATCH /api/v1/students/{id}
POST /api/v1/students/import-csv
POST /api/v1/classes/{id}/teachers
POST /api/v1/exams
GET /api/v1/exams
GET /api/v1/exams/{id}
PATCH /api/v1/exams/{id}
POST /api/v1/exams/{id}/archive
POST /api/v1/exams/{id}/status
POST /api/v1/exams/{examId}/papers
GET /api/v1/exams/{examId}/papers
POST /api/v1/exams/{examId}/questions
GET /api/v1/exams/{examId}/questions
PATCH /api/v1/questions/{id}
DELETE /api/v1/questions/{id}
POST /api/v1/questions/{id}/rubric
POST /api/v1/exams/{examId}/validate-paper-config
POST /api/v1/files
GET /api/v1/files/{id}
GET /api/v1/files/{id}/download
DELETE /api/v1/files/{id}
POST /api/v1/exams/{examId}/submissions
GET /api/v1/exams/{examId}/submissions
GET /api/v1/submissions/{id}
POST /api/v1/submissions/{id}/pages
GET /api/v1/submissions/{id}/pages
POST /api/v1/submissions/{id}/quality-check
POST /api/v1/submissions/{id}/status
POST /api/v1/submissions/{id}/ocr-tasks
GET /api/v1/submissions/{id}/ocr-tasks
GET /api/v1/ocr-tasks/{id}
POST /api/v1/ocr-tasks/{id}/start
POST /api/v1/ocr-tasks/{id}/results
POST /api/v1/ocr-tasks/{id}/fail
POST /api/v1/submissions/{id}/segment-answers
GET /api/v1/submissions/{id}/answer-segments
PATCH /api/v1/answer-segments/{id}
POST /api/v1/orchestrations
GET /api/v1/orchestrations/{id}
GET /api/v1/orchestrations/{id}/tasks
POST /api/v1/orchestrations/{id}/tasks
POST /api/v1/agent-tasks/{id}/start
POST /api/v1/agent-tasks/{id}/complete
POST /api/v1/agent-tasks/{id}/fail
POST /api/v1/agent-tasks/{id}/retry
PUT /api/v1/answer-segments/{id}/answer
POST /api/v1/answer-segments/{id}/rule-grade
GET /api/v1/answer-segments/{id}/ai-grades
POST /api/v1/answer-segments/{id}/subjective-ai-grade
POST /api/v1/ai-grades/{id}/verify-evidence
PUT /api/v1/exams/{examId}/double-mark-policy
PUT /api/v1/questions/{id}/double-mark-policy
GET /api/v1/double-mark-policies
POST /api/v1/double-mark-sessions
GET /api/v1/double-mark-sessions
GET /api/v1/double-mark-sessions/{id}
POST /api/v1/arbitration-tasks
GET /api/v1/arbitration-tasks
GET /api/v1/arbitration-tasks/{id}
POST /api/v1/arbitration-tasks/{id}/assign
POST /api/v1/arbitration-tasks/{id}/submit
POST /api/v1/exams/{examId}/finalize
GET /api/v1/exams/{examId}/grades
POST /api/v1/exams/{examId}/confirm-grades
POST /api/v1/exams/{examId}/publish
GET /api/v1/exams/{examId}/grades/export
GET /api/v1/students/{studentId}/exams/{examId}/grade
POST /api/v1/appeals
GET /api/v1/appeals
GET /api/v1/appeals/statistics
GET /api/v1/appeals/{id}
POST /api/v1/appeals/{id}/review
POST /api/v1/appeals/{id}/close
GET /api/v1/exams/{examId}/reports/overview
GET /api/v1/exams/{examId}/reports/classes
GET /api/v1/exams/{examId}/reports/questions
GET /api/v1/exams/{examId}/reports/grading-quality
GET /api/v1/students/{studentId}/reports/{examId}
POST /api/v1/exams/{examId}/reports/export
POST /api/v1/review-tasks
GET /api/v1/review-tasks
GET /api/v1/review-tasks/{id}
POST /api/v1/review-tasks/batch-assign
POST /api/v1/review-tasks/{id}/assign
POST /api/v1/review-tasks/{id}/submit
POST /api/v1/review-tasks/{id}/return
```

`/ready` 会使用 PostgreSQL driver、Redis client 和 MinIO SDK 检查依赖连接。如果依赖未启动，会返回 `503 not_ready`。
