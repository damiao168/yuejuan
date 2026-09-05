# 本地 CPU OCR 性能优化报告（2026-09-01；2026-09-05 复核）

本轮实现与本机验证已经补齐，但真实手写准确率验收尚未完成。2026-09-05 查询本机硬件为
**AMD Ryzen 5 5500U，6 核 12 线程**，不是目标 Ryzen 7 5800H；本文数字不能标作 5800H 实测。
后文历史实验保留原始结果，最终配置复测与验证限制见第 8 节。

## 1. 修改前实际链路与瓶颈

调用链为：API Gateway `POST /submissions/{id}/ocr-tasks` 创建 `ocr_task` 与 `worker_runtime_task`；
OCR worker 每次 `claim` 一个任务，建立独立 heartbeat，启动 source task，读取隐私最小化 input，
下载页面，调用 `PaddleOCREngine.recognize`，校验 text/bbox/confidence，最后由 Gateway 在 tenant
事务边界内持久化结果并触发低置信人工复核。`paper_import_job` 走同一 engine，但保持整页路径。

修改前普通 OCR input 是整张 `submission_page`，没有 answer region；`answer_area`/早期
`answer_segment` 只生成元数据，不参与 OCR crop。capture registration 后续链路已经会生成
`pixel_bbox` 与 crop asset，但 OCR worker 没有复用。页面处理的标准 render 默认 300 DPI
（`decoder.py`、capture/registration payload）；paper import 的独立解析路径使用 220 DPI。
本轮没有降低任何全局 DPI。

Paddle 栈固定为 PaddleOCR 3.7.0 + PaddlePaddle 3.3.1，默认 PP-OCRv5 mobile det/rec、CPU。
修改前 engine 固定 `enable_mkldnn=false`、doc orientation/unwarping=false、
textline orientation=true；未显式传 CPU threads、检测边长和 recognition batch。

任务 `batch_size=1` 是可靠性约束，不是 recognition batch：一次 claim 多任务会使排在后面的
task 在前一个长推理期间得不到自己的 heartbeat。当前 lease 默认 300s，heartbeat 10s，
每个 task 一个后台续租线程。并行扩容必须使用多个独立 process，每个仍 claim 1；本地 16 GB
不默认加载两个模型。compose 中 OCR 与 math-recognition 原来都可加载重模型，各自最多 4 GB，
容易出现 CPU/RAM 竞争。

已有评测指标包括 CER、WER、region hit、bbox IoU、page success、blank false positive、
low-confidence recall、failure rate、review trigger、平均耗时和 P95。主要瓶颈按影响排序：

1. 整页文本检测处理大量非答案区域。
2. CPU detector/recognizer 推理本身，且 oneDNN/PIR 在当前版本组合不稳定。
3. 每页无条件 textline orientation。
4. 未显式约束 CPU threads，且 OCR/math worker 可能 oversubscribe。
5. bytes -> 临时 PNG -> Paddle 文件读取的 I/O（远小于推理）。

## 2. 修改内容

- `ocr_worker/config.py`：新增并严格校验 CPU threads、MKLDNN mode、HPI、方向、检测边长、recognition batch。
- `ocr_worker/engine.py`：参数透传；mobile/auto MKLDNN readiness fallback；暴露 effective MKLDNN；
  ROI 保持 `min=64` 小区域放大；未知错误继续 fail closed，避免将 `expired` 中的子串误判为 PIR 错误。
- `ocr_worker/runner.py`：安全 ROI crop、bbox page-global 平移、非法 ROI fail closed、稳定 config hash、
  分阶段结构化耗时日志；每页只解码一次，小数框向外取整并使用实际裁剪起点平移；
  保留单任务 heartbeat 和 paper import 整页 fallback。
- `api-gateway/internal/ocr/handlers.go`、`internal/submission/*`：按 tenant/submission/page 查询并返回
  privacy-safe regions；兼容 legacy 数组 bbox；联查 page 所属答卷，配准/模板坐标或待复核区域
  使整页回退，避免把 capture `pixel_bbox` 错用于原始图像；不返回身份字段。
