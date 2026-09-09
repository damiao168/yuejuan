# Submission Collection API

当前文档属于 `STORY-011 答卷采集与 Submission`。

## 边界

本模块建立答卷采集数据结构：submission、submission page、文件关联、答卷完整性门禁和进入 OCR 前的状态流转。

`POST /quality-check` 只做元数据级的答卷完整性检查：是否有页面、页数是否匹配、页码是否缺失。页面图像模糊、倾斜、分辨率等由 `POST /run-quality-check` 创建的逐页图像质检任务处理；条码识别、OCR 和答题区域切分不属于完整性检查。

## 权限

所有接口必须携带 Bearer token，并要求 `submission:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## 状态

```text
created -> pages_uploaded -> quality_checked -> ready_for_ocr
created/pages_uploaded/quality_checked -> rejected
```

`quality_checked` 不能由客户端直接改状态。只有答卷完整性通过，并且所有页面的图像质检均为 `passed`（包含有效人工放行）时，服务端才会聚合为 `quality_checked`；此后才能进入 `ready_for_ocr`。

## POST /api/v1/exams/{examId}/submissions

创建答卷采集记录。

请求：

```json
{
  "student_id": "student_uuid_optional",
  "candidate_no": "S20260703001",
  "source_type": "scanner_upload",
  "expected_page_count": 2
}
```

`source_type` 可选：

```text
scanner_upload
image_upload
pdf_upload
mobile_capture
lms_import
manual_import
```

响应：`201 Created`

```json
{
  "submission": {
    "id": "submission_001",
    "exam_id": "exam_001",
    "candidate_no": "S20260703001",
    "source_type": "scanner_upload",
    "status": "created",
    "expected_page_count": 2,
    "actual_page_count": 0,
    "quality_status": "unchecked",
    "quality_issues": []
  }
}
```

## GET /api/v1/exams/{examId}/submissions

列出考试下的答卷采集记录。

## GET /api/v1/submissions/{id}

查询答卷详情，包含已关联页面。

## POST /api/v1/submissions/{id}/pages

把已上传文件关联为答卷页面。`file_asset_id` 必须属于当前租户且未删除。

请求：

```json
{
  "file_asset_id": "file_001",
  "page_no": 1
}
```

同一 submission 下页码不可重复；重复返回 `409 duplicate_page`。

## PUT /api/v1/submissions/{id}/pages/{pageNo}

重新上传并替换已有页码的文件。`file_asset_id` 必须属于当前租户且未删除。

请求：

```json
{
  "file_asset_id": "file_002"
}
```

规则：

- 只替换已存在的 `pageNo`，不存在返回 `404 submission_not_found`。
- 不创建新页，不改变页数。
- 替换成功后页面状态重置为 `uploaded`，页面质量问题清空。
- 替换成功后 submission 重新变为 `pages_uploaded`，`quality_status` 重置为 `unchecked`，需要重新执行质量门禁。
- `ready_for_ocr` 或 `rejected` 状态下不允许替换，返回 `409 submission_locked`。

## GET /api/v1/submissions/{id}/pages

列出答卷页面。

## POST /api/v1/submissions/{id}/quality-check

运行进入 OCR 前的答卷完整性门禁。

响应示例：

```json
{
  "result": {
    "valid": false,
    "issues": [
      {"code": "page_count_mismatch", "message": "expected 2 pages, got 1"},
      {"code": "missing_page", "message": "page 2 is missing"}
    ]
  }
}
```

`valid: true` 仅表示答卷完整性通过，不会单独把 submission 标记为 `quality_checked`。完整性失败时 submission 保持在 `pages_uploaded`，`quality_status` 为 `failed`；完整性通过后仍需执行 `POST /run-quality-check`，待所有页面图像质检通过或被有效人工放行，submission 才会聚合为 `quality_checked`、`quality_status: passed`。

## POST /api/v1/submissions/{id}/run-quality-check

为答卷当前的每一页创建异步图像质量检测任务。接口返回 `202 Accepted`；客户端应查询答卷及页面质量状态，只有 submission 聚合为 `quality_checked` 后才能请求进入 `ready_for_ocr`。

## POST /api/v1/submissions/{id}/status

执行受控状态流转。

请求：

```json
{
  "status": "ready_for_ocr"
}
```

非法流转返回 `409 invalid_submission_status_transition`。

## 审计

以下动作写入 `audit_log`：

- `submission.created`
- `submission.page_added`
- `submission.page_replaced`
- `submission.quality_checked`
- `submission.status_changed`
