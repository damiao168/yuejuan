# Answer Segmentation API

当前文档属于 `STORY-013 答题区域切分模块`。

## 边界

本模块基于题目配置中的 `answer_area` 和答卷页面生成 `answer_segment` 元数据。它不裁剪图片、不保存单题图片、不执行模型版面识别，也不把 OCR 文本自动归属到题目。

该接口本身仍只管理元数据。OCR worker 现可复用原始页面像素坐标的 segment，在内存中裁剪后
识别，并将 bbox 平移回原始页面。配准/模板区域、待复核或被拒绝区域使对应页回退为整页 OCR；
它们不能与原始页面坐标混用。详见 [OCR API](ocr.md)。OCR 文本自动归题不属于本轮性能改动。

## 权限

所有接口必须携带 Bearer token，并要求 `segment:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## POST /api/v1/submissions/{id}/segment-answers

根据考试题目配置和 submission pages 生成 answer segments。submission 必须处于 `ready_for_ocr`。

生成规则：

- 读取 submission 所属 exam 的题目列表。
- 每道题必须配置 `answer_area`。
- `answer_area` 必须包含 `page`、`x`、`y`、`w`、`h`。
- `answer_area.page` 必须匹配 submission page。
- 同一 submission + question 不重复生成。

响应：

```json
{
  "result": {
    "valid": false,
    "issues": [
      {
        "code": "submission_page_missing",
        "message": "question Q1 expects page 2, but submission page is missing"
      }
    ],
    "segments": [
      {
        "id": "segment_001",
        "submission_id": "submission_001",
        "submission_page_id": "page_001",
        "question_id": "question_001",
        "question_no": "Q1",
        "bbox": [10, 20, 100, 40],
        "source": "configured_answer_area",
        "status": "generated"
      }
    ]
  }
}
```

## GET /api/v1/submissions/{id}/answer-segments

列出 submission 下的 answer segments。

## PATCH /api/v1/answer-segments/{id}

人工修正 bbox、状态和备注。

请求：

```json
{
  "bbox": [11, 21, 101, 41],
  "status": "accepted",
  "review_notes": "aligned manually"
}
```

状态：

```text
generated
accepted
needs_manual_review
rejected
```

人工修正 bbox 后，`source` 会变为 `manual`，并记录 reviewer 和 reviewed_at。

## 审计

以下动作写入 `audit_log`：

- `segment.generated`
- `segment.updated`
