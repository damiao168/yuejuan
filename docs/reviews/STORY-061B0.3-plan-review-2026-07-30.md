# STORY-061B0.3 内部图片契约计划评审（2026-07-30）

## 结论

计划评审通过，但只批准分阶段实现**不可达的内部图片契约与服务器端题块解析器**，不批准真实厂商 Adapter、外部网络调用或生产图片路由。

本次评审作出两个明确决定：

1. 图片输入必须建立 `grading-agent-v2`，不能向严格的 v1 请求临时添加可选字段。
2. 首版使用 Go API 在服务端验证并读取题块后，将一个受限 Base64 PNG 内联到内部 v2 请求；暂不新增 grading-agent 反向取图端点或一次性资源令牌。

## 现状证据

### 已有可复用能力

- `answer_segment` 已保存 tenant、submission、question、registration、normalized/pixel bbox、crop file asset 和 crop SHA-256。
- `segment.GetEvidence` 使用 tenant 范围查询，并能得到考试、题目、注册状态、修正记录和当前题块哈希。
- `segment.GetImage` 已验证：
  - 题块处理和配准均已完成；
  - file asset 与 evidence 属于同一考试；
  - asset hash 与 crop hash 一致；
  - asset 未删除、是图片且大小非零；
  - owner 是当前注册运行的 `answer_segment_crop`，或是已经应用的配准修正预览。
- Go → grading-agent 已使用独立 Bearer service token、请求 ID 与幂等键。
- grading-agent 已有请求体上限、严格 JSON、身份字段白名单、结构化输出、证据和人工复核边界。

### 当前不能复用的部分

- `subjective.PostgresStore.LoadContext` 只返回 `submission_page_id + bbox`，没有读取 active crop asset、crop hash、注册状态或对象内容，不能作为外发依据。
- `AnswerImageRef` 当前会被 Go HTTP Adapter 刻意丢弃，这是正确的 v1 行为。
- `grading-agent-v1` 使用 `additionalProperties: false`，并在 Go、Python和响应验证中写死 `grading-agent-v1`。
- v1 evidence 只允许 `answer_text`，无法准确表达来自题块图像坐标的证据。
- 当前没有 grading-agent → API Gateway 的独立服务凭据、私有监听器或资源授权存储。

## 为什么必须使用 v2

图片不是普通可选字段，它改变了四个安全和契约事实：

1. 内部请求体从约 256 KiB 提升到最多约 8 MiB。
2. 输入需要验证租户所有权、对象内容、MIME、哈希、尺寸和题块范围。
3. 幂等摘要必须绑定图片内容，不能只绑定 OCR 文本。
4. 输出证据需要支持 `answer_crop` 及归一化 bbox，而不是伪装成 `answer_text`。

因此保持 v1 完全不变；文本链路继续使用 v1，只有明确的图片影子任务可以在后续门禁通过后使用 v2。

## v2 请求最小结构

`grading-agent-v2` 继承 v1 的题目、Rubric、OCR 文本、模型策略和提示注入边界，并且必须新增一个必填的 `media_evidence`：

```json
{
  "schema_version": "grading-agent-v2",
  "request_id": "opaque-request-id",
  "question_id": "internal-question-id",
  "answer_segment_id": "internal-segment-id",
  "answer_text": "OCR text remains untrusted",
  "media_evidence": {
    "kind": "answer_segment_crop",
    "encoding": "base64",
    "media_type": "image/png",
    "sha256": "64-lowercase-hex",
    "byte_size": 123456,
    "width_pixels": 1600,
    "height_pixels": 900,
    "normalized_bbox": {
      "x": 0.1,
      "y": 0.2,
      "width": 0.5,
      "height": 0.25
    },
    "binding_hash": "64-lowercase-hex",
    "data_base64": "..."
  }
}
```

约束：

- 只允许一个 `answer_segment_crop`。
- 只允许 `encoding=base64` 和 `media_type=image/png`。
- 解码后最多 5 MiB、最多 1200 万像素；完整内部 JSON 最多 8 MiB。
- bbox 必须在 `[0,1]` 内，面积小于来源页 90%。
- `byte_size`、SHA-256、PNG 头真实宽高必须和解码内容一致。
- 不包含 tenant、school、student、exam、submission、原始文件名、bucket、storage key、URL、最终成绩或发布状态。
- `binding_hash` 是 request、question、answer segment、crop hash 和 canonical bbox 的一致性摘要；它用于发现错误拼装，不代替租户授权。
- `question_id` 和 `answer_segment_id` 只存在于 Go → grading-agent 的内部契约；构造厂商请求时必须像 B0.2 一样删除这些内部关联标识。

## v2 响应证据

v2 建议结果继续保持人工复核，并扩展 evidence：

- `location=answer_text`：保持 v1 的 excerpt 验证。
- `location=answer_crop`：必须包含当前 crop 内归一化 bbox、对应 Rubric point 和 crop SHA-256。
- 图片证据 bbox 必须有限、正数且落在 crop 范围内。
- 模型返回的图片哈希必须等于请求哈希。
- 模型不得返回学生身份、整卷坐标、文件 URL 或新的外部资源引用。
- v2 结果仍只是 `shadow_only` / `teacher_suggestion`，不能写入最终成绩。

## 服务端题块解析流程

```text
Subjective grading request
        |
        v
tenant + segment + question scoped DB query
        |
        v
active registration/correction + crop asset ownership checks
        |
        v
object read with 5 MiB + 1 byte hard limit
        |
        v
SHA-256 + PNG header dimensions + bbox + page coverage checks
        |
        v
build grading-agent-v2 JSON (one inline crop)
        |
        v
internal authenticated grading-agent call
```

