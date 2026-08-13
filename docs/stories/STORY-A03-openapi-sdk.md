# STORY-A03 OpenAPI 与 Generated SDK

状态：Implemented（定向校验通过，未宣称 OpenAPI 已覆盖全仓 API）

## 范围

- 从 `services/api-gateway/openapi/edugrade-api.openapi.json` 确定性生成 `packages/sdk/src/generated/{types,client}.ts`。
- 将 A01 Assessment API 和 A02 Exam Workspace API 纳入生成契约，Web 端只保留极薄适配层。
- 提供统一的 cursor page schema、wire error schema 与 SDK 规范化错误结构。
- CI 检查生成物无差异，并相对受控 baseline 阻止未审批的破坏性变更。

## 非范围

- 不为仓库中尚未进入 OpenAPI 的接口虚构类型或 endpoint。
- 不在本 Story 大面积重写 Web/Desktop API；review/score 在其真实 response schema 补齐后逐步迁移。
- 不改变既有认证、CSRF、幂等键和文件下载传输实现；SDK 复用调用端 `ApiTransport`。

## 验证

```powershell
npm run generate:sdk
npm run check:openapi-breaking
npm --workspace packages/sdk run typecheck
npm --workspace apps/web-admin run typecheck
```

## 证据边界

- Assessment 的核心枚举和 DTO 已不再由 Web 手写复制。
- Exam Workspace 使用真实服务端投影并由 OpenAPI 生成类型。
- 当前 OpenAPI 的 `x-edugrade-coverage` 仍为 `core-pilot-partial`；生成 SDK 不代表未登记 API 已获得契约保障。