- `benchmarks/benchmark_local.py`：确定顺序、5 页 warmup、硬件/config/input hash、P50/P95/RSS、
  ROI ground truth CER/WER/hit/review 指标和 JSON/Markdown 报告。
- Docker/compose/env：HPI 独立 build arg；单 OCR worker CPU limit；新增配置项。
- `scripts/start-local-ocr-fast.*`：仅启动必要 control plane 与一个 OCR worker，避免 math worker 竞争。

## 3. 参数变化

| parameter | before | safe default | reason |
| --- | --- | --- | --- |
| task batch | 1 | 1 | lease/heartbeat invariant |
| model | PP-OCRv5 mobile | 不变 | 未经过模型切换 gate |
| CPU threads | Paddle 隐式 | 4 | 本机矩阵 avg 最佳；12 threads 仅 P95 略低 |
| MKLDNN | false | auto（本机 effective=false） | mobile 先试；已知 oneDNN/PIR 错误回退 |
| HPI | 无 | false | 普通镜像缺 ultra-infer，readiness 失败，未达启用条件 |
| textline orientation | true | true | false 仅改善 avg 1.8%，P95 反而恶化约 12% |
| page det limit | Paddle 默认 min/64 | min/64 | max/960 的 ROI CER gate 未通过 |
| ROI det limit | 不存在 | min/64 | 保留小 crop 放大和基线识别结果 |
| recognition batch | Paddle 默认 1 | 1 | 4 在 ROI 上变慢且未改善准确率 |
| ROI | 无 | 原始页像素坐标合法且该页区域均适用时启用 | 配准/复核/旧任务/导卷保持整页 |

## 4. 历史探索性 Benchmark

环境为本机 Docker Desktop/WSL2、x86-64、CPU-only、PaddleOCR 3.7.0/PaddlePaddle 3.3.1；
输入为仓库 8 页匿名合成福建数学答卷，bundle hash
`854007ab5219bec9f7075c578459db98656cbc0cca4ee2be18c770fed701fe57`。
这不是营销 benchmark，也不能替代获授权的真实手写验收集。

| run | avg ms/page | P50 | P95 | pages/min | peak RSS | CER | WER | region hit | failure | review |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| local-baseline-v1（整页，显式 8 线程探索基线） | 20777.5 | 19920.6 | 25877.7 | 2.89 | 622 MiB | N/A | N/A | N/A | 0% | N/A |
| optimized-aggressive（整页，拒绝作为默认） | 11839.2 | 10949.1 | 15873.3 | 5.07 | 651 MB | N/A | N/A | N/A | 0% | N/A |
| ROI baseline/safe | 9429.1 | 9638.6 | 11898.9 | 6.36 | 623 MB | 1.7866 | 1.4474 | 78.95% | 0% | 55.26% |
| ROI aggressive（拒绝） | 6063.9 | 6215.0 | 7281.2 | 9.89 | 651 MB | 1.8129 | 1.5395 | 78.95% | 0% | 56.58% |
| ROI orientation off / min64 / batch1 | 9260.5 | 8948.5 | 13332.0 | 6.48 | 602 MB | 1.7866 | 1.4474 | 78.95% | 0% | 55.26% |

ROI safe 相对整页 baseline：avg 改善约 54.6%，P95 改善约 54.0%。ROI aggressive 虽更快，
但 CER 绝对恶化约 2.64 个百分点、WER 恶化约 9.21 个百分点，未过 gate。
合成集包含本应由 OMR/公式/人工路径处理的区域，所以绝对 CER 很高；本轮没有利用题型自动 skip OCR。

线程矩阵仅使用 5 页正式集（不同于上述 8 页），方向关闭、page det=max/960、ROI=min/64。
矩阵内部使用相同 5 页 warmup/正式顺序：4/6/8/10/12 threads 的 avg 分别为
5279.7/8809.7/9347.5/6840.9/5576.8 ms，P95 为
6382.5/9438.8/19145.6/10373.4/5947.3 ms。4 threads 的 avg 最佳，故选择 4；
12 threads 的 P95 略低，但不值得在同时运行 control plane 时增加 oversubscription 风险。

