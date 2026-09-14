# STORY-MATH-10A：SolutionGraph v2

## 状态

Implemented / Real-layout Benchmark Pending

编号使用 `10A`，以保留既有 `STORY-MATH-10` Image Quality Geometry 历史。

## 目标

将 MATH-09A 的细粒度 mixed blocks 还原成可审计的解题步骤，避免服务器 canonical builder 再次退化为“一 block 一 step”。

## 实现

- 新增 `SegmentSolutionSteps`，按 y-overlap、baseline、水平间距和左右顺序合并同一视觉行的文字/公式 block。
- 相隔较远的同高 block 保持为并行解题流，不因 y 坐标接近而错误合并。
- 每个 step 保存 union bbox、`setup/transformation/calculation/conclusion/explanation/branch` 类型、recognition confidence 和 structure confidence。
- step critical confidence 取识别与结构关键链路的保守最小值；graph overall confidence 取所有 step 的最小值，不用平均值掩盖低质量关键步骤。
- block 级空间关系在合并后投影到 step；同一 step 内部关系不会生成自环，既有 `next/derives/supports/corrects/branches` DAG 边界保持不变。
- runtime canonical builder 升级为 `math-runtime-step-segmenter-v2`，Worker 提交的任意临时 graph 仍由服务器统一重建。
- OpenAPI 与 generated SDK 正式暴露 typed `MathSolutionGraph` / `MathSolutionStep` / `MathSolutionEdge`。

## 兼容边界

- 新 step 字段在 API 中为可选，历史 artifact 和历史 correction 仍可读取与校验。
- 本 Story 使用确定性几何规则，不宣称 learned layout、复杂多栏手写阅读顺序或分类讨论识别已达到真实门禁。
- 数学正确性不写入 step；syntax/equivalence/transition/constraint 仍属于后续 `MathVerification`。

## 验收

- `由题意` + 公式 + `得` 的三个同一行 block 合成一个 setup step，并保留全部 block/formula 引用。
- 下一行公式形成独立 step；平行列不会被合并。
- crossed-out block 仍从 active solution steps 中排除。
- 新 graph 通过 DAG、引用完整性、bbox、kind 和 confidence 验证。
