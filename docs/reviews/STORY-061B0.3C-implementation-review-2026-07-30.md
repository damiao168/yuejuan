# STORY-061B0.3C v2 内部装配实现复审（2026-07-30）

## 结论

通过。已实现不可达的 Go `grading-agent-v2` 请求构造器与 Python fixture-only application seam，并完成请求上限、图片证明、内存生命周期、幂等、并发、输出证据和日志边界验证。

本切片没有新增 Router、HTTP Handler、生产配置、Provider 注册、外部凭据或网络传输。现有 `/grading/grade` 仍只接受 `grading-agent-v1`；任何真实厂商 API、影子流量和学生图片外发仍未获批。

## Go v2 请求构造器

- 输入图片只能来自 `ResolvedActiveCrop`，构造前重新校验：
  - `answer_segment_crop`、`image/png` 和 5 MiB 上限；
  - 实际字节数、PNG/IHDR 尺寸、1200 万像素上限和 SHA-256；
  - 最多六位小数、位于来源页内且面积小于 90% 的 normalized bbox。
- 重新规范化并验证 request、question、segment、subject、grade、question type、Rubric、模型版本、提示版本和 OCR 置信度。
- `binding_hash` 使用与 Python 契约一致的长度前缀算法，绑定 schema、request、question、segment、crop hash 和 canonical bbox。
- 只生成一个内联 Base64 PNG；完整 JSON 独立限制为 8 MiB。
- 返回值只保留请求体和非敏感摘要事实；`Clear` 会覆盖并释放请求体。
- 幂等摘要使用完整规范化请求和已验证的图片 hash/元数据，不把 Base64 放入摘要缓存事实。
- `AnswerImageRef`、tenant、student、exam、submission、bucket、storage key、URL 和最终成绩不会进入 v2 请求。

## Python v2 application seam

- seam 只能接受精确的 `OfflineV2FixtureAdapter`，不读取生产 `ProviderAdapter` 注册表，不构建本地或外部模型。
- 调用方必须提供已接收请求体大小；独立执行 8 MiB 上限并拒绝任何 `Content-Encoding`。
- v2 校验不再深拷贝包含 Base64 的完整请求；临时 media envelope 在调用结束时主动清空 Base64 引用。
- 幂等缓存只保留 SHA-256 摘要和已验证建议，不保留请求、OCR 或 Base64。
- 同键同内容并发共享一次推理；同键不同内容返回 conflict；不同请求也由进程级容量 1 的 semaphore 串行执行。
- fixture 输出必须再次通过 v2 响应和文字/图片证据验证，且必须保持 `needs_human_review=true`。
- 失败不会缓存建议或产生有效评分；后续生产调用方只能将此类失败送入人工处理，不能写入最终成绩。
- 日志只包含 request ID、状态、次数、耗时和安全错误码；fixture 调用记录仅包含 crop hash 与字节数。

## 不可达性证据

- `BuildGradingAgentV2Request` 只有单元测试调用方。
- `GradingAgentV2ApplicationSeam` 只有单元测试调用方。
- `server.py` 仍只注册 `POST /grading/grade`，并继续调用 v1 `GradingAgentApplication`。
- v1 Schema、fixture、请求上限、响应上限、Provider 注册和 HTTP Adapter 均未修改。
- DashScope 文本/多模态 codec 仍未注册为 Adapter，也没有鉴权或网络传输实现。

## 测试覆盖

- Go 合法 v2 JSON、跨运行时 binding fixture、禁止字段、错误 crop、8 MiB 上限、安全摘要和请求体清零。
- Python 合法请求/响应、8 MiB 与压缩拒绝、缓存脱敏、日志脱敏、幂等 replay/conflict、同键合并、不同键并发容量 1、非法证据 fail closed 和 fixture-only Adapter。
- 现有测试继续证明 v2 无法进入 v1 application。

## 验证结果

- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `gofmt -l .`：通过，无未格式化文件。
- `staticcheck v0.7.0 ./...`：通过。
- grading-agent Python 全量 66 项 unittest：通过。
- `ruff check ai-services`：通过。
- `npm run check:story057`：通过。
- `git diff --check`：通过。

## 下一决策点

061B0.3 至此完成。下一步不能直接实现或调用真实 API；只能先独立评审 061B1 沙箱接入，补齐厂商账号、合同与留存、区域、Secret、图片外发授权、预算和合成 fixture 结论。首次获批联调也只能是合成数据的 `shadow_compare`，不能改变教师成绩。
