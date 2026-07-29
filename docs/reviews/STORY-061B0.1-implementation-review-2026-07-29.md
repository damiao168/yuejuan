# STORY-061B0.1 实现复审（2026-07-29）

## 结论

通过。该切片只建立 DashScope 原生文本协议的离线契约和合成 fixture，不构成真实厂商接入。代码没有鉴权、HTTP 传输、外部域名、Secret 读取或 Adapter 注册路径，因此生产服务仍只启用 `local_llama_cpp`。

## 官方协议依据

- 原生文本生成请求使用 DashScope `/api/v1/services/aigc/text-generation/generation`，正文为 `model`、`input.messages` 和 `parameters`：
  <https://help.aliyun.com/zh/model-studio/text-generation>
- HTTP 原生调用的 `response_format`、`enable_thinking`、`max_completion_tokens` 和 `seed` 位于 `parameters`：
  <https://help.aliyun.com/zh/model-studio/qwen-api-via-dashscope>
- JSON mode 要求请求 `response_format={"type":"json_object"}`，且提示词明确要求 JSON：
  <https://help.aliyun.com/zh/model-studio/qwen-structured-output>
- 401、429、超时和服务异常的状态/错误码依据：
  <https://help.aliyun.com/zh/model-studio/error-code/>

## 实现范围

- 新增无网络能力的 `dashscope_native_contract`：
  - 构造原生文本生成正文；
  - 强制非流式 message 结果和 JSON object 输出；
  - 投影现有 PromptRegistry 生成的单题评分消息；
  - 阻止租户、学校、学生、考试、提交、内部题目/答案段 ID、最终成绩和图片引用成为外发结构字段；
  - 解析原生 `request_id`、输入/输出/总 Token 和结构化内容；
  - 将限流、鉴权、超时和未知服务失败映射到现有 `model_unavailable` / `model_timeout` 契约，并丢弃厂商原始错误文本。
- 新增纯合成 fixture：
  - 原生文本请求；
  - 成功响应；
  - 429 限流；
  - 401 鉴权失败。
- fixture 中的 dated model 只用于固定协议结构，不代表已批准的模型 Deployment。

## 安全与架构复审

- 默认 Provider Adapter 注册表没有变化，未注册 DashScope Adapter。
- `Settings.from_env()` 仍拒绝 `local_llama_cpp` 以外的 Adapter。
- 没有新增依赖、API Key、端点配置或网络库调用。
- 请求正文不包含 `request_id`、`question_id`、`answer_segment_id` 等内部关联标识。
- 成功响应在进入业务契约前仍必须通过现有模型输出字段、分值、证据和人工复核校验。
- 错误消息固定为平台文案，避免厂商响应中的敏感内容进入浏览器、日志或业务错误。

## 验证覆盖

- 离线 builder 与请求 fixture 完全一致，且路径不含 `compatible-mode`。
- JSON mode 明确指令缺失时 fail closed。
- 内部身份/关联字段和图片引用不进入原生请求结构。
- 成功响应保留 request id、用量和 JSON 输出，且可通过现有 grading-agent 契约。
- 截断、非 JSON、非法用量均返回 `model_output_invalid`。
- 限流可重试，鉴权不可重试，超时映射到现有超时契约；原始错误文本不穿透。
- grading-agent 全部 40 个 Python 单元测试、Ruff 和源码编译检查通过。
- STORY-057 评分契约门禁与 grading-agent 容器镜像构建通过。

## 明确未批准

- DashScope SDK 或真实 HTTP Adapter。
- API Key、业务空间域名、真实模型调用或计费。
- 多模态/图片协议。
- 真实答卷、学生答案、冻结集或任何生产数据外发。
- 影子路由、promotion、自动回退或最终成绩变更。

## 下一决策点

061B1 仍需外部前置材料：沙箱业务空间、最小权限 Secret 引用、北京区域端点、固定模型版本与价格、合同约定的数据留存/删除/不用于训练结论。材料不完整时停止在离线契约，不实现网络 Adapter。
