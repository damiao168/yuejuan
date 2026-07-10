# STORY-011 答卷采集与 Submission

## 状态

Approved

## 目标

在真实文件上传能力之上，建立答卷采集数据模型与 API。支持为考试创建答卷 submission、关联上传文件为页面、记录采集来源、学生/准考证识别字段、基础质量门禁和进入 OCR 前的状态流转。

## Plan

- 新增 migration：
  - `submission`
  - `submission_page`
  - `submission:manage` 权限。
- 新增 `internal/submission`：
  - 类型定义和状态机。
  - Store interface。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 接口：
  - `POST /api/v1/exams/{examId}/submissions`
  - `GET /api/v1/exams/{examId}/submissions`
  - `GET /api/v1/submissions/{id}`
  - `POST /api/v1/submissions/{id}/pages`
  - `GET /api/v1/submissions/{id}/pages`
  - `POST /api/v1/submissions/{id}/quality-check`
  - `POST /api/v1/submissions/{id}/status`
- 规则：
  - 所有接口要求 `submission:manage`。
  - 页面必须引用已上传的 `file_asset`。
  - 同一答卷页码不可重复。
  - 基础质量门禁只检查元数据完整性、页数、重复页、缺失页，不做图像模糊/倾斜/OCR 假检测。
  - 状态受控：`created -> pages_uploaded -> quality_checked -> ready_for_ocr`，可进入 `rejected`。
  - 关键操作写审计。

## Plan Review

- 不越界：不实现 OCR、不切题、不做 AI 阅卷、不生成 answer_segment。
- 不做假功能：图像质量检测暂不声明真实模糊/倾斜检测，只做元数据级质量门禁并返回 issue 列表。
- 与 STORY-010 衔接：本 Story 复用 `file_asset`，不再接收二进制上传。
- 与后续 Story 衔接：`ready_for_ocr` 是 STORY-012 OCR 任务的入口状态。

## 非范围

- OCR。
- 答题区域切分。
- 图像增强、纠偏、模糊检测。
- 学生条码真实识别。
- 扫描仪/EXE 客户端。

## 验收标准

- 可以创建考试答卷 submission。
- 可以把已上传文件关联为答卷页面。
- 页面重复、无页面、页数不一致能被质量门禁发现。
- 状态流转受控，不能跳过质量门禁直接进入 OCR。
- 所有接口受 `submission:manage` 权限保护。
- 创建、加页、质量检查、状态流转写审计。
- 测试通过。

## Implementation

- 新增 `000006_submission_collection.sql`：
  - `submission:manage` 权限。
  - `submission` 表。
  - `submission_page` 表。
  - candidate 和 page 索引。
- 新增 `internal/submission`：
  - 类型定义、状态机和输入校验。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- API Gateway 挂载：
  - `POST /api/v1/exams/{examId}/submissions`
  - `GET /api/v1/exams/{examId}/submissions`
  - `GET /api/v1/submissions/{id}`
  - `POST /api/v1/submissions/{id}/pages`
  - `GET /api/v1/submissions/{id}/pages`
  - `POST /api/v1/submissions/{id}/quality-check`
  - `POST /api/v1/submissions/{id}/status`
- 更新 `system/info` capabilities：
  - `submission_collection`
  - `submission_pages`
  - `submission_quality_gate`
- 新增 `docs/api/submissions.md`，更新 README。

## Implementation Review

逐项检查结果：

- 创建 submission：已实现并测试。
- 关联上传文件为页面：Handler 会校验 `file_asset_id` 属于当前租户且未删除。
- 重复页码：Store 层拒绝同 submission 的重复 `page_no`，测试覆盖。
- 元数据级质量门禁：检查无页面、页数不匹配、缺失页，返回 issue 列表。
- 状态流转：不能跳过质量门禁直接进入 `ready_for_ocr`，测试覆盖。
- 权限：所有接口要求 `submission:manage`，测试覆盖权限不足。
- 审计：创建、加页、质量检查、状态流转均调用 audit。
- 不越界：未实现 OCR、条码识别、图像模糊检测、答题区域切分。

实现审阅中发现并修正的问题：

- 状态机最初允许客户端手动流转到 `quality_checked`，已改为只能由 `quality-check` 产生。
- 质量门禁通过后如果继续加页，原质量结果必须失效；已将状态重置为 `pages_uploaded`、`quality_status` 重置为 `unchecked`。
- 已补充对应回归测试。

## Fixes

- 收紧 `CanTransition`，禁止手动进入 `quality_checked`。
- 加页时强制重置质量状态。
- 禁止已进入 `ready_for_ocr` 或 `rejected` 的 submission 再执行质量检查或加页。
- 增加状态机和质量门禁失效测试。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

启动验证：

```powershell
GET http://127.0.0.1:18091/api/v1/system/info
GET http://127.0.0.1:18091/api/v1/exams/exam-1/submissions
```

结果：

```text
system/info -> capabilities include submission_collection, submission_pages, submission_quality_gate
system/info -> not_implemented still includes ocr, ai_grading
GET /api/v1/exams/exam-1/submissions without token -> 401
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-012 OCR 服务接口与任务队列`。