应用过的配准修正预览可以被规范化为逻辑 `answer_segment_crop`，但必须同时满足：

- correction 状态为 applied；
- correction ID 与 evidence 当前记录一致；
- asset owner ID 等于该 correction ID；
- crop SHA-256 等于当前 evidence hash。

任何检查失败都进入人工队列，不回退到整页或原始答卷。

## 为什么首版不使用一次性取图令牌

一次性反向取图令牌看似可以缩小 JSON，但在现有架构中会新增：

- grading-agent → API Gateway 的反向服务身份和网络入口；
- token hash、有效期、消费状态与审计存储；
- token 已消费但响应中断时的恢复语义；
- Go 重试、grading-agent 幂等重放与单次消费之间的冲突；
- 被公开 Nginx 或错误路由暴露内部资源端点的风险。

当前题块上限为 5 MiB，Base64 后加提示文本仍可受控在 8 MiB 内。首版内联方案更简单、可审计且没有资源 URL。只有真实压测证明内联成为瓶颈后，才重新评审 request-bound 短期 grant；不能无证据提前增加令牌系统。

## 威胁模型

| 威胁 | 强制控制 |
| --- | --- |
| 跨租户读取 | 所有 segment、submission、file asset 查询必须带 tenant；数据库复合外键继续生效 |
| 跨题/跨答案段拼图 | segment-question 关系查询 + canonical `binding_hash`，两端重新计算 |
| 旧配准或旧修正题块 | 只读取当前 active evidence；请求前重新查询 hash 和 correction 状态 |
| 对象存储内容被替换 | 读取后重新计算 SHA-256，与 DB evidence 和 file asset 双重比对 |
| MIME/扩展名伪造 | 只接受 PNG；校验签名、IHDR 和真实宽高，不只信 Content-Type |
| 整页伪装成题块 | owner、registration/correction 绑定 + normalized bbox + 页面覆盖率门禁 |
| 超大对象/内存耗尽 | DB size 预检、`LimitReader(5 MiB + 1)`、像素上限、v2 HTTP 8 MiB 上限 |
| Base64 多次复制放大内存 | v2 解码校验后只保留一份媒体内容；幂等摘要使用已验证的 media hash/元数据，不把 Base64 放入缓存；每进程 v2 推理并发先限制为 1 |
| 压缩/多值 JSON 绕过 | v2 只接受 UTF-8 `application/json`，拒绝 Content-Encoding，严格 EOF 与未知字段 |
| 幂等键复用不同图片 | 幂等摘要包含完整规范化请求和图片哈希；同键不同内容返回 conflict |
| 图片中的提示注入 | 图片和 OCR 均标记为不可信；输出仍需 Schema/证据验证并强制人工复核 |
| 敏感图片进入日志 | 不记录请求 body、Base64、OCR 原文、bucket/key；只记录 hash、大小和结果状态 |
| 外部调用绕过租户策略 | v2 构造前检查 tenant/exam policy、deployment promotion 和 image export 授权；默认关闭 |

## 实现切片

### 061B0.3A：契约与 fixture（可实施）

- 新建 `contracts/grading-agent/v2`，保持 v1 文件完全不变。
- 添加合法/非法图片请求、图片证据响应和错误 fixture。
- Python 只实现 v2 Schema/媒体证据验证器，不给 HTTP 路由接入 v2。
- Go 只添加契约 fixture 校验测试，不改变 HTTP Adapter。

### 061B0.3B：服务器端题块解析器（A 复审后）

- 新建独立 resolver，复用 segment、file asset 与 object storage，不复用用户 `GetImage` Handler。
- 只在内存中读取一个当前题块，严格限制大小并在使用后释放。
- 测试跨租户、跨题、旧修正、owner/hash/MIME/尺寸/bbox 和对象替换。
- 不修改生产路由和外部策略。

### 061B0.3C：v2 内部装配（B 复审后）

- Go v2 request builder 与 Python v2 application seam。
- 新增独立 8 MiB 请求上限，v1 仍保持现有限制。
- v2 解析后清除临时 JSON/Base64 副本，幂等缓存和日志只保留 hash/元数据。
- 只接 fixture Adapter，外部 Adapter 注册表仍无 DashScope。
- 覆盖幂等、并发、失败进入人工队列和日志不含图片。

### 061B1：真实沙箱（仍未批准）

必须在 061B0.3A～C 全部复审后，再叠加厂商沙箱、区域、Secret、留存、价格、固定模型版本和 image export 租户授权，才能独立申请批准。

## 验收门禁

1. v1 的 Schema、fixture、路由、请求上限和测试完全不变。
2. v2 合法请求只有一个经过证明的 PNG crop；所有 URL、多图片、整页和身份字段被拒绝。
3. Go resolver 在真实 PostgreSQL 测试中证明 tenant/segment/question/asset 关联无法绕过。
4. 对象读取后 hash、PNG 尺寸和 bbox 均重新验证。
5. v2 请求/错误/日志测试证明不输出 Base64、OCR 原文、学生身份或存储位置。
6. v2 输出的文字/图片证据分别验证，任何失败都不产生有效建议。
7. v2 路由默认关闭，未批准 deployment 或未授权 image export 时不可达。
8. 本地、CI 和容器门禁全部通过后才能进入下一切片。

## 决策

批准 061B0.3A 契约与 fixture 实施；061B0.3B、061B0.3C 和任何真实厂商调用必须逐片复审。首版明确采用内部认证请求中的受限 Base64 PNG，不建设反向取图令牌。
