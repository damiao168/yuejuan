# STORY-061B 独立计划评审（2026-07-29）

## 结论

STORY-061B **尚未批准真实厂商 API 接入**。当前只批准不产生任何外部网络调用的 `061B0`：

1. 抽出供应商无关的 `ProviderAdapter` 执行接口。
2. 建立显式 Adapter 注册表，未显式注册的类型必须 fail closed。
3. 保持默认构建只启用 `local_llama_cpp`。
4. 为后续厂商原生协议 fixture、错误映射和影子执行器提供稳定接缝。

任何真实 API Key、厂商 SDK、外部请求或学生数据外发仍不在本次批准范围内。

## 官方能力核对

### 阿里云百炼 / DashScope：保留为首批候选

- 官方仍提供独立 DashScope 原生接口：纯文本使用 `Generation`/`text-generation`，多模态使用 `MultiModalConversation`/`multimodal-generation`，不需要 OpenAI-compatible 接口。
- Qwen 视觉模型支持图片理解、OCR 与非思考模式结构化输出，可用于题块图片的合成数据影子评测。
- 业务空间专属域名支持华北 2（北京）地域和明确部署范围；地域、API Key 与模型列表相互隔离。
- 官方说明调用数据不会用于模型训练，但会依法存储调用数据。因此在采购/法务确认具体留存周期、删除机制和日志字段前，只能使用合成数据，不能发送真实答卷。

官方资料：

- <https://help.aliyun.com/zh/model-studio/text-generation>
- <https://help.aliyun.com/zh/model-studio/qwen-api-via-dashscope>
- <https://help.aliyun.com/zh/model-studio/vision-model/>
- <https://help.aliyun.com/zh/model-studio/regions/>
- <https://help.aliyun.com/zh/model-studio/faq-about-alibaba-cloud-model-studio>

### 腾讯云：暂不进入首批实现

- 旧混元原生 API 具备腾讯云 API 3.0 调用方式和多模态能力，但官方已公告能力逐步迁移至 TokenHub，旧平台不再新增模型能力。
- TokenHub 当前模型调用明确采用 OpenAI API / Anthropic API 兼容协议，不符合本项目“不接 OpenAI-compatible 兼容层”的协议决策。
- 因生命周期与协议方向均不稳定，不能为了凑齐第二家厂商而接入旧接口。

官方资料：

- <https://cloud.tencent.com/document/product/1729/104753>
- <https://cloud.tencent.com/document/product/1729/101848>
- <https://cloud.tencent.com/document/product/1823/130078>

### 百度千帆：暂不进入首批实现

- 当前文本生成端点为 `/v2/chat/completions`，官方快速开始直接采用 OpenAI SDK，并声明兼容 OpenAI SDK。
- 即使该端点由百度官方托管，也不满足本项目要求的“厂商原生协议差异由独立 Adapter 明确表达”。

官方资料：

- <https://cloud.baidu.com/doc/qianfan-api/s/3m7of64lb>
- <https://cloud.baidu.com/doc/qianfan-docs/s/qm8qxemze>

## 061B0 技术边界

`061B0` 只能改变 grading-agent 内部构造方式：

```text
GradingAgentApplication
        |
        v
ProviderAdapterRegistry -- exact adapter_type allowlist
        |
        +-- local_llama_cpp（唯一默认实现）
```

必须保持：

- Go API、浏览器和评分业务不出现厂商 SDK 类型。
- 未注册 Adapter 在服务启动阶段失败，不允许静默回退到本地或兼容接口。
- 现有内部 `GradeRequest -> GradeSuggestion` 契约不变。
- 本地模型的建议/人工复核边界不变。
- 不新增 API Key 配置，不读取外部 Secret，不产生外部请求。

## 真实接入前置条件

### 阿里云候选进入 061B1 前

1. 提供独立沙箱业务空间与最小权限 API Key，并接入受支持的 Secret Manager。
2. 明确华北 2（北京）地域、部署范围、调用数据留存周期、删除机制和“不用于训练”条款。
3. 固定一个文本模型版本和一个多模态模型版本，不使用 `latest`。
4. 只允许合成答案文本与合成题块图片；真实学生数据继续禁止。
5. 建立原生请求、响应、限流、错误、用量和 request id fixture。

### 第二家厂商进入 061B2 前

必须找到仍由厂商维护的原生协议，并完成与阿里云相同的地域、留存、训练用途、计量和错误语义评审。没有合格候选时保持单候选，不以兼容代理补位。

## 停止条件

出现以下任一情况立即停止该厂商实现：

- 只能通过 OpenAI-compatible 兼容层调用。
- 无法固定模型版本或无法取得原生 request id / 用量事实。
- 无法确认处理地域、数据留存或训练用途。
- 结构化输出经过一次修复仍不能满足内部 Schema。
- 供应商要求发送整卷、身份字段或与当前题无关的上下文。
- 合成冻结集出现严重错误、证据伪造或不可解释的分数漂移。

## 决策

批准实施 `061B0` Provider Adapter 接缝；不批准任何真实厂商 Adapter。完成 `061B0` 后，下一决策点是获取阿里云沙箱/合同信息并选择第二个真正原生协议厂商，而不是立即开始外部调用。
