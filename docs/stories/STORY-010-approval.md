# STORY-010 自审审批记录

## Story

STORY-010 文件上传与对象存储

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 真实文件上传 | 生产路径使用 MinIO/S3 `PutObject`，测试路径使用显式 MemoryObjectStorage | 通过 |
| 数据库只保存元数据 | `file_asset` 保存名称、类型、大小、hash、bucket/key；响应不暴露 bucket/key | 通过 |
| 文件类型校验 | 白名单扩展名和 Content-Type；`TestUnsupportedFileTypeRejected` 覆盖 | 通过 |
| 文件大小校验 | `EDUGRADE_FILE_MAX_UPLOAD_BYTES`；`TestOversizedFileRejected` 覆盖 | 通过 |
| 文件名安全 | 清理路径和控制字符；`TestPathFilenameIsSanitized` 覆盖 | 通过 |
| hash 和重复上传 | 服务端计算 SHA-256；`TestDuplicateUploadRejected` 覆盖 | 通过 |
| 私有下载 | `GET /api/v1/files/{id}/download` 后端流式返回，不暴露 storage key | 通过 |
| 删除 | 软删除元数据并尝试清理对象存储；测试覆盖删除后 404 | 通过 |
| 权限 | 所有文件路由要求 `file:manage`；`TestFilePermissionDenied` 覆盖 | 通过 |
| 审计 | 上传、下载、删除均调用 audit；测试覆盖 action 存在 | 通过 |
| 系统能力声明 | `file_upload` 已在 capabilities，未留在 not_implemented | 通过 |
| API 文档 | `docs/api/files.md` 已新增 | 通过 |
| 不越界 | 未实现 OCR、断点续传、答卷采集、前端页面 | 通过 |

## 运行命令与结果

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

## 剩余风险

- 尚未跑真实 PostgreSQL + MinIO 集成上传测试。
- 删除后的对象存储清理失败目前返回 `pending`，后续需要运维补偿任务。
- 病毒扫描、内容安全检测、断点续传属于后续 Story。

## 下一步

进入 `STORY-011 答卷采集与 Submission`，在文件上传能力之上建立答卷、页面、学生识别状态、采集批次和质量检查入口。
