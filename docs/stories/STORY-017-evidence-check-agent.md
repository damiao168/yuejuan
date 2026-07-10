# STORY-017 证据校验 Agent

## 状态

Approved

## 目标

实现规则级证据校验 Agent，防止 AI 建议分在证据缺失、采分点不存在、超分、OCR 低置信度或证据不在答案文本中时进入后续最终成绩流程。

本 Story 不做复杂 NLP、不做图片视觉理解、不验证证据语义充分性；只做可测试、可审计、可解释的规则级校验。

## Plan

- 新增 migration：
  - `evidence:manage` 权限。
  - `agent_job` 或等价证据校验任务记录，用于记录 evidence_check_agent 的运行结果。
- 新增 `internal/evidence`：
  - 类型定义。
  - 规则校验 Engine。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 接口：
  - `POST /api/v1/ai-grades/{id}/verify-evidence`
- 输入：
  - ai_grade。
  - answer_segment。
  - 最新 answer_segment_answer / OCR 置信度。
  - rubric。
  - ai_grade.evidence。
- 校验规则：
  - suggested_score 不得超过 max_score。
  - matched_points 的分数合计应与 suggested_score 一致，或明确可解释。
  - matched_points / missing_points 必须能对应 Rubric 采分点。
  - evidence 不得为空。
  - evidence 中的文本证据必须能在学生答案中找到。
  - evidence bbox 如果存在，必须落在当前 answer_segment bbox 内。
  - OCR 低置信度必须触发人工复核。
- 输出 verification_result：
  - `passed`
  - `failed`
  - `warnings`
  - `corrected_flags`
  - `needs_human_review`
- 如果证据校验失败，强制 `needs_human_review=true`。
- 写 audit。

## Plan Review

- 不越界：不做复杂 NLP，不做图片视觉理解，不判断语义是否真正充分。
- 不做假功能：不会声称“证据语义正确”，只校验引用、文本包含、bbox 范围、Rubric 点和分数一致性。
- 与 STORY-015/016 衔接：消费 `ai_grade`、`answer_segment_answer`、`question_rubric`。
- 与 STORY-018 衔接：证据校验失败会作为人工复核任务来源。

## 非范围

- 真实多模态视觉证据校验。
- LLM 语义核验。
- 人工复核任务生成。
- final_grade 更新。

## 验收标准

- 超分会失败并强制人工复核。
- 不存在的采分点会失败。
- 空 evidence 会失败。
- OCR 低置信度会触发人工复核。
- evidence 文本不在答案中会失败。
- evidence bbox 越出 answer_segment 会失败。
- 校验结果记录 agent job。
- 接口受权限保护。
- 写审计。
- 测试通过。

## Implementation

- 新增 migration `000012_evidence_check.sql`：
  - 增加 `evidence:manage` 权限。
  - 为平台管理员、租户管理员、学校管理员、教师和审计员分配权限。
  - 新增 `agent_job` 表，用于记录 `evidence_check` 任务结果。
- 新增 `internal/evidence`：
  - 类型定义：`Grade`、`Context`、`VerificationResult`、`AgentJob`。
  - `Engine` 规则校验：
    - 超分失败。
    - matched_points 分数合计不一致失败。
    - Rubric 缺失或采分点不存在失败。
    - evidence 为空失败。
    - evidence 文本不在答案中失败。
    - evidence bbox 超出 answer_segment bbox 失败。
    - OCR 低置信度触发人工复核 warning。
  - MemoryStore 和 PostgresStore。
  - Handler。
- API Gateway 挂载：
  - `POST /api/v1/ai-grades/{id}/verify-evidence`
  - 权限要求：`evidence:manage`
- 更新 `system/info` capabilities：
  - `rule_based_evidence_verification`
  - `evidence_agent_job_recording`
  - `evidence_failure_review_trigger`
- 更新 `system/info` not_implemented：
  - `semantic_evidence_verification`
  - `visual_evidence_verification`
- 新增 `docs/api/evidence-check.md`，更新根 README 和 API Gateway README。

## Implementation Review

逐项检查结果：

- 超分失败：`TestVerifyFailsWhenSuggestedScoreExceedsMax` 覆盖。
- 不存在采分点失败：`TestVerifyFailsWhenRubricPointIsUnknown` 覆盖。
- Rubric 缺失失败：`TestVerifyFailsWhenRubricIsMissingForPointReferences` 覆盖。
- evidence 为空失败：`TestVerifyFailsWhenEvidenceIsEmpty` 覆盖。
- OCR 低置信复核：`TestVerifyFlagsLowOCRConfidenceForReview` 覆盖。
- evidence 文本不在答案中失败：`TestVerifyFailsWhenEvidenceTextIsNotInAnswer` 覆盖。
- evidence bbox 越界失败：`TestVerifyFailsWhenEvidenceBBoxLeavesSegment` 覆盖。
- agent job 记录：`TestVerifyEvidenceRouteRecordsJobAndAudit` 断言返回 `evidence_check` job。
- 权限保护：`TestVerifyEvidenceRouteRequiresEvidenceManage` 覆盖 401/403。
- 审计：`TestVerifyEvidenceRouteRecordsJobAndAudit` 断言 `evidence.checked`。
- 不越界：未做 NLP、视觉理解、语义充分性判断，也未更新 final_grade。

实现审阅中关注点：

- 证据校验失败只强制 `verification_result.needs_human_review=true`，为后续人工复核流程提供依据。
- 低 OCR 置信度作为 warning 处理，不把证据校验整体标为 failed，但会触发人工复核。
- 语义/视觉证据核验明确保留在 `not_implemented` 中，避免把规则校验冒充真实语义理解。

## Fixes

- 补充 `rubric_missing` 规则：当评分引用 matched/missing points 但没有 Rubric points 可校验时，证据校验失败并强制人工复核。
- 补充 `system/info` 测试断言，确保 evidence capabilities 不被误删。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
gofmt -w internal
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
gofmt -w internal -> passed
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

接口验证证据：

```text
system/info -> capabilities include rule_based_evidence_verification, evidence_agent_job_recording, evidence_failure_review_trigger
system/info -> not_implemented includes semantic_evidence_verification, visual_evidence_verification
POST /api/v1/ai-grades/{id}/verify-evidence without evidence:manage -> 403 covered by TestVerifyEvidenceRouteRequiresEvidenceManage
POST /api/v1/ai-grades/{id}/verify-evidence without token -> 401 covered by TestVerifyEvidenceRouteRequiresEvidenceManage
```

## Approval

Approved。

本 Story 满足验收标准，可以进入后续人工复核和最终成绩相关 Story。
