# STORY-061B0.3B 服务器端题块解析器实现复审（2026-07-30）

## 结论

通过。已实现不可达的服务器端 active crop resolver，并补充 tenant/segment/question 范围查询、对象内容证明和读取后重新确认机制。没有新增路由、Handler、Adapter 调用、外部网络请求或数据库迁移。

## 实现内容

- `segment.PostgresStore` 新增 `GetEvidenceForQuestion`：
  - tenant、segment 和 question 同时进入 SQL 条件；
  - 原有 `GetEvidence` 行为保持不变；
  - memory store 继续 fail closed，不伪造图片 evidence。
- `subjective.ActiveCropResolver`：
  - 只接受 tenant、segment、question 三个非空范围字段；
  - 验证 segment、submission、exam、registration、processing、crop asset 和 hash；
  - 区分普通 registration owner 与当前 applied correction owner；
  - file asset 必须是 private、精确 `image/png`、大小在 5 MiB 内；
  - normalized bbox 必须严格四字段、在来源页内、最多六位小数且面积小于 90%；
  - `LimitReader(5 MiB + 1)` 后重新验证实际大小、PNG/IHDR、1200 万像素和 SHA-256；
  - 对象读取后再次读取 evidence 与 asset，快照变化即拒绝。
- `ResolvedActiveCrop` 只输出 v2 所需的图片事实和内存 bytes，不包含身份、考试、答卷或存储位置。

## 测试覆盖

- 普通 active crop 成功解析；
- 当前 applied correction preview 成功解析；
- 旧 correction owner 被拒绝；
- tenant、segment、question 任一不匹配时不访问 asset/object；
- correction 在对象读取期间撤销时被拒绝；
- 对象内容替换被拒绝；
- processing/registration 未完成、跨 exam/submission、错误 owner、公开 visibility、JPEG、deleted asset、hash 不符被拒绝；
- 整页 bbox、未知 bbox 字段、超限元数据和 5 MiB 以上实际对象被拒绝；
- context cancellation 不被错误包装掩盖；
- PostgreSQL E2E 使用真实 segment/file asset 复合关系验证正常解析、跨 tenant、跨 question 和 owner 篡改。

## 运行边界

- `NewActiveCropResolver` 当前只有测试调用方。
- Router、segment Handler、subjective Handler 和 HTTP Adapter 均未构造 resolver。
- resolver 不执行 Base64 编码，不构造 `grading-agent-v2` 请求。
- resolver 不记录图片、OCR、bucket、key 或 hash 日志。
- v1 grading-agent 生产路径保持不变。

## 验证结果

- `go test ./... -count=1`：通过；未配置 PostgreSQL 环境时新 E2E 按既有约定跳过。
- `go vet ./...`：通过。
- `go test ./internal/subjective ./internal/segment -count=1`：通过。
- `npm run check:story057`：通过。
- grading-agent Python 全量测试与 Ruff：通过。
- `git diff --check`：通过。

## 下一决策点

下一步只能独立评审 061B0.3C：不可达的 Go v2 request builder 与 Python v2 application seam。必须保持 v1 请求上限和路由不变，图片只从本 resolver 的返回类型进入 builder，并验证 Base64 内存放大、8 MiB 上限、幂等摘要、日志和失败进入人工队列。
