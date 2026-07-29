# STORY-061A3 实现复审（2026-07-29）

## 结论

STORY-061A3 的供应商/部署最小管理界面、租户模型策略中心和生产配置拒绝门禁已完成实现复审。STORY-061A 三个切片至此全部完成。

本切片没有接入真实外部厂商，没有增加 OpenAI-compatible 外部兼容接口，也没有让模型输出进入最终成绩。STORY-061 总体仍为 `In Progress`；061B 必须独立计划评审后才能启动。

## 已完成

### 模型治理管理界面

- 新增生产路由 `/system/models`，由 `model:read` 权限控制导航和访问。
- 页面集中展示当前路由模式、外部调用状态、本地部署数量、待验收外部部署数量和策略版本。
- Provider、Deployment 与租户策略使用同一治理工作区，避免业务页面感知厂商差异。
- 平台管理员可通过抽屉登记原生外部 Provider、Deployment，并探测 Secret 引用；租户管理员只能编辑本租户策略。
- Secret 探测只展示引用协议、解析器支持、是否配置和最小强度结果，不回显引用值或 Secret。
- 外部调用默认关闭；关闭时文本外发、题块图片外发、Deployment allowlist 和外部路由模式保持禁用。
- 未验收 Deployment 只显示治理状态，不能通过此页面绕过后端 promotion 和路由门禁。

界面采用克制的运维工作台结构：一个状态带、一个表格工作区和受控抽屉；桌面表格与移动端记录视图均完成真实浏览器检查。

### 生产配置拒绝门禁

- API Gateway 在 `production` 环境启动时读取实际 Provider/Deployment 库存并执行完整性校验。
- 外部 Provider 的非法 Adapter、缺失区域、非法 Secret 引用和缺失数据策略会阻止生产启动。
- 已安装解析器对应的 Secret 缺失或低于最小强度时阻止生产启动。
- 已激活 Provider 未安装对应 Secret 解析器时阻止生产启动。
- 外部 Deployment 缺少有效价格/计量方式时阻止生产启动。
- 标记为可影子路由的 Deployment 若 Provider 未激活或健康状态不可用，会阻止生产启动。
- 完整但仍为 `unverified` 的原生 Provider/Deployment 可作为治理元数据暂存，不因此错误阻止启动，也不会获得路由资格。

### Secret 探测增强

`SecretProbe` 新增 `meets_minimum_strength`。当前环境变量和 Docker Secret 解析器要求值至少 24 个字符；Vault 和云 Secret Manager 仍明确显示解析器尚未安装，值不会被读取或回显。

## 验证

- `go test ./... -count=1`
- `go vet ./...`
- `staticcheck ./...`
- `go mod tidy -diff`
- `TestStory061GovernanceFoundationE2EWithPostgresTestDatabase`（真实 PostgreSQL）
- 根工作区 `npm.cmd run typecheck`
- 根工作区 `npm.cmd run build`
- `npm.cmd --workspace @edugrade/web-admin run check:production-routes`
- Docker 构建并部署 `api-gateway`、`web-admin`
- Playwright 桌面与 390px 移动端回归：治理路由、Provider/Deployment/Policy 切换、策略抽屉、默认关闭联动和表格截断

浏览器首次进入登录页时出现一次预期的 `/api/v1/auth/me` 401，用于判定无会话并进入登录页；成功登录后的模型治理接口和页面无新增控制台错误。

## 复审中修复

真实桌面截图发现本地 Provider 名称的最小宽度会越过首列并覆盖“协议边界”。已移除固定最小宽度，为身份文本容器增加收缩和截断规则，并在重新构建、部署后复测通过。

## 仍未完成，不能越界宣称

- 尚无真实外部厂商原生 Adapter 或授权账号联调。
- Vault 与云 Secret Manager 解析器尚未实现。
- 尚无冻结集、多模型效果对比、promotion/revocation 和公平性报告。
- 尚无真实影子流量、预算扣减、限流、熔断和跨 Deployment 回退执行器。
- 尚无任何外部模型用于教师建议，更没有自动评分或自动发布。

## 下一步决策门

若继续 STORY-061B，必须先独立确认：

1. 首批厂商及其官方原生协议、区域、留存和“不用于训练”条款。
2. 授权沙箱账号与独立 Secret Manager 方案。
3. 仅合成/脱敏冻结集的 `shadow_compare` 数据范围。
4. 每家 Adapter 的超时、限流、计量、结构化输出和协议 fixture。
5. 质量、成本和严重错误的停止条件。

上述评审完成前，系统继续保持 `local_only`。
