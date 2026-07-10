# STORY-005 后端基础服务骨架

## 状态

Approved

## 目标

实现 EduGrade Enterprise 的 Go 后端基础服务骨架。

## Plan

在 `services/api-gateway` 中实现第一版后端入口服务：

- 配置加载：环境变量 + `.env` 文件加载。
- 结构化日志：JSON 日志、request_id。
- 健康检查：`GET /health`。
- 就绪检查：`GET /ready`，检查 PostgreSQL、Redis、MinIO/S3 抽象。
- 系统信息：`GET /api/v1/system/info`。
- 统一错误响应。
- 请求 ID、访问日志、panic recovery 中间件。
- PostgreSQL、Redis、MinIO 客户端封装。
- 基础测试。
- `.env.example`。
- README 运行说明。
- Docker Compose 基础依赖校验。

## Plan Review

- 不越界：不实现认证、租户、考试、试卷、阅卷等业务 API。
- 当前仓库没有 Go 后端，因此选择 `services/api-gateway` 作为第一版后端入口，符合现有目录边界。
- 依赖连接必须是真实客户端封装，不用 mock 冒充运行能力；测试中使用 fake checker 只验证 HTTP 行为。
- MinIO/S3 在本阶段实现客户端抽象和 Ping，不实现文件上传业务。
- Docker Compose 当前已有基础依赖，但密码硬编码；本 Story 会改为 `.env` 变量驱动。

## Implementation

- 在 `services/api-gateway` 新增 Go 服务。
- 实现配置加载、结构化日志、请求 ID、访问日志、panic recovery、统一 JSON 错误响应。
- 实现 PostgreSQL driver、Redis client、MinIO SDK 的依赖检查。
- 实现 `GET /health`、`GET /ready`、`GET /api/v1/system/info`。
- 新增基础测试。
- 新增根 `.env.example`。
- 将 Docker Compose 的关键密码改为环境变量驱动。

## Implementation Review

实现审阅发现：

- 初次 `go mod tidy` 因默认 Go proxy 网络失败，依赖无法下载。
- 修复方式：使用 `GOPROXY=https://goproxy.cn,direct` 成功拉取真实依赖。
- 曾临时考虑标准库 TCP/HTTP 检查，但最终已回到真实 PostgreSQL、Redis、MinIO 客户端。
- 补充了 404 统一错误响应测试。

## Fixes

- 接入 `github.com/jackc/pgx/v5` PostgreSQL driver。
- 接入 `github.com/redis/go-redis/v9`。
- 接入 `github.com/minio/minio-go/v7`。
- 更新 README，明确 `/ready` 使用真实客户端检查依赖。
- 补充 `.gitignore` 的 `bin/`，避免构建产物进入仓库。

## 自审审批

审批文件：`docs/stories/STORY-005-approval.md`

## 非范围

- 不实现登录。
- 不实现 RBAC。
- 不实现业务数据库表 migration。
- 不实现文件上传。
- 不实现 OCR/AI。

## 验收标准

- `go test ./...` 通过。
- `GET /health` 返回 200。
- `GET /api/v1/system/info` 返回系统信息。
- `GET /ready` 能返回依赖检查结果。
- `docker compose config` 校验通过。
- README 和 `.env.example` 存在。
