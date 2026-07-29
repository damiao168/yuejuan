# STORY-061A1 实现复审（2026-07-29）

## 结论

STORY-061A1 的“供应商中立身份、默认关闭策略和调用事实地基”实现通过复审。

本切片没有接入任何真实外部厂商，没有新增 OpenAI-compatible 接口，也没有开放教师建议或最终成绩自动化。STORY-061 与 061A 仍保持 `In Progress`。

## 已完成

### 1. 供应商中立契约身份

- grading-agent 的遥测新增 `provider`、`deployment` 和 `region`。
- 本地实现默认标识为：
  - Provider：`local`
  - Deployment：`local-qwen3-4b-q4-k-m`
  - Adapter：`local_llama_cpp`
  - Region：`on_premise`
- Go API 不再把 `local_llama_cpp` 和 `local-pilot-v1` 写死在通用输出验证器中；具体期望身份由受控部署配置提供。
- capability profile 必须与 grading-agent 加载的治理矩阵一致，配置漂移时服务拒绝启动。
- v1 Schema 以向后兼容方式增加供应商中立遥测字段；旧消费者不会因新增必填字段被强制破坏。

### 2. 治理数据模型

Migration `000057_story061_model_governance_foundation.sql` 新增治理模型，
`000058_story061_governance_tenant_constraints.sql` 将创建人、题目和答题切片引用收紧为同租户复合外键：

- `model_provider`
- `model_deployment`
- `tenant_model_policy`
- `model_call_fact`

约束保证：

- 外部 Provider 只能保存 Secret 引用，不能保存可回显明文密钥。
- Secret 引用只接受环境变量、Docker Secret 或受控 Secret Manager/Vault URI。
- 外部 Provider 必须声明区域和“不用于训练”的数据策略。
- Adapter 类型显式拒绝 OpenAI-compatible 兼容层。
- 新租户自动生成 `local_only`、外部关闭、文本/图片外发关闭、人工降级的默认策略。
- 外部功能关闭时，数据库不允许保留外部模式、外发权限或 Deployment allowlist。
- 调用事实不保存答案原文，只保存内部 ID、供应商/部署/版本、路由、状态、计量和 hash 等最小必要字段。
- `request_id` 在租户内唯一，幂等重放不能形成重复计费事实。

### 3. 当前评分链路

- `ai_grade` 新增 Provider、Deployment 和 Region 字段。
- 非 mock grading-agent 调用会与 `model_call_fact` 在同一数据库事务内写入。
- 成功和失败调用都保留供应商中立身份；mock 不冒充真实模型调用，也不会形成计费事实。
- 当前路由事实仍明确标记 `local_only`，因为本切片唯一启用的实现仍是本地 llama.cpp。

### 4. 默认关闭路由规则

新增 `internal/modelgovernance` 领域规则：

- 零配置策略等价于 `local_only`。
- 租户外部开关、文本/图片外发开关和 Deployment allowlist 缺一不可。
- 外部 Provider、Deployment、健康状态或影子状态不合格时不能被选择。
- 租户关闭外部后，故障回退不能绕过。
- 明文凭据和 OpenAI-compatible Adapter 在进入路由前被拒绝。

## 验证

- `go test ./internal/modelgovernance -count=1 -v`：4 项通过。
- `PYTHONPATH=ai-services python -m unittest discover -s ai-services/tests -p 'test_*.py'`：23 项通过。
- `python -m ruff check ...`：通过。
- `TestStory061GovernanceFoundationE2EWithPostgresTestDatabase`：真实 PostgreSQL 16 通过。
- CI 的 PostgreSQL 过滤条件包含 `PostgresTestDatabase`，因此该 STORY-061 E2E 已进入 main 持续回归。

PostgreSQL E2E 覆盖：

1. 既有租户默认策略外部关闭。
2. 新租户触发器同样默认关闭。
3. 明文密钥被数据库约束拒绝。
4. OpenAI-compatible Adapter 被数据库约束拒绝。
5. 合法 Secret 引用、区域、数据策略和部署元数据可保存。
6. 只有显式外发授权才能切换为影子策略。
7. 关闭外部但保留外部路由的更新被拒绝。
8. 重复 `request_id` 不能产生第二条调用/计费事实。
9. Provider 创建人不能引用其他租户的用户。

## 未完成，不能越界宣称

- 尚无 Provider/Deployment/Policy 管理 API。
- 尚无管理端“供应商与部署”或“模型策略中心”页面。
- 尚无真实外部厂商原生 Adapter。
- 尚无真实 Secret Manager 取密实现；当前只建立不可回显引用模型。
- 尚无题型级 promotion/revocation。
- 尚无预算扣减、熔断、限流和多 Deployment 回退执行器。
- 尚无外部冻结集效果、成本或教师接受率报告。

## 下一切片

STORY-061A2：

1. 平台 Provider/Deployment 管理 API 与权限边界。
2. 租户外发授权和策略读取/更新 API。
3. Secret 引用解析接口，但不返回 Secret 值。
4. 本地基线 Provider/Deployment 注册和健康状态同步。
5. 管理操作审计、租户隔离和 PostgreSQL E2E。