## 5. 本地候选配置（5800H 需复测）

```dotenv
EDUGRADE_OCR_MODEL_VERSION=ppocr-v5-mobile
EDUGRADE_OCR_DEVICE=cpu
EDUGRADE_OCR_CPU_THREADS=4
EDUGRADE_OCR_ENABLE_MKLDNN=auto
EDUGRADE_OCR_ENABLE_HPI=false
EDUGRADE_OCR_USE_TEXTLINE_ORIENTATION=true
EDUGRADE_OCR_TEXT_DET_LIMIT_TYPE=min
EDUGRADE_OCR_TEXT_DET_LIMIT_SIDE_LEN=64
EDUGRADE_OCR_TEXT_RECOGNITION_BATCH_SIZE=1
EDUGRADE_OCR_BATCH_SIZE=1
EDUGRADE_OCR_WORKER_CPU_LIMIT=4.0
```

## 6. 未采用优化与风险

没有增加 task batch、降低 300 DPI、切 server/v6、默认启动第二 worker、默认 HPI、或按题型跳过 OCR。
临时文件路径保留；官方支持 ndarray，但本轮尚无文件与内存路径的独立等价性/收益测量，
因此没有以未经验证的 I/O 收益替换生产推理输入。
HPI 实验在普通镜像 readiness 明确失败（缺 `ultra-infer`）；可选 build 改为官方
`paddlex --install hpi-cpu`，但只有完成独立 accuracy/RSS/stability benchmark 后才可启用。

主要剩余风险是：真实手写与合成数据分布差异；Paddle 3.3.1 oneDNN/PIR 兼容性；方向异常页；
小字/小数点/负号在缩放下丢失；ROI 坐标使用错误资产空间；多个模型造成 RAM/CPU 竞争。
缓解措施为 readiness + known-error-only fallback、page-global bbox 平移、严格越界验证、
legacy full-page fallback、低置信复核与现有 audit/lease/tenant isolation 全部保留。

## 7. 资料对设计的影响

PP-OCR/PP-OCRv3、PP-LCNet、SVTR 说明轻量 detector/recognizer、CPU 友好 backbone 与
检测/识别分阶段优化值得优先测试，但论文数字不能替代本机测量。官方 PaddleOCR 文档确认
`cpu_threads`、MKLDNN、HPI、det limit 和 recognition batch 均为受支持参数；HPI 文档同时说明
首次 engine build、模型/算子不支持和额外插件会影响收益，因此保持默认关闭。

参考：PP-OCR (arXiv:2009.09941)、PP-OCRv3 (arXiv:2206.03001)、PP-LCNet
(arXiv:2109.15099)、SVTR (arXiv:2205.00159)、PaddleOCR OCR Pipeline 与 High Performance
Inference 官方文档。

