# STORY-MATH-09：Runtime Spatial / Solution Baseline Integration

状态：Implemented

## 目标

将已有确定性 `BuildSpatialRelations` 与 `BuildSolutionGraph` 正式接入 math-understanding runtime，使 Worker 识别事实在入库前生成 canonical 空间关系和解题过程图。

## 现状

Python Worker 已能输出 blocks、formulas 与 verifications，但仍发送空 relations 和单步骤占位图；Go runtime 完成入口此前直接保存该结果，已有空间关系与 DAG 构建器未进入生产调用链。

## 本 Story 范围

- immutable task binding 通过后，在 relations 为空时运行现有 deterministic spatial baseline；保留合法的非空 Worker relations。
- 始终由 Go `BuildSolutionGraph` 重建 canonical graph，并记录稳定 builder version。
- 保留 Worker 的人工复核信号，与 builder 的人工复核判断执行 OR 合并。
- syntax verification 只引用 formula，不再引用临时 `step-1`。
- 保持多 OCR block、`crossed_out` 与同一 InputHash 的现有幂等语义。

## 非范围

不新增统一 IR、模型、数据库迁移、端点、OpenAPI/SDK、前端或 Docker 改动；不实现 Answer Perception v2、learned spatial model、公式推理器或图形解析器。

## 实现

`CompleteRuntimeTask` 在既有不可变输入校验后调用小型归一化函数：仅当 Worker 未提供 relations 时构建空间关系，随后使用 blocks、formulas、relations 与 verifications 重建 SolutionGraph。最终人工复核标记同时尊重 builder 和 Worker 的 fail-closed 结果。无公式时模型版本记为 `text-only`。

## 测试

```text
cd services/api-gateway
go test ./internal/mathunderstanding -count=1

cd services/ocr-worker
python -m pytest tests/test_math_runner.py -q

git diff --check
```

## 结果

Runtime 集成测试覆盖两个独立 block 的空间关系和 DAG edge、占位图替换、Worker 人工复核信号保留及既有幂等落库；Python 测试覆盖公式引用保留和假 StepID 移除。已有 builder 测试继续覆盖 `crossed_out` 排除语义。

## 剩余边界

这是 deterministic baseline 的 runtime 接入，不是 learned layout model，也不证明复杂手写阅读顺序准确率或真实学校数据效果。AI 结果仍只作为教师建议，低置信与不支持场景继续进入人工复核。
