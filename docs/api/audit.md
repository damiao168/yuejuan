# Audit Log API

当前文档随 `STORY-028 Web 双评仲裁页面` 增补，并在 `STORY-032 Web 审计日志页面` 扩展筛选与导出能力，用于支持页面按真实后端 `audit_log` 显示关键操作记录。

## 边界

本接口提供只读审计查询能力：

- 按当前登录用户的 `tenant_id` 隔离数据。
- 支持按 action、actor、target_type、target_id、exam_id、ip_address、created_from、created_to 查询。
- 用于业务页面展示相关审计记录，也可作为后续 Web 审计日志页面的基础。

本接口不实现审计日志删除、修改、归档、不可篡改链或跨租户监管。`before_value` 与 `after_value` 只返回业务写入审计事件时真实提供的 JSON；既有事件未提供时为空。

## 权限

- `GET /api/v1/audit-logs` 要求 `audit:read`。
- `POST /api/v1/audit-logs/export` 要求 `audit:export`。

## GET /api/v1/audit-logs

查询审计日志。支持查询参数：

- `action`
- `actor_id`
- `target_type`
- `target_id`
- `exam_id`，匹配当前审计模型中 `target_id=exam_id` 的考试级事件
- `ip_address`
- `created_from`，RFC3339 或 `YYYY-MM-DD`
- `created_to`，RFC3339 或 `YYYY-MM-DD`
- `limit`，默认 50，最大 200

响应：

```json
{
  "audit_logs": [
    {
      "id": "audit-id",
      "tenant_id": "tenant-id",
      "actor_id": "user-id",
      "action": "arbitration.submitted",
      "target_type": "arbitration_task",
      "target_id": "task-id",
      "before_value": {},
      "after_value": {},
      "reason": "submit arbitration final score",
      "ip_address": "127.0.0.1",
      "user_agent": "browser",
      "request_id": "request-id",
      "created_at": "2026-07-06T08:00:00Z"
    }
  ]
}
```

## POST /api/v1/audit-logs/export

导出审计日志 CSV。支持与列表接口相同的查询参数。响应头：

- `Content-Type: text/csv; charset=utf-8`
- `Content-Disposition: attachment; filename="audit-logs.csv"`
- `X-EduGrade-Watermark`

导出操作会写入一条 `audit.exported` 审计记录，目标类型为 `audit_log`。