- [PP-OCR](https://arxiv.org/abs/2009.09941)、[PP-OCRv3](https://arxiv.org/abs/2206.03001)：支持先拆分检测/识别瓶颈。
- [PP-LCNet](https://arxiv.org/abs/2109.15099)、[SVTR](https://arxiv.org/abs/2205.00159)：支持评估轻量骨干，不作为本机速度证据。
- [PaddleOCR Pipeline](https://www.paddleocr.ai/main/en/version3.x/pipeline_usage/OCR.html)：明确支持 CPU 参数和 ndarray 输入；本轮继续显式固定 v5 模型名称。
- [HPI](https://www.paddleocr.ai/main/en/version3.x/inference_deployment/local_inference/high_performance_inference.html)：可选推理后端需要独立依赖和验证，保持关闭。

## 8. 2026-09-05 最终复核

### 本轮补齐的实现

- `runner.py` / `test_runner.py`：单页解码复用、小数 ROI 坐标一致性、颜色通道、越界/NaN/Inf/非正尺寸拒绝。
- `engine.py` / `test_engine.py`：PIR 必须匹配完整词和转换/属性错误；过期凭据等未知异常不会触发自动降级。
- `internal/submission/{types,store_postgres}.go`、`internal/ocr/{handlers,handlers_test}.go`：区分原始页和配准坐标；同页存在不适用区域时整体回退，防止部分识别遗漏；查询校验页所属 tenant/submission。
- `benchmark_local.py` / `test_benchmark.py`：原构造器严格基线、至少五次同路径 warmup、标签 hash、OS 高水位 RSS、完整 Markdown 指标。
- `api.py`：修正导入格式；`start-local-ocr-fast.ps1`：保留 Docker 失败退出码。
- OCR API、answer-segments、metrics、runbook 与本报告同步说明适用边界。

### 最终配置测量

禁网 Docker 容器，4 CPU 配额、4 GiB 内存上限，使用本机缓存模型；5 页 ROI warmup、8 页正式集，
共 76 个区域。`optimized-safe-20260905` 采用第 5 节配置，readiness 实际触发已知 oneDNN 错误并
成功回退 `effective_mkldnn=false`。没有连接任何 OCR API。

| run | avg ms | P50 ms | P95 ms | pages/min | peak RSS MiB | CER | WER | ROI 非空率 | failure | review |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 20260905-exact/local-baseline-v1（旧构造器、整页） | 31710.9 | 28821.2 | 44453.3 | 1.89 | 1777.3 | N/A | N/A | N/A | 0% | N/A |
| optimized-safe-20260905 | 7435.3 | 7208.4 | 9987.2 | 8.07 | 890.0 | 1.786571 | 1.447368 | 78.95% | 0% | 55.26% |

原始数据：`reports/ocr-evaluation/optimized-safe-20260905/summary.json`、同目录 `summary.md`。
严格基线：`reports/ocr-evaluation/20260905-exact/local-baseline-v1/summary.json`、同目录 `summary.md`；
硬件与资源条件：`reports/ocr-evaluation/run-context-20260905.json`。两次输入图片 bundle hash 相同，
容器均限 4 CPU / 4 GiB，均禁网并串行运行。基线沿用旧构造器的隐式 CPU threads，JSON 为 null，
表示未覆盖运行时默认值。最终配置同时包含 4 线程和 ROI，因此这是组合收益，不是 ROI 单因素收益。
本次平均耗时减少 **76.6%**，P95 减少 **77.5%**；这是有背景服务运行时的单次合成集测量，
不能外推为目标 5800H、真实答卷或业务端到端改善，也不构成统计显著性结论。
已测 CER/WER、ROI 非空率、空白误报率和复核比例与历史 ROI baseline 相同。
历史 RSS 是页间采样值，新值是进程高水位（包括初始化与回退），不能据此判断内存退化。

### 验收边界

- CER=1.786571 表示 178.6571%，并非 1.79%；空白误报率=100%。这一合成数学集合包含选项、公式和其他非普通文本，绝对质量不满足生产上线结论。
- bbox IoU、low-confidence recall 缺少匹配标注，输出 N/A；整页基线没有全文真值，不能宣称完整准确率 gate 已通过。
- ROI 收益只适用于原始页面像素坐标的区域；已配准区域暂用整页路径，尚未实现配准坐标到原始证据空间的逆变换。
- 640/1280 检测边长、recognition batch=8、HPI 额外依赖镜像、ndarray 独立比较、真实手写/异常方向分类集尚未完整评测；这些选项没有成为默认。
- 本地 harness 测量页面推理，不包含队列等待、下载和 API 回写；不能当作业务端到端耗时。

### 实际验证

- OCR worker pytest：57 tests、5 subtests（覆盖新增 benchmark 与 ROI 边界）。
- page-processing worker pytest：57 passed。
- Go：`go test ./internal/ocr ./internal/submission ./internal/paper` 通过。
- Ruff：OCR 实现、benchmark 和本轮测试文件通过。
- `npm run check:story049`、`npm run check:story050` 通过。
- `docker compose -f infra/docker-compose/docker-compose.yml config --quiet`、`git diff --check` 通过。
- 真实 Paddle 就绪、已知 MKLDNN 降级及 8 页 ROI 推理通过。
