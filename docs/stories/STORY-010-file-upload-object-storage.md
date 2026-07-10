# STORY-010 文件上传与对象存储

## 状态

Approved

## 目标

实现企业级私有文件上传与对象存储能力，支持试卷、答卷扫描件、图片、CSV、报告文件等后续业务使用。文件内容写入 MinIO/S3，数据库只保存 `file_asset` 元数据。

## Plan

- 新增文件模块 `internal/files`：
  - 类型定义和校验。
  - 元数据 Store interface。
  - MemoryStore，用于测试。
  - PostgresStore，读写 `file_asset`。
  - ObjectStorage interface。
  - MemoryObjectStorage，用于测试。
  - MinIOObjectStorage，用于真实对象存储。
  - Handler，实现上传、元数据查询、后端流式下载、删除。
- 新增配置：
  - 上传 bucket。
  - 最大上传大小。
  - 允许的文件类型。
- 新增/补充 migration：
  - `file:manage` 权限。
  - `file_asset` 约束和索引补强。
- 新增接口：
  - `POST /api/v1/files`
  - `GET /api/v1/files/{id}`
  - `GET /api/v1/files/{id}/download`
  - `DELETE /api/v1/files/{id}`
- 文件上传规则：
  - 使用 `multipart/form-data`。
  - 文件大小受配置限制。
  - 文件类型白名单：pdf、png、jpg、jpeg、csv、docx。
  - 服务端计算 SHA-256。
  - 服务端生成 storage key，客户端不能指定真实对象存储路径。
  - 同一租户、同一归属对象下同 hash 文件视为重复上传并拒绝。
- 文件下载规则：
  - 必须鉴权。
  - 后端读取对象存储并流式返回。
  - 不向客户端暴露 bucket/storage_key。
- 审计：
  - 上传、下载、删除都写 `audit_log`。

## Plan Review

- 不越界：不实现断点续传，不实现答卷采集，不触发 OCR，不创建 submission。
- 不做假功能：生产路径使用真实 MinIO/S3 客户端；测试使用明确的内存 ObjectStorage。
- 安全边界：客户端不能提交 storage key；文件名会清理路径；下载不返回对象存储真实路径。
- 与 STORY-009 衔接：`file_asset` 表已存在，本 Story 复用并补强约束；试卷配置仍可使用元数据登记，后续可再把 paper 创建改为引用真实上传文件。

## 非范围

- 断点续传实际协议。
- 文件病毒扫描。
- OCR、版面解析、答卷切分。
- MinIO bucket 初始化脚本和运维面板。
- Web/EXE 上传界面。

## 验收标准

- 可以通过 API 上传真实文件内容到对象存储。
- 数据库保存文件元数据和 hash，不保存二进制内容。
- 文件类型、大小、文件名安全校验生效。
- 重复上传可被阻止。
- 下载接口鉴权后由后端流式返回，不暴露 storage key。
- 删除为软删除，并尝试清理对象存储。
- 上传、下载、删除写审计。
- 测试通过。

## Implementation

- 新增 `internal/files`：
  - `types.go`：文件元数据、响应 DTO、Store/ObjectStorage interface。
  - `validation.go`：文件名清理、owner type、文件类型、UUID 形态、storage key 生成。
  - `store_memory.go`：测试用内存元数据 Store。
  - `store_postgres.go`：真实 `file_asset` 持久化。
  - `storage_memory.go`：测试用内存对象存储。
  - `storage_minio.go`：真实 MinIO/S3 对象存储。
  - `handlers.go`：上传、元数据查询、流式下载、删除。
- 新增 `000005_file_upload.sql`：
  - 新增 `file:manage` 权限。
  - 为 platform/tenant/school admin 和 teacher 授权。
  - 补强 `file_asset` 大小、visibility 约束和索引。
- 新增文件上传配置：
  - `EDUGRADE_FILE_BUCKET`
  - `EDUGRADE_FILE_MAX_UPLOAD_BYTES`
  - `EDUGRADE_FILE_ALLOWED_EXTENSIONS`
- API Gateway 生产启动接入 PostgresStore + MinIOObjectStorage。
- 新增 `NewRouterWithFiles`，使测试可以显式注入内存对象存储。
- 新增 `docs/api/files.md`，更新 README 和 paper-question API 边界说明。
- 更新 `system/info`：
  - capabilities 新增 `file_upload`、`object_storage`、`private_file_download`、`file_hash_deduplication`。
  - `file_upload` 从 `not_implemented` 移除。

## Implementation Review

逐项检查结果：

- 真实上传：生产路径使用 MinIO client `PutObject`，并自动确保 bucket 存在。
- 元数据：只写入 `file_asset`，响应不暴露 `storage_bucket` 和 `storage_key`。
- 类型/大小/hash：服务端校验扩展名、Content-Type、大小，并计算 SHA-256。
- 文件名安全：服务端清理路径和控制字符。
- 重复上传：同租户、同 owner、同 hash 返回 `409 duplicate_file`。
- 下载：后端鉴权后从对象存储读取并流式返回，带 `X-Content-Type-Options: nosniff`。
- 删除：软删除元数据，并尝试清理对象存储；清理失败时明确返回 `object_cleanup: pending`。
- 权限：所有文件接口要求 `file:manage`，测试覆盖权限不足。
- 审计：上传、下载、删除均写 audit。
- 不越界：未实现断点续传、OCR、submission、病毒扫描和前端页面。

实现审阅中发现并修正的问题：

- 直接构造测试配置时文件上传配置可能为空，已增加 Handler 默认值兜底。
- owner/school/exam/submission id 的坏 UUID 原本可能落到数据库错误，已提前返回 400。
- 重复上传响应已补 `request_id`。
- 下载响应已补 `X-Content-Type-Options: nosniff`。
- MinIO 下载在返回流前先 `Stat`，避免写出 200 后才发现对象不存在。

## Fixes

- 增加 UUID 形态校验。
- 增加 CSV 常见 Content-Type 兼容。
- 增加路径文件名清理测试。
- 增加超限大小测试。
- 增加非法 owner id 测试。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

启动验证：

```powershell
GET http://127.0.0.1:18090/api/v1/system/info
POST http://127.0.0.1:18090/api/v1/files
```

结果：

```text
system/info -> capabilities include file_upload, object_storage, private_file_download, file_hash_deduplication
system/info -> not_implemented only includes ocr, ai_grading
POST /api/v1/files without token -> 401
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-011 答卷采集与 Submission`。
