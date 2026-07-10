# Paper And Question Config API

当前文档属于 `STORY-009 试卷与题目配置模块`。

## 边界

本 Story 只登记试卷文件元数据、题目、标准答案和 Rubric 配置。真实文件二进制上传、MinIO/S3 写入和下载授权由 `docs/api/files.md` 中的文件 API 提供；当前试卷配置接口仍不直接接收 multipart 文件内容。

## 权限

所有接口必须携带 Bearer token，并要求 `exam:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## POST /api/v1/exams/{examId}/papers

登记试卷文件。推荐链路是先用文件 API 上传真实二进制，再把返回的 `file.id` 作为 `file_asset_id` 传给本接口；后端会从真实 `file_asset` 记录读取文件元数据和对象存储位置，并关联为 `exam_paper`。

推荐请求：

```json
{
  "file_asset_id": "file_001"
}
```

兼容请求：仍支持直接登记文件元数据。此兼容模式不接收 multipart 文件内容，只保存文件名、类型、大小、哈希和对象存储 key。

请求：

```json
{
  "file": {
    "original_name": "physics-final.pdf",
    "content_type": "application/pdf",
    "size_bytes": 2481024,
    "hash_sha256": "7b1f...",
    "storage_bucket": "papers",
    "storage_key": "tenant/demo/exams/exam_001/papers/physics-final.pdf"
  }
}
```

响应：`201 Created`

```json
{
  "paper": {
    "id": "paper_001",
    "exam_id": "exam_001",
    "version_no": 1,
    "status": "uploaded"
  },
  "note": "file metadata registered; binary upload is handled by file upload story"
}
```

## GET /api/v1/exams/{examId}/papers

列出考试下的试卷文件版本。

## POST /api/v1/exams/{examId}/questions

创建题目配置。

受控题型：

```text
single_choice
multiple_choice
true_false
fill_blank
numeric
formula
short_answer
calculation
essay
discussion
coding
```

请求：

```json
{
  "exam_paper_id": "paper_001",
  "question_no": "Q1",
  "question_type": "short_answer",
  "score": 10,
  "stem": "Explain Newton's second law.",
  "knowledge_points": ["force", "motion"],
  "answer_area": {"page": 1, "x": 120, "y": 340, "w": 680, "h": 180},
  "sort_order": 1,
  "answer_key": {
    "standard_answer": "F=ma",
    "equivalent_answers": ["force equals mass times acceleration"],
    "tolerance": {}
  }
}
```

如果传入 `exam_paper_id`，服务端会校验该试卷属于同一租户和同一考试。

## GET /api/v1/exams/{examId}/questions

列出考试题目配置。

## PATCH /api/v1/questions/{id}

修改题号、题型、分值、题干、知识点、答题区域、排序或标准答案。题型仍必须属于受控枚举，分值必须大于 0。

## DELETE /api/v1/questions/{id}

软删除题目。

## POST /api/v1/questions/{id}/rubric

为题目创建新的 Rubric 版本。每次提交都会生成递增版本号，不覆盖历史版本。

请求：

```json
{
  "status": "draft",
  "max_score": 10,
  "points": [
    {"id": "p1", "description": "Key concept is correct", "score": 6, "required": true},
    {"id": "p2", "description": "Expression is clear", "score": 4}
  ],
  "deductions": [],
  "examples": []
}
```

规则：

- `points` 总分必须等于题目分值。
- `max_score` 必须等于题目分值。
- 最新 Rubric 为 `locked` 时不能再创建新版本，返回 `409 rubric_locked`。
- 支持状态：`draft`、`pending_review`、`approved`、`locked`。

## POST /api/v1/exams/{examId}/validate-paper-config

检查试卷配置完整性，返回问题列表。

响应示例：

```json
{
  "result": {
    "valid": false,
    "issues": [
      {
        "code": "total_score_mismatch",
        "message": "question total 90.00 does not equal exam total 100.00"
      }
    ]
  }
}
```

## 审计

以下动作会写入 `audit_log`：

- `paper.created`
- `paper.question_created`
- `paper.question_updated`
- `paper.question_deleted`
- `paper.rubric_created`
- `paper.config_validated`
