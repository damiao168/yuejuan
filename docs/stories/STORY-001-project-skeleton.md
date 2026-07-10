# STORY-001 企业级项目骨架

## 状态

Approved

## 自审审批

审批文件：`docs/stories/STORY-001-approval.md`

## 目标

为 EduGrade Enterprise 补齐企业级 monorepo 项目骨架，不实现复杂业务代码。

## 范围

创建或补齐目录：

- `apps/web-admin`
- `apps/desktop-client`
- `apps/teacher-portal`
- `apps/student-portal`
- `services/api-gateway`
- `services/auth-service`
- `services/tenant-service`
- `services/exam-service`
- `services/paper-service`
- `services/submission-service`
- `services/grading-service`
- `services/report-service`
- `services/appeal-service`
- `services/audit-service`
- `services/notification-service`
- `ai-services/orchestrator`
- `ai-services/ocr-service`
- `ai-services/layout-service`
- `ai-services/grading-agent-service`
- `ai-services/evidence-agent-service`
- `ai-services/consistency-agent-service`
- `ai-services/analytics-agent-service`
- `packages/shared-types`
- `packages/ui`
- `packages/sdk`
- `packages/auth-client`
- `packages/design-tokens`
- `infra/docker-compose`
- `infra/k8s`
- `infra/helm`
- `infra/monitoring`
- `infra/scripts`
- `docs/prd`
- `docs/architecture`
- `docs/api`
- `docs/database`
- `docs/agent-spec`
- `docs/ui-spec`
- `docs/deployment`
- `docs/security`
- `tests/e2e`
- `tests/load`
- `tests/security`
- `tests/ai-evaluation`

创建基础文档：

- `README.md`
- `docs/architecture/overview.md`
- `docs/prd/product-scope.md`
- `docs/security/security-principles.md`
- `docs/agent-spec/agent-overview.md`
- `docs/ui-spec/ui-principles.md`

## 非范围

- 不实现业务 API。
- 不实现真实 OCR。
- 不实现 AI 阅卷。
- 不实现 Web 具体页面。
- 不创建假数据冒充真实能力。

## 验收标准

- 目录结构完整。
- README 准确描述当前项目状态。
- 基础文档存在并说明边界。
- 没有新增不可运行的业务承诺。

## 建议测试

- 检查目录结构。
- 检查文档文件存在。
- 如保留现有前端 package，运行 `npm.cmd run typecheck` 前需先补齐缺失入口，否则本 Story 只做结构验收。
