# STORY-061B0 实现复审（2026-07-29）

## 结论

通过。061B0 只建立 Provider Adapter 内部接缝，没有接入真实厂商、读取第三方密钥或产生外部网络调用。服务的默认行为、内部评分契约和人工复核边界保持不变。

## 实现范围

- 新增供应商无关的 `ProviderAdapter` 协议，只约束 grading-agent 当前需要的 `session`、`request` 和 `ready` 能力。
- 新增显式 `ProviderAdapterRegistry`，以完整 `adapter_type` 精确选择工厂；非法标识、重复注册、未知类型和不完整实现均拒绝。
- 默认注册表只启用 `local_llama_cpp`，并在创建时继续复用现有 `LocalLlamaCppAdapter`。
- `GradingAgentApplication` 通过注册表创建默认模型，同时保留测试和受控调用方显式注入 model 的能力。
- 未修改评分请求/建议 Schema、遥测 Schema、Go API、浏览器接口或数据库。

## 安全与产品边界复审

- `Settings.from_env()` 仍只接受 `local_llama_cpp`；仅凭环境变量不能打开尚未实现的厂商。
- 注册表没有动态模块路径、反射导入或静默回退；只有代码显式注册的 Adapter 可以创建。
- 本切片没有新增 API Key 配置、SDK 依赖、供应商 URL、学生数据字段或外部调用。
- 未注册类型在服务构造阶段 fail closed，不会自动降级成本地模型并掩盖配置错误。
- 内部 `GradeRequest -> GradeSuggestion` 契约和所有既有结构化输出、证据、分数重算及人工复核校验保持原位。

## 验证证据

- Provider Adapter 单元测试覆盖：默认只启用本地 Adapter、显式选择、应用装配、未知类型拒绝、重复注册拒绝和协议不完整拒绝。
- grading-agent 全部 29 个 Python 单元测试通过。
- Python 源码编译检查和 Ruff 检查通过。
- grading-agent 容器镜像构建通过。
- STORY-057 评分契约门禁和实验室集成门禁通过。

## 明确未批准

- 阿里云或其他厂商的真实 Adapter。
- 真实/测试 API Key、供应商 SDK 和外部网络请求。
- 真实答卷、学生答案、题块图片或冻结集数据外发。
- `shadow_compare` 流量、模型 promotion、自动路由或教师成绩变更。

## 下一决策点

061B1 进入实现前必须提交并评审：

1. 厂商原生协议请求、响应、错误、限流、用量和 request id 的合成 fixture。
2. 沙箱业务空间、最小权限 Secret 引用和固定模型版本。
3. 处理区域、留存周期、删除机制及“不用于训练”条款结论。
4. 严格字段白名单和仅限单题脱敏数据的外发测试。
5. 仅影子运行、失败进入人工流程且不改变教师成绩的验收设计。
