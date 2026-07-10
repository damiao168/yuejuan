# File Upload And Object Storage API

当前文档属于 `STORY-010 文件上传与对象存储`。

## 边界

文件 API 负责真实二进制上传、对象存储写入、元数据保存、私有下载和删除。数据库只保存 `file_asset` 元数据，不保存文件内容。

本模块不触发 OCR、不创建答卷 submission、不实现断点续传协议。

## 权限

所有接口必须携带 Bearer token，并要求 `file:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离文件。

## 支持类型

```text
pdf
png
jpg
jpeg
csv
docx
```

服务端同时检查扩展名、Content-Type、文件大小和 SHA-256。客户端不能提交 `storage_bucket` 或 `storage_key`。

## POST /api/v1/files

上传文件。请求必须为 `multipart/form-data`。

字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | 文件内容 |
| `owner_type` | 否 | 默认 `generic`，可选 `exam`、`exam_paper`、`submission`、`answer_page`、`report`、`import` |
| `owner_id` | 否 | UUID |
| `school_id` | 否 | UUID |
| `exam_id` | 否 | UUID |
| `submission_id` | 否 | UUID |

响应：`201 Created`

```json
{
  "file": {
    "id": "file_001",
    "tenant_id": "tenant_001",
    "owner_type": "exam",
    "owner_id": "exam_001",
    "original_name": "paper.pdf",
    "content_type": "application/pdf",
    "size_bytes": 2481024,
    "hash_sha256": "7b1f...",
    "visibility": "private",
    "uploaded_by": "user_001",
    "created_at": "2026-07-03T12:00:00Z"
  }
}
```

响应不会包含 `storage_bucket` 或 `storage_key`。

重复上传同一租户、同一 owner 下相同 hash 的文件返回 `409 duplicate_file`，并带 `existing_file`。

## GET /api/v1/files/{id}

查询文件元数据。响应不暴露对象存储路径。

## GET /api/v1/files/{id}/download

鉴权后由后端从对象存储读取文件并流式返回。

响应头：

```text
Content-Type: <file content type>
Content-Disposition: attachment; filename="<original_name>"
X-Content-Type-Options: nosniff
```

## DELETE /api/v1/files/{id}

软删除文件元数据，并尝试清理对象存储对象。

响应：

```json
{
  "status": "deleted",
  "object_cleanup": "removed"
}
```

如果对象清理失败，`object_cleanup` 返回 `pending`，文件元数据仍已软删除，后续运维清理任务可补偿处理。

## 审计

以下动作写入 `audit_log`：

- `file.uploaded`
- `file.downloaded`
- `file.deleted`
