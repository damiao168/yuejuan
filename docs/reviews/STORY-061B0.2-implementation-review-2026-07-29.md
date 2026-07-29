# STORY-061B0.2 实现复审（2026-07-29）

## 结论

通过。该切片只建立 DashScope 原生多模态协议的离线契约和合成图片 fixture，没有注册厂商 Adapter、读取 Secret 或执行网络请求。当前 `grading-agent-v1` 请求 Schema 仍不接收图片，Go 主观题 Adapter 仍不会把 `answer_image_ref` 发送给 grading-agent，因此本切片不能被生产流量触发。

## 官方协议依据

- 原生多模态模型使用 `/api/v1/services/aigc/multimodal-generation/generation`，用户消息内容按 `image` 与 `text` 数组组织：
  <https://help.aliyun.com/en/model-studio/vision>
- DashScope 原生 API 明确区分文本和多模态端点，并支持 `data:image/<format>;base64,<data>` 图片输入：
  <https://help.aliyun.com/en/model-studio/qwen-api-via-dashscope>
- 官方说明 HTTP 可使用 Base64，图片较小时适用；本项目进一步限定为服务器生成的单个 PNG 题块：
  <https://help.aliyun.com/en/model-studio/qwen-vl-ocr>

## 实现范围

- 新增原生多模态请求构造：
  - 路径固定为 `multimodal-generation/generation`，不包含 `compatible-mode`；
  - 只接受一个 `ApprovedImageCrop`；
  - 只生成一个内嵌 `data:image/png;base64,...` 图片部分和一个文本部分；
  - 继续使用 JSON object、非思考、message result 等受控参数。
- `ApprovedImageCrop` 只作为本地证明，包含：
  - 当前答案段绑定哈希；
  - 固定 `answer_segment_crop` scope；
  - 内容 SHA-256；
  - PNG Base64；
  - 像素宽高；
  - 来源页归一化 bbox。
- 证明字段不发送给厂商，只用于构造请求前的本地验证。
- 新增 Qwen-VL 原生数组响应解析，要求恰好一个 text 结果，并继续进入现有评分输出契约。

## 数据最小化与拒绝规则

- 不接受 `http://`、`https://`、`file://` 或 `oss://`；避免厂商下载任意资源、公开学生图片或依赖临时 URL。
- 不接受多张图片、视频、PDF、整卷或整页。
- scope 不是 `answer_segment_crop` 时拒绝。
- 绑定哈希与当前答案段不匹配时拒绝，防止跨题/跨答案段复用。
- bbox 越界、面积达到来源页 90% 及以上时拒绝。
- Base64 非法、超过 5 MiB、不是 PNG、PNG 头宽高与证明不一致、内容哈希不一致时拒绝。
- 解码像素超过 1200 万时拒绝。
- 请求中不包含题目 ID、答案段 ID、绑定哈希、图片哈希、学生身份或最终成绩。

## 验证覆盖

- 多模态 builder 与合成原生请求 fixture 完全一致。
- 请求只包含一个内嵌 PNG，不包含任何远程、本地或 OSS URL。
- 跨题绑定、错误 scope、整页 bbox、伪造 MIME、错误哈希、非法 Base64、尺寸不一致和超限像素均 fail closed。
- 原生数组响应保留 request id 与 Token 用量，结果可通过既有 grading-agent 字段、分值、证据和人工复核契约。
- 多文本结果或非数组 content 被拒绝。
- grading-agent 全部 51 个 Python 单元测试、Ruff 和源码编译检查通过。
- STORY-057 评分契约门禁与 grading-agent 容器镜像构建通过。

## 明确未批准

- 修改 `grading-agent-v1` 图片输入 Schema。
- Go API 将题块二进制或资源引用传给 grading-agent。
- DashScope 多模态真实 Adapter、SDK、API Key、端点或计费。
- 真实答卷/题块外发、影子运行、模型 promotion 或最终成绩变更。
- JPEG、WebP、多图片、视频、PDF、整卷和公开 URL。

## 下一决策点

061B1 如要进入真实沙箱，必须先设计新的内部图片输入契约和服务间资源读取方式，证明题块所有权、租户隔离、当前题绑定、内容哈希、大小和 bbox 均由服务端校验；同时补齐厂商区域、留存、删除、不用于训练、固定多模态模型版本和价格结论。在这些条件完成前，离线多模态 codec 保持不可达。
