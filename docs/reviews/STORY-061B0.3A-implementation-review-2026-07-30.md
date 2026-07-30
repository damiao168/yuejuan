# STORY-061B0.3A 内部图片契约实现复审（2026-07-30）

## 结论

通过。该切片只建立不可达的 `grading-agent-v2` Schema、合成 fixture 和 Go/Python 离线验证器，没有修改生产 HTTP 路由、Go 主观题 Adapter、数据库、对象存储读取逻辑或外部厂商注册表。

`grading-agent-v1` 保持不变，当前应用仍拒绝 v2 请求；因此本实现不能被生产流量或外部调用触发。

## 已实现范围

- 新增独立的 `contracts/grading-agent/v2`：
  - request、response 和 error JSON Schema；
  - 明确 `unreachable_fixture_only`、`http_route_enabled=false`、`external_provider_enabled=false`；
  - 单个 `answer_segment_crop`、Base64、PNG、5 MiB、1200 万像素和 8 MiB 内部请求上限；
  - 文字证据与 crop 局部坐标证据的严格联合类型；
  - 身份、存储位置、URL 和最终成绩字段黑名单。
- 新增合成 fixture：
  - 合法图片请求与图片证据响应；
  - 远程 URL、整页 bbox、图片证据哈希不一致等非法样例；
  - v2 非重试错误样例。
- 新增 Python 离线验证器：
  - 复用 v1 的题目、Rubric、模型策略和提示防护校验；
  - 重新计算 Base64 解码大小、PNG 签名/IHDR 尺寸、像素数和内容 SHA-256；
  - 验证来源页 bbox、六位小数规范和 90% 面积上限；
  - 重新计算 request/question/segment/crop/bbox 的长度前缀绑定摘要；
  - 验证 `answer_text` 和 `answer_crop` 两类响应证据及其 Rubric 链接。
- 新增 Go 测试侧 fixture 校验器：
  - 独立读取相同 fixture；
  - 重新计算 PNG、SHA-256、尺寸、bbox 和跨运行时绑定摘要；
  - 证明非法 fixture fail closed；
  - 明确锁定生产 HTTP Adapter 仍使用 v1。
- 扩展 STORY-057 契约门禁，持续检查 v1 不变、v2 不可达及合成图片证明。

## 安全边界

- v2 只允许一个内嵌 PNG，不允许 URL、bucket、storage key、原始文件名或多媒体数组。
- 图片哈希和 bbox 与 request、question、answer segment 共同进入绑定摘要；绑定摘要只用于发现拼装错误，不替代租户授权。
- 输入图片最多 5 MiB、1200 万像素，Base64 字符串在解码前也有长度上限。
- 来源页 bbox 必须在 `[0,1]` 内且面积小于 90%；响应图片证据使用 crop 内局部坐标。
- 所有 v2 结果仍强制人工复核、禁止发布最终成绩。
- v2 验证代码未被 `app.py`、`server.py` 或 Go 生产 Adapter 导入。

## 验证结果

- `npm run check:story057`：通过。
- `PYTHONPATH=ai-services python -m unittest discover -s ai-services/tests -p 'test_*.py'`：59 项通过。
- `python -m ruff check ai-services`：通过。
- `go test ./internal/subjective -count=1`：通过。
- `git diff --check`：通过。

## 明确未批准

- 061B0.3B 服务器端 active crop 解析器；
- 061B0.3C v2 内部装配、请求上限和 fixture Adapter；
- v2 HTTP 路由或生产开关；
- grading-agent 读取对象存储或反向调用 API Gateway；
- DashScope 或其他外部厂商 Adapter、Secret、网络请求和真实答卷外发；
- 自动写分、成绩发布或绕过人工复核。

## 下一决策点

下一步只能独立评审 061B0.3B。实现时应复用数据层的 tenant/segment/question/registration/correction/file asset 关系，在服务器端读取一个当前题块并重新验证 owner、hash、PNG、尺寸和 bbox；不得复用面向用户的图片 Handler，也不得接入 v2 HTTP 调用。
