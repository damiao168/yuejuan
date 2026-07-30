# STORY-061B0.3B 服务器端题块解析器计划评审（2026-07-30）

## 结论

计划评审通过，批准实现一个**不可达的服务器端 active crop resolver**。该 resolver 只能读取当前 tenant、answer segment 和 question 共同绑定的单个题块，并在内存中完成内容证明；不批准 v2 HTTP 路由、主观题 Adapter 接入、外部厂商调用或真实图片外发。

## 现状判断

- `segment.PostgresStore.GetEvidence` 已能从 answer segment 得到 exam、question、registration、applied correction、crop asset 和 crop SHA-256。
- `files.Store.Get` 使用 tenant 范围查询，`files.ObjectStorage.Get` 可以流式读取对象。
- 面向用户的 `segment.Handler.GetImage` 已验证 processing/registration、exam、hash、owner 和删除状态，但它是 HTTP 流式响应：
  - 不接受独立 question 绑定；
  - 不限制读取后的实际字节数；
  - 不重新计算对象 SHA-256 和 PNG 头尺寸；
  - 不在对象读取后再次确认修正仍为 applied；
  - 会产生面向浏览器的响应头和审计语义。

因此不能从 grading-agent 反向调用该 Handler，也不能复用 Handler 作为内部取图接口。

## 批准的设计

### 查询边界

- 新增 `GetEvidenceForQuestion(ctx, tenantID, segmentID, questionID)`。
- PostgreSQL 查询同时包含 tenant、segment 和 question 条件。
- resolver 输入缺少任一范围字段时 fail closed。
- file asset 继续使用 tenant + asset ID 查询，并额外核对 exam、可选 submission、owner、visibility、MIME、大小和 hash。

### active owner 规则

- 普通配准题块：
  - correction ID 必须为空；
  - asset owner type 必须是 `answer_segment_crop`；
  - asset owner ID 必须等于当前 registration run ID。
- 已应用人工修正：
  - `GetEvidenceForQuestion` 必须通过 `status='applied'` 的 correction join 得到 correction ID；
  - asset owner type 必须是 `page_registration_correction_preview`；
  - asset owner ID 必须等于当前 correction ID。
- 不允许在存在当前 correction 时回退到旧 registration crop。

### 对象读取与竞态

1. 第一次读取 tenant/segment/question 范围内的 evidence 和 file asset。
2. 验证 active 状态、owner、hash、private visibility、`image/png` 和数据库大小上限。
3. 使用 `LimitReader(5 MiB + 1)` 读取一个对象。
4. 验证实际大小、PNG signature、IHDR、像素数和内容 SHA-256。
5. 第二次读取同一范围内的 evidence 和 file asset。
6. 两次快照完全一致才返回；修正撤销、crop 替换、owner/对象位置变化均 fail closed。

### 输出边界

resolver 只返回：

- `kind=answer_segment_crop`；
- `media_type=image/png`；
- SHA-256、字节数、真实宽高；
- 来源页 normalized bbox；
- 内存中的 PNG bytes。

不返回 tenant、school、exam、submission、asset ID、owner、bucket、storage key、URL 或文件名。

## 威胁与控制

| 威胁 | 控制 |
| --- | --- |
| 跨租户或跨题读取 | tenant + segment + question 同时进入 PostgreSQL 查询 |
| asset 指向其他考试/答卷 | exam 必须相同；asset submission 非空时必须相同 |
| 旧 registration crop | registration owner 必须等于当前 run；存在 correction 时禁止回退 |
| 未应用/已撤销 correction | 只接受 applied correction join；对象读取后重新查询 |
| asset 删除或改为公开 | tenant 范围 file query、deleted 过滤、`visibility=private` |
| MIME/扩展名伪造 | 只接受精确 `image/png`，再验证 PNG signature 和 IHDR |
| 对象内容被替换 | 实际字节 SHA-256 必须等于 segment 与 file asset 双重记录 |
| 超大对象或解压炸弹 | 元数据预检、5 MiB + 1 限制读取、1200 万像素上限 |
| 整页伪装成题块 | bbox 严格四字段、范围校验、六位小数规范、面积小于 90% |
| 存储位置泄露 | resolver 输出类型不含 asset、bucket、key 或 URL |

## 验收门禁

1. 普通 registration crop 和当前 applied correction crop 均可解析。
2. 跨租户、跨 segment、跨 question 在访问 asset/object 前失败。
3. 旧 correction owner、撤销竞态、错误 exam/submission/visibility/MIME/hash/bbox 均失败。
4. 对象替换、实际大小不符、非 PNG、超限对象和超限像素均失败。
5. 真实 PostgreSQL E2E 证明 tenant/question 条件与 asset owner 无法绕过。
6. resolver 没有 Router、Handler、Adapter 或 grading-agent v2 调用方。
7. Go 全量测试、vet、契约门禁和现有 Python 测试保持通过。

## 明确未批准

- v2 HTTP 路由与请求体上限；
- Base64 编码和 `grading-agent-v2` request builder；
- fixture Adapter 或任何真实厂商 Adapter；
- 图片日志、磁盘缓存、反向取图 URL/令牌；
- 影子流量、真实图片外发和自动写分。
