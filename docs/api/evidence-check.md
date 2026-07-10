# Evidence Check API

当前文档属于 `STORY-017 证据校验 Agent`。

## 边界

本模块实现规则级证据校验 Agent，用于在 AI 建议分进入后续最终成绩流程前检查证据是否可追溯、可审计。

本模块只做规则级校验：

- 分数上限校验。
- matched/missing points 与 Rubric 采分点对应关系校验。
- matched points 分数合计与 suggested_score 一致性校验。
- evidence 非空校验。
- evidence 文本是否存在于学生答案文本中。
- evidence bbox 是否落在 answer_segment bbox 内。
- OCR 低置信度人工复核触发。

本模块不做复杂 NLP、图片视觉理解或证据语义充分性判断。

## 权限

所有接口必须携带 Bearer token，并要求 `evidence:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## POST /api/v1/ai-grades/{id}/verify-evidence

对指定 `ai_grade` 执行证据校验。系统会读取：

- ai_grade
- answer_segment
- 最新 answer_segment_answer 与 OCR 置信度
- 最新 question_rubric
- ai_grade.evidence

请求体：

```json
{}
```

响应：`201 Created`

```json
{
  "job": {
    "job_type": "evidence_check",
    "target_type": "ai_grade",
    "target_id": "grade-id",
    "status": "failed",
    "needs_human_review": true,
    "result": {
      "passed": false,
      "failed": [
        {
          "code": "evidence_text_not_found",
          "message": "evidence answer_text is not found in answer segment text"
        }
      ],
      "warnings": [],
      "corrected_flags": ["evidence_verification_failed"],
      "needs_human_review": true
    }
  }
}
```

## 校验结果

- `passed=true`：规则级证据校验通过。
- `passed=false`：至少一个失败项，必须 `needs_human_review=true`。
- `warnings`：不阻断通过但需要关注，例如 OCR 低置信度。
- `corrected_flags`：系统根据校验结果补充的风险标记。

## 失败代码

- `score_exceeds_max`：suggested_score 超过 max_score。
- `matched_score_mismatch`：matched_points 分数合计与 suggested_score 不一致。
- `rubric_missing`：评分引用了采分点，但没有可校验的 Rubric points。
- `rubric_point_not_found`：matched/missing point 不存在于 Rubric。
- `empty_evidence`：ai_grade.evidence 为空。
- `empty_evidence_text`：证据文本为空。
- `evidence_text_not_found`：证据文本不在答案文本中。
- `evidence_bbox_out_of_segment`：证据 bbox 超出 answer_segment bbox。

## 警告代码

- `low_ocr_confidence`：OCR 置信度低于证据校验阈值，会触发人工复核。

## 审计

以下动作写入 `audit_log`：

- `evidence.checked`
