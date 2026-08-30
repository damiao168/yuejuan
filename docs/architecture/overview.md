# Architecture Overview

本文描述仓库当前实现，不作为未来服务拆分清单。

## 运行拓扑

```text
Web Admin / Student Portal / Desktop Client
  -> services/api-gateway (Go modular monolith)
       -> internal domain modules
       -> PostgreSQL / Redis / MinIO / Qdrant / Observability
       -> asynchronous worker tasks
            -> OCR Worker
            -> Page Processing Worker
            -> Image Quality Worker
            -> Subjective Grading Worker
            -> Math Verification Worker
            -> controlled AI runtimes
```

## 业务后端

`services/api-gateway` 是统一 HTTP 入口，也是当前核心业务应用运行时。认证、组织、考试、试卷、答卷、阅卷、复核、质量、成绩发布、申诉、报告、审计、AI 治理和数学理解等能力在 `services/api-gateway/internal/*` 内按领域模块化。

这些模块共享进程和基础设施，通过明确的 Store、Service、Handler 与路由边界协作；它们不是一组已经独立部署的网络微服务。

## 独立 Worker

`services/ocr-worker`、`services/page-processing-worker`、`services/image-quality-worker`、`services/subjective-grading-worker` 和 `services/math-verification-worker` 承担异步、模型或图像计算任务。独立进程边界用于不同运行时、资源隔离、长任务执行和按计算负载扩缩容。

## 数据与基础设施

- PostgreSQL：核心业务数据、状态流转、审计和任务事实。
- Redis：缓存、限流、租约和异步协调。
- MinIO/S3：试卷、答卷图像、报告与附件。
- Qdrant：受控的向量检索能力。
- Prometheus/Grafana：指标与运行监控。
- Docker Compose：当前私有化部署与本地集成环境。

## 拆分原则

新增网络服务必须由独立扩缩容、故障域、安全边界、资源需求或发布生命周期等真实运行约束驱动。业务领域划分本身不是拆成微服务的充分理由。
