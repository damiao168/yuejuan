# STORY-061B1B DashScope 原生 fixture 传输接缝实现复审（2026-07-30）

## 结论

通过。已建立 DashScope 原生文本/多模态协议的离线执行接缝，但它只接受精确的 `FixtureDashScopeTransport`，没有 URL、Socket、HTTP 客户端或默认网络实现，也未注册为 `ProviderAdapter`。

## 实现边界

- 只使用既有原生路径：
  - `/services/aigc/text-generation/generation`
  - `/services/aigc/multimodal-generation/generation`
- 不包含 OpenAI-compatible 路径或请求结构。
- 文本请求最大 512 KiB，多模态请求最大 8 MiB，响应最大 1 MiB。
- 请求 JSON 禁止 NaN/Infinity，并在使用后覆盖、清空临时可变字节。
- 响应必须是 `application/json`、UTF-8、单一严格 JSON；重复字段、NaN/Infinity、错误 MIME、空响应和超限响应全部 fail closed。
- 只允许一次有界重试；仅 timeout、429/节流和服务端暂态错误可重试。
- 厂商错误统一映射为现有安全错误，不返回厂商原始 message。
- payload 再次拒绝内部身份字段；文本路径拒绝图片，多模态路径只允许一个内联 `data:image/png;base64`，拒绝 HTTP、文件和对象存储 URL。
- fixture transport 只记录路径、请求体 SHA-256、字节数、Header 名称和 timeout，不记录 body、Base64 或 Authorization 值。

## 不可达性证据

- `DashScopeNativeTransportSeam` 只接受类型精确匹配的 `FixtureDashScopeTransport`，自定义或具备网络能力的 transport 无法注入。
- 使用固定的合成 Authorization 标记，不读取环境变量、Secret 文件或治理数据库。
- `default_provider_adapter_registry()` 仍只包含 `local_llama_cpp`。
- grading-agent HTTP Server 仍只注册 v1 `/grading/grade`。
- Go Router、subjective Adapter 和 v2 application seam 均未引用该传输接缝。

## 测试覆盖

- 文本与多模态成功 fixture 使用精确原生路径并解析结构化结果。
- 调用记录与日志不包含图片 Base64、请求 body 或 Authorization。
- 节流错误重试一次后成功；认证错误不重试且错误信息脱敏。
- timeout 最多重试一次并映射为 `model_timeout`。
- 请求/响应大小、MIME、重复 JSON 字段、非有限数值和禁止身份字段均 fail closed。
- 远程图片 URL 和自定义网络 transport 被拒绝。
- 默认 Provider 注册表保持仅本地 Adapter。

## 验证结果

- grading-agent Python 全量 75 项 unittest：通过。
- `ruff check ai-services`：通过。
- `npm run check:story057`：通过。
- `git diff --check`：通过。

## 下一决策点

061B1C 不能由代码自动假定批准。只有提供真实沙箱账号、合同/留存与区域结论、Secret 引用、价格、预算、合成数据范围以及图片外发授权，并由 061B1A 门禁全部通过后，才能独立评审实际 HTTP transport 与影子 Adapter 注册。首次网络调用仍只能发送合成数据。
