# STORY-061A2 实现复审（2026-07-29）

## 结论

STORY-061A2 的 Provider、Deployment、Policy 管理 API、RBAC、审计、Secret 引用解析边界和本地基线注册已完成实现复审。

本切片没有接入真实外部厂商，没有增加 OpenAI-compatible 兼容接口，也没有允许未验收外部部署进入评分路由。STORY-061 仍为 `In Progress`，下一步仅进入 061A3。

## 已完成

### 管理 API

- `GET/POST /api/v1/model-providers`
- `PATCH /api/v1/model-providers/{id}/status`
- `GET/POST /api/v1/model-deployments`
- `PATCH /api/v1/model-deployments/{id}/state`
- `GET/PUT /api/v1/model-policy`
- `POST /api/v1/model-secrets/probe`

所有写接口使用严格 JSON 解码，未知字段和多余 JSON 文档被拒绝。策略更新使用 `expected_version` 乐观锁，避免管理员并发覆盖。

### 权限与租户边界

Migration `000059_story061_model_governance_rbac.sql` 新增：

- `model:read`
- `model:provider:manage`
- `model:policy:manage`

平台管理员拥有三项权限；租户管理员只有只读和本租户策略管理权限。租户管理员不能管理 Provider/Deployment，也不能通过 `tenant_id` 查询或修改其他租户。

平台管理员跨租户操作记录在平台租户审计中，并在审计元数据中保留目标租户 ID，避免破坏审计日志的同租户 Actor 外键。

### Secret 边界

- Provider API 不返回 `credential_ref`，只返回是否已设置引用及引用类型。
- 审计事件不保存 Secret 引用、环境变量名或 Secret 值。
- Secret 探测只返回 `scheme`、`resolver_supported` 和 `configured`。
- 当前可探测 `env://` 与 `docker_secret://`；Vault 和云 Secret Manager URI 可登记，但解析器明确返回尚未支持。
- 明文、路径穿越、用户信息、查询参数和未知协议被拒绝。
- 即使引用合法，真实 Secret 值也不会离开解析器边界。

### 本地基线注册

API 启动时为现有租户幂等注册当前配置的：

- Local Provider
- Local Deployment
- Adapter、Region、Model Version 和 Capability Profile

新租户首次读取模型治理资源时同样幂等补齐本地基线。注册不会覆盖管理员维护的部署健康状态或策略。

### 外部部署保持关闭

- 外部 Provider 只能通过 API 创建为 `unverified` 或 `disabled`。
- 外部 Deployment 只能通过 API 创建为 `unverified`。
- 061A2 API 不允许外部 Provider/Deployment 进入 active、available 或 shadow routing。
- 租户策略允许预配置授权范围，但 A1 路由器仍要求 Provider、Deployment、健康状态和显式外发权限全部满足，缺一即进入人工路径。

## 验证

- `go test ./internal/modelgovernance -count=1 -v`
- `TestModelGovernanceRoutesEnforceSeparateReadAndManagePermissions`
- `TestStory061GovernanceFoundationE2EWithPostgresTestDatabase`

覆盖：

1. Provider 响应和审计不泄露 Secret 引用或值。
2. 未知 JSON 字段被拒绝。
3. 未验收外部 Provider/Deployment 不可激活。
4. 租户管理员不能跨租户访问治理资源。
5. 策略引用未知 Deployment 被拒绝。
6. 策略陈旧版本更新返回冲突。
7. 本地 Provider/Deployment 幂等注册。
8. PostgreSQL Provider、Deployment、Policy 实际读写。
9. RBAC 权限分配符合平台/租户边界。
10. 原有默认关闭、Secret 约束、调用事实幂等和跨租户外键继续通过。

## 未完成，不能越界宣称

- 尚无管理端页面。
- 尚无生产配置拒绝门禁。
- 尚无真实外部厂商原生 Adapter。
- Vault 与云 Secret Manager 解析器尚未实现。
- 尚无外部协议联调、冻结集效果报告、promotion 或 revocation。
- 尚无预算扣减、熔断、限流和多 Deployment 回退执行器。

## 下一切片

STORY-061A3：

1. 供应商与部署最小管理页面。
2. 模型策略中心最小管理页面。
3. 外发范围、Secret 状态、部署状态和策略版本的可视化。
4. 生产配置对弱密钥、缺区域、缺价格、缺数据策略和非法 Adapter 的拒绝门禁。
