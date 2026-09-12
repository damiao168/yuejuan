# EduGrade 试卷 OCR、数学公式与 AI 解析链路：证据综述与工程决策

日期：2026-09-12
适用范围：管理员上传初高中试卷；印刷文字、印刷数学公式、复杂/多栏版面；为后续学生手写答卷链路预留独立能力。
结论类型：论文与官方资料综述 + 本项目已测数据。本文不把未经项目数据验证的参数称为“最优”。

## 1. 结论摘要

1. **不能用一个固定内存阈值跨电脑判断模型是否常驻。** NVIDIA Model Analyzer 的官方方法是在“给定硬件”上扫描批大小、并发、实例数，并同时观察延迟、吞吐与内存；MLPerf 也按部署场景和性能/准确率约束测量，而不是按 RAM 容量直接选策略。[NVIDIA Model Analyzer](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/model_analyzer/README.html) [MLPerf Inference](https://docs.mlcommons.org/inference/index_gh/)
2. **当前实现中的 `8 GB => resident` 只能算临时经验值，不能作为通用最优规则。** 正式方案应改为“安全默认配置 + 部署机实测标定 + 持久化硬件画像”。在画像不存在时采用低风险模式；画像存在后才自动选择常驻、批大小和并发数。
3. **试卷不能先压成一段纯文本再交给大模型。** 主流文档 OCR 产品保留 block、bounding box、confidence 和关系；近年的阅读顺序研究也把顺序建模为元素间关系，而不只是一次从上到下排序。[Amazon Textract Block](https://docs.aws.amazon.com/textract/latest/APIReference/API_Block.html) [Google Document AI OCR](https://docs.cloud.google.com/document-ai/docs/enterprise-document-ocr) [Reading-order relations, EMNLP 2024](https://aclanthology.org/2024.emnlp-main.540/)
4. **多栏与不同顺序应由“版面元素图 + 分区内排序 + 语义约束”处理。** 简单单栏可走确定性 XY 分割；跨栏、题组、浮动图、答案区等歧义场景再走关系模型或人工确认，不能假定所有页面都是固定两栏。
5. **印刷中文数学公式继续以 PP-FormulaNet_plus-M 为主、plus-L 为受控回退是有官方数据依据的。** plus-M 的英文/中文 BLEU 为 91.45/89.76，plus-L 为 92.22/90.64；官方参考 CPU 时间约 1615.80 ms 与 3125.58 ms，且两者支持中文公式和最长 2560 token。[PaddleOCR 公式识别模型表](https://www.paddleocr.ai/main/en/version3.x/module_usage/formula_recognition.html)
6. **M→L 的回退条件不能只看模型自报置信度。** 应同时检查 LaTeX 语法、渲染结果与原 ROI 的图像相似性、裁剪完整性和业务上下文；公式评估论文 CDM 指出，只比较 LaTeX 字符串不能充分反映视觉等价性。[CDM](https://arxiv.org/abs/2409.03643)
7. **手写公式必须独立路由。** MathWriting、NAMER、FERMAT 和 2026 年 OmniHandwritingOCR 都表明，手写数学有独立的数据与结构难点；复杂多行公式和学生原始错误仍会显著降低通用多模态模型的忠实转录能力。[MathWriting](https://arxiv.org/abs/2404.10690) [NAMER](https://arxiv.org/abs/2407.11380) [FERMAT](https://aclanthology.org/2025.acl-long.720/) [OmniHandwritingOCR](https://arxiv.org/abs/2608.18586)
8. **大模型只处理“未被确定性规则可靠解决的结构歧义”，不重复做已经完成的 OCR。** 题号、显式答案标记、页码、固定模板等先由确定性解析器处理；按题目分块后，大模型只接收紧凑的文本/公式片段引用和必要坐标。长上下文本身会降低中间信息利用率。[Lost in the Middle](https://aclanthology.org/2024.tacl-1.9/)
9. **准确率必须按端到端业务结果衡量。** 除 CER/BLEU 外，还要测公式渲染匹配、阅读顺序、题目边界、答案归属、整题可用率、拒识后风险和人工复核率。2026 年工业 OCR 基准进一步显示，字符准确率高不等于下游任务可靠。[When Good OCR Is Not Enough](https://aclanthology.org/2026.acl-industry.60/)

## 2. 研究问题与证据等级

本次检索回答五个问题：

1. 怎样在单栏、多栏、题组、图文混排下恢复真实阅读顺序？
2. 怎样选择印刷公式模型、ROI、回退与验证策略？
3. 手写公式是否可直接复用印刷公式模型或通用视觉大模型？
4. OCR 结果应怎样交给大模型，避免超长上下文和重复推理？
5. 怎样在不同 CPU/GPU、不同内存的部署机上选择模型生命周期、批大小和并发？

证据分三级：

- **A级：同行评审论文、公开基准、官方 API/部署文档。** 用于确定架构原则和候选方法。
- **B级：项目真实运行数据。** 用于确定 EduGrade 当前机器与当前样本的参数，不外推为所有机器最优。
- **C级：待验证工程假设。** 必须经过项目验证集和目标硬件压测后才能启用。

厂商对自家准确率的宣传只用来确认“产品支持什么能力”，不作为跨产品准确率结论。比如 Mathpix 官方说明支持印刷/手写 STEM、图片/笔迹/PDF 和批处理，但这不等于它在本项目数据上一定优于本地模型。[Mathpix API](https://docs.mathpix.com/)

## 3. 本项目当前事实：哪些已验证，哪些尚未验证

### 3.1 已验证事实

- 原失败试卷包含 170 个 OCR block 和 87 个公式 ROI。
- 旧公式链路约 2357.966 秒；当前 M 批处理、受控 L 回退、验证和缓存链路约 392.246 秒，当前样本约减少 83.4%，约 6 倍提速。
- 该次运行中 plus-M 调用 30 批、plus-L 仅 4 次；28 个公式自动接收，59 个进入复核，其中 48 个表现为裁剪不完整。这说明主要剩余问题不只是模型，而是 ROI 完整性和拒识标准。
- 原 AI 解析请求约 29,619 token，超过本地 Qwen 16,384 上下文；紧凑请求、题目锚点解析和服务端 provenance 回填后，原失败数据在本机 HTTP 路径约 34 ms 返回，恢复任务约 110 ms 完成，得到 7 道题、7 个答案、6 个解析。
- 当前电脑上公式模型常驻曾占约 2.2 GiB；公式 worker 改为任务后释放时，空闲约 16.9 MiB。这只证明“当前机器的低内存运行方式有效”，不证明某个固定 RAM 阈值是通用分界线。

### 3.2 尚未验证

- plus-M 批大小 4 是否在其他 CPU/GPU 上最优。
- 同一模型多实例是否比单实例批处理更快；在 CPU 上并行还可能因线程争抢而变慢。
- plus-L 回退阈值、渲染相似度阈值是否已经在足够大的校准集上达到目标风险。
- 当前布局模型在真实多栏试卷、跨页题干、图表、密集公式和答案页上的召回率。
- 手写数学在本地候选模型、商业 API 和通用 VLM 中的真实准确率、成本和隐私权衡。

## 4. 版面与阅读顺序：不依赖固定栏数

### 4.1 为什么坐标必须保留到最后

Amazon Textract 的结果以 Page/Line/Word 等 Block 表示，并携带 geometry、confidence 和 relationships；Google Math OCR 将公式作为带 layout、text anchor、confidence 和 bounding polygon 的视觉元素；Azure 也把公式作为结构化集合输出。[Amazon Textract](https://docs.aws.amazon.com/textract/latest/APIReference/API_Block.html) [Google Math OCR](https://docs.cloud.google.com/document-ai/docs/enterprise-document-ocr) [Azure Formula OCR](https://learn.microsoft.com/en-us/azure/ai-services/document-intelligence/concept/add-on-capabilities?view=doc-intel-4.0.0)

这支持以下数据契约：

```text
Page
 ├─ Element(id, type, bbox, confidence, source_model)
 │   ├─ TextRun(text)
 │   ├─ Formula(latex, rendered_check, crop_ref)
 │   ├─ Figure/Table/Option/QuestionNumber
 │   └─ HandwritingRegion
 └─ Relation(from, to, type, confidence, evidence)
     ├─ reading_before
     ├─ belongs_to_question
     ├─ caption_of / option_of
     └─ answer_for / solution_for
```

文本和公式不是先拼成不可逆字符串，而是保留为 segment；最终展示时再生成 Markdown/纯文本视图。

### 4.2 多栏不是“检测到两栏然后左右读完”

LayoutReader 把阅读顺序作为文本与布局联合建模问题；EMNLP 2024 的工作进一步指出，一条全局排列不足以表达完整顺序，提出元素间 ordering relations；2025 年 ICCV 阅读顺序综述把方法归纳为规则、图模型、注意力/多模态模型等类别。[LayoutReader](https://arxiv.org/abs/2108.11591) [Ordering Relations](https://aclanthology.org/2024.emnlp-main.540/) [ICCV 2025 Survey](https://openaccess.thecvf.com/content/ICCV2025W/VisionDocs/html/Giovannini_A_Survey_on_Reading_Order_Table_of_Contents_and_Structure_ICCVW_2025_paper.html)

因此建议采用分级方法：

1. **页面分区。** 使用布局模型得到题干、正文、公式、图、表、页眉页脚、题号等候选区域。
2. **确定性基础顺序。** 对互不重叠的简单区域，使用递归 XY-cut/空白带切分；在每个叶分区内按基线聚类后从上到下、从左到右。
3. **建立关系图。** 对跨栏题干、共享材料、浮动图、跨页续题等，预测 `reading_before` 和 `belongs_to_question`，并保存置信度及证据。
4. **全局约束求解。** 题号单调、选项跟随题干、解析/答案绑定相同题号、页眉页脚排除等作为软/硬约束；图中冲突不强行拍平，进入复核。
5. **大模型只解歧义。** 输入歧义子图及附近文字，不把整页所有 OCR block 发给模型重排。

PP-StructureV3 官方管线本身也把 layout、formula、reading-order restoration 作为不同能力组合，并给出不同硬件/配置的页级性能与内存数据，说明“版面恢复”不是普通 OCR 排序的附属步骤。[PP-StructureV3](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-StructureV3/PP-StructureV3.en.md)

### 4.3 ROI 无法“保证 100%”，只能建立可测的完整性门控

现实图片会有阴影、弯曲、低分辨率、拍摄裁边、公式跨行、题号贴近公式等问题，没有论文能给任意输入下 100% ROI 保证。可做的是：

- 检测框先去重/合并，但保留原框和合并原因；
- 根据相邻字符高度、公式上下标和根号/分式方向自适应扩边；
- 对靠近图像边界、墨迹触边、括号/根号未闭合的 ROI 标记 `crop_incomplete`；
- 生成原框、扩边框、邻行合并框等少量候选；识别后用语法和渲染图像匹配选候选；
- 仍不一致则回到原页面人工框选，不能仅升级更大模型。

该策略与本项目 59 个复核项中 48 个裁剪不完整的事实一致，也与 CDM 强调视觉空间匹配的评估方向一致。[CDM](https://arxiv.org/abs/2409.03643)

## 5. 印刷公式识别：模型选择、验证与并行

### 5.1 模型选择

PaddleOCR 官方数据显示：plus-M 和 plus-L 都针对中文公式增强，训练来源包含教材、考试试卷、专业书籍、中文论文/数学期刊，最大输出 2560 token；L 的 BLEU 略高，但 CPU 参考时间约为 M 的 1.93 倍。[PaddleOCR Formula Recognition](https://www.paddleocr.ai/main/en/version3.x/module_usage/formula_recognition.html)

因此本项目的**候选架构**有证据支持：

```text
印刷公式 ROI
  → plus-M 批处理
  → 规范化 + LaTeX 解析 + 渲染/原图比对 + 裁剪完整性检查
  → 仅“ROI 完整但识别失败”的样本转 plus-L
  → 仍不可靠则人工复核
```

但“什么阈值触发 L”必须由本项目校准集确定，不应从 BLEU 表直接推导。

### 5.2 为什么不能 M、S、L 全串

PP-FormulaNet 论文的出发点是准确率/效率不同的模型变体，说明模型选择应服从场景约束，而不是所有模型顺序执行。[PP-FormulaNet](https://arxiv.org/abs/2503.18382) 对中文考试，plus-S 的中文基准明显弱于 M/L；在 M 之后再跑 S 缺少证据，反而增加延迟和维护面。

### 5.3 公式能否并行

能，但必须区分三种并行：

- **单模型批处理：优先。** 同一页/同一尺寸桶的 ROI 组成 batch，减少调度和模型调用开销。
- **多任务动态批处理：吞吐优先。** 多份试卷同时到达时，在短等待窗口内合批；窗口由 p95 延迟预算约束。
- **多模型实例：最后评估。** 只有单实例未吃满硬件且内存允许时才增加实例；CPU 上多实例可能争抢核和内存带宽。

Triton 官方明确建议先用性能分析器测默认动态批处理，再在延迟预算内提高最大 batch 或等待时间；还特别指出，大多数模型不应预设“偏好 batch size”，除非实测该大小显著更快。[Triton Dynamic Batcher](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/user_guide/batcher.html)

所以不能把“batch=4/8”写成跨机器最优结论。正式标定至少扫描：

```text
batch_size       ∈ {1, 2, 4, 8}
worker_instances ∈ {1, 2}（GPU 可视显存允许时再扩大）
request_concurrency ∈ {1, 2, 4}
CPU threads      ∈ 与物理核匹配的候选集合
```

以公式准确率不下降为硬约束，在 p95 延迟、页吞吐、峰值 RSS/VRAM 之间选 Pareto 前沿，而不是只追求某一个速度数值。

## 6. 手写公式：必须与印刷链路分开

### 6.1 证据边界

MathWriting 提供 23 万真实在线手写公式和 40 万合成样本，并可渲染为离线图像；这说明“有笔迹轨迹”和“只有扫描图”是两个不同输入条件。[MathWriting](https://arxiv.org/abs/2404.10690)

NAMER 针对 HMER 的自回归误差累积和慢解码提出并行图解码，在 CROHME/HME100K 上报告整体 FPS 6.7 倍提升；2025 年结构化 HMER 工作又强调符号—笔迹对齐对错误分析和可解释性的重要性。[NAMER](https://arxiv.org/abs/2407.11380) [Structural HMER](https://arxiv.org/abs/2508.19773)

更重要的是，FERMAT 在 7–12 年级 2,200 多份带扰动的手写解答上发现 VLM 的错误检测/定位/修正仍有明显不足；2026 年 OmniHandwritingOCR 进一步报告复杂多行公式准确率显著下降，并观察到生成模型会产生“看似合理但图像并不支持”的纠正。[FERMAT](https://aclanthology.org/2025.acl-long.720/) [OmniHandwritingOCR](https://arxiv.org/abs/2608.18586)

因此：**阅卷系统不能允许识别器自动修正学生写错的符号。** `x+1` 被“理解”为 `x-1` 或把错误等式改成正确等式，会直接改变评分事实。

### 6.2 推荐路线

- 电子笔/平板采集：优先保留 strokes、时间顺序、笔画边界，再用在线 HMER；Mathpix 等行业 API 也单独提供 stroke endpoint，说明笔迹数据是重要输入形态。[Mathpix Process Strokes](https://docs.mathpix.com/)
- 扫描答卷：先检测手写区，再使用离线 HMER 候选；保留原图、候选 LaTeX、字符/结构对齐和不确定性。
- VLM：可用于识别候选之间的上下文消歧或评分推理，但不得覆盖原始转录；任何“纠错”必须作为独立建议字段。
- 低置信、符号冲突、多行推导、涂改、越界书写：拒识并展示原图给教师。

候选技术不直接拍板。应在本地模型（NAMER/BAT/其他可部署实现）、商业 API（如 Mathpix）和具备隐私协议的 VLM 之间做盲测，使用本校匿名化样本决定。

## 7. OCR 结果交给 AI：结构化、分块、可追溯

### 7.1 不发送什么

- 不发送包含 170 个 block 全量重复 provenance 的 29k token 巨型 JSON。
- 不让大模型重新猜测已经有高置信题号、答案标记和几何关系的内容。
- 不只发送一段失去坐标的纯文本。
- 不要求一次生成完整数据库对象和所有内部审计字段。

### 7.2 发送什么

按题目或歧义区域发送紧凑结构：

```json
{
  "page": 1,
  "region": [0.08, 0.15, 0.92, 0.48],
  "candidate_question_no": "3",
  "segments": [
    {"id": "s21", "kind": "text", "text": "已知函数"},
    {"id": "s22", "kind": "formula", "latex": "f(x)=...", "quality": "accepted"},
    {"id": "s23", "kind": "option", "text": "A. ..."}
  ],
  "relations": [["s21", "s22", "reading_before"]],
  "requested_fields": ["question_boundary", "option_binding"]
}
```

大模型只返回 segment id、关系和必要业务字段；原坐标、模型版本、原图引用由服务端根据 id 回填。这样既减少 token，又防止模型伪造 provenance。

### 7.3 解析优先级

1. 确定性锚点：题号、`【答案】`、`【解析】`、固定模板、页码。
2. 几何和关系图：题目边界、选项归属、跨栏/跨页连接。
3. 小块 LLM：仅处理冲突或缺失关系。
4. 全页视觉模型：只作为困难样本的独立候选，不作为默认路径。
5. 人工复核：模型之间冲突或影响答案/分值时。

“Lost in the Middle”显示，即使模型支持长上下文，相关信息处在长输入中部时性能也会显著下降；2025 年 LayTextLLM 也专门尝试把一个 bounding box 压成一个 token 来减少布局序列膨胀。[Lost in the Middle](https://aclanthology.org/2024.tacl-1.9/) [LayTextLLM](https://aclanthology.org/2025.findings-acl.379/)

## 8. 跨硬件部署：用标定画像取代固定阈值

### 8.1 正式模式

保留两个明确可解释的运行模式：

- `per_job`：空闲不加载公式模型；领取任务后加载，任务完成后释放进程。适合内存紧张、低频上传、无法容纳多模型同时常驻的机器。
- `resident`：模型常驻；适合内存/显存充足、上传频繁、冷启动延迟不可接受的机器。

模型文件存储与模型进程生命周期必须分开：模型权重缓存放持久卷/本地目录，`per_job` 只重新加载，不重新下载。Triton 官方也把模型仓库与显式 load/unload 作为标准模型管理能力。[Triton Model Management](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/user_guide/model_management.html)

### 8.2 自动选择必须来自实测

部署时运行一次“快速标定”，输出硬件画像：

```json
{
  "profile_version": 1,
  "hardware_fingerprint": "cpu/gpu/ram/runtime hash",
  "formula": {
    "model": "PP-FormulaNet_plus-M",
    "load_ms": 0,
    "batch_candidates": {
      "1": {"p50_ms": 0, "p95_ms": 0, "peak_rss_mb": 0},
      "2": {"p50_ms": 0, "p95_ms": 0, "peak_rss_mb": 0},
      "4": {"p50_ms": 0, "p95_ms": 0, "peak_rss_mb": 0}
    }
  },
  "combined_peak_mb": 0,
  "recommended": {"lifecycle": "per_job", "batch_size": 0, "instances": 1}
}
```

决策输入包括：

- 代表性短/长/宽/多行公式样本；
- 冷启动时间、热启动 p50/p95；
- 每种 batch/concurrency 的吞吐；
- 公式 M、按需 L、普通 OCR、本地 LLM 的单独与组合峰值内存；
- 容器 cgroup 限制和宿主机可用资源；
- 产品 SLO，例如“单页 p95”“10 页试卷 p95”“同时 3 份试卷”。

只有当组合峰值加安全余量不超过可用资源，并且常驻带来的冷启动收益符合 SLO，才选择 `resident`。具体安全余量也应由长时间稳定性测试和目标运维规范决定，不能在论文综述里虚构一个通用百分比。

NVIDIA Model Analyzer 明确说明其目标是在给定硬件上搜索批大小、动态批处理和实例数，并报告计算/内存权衡；INFaaS 的研究也把模型/硬件/资源变体选择置于准确率、延迟和成本目标之下。[Model Analyzer](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/model_analyzer/README.html) [INFaaS](https://www.usenix.org/conference/atc21/presentation/romero)

### 8.3 安全默认值

在硬件画像不存在、过期或指纹变化时：

- 默认 `per_job`；
- batch 采用经过项目最低支持机器验证的保守值；
- 不自动启动会与公式模型争内存的本地大模型；
- UI 明确提示“尚未标定，当前为兼容模式”，后台提供一键标定；
- 标定失败保留兼容模式，不阻断基本 OCR。

这是“安全可运行默认”，不是“性能最优默认”。性能最优只对具体硬件画像成立。

## 9. 进度显示：只展示真实工作量和真实事件

进度条不能用定时器平滑到某个百分比冒充真实进度。服务端应发布不可回退的阶段事件：

```text
上传/解码       page i / N
页面预处理      page i / N
文字 OCR        block batch i / N
版面与顺序      page i / N
公式检测        detected K ROI
公式 M          batch i / N, completed_roi / K
公式 L 回退     item i / fallback_total（fallback_total 可动态增加）
结构组装        question i / discovered_questions
AI 消歧         chunk i / N
持久化          current operation
```

进度百分比由“已完成工作单位 / 当前已知总工作单位”计算；新增 L 回退任务时允许重新估算总量，但完成量不倒退。UI 同时显示：阶段、计数、已运行时间、最后更新时间、是否冷启动/加载模型、当前模型、预计剩余时间的置信区间。ETA 只从相同硬件画像和同类尺寸桶的历史 p50/p95 估计；样本不足时显示“正在估算”，不显示伪精确秒数。

异步文档服务通常以 job 状态与分页结果工作，例如 Amazon Textract 的异步 API 返回 JobStatus 和 page/block 结果；这支持将识别建模为可查询的阶段任务，而不是一次长 HTTP 请求。[Amazon Textract Async](https://docs.aws.amazon.com/textract/latest/dg/api-async.html)

## 10. 评估集与验收指标

### 10.1 数据集组成

至少建立以下项目私有、匿名化分层测试集：

- 语文/英语纯文字试卷；
- 单栏数学、双栏数学、栏数变化、左右栏交错；
- 题组共享材料、跨页续题、图表/几何图、页眉页脚、水印；
- 行内/行间公式、长分式、根式、矩阵、分段函数、中文混排；
- 拍照倾斜、阴影、弯曲、低分辨率、裁边；
- 答案/解析页，不同出版社和不同模板；
- 手写单行、多行推导、涂改、越界、不同年级和书写者。

公开集用于覆盖能力，但不能代替本项目集：DocLayNet 强调现有科学论文数据版面多样性不足；M6Doc 和 OmniDocBench 则扩大了多类型/多布局、考试卷和端到端解析覆盖。[DocLayNet](https://arxiv.org/abs/2206.01062) [M6Doc](https://openaccess.thecvf.com/content/CVPR2023/papers/Cheng_M6Doc_A_Large-Scale_Multi-Format_Multi-Type_Multi-Layout_Multi-Language_Multi-Annotation_Category_Dataset_CVPR_2023_paper.pdf) [OmniDocBench](https://openaccess.thecvf.com/content/CVPR2025/papers/Ouyang_OmniDocBench_Benchmarking_Diverse_PDF_Document_Parsing_with_Comprehensive_Annotations_CVPR_2025_paper.pdf)

### 10.2 指标

| 层级 | 必测指标 | 为什么 |
|---|---|---|
| 文字 | CER/WER、低置信召回 | 基础转录 |
| 版面 | 元素检测 P/R、reading-order edge F1、题目归属 F1 | 多栏和题组不能由 CER 表示 |
| 公式 | ExpRate、归一化 edit/BLEU、CDM/渲染相似、ROI 完整率 | 字符串等价与视觉等价都要测 |
| 组卷 | 题目完整率、答案/解析绑定准确率、分值恢复准确率 | 直接对应导入是否可用 |
| 拒识 | risk-coverage、人工复核率、漏报高风险错误率 | 系统必须知道何时不自动接受 |
| 性能 | 冷/热 p50/p95、页吞吐、峰值 RSS/VRAM、OOM 率 | 跨硬件与并发 |
| 进度 | 事件延迟、最后更新时间、预计时间误差 | 防止“假进度” |

SelectiveNet 的核心思想是联合衡量覆盖率与被接受样本风险，因此本项目不应只追求“自动识别率”，而要约束自动接收部分的错误风险。[SelectiveNet](https://proceedings.mlr.press/v97/geifman19a.html)

### 10.3 上线门槛

具体数值应由产品负责人、学科教师和测试集共同确定。推荐用以下形式，而不是本文杜撰百分比：

```text
硬门槛：答案归属错误率 <= 业务允许上限
硬门槛：自动接收公式的高影响错误率 <= 业务允许上限
硬门槛：目标最低配置无 OOM，任务可恢复
软目标：p95 总耗时、人工复核率、吞吐达到上线 SLO
回归规则：任何一个关键分层数据集显著退化则阻止发布
```

## 11. 建议的端到端架构

```text
文件接收与不可变原件
  → 页级解码/预处理
  → 版面元素检测（保留 bbox/confidence/source）
  → 学科与区域路由
      ├─ 普通印刷文字 → PP-OCR
      ├─ 印刷公式 ROI → plus-M batch → 验证 → 受控 plus-L
      └─ 手写区域 → 独立 HMER 候选/商业 API 候选
  → 元素关系图与阅读顺序
  → 确定性题目/答案/解析锚点解析
  → 仅歧义 chunk 交给 LLM
  → 服务端 provenance 回填与 schema 校验
  → 风险门控
      ├─ 自动接收
      └─ 人工复核（原图、ROI、候选与失败原因并列）
  → 持久化与可重跑阶段
```

每一阶段必须输出版本化中间产物，允许“仅重跑 AI 解析”“仅重跑公式”“仅调整阅读顺序”，避免任何下游失败都重新做几十分钟 OCR。

## 12. 分阶段实施顺序

### P0：纠正没有证据的自动策略

- 删除固定 `8 GB` 自动常驻判断，不再把它描述为通用最优。
- 模型权重继续放持久缓存；默认兼容模式为 `per_job`。
- 保留明确的 `resident` 配置，供标定结果选择。
- 记录冷启动、模型加载、每批推理、峰值内存和退出原因。

### P1：建立可复现的部署标定器

- 提供内置代表性公式样本和可选真实匿名样本。
- 扫描 batch、线程、实例与并发；分别测冷/热状态。
- 同时启动可能共存的 OCR/公式/LLM 服务测组合峰值。
- 生成带硬件指纹、软件版本和有效期的 JSON 画像。
- Compose/安装器读取画像选择模式；硬件或运行时变化后自动失效并回到兼容模式。

### P2：布局与质量闭环

- 固化元素/关系数据契约和可视化调试页。
- 增加真实多栏、跨页、题组数据；评估 XY-cut + 关系回退。
- 以 ROI 完整性、CDM/渲染比较和 risk-coverage 校准 M→L/人工复核门控。
- 进度改为服务端阶段事件，按画像历史估算 ETA。

### P3：手写数学试点

- 先收集合法授权、匿名化的本校样本和标注规范。
- 区分在线 strokes 与离线扫描；分别评测。
- 本地 HMER、商业 API、VLM 做同一盲测，禁止模型纠正学生原始错误。
- 只在达到业务风险门槛后进入自动评分；此前作为教师辅助转录。

## 13. 明确否决或暂停的做法

- 否决：固定 RAM 阈值直接宣称跨电脑自动最优。
- 否决：所有科目、所有页面都跑公式模型。
- 否决：所有公式一律 M→S→L 串行。
- 否决：检测框不做完整性检查就用更大模型兜底。
- 否决：把整页/整份 OCR 巨型 JSON 一次交给本地大模型。
- 否决：大模型生成并覆盖原始 OCR provenance。
- 否决：用字符准确率或单个 BLEU 代表端到端阅卷可用性。
- 暂停：在没有项目手写验证集时承诺某个手写模型“成熟可直接自动评分”。

## 14. 来源

### 同行评审论文与公开研究

1. [PP-FormulaNet: Bridging Accuracy and Efficiency in Advanced Formula Recognition](https://arxiv.org/abs/2503.18382)
2. [UniMERNet: A Universal Network for Real-World Mathematical Expression Recognition](https://arxiv.org/abs/2404.15254)
3. [Image Over Text: Character Detection Matching](https://arxiv.org/abs/2409.03643)
4. [NAMER: Non-Autoregressive Modeling for HMER](https://arxiv.org/abs/2407.11380)
5. [MathWriting Dataset](https://arxiv.org/abs/2404.10690)
6. [The Return of Structural HMER](https://arxiv.org/abs/2508.19773)
7. [Can Vision-Language Models Evaluate Handwritten Math? / FERMAT](https://aclanthology.org/2025.acl-long.720/)
8. [OmniHandwritingOCR](https://arxiv.org/abs/2608.18586)（2026 新近论文，结论需等待更多复现）
9. [LayoutReader](https://arxiv.org/abs/2108.11591)
10. [Modeling Layout Reading Order as Ordering Relations](https://aclanthology.org/2024.emnlp-main.540/)
11. [A Survey on Reading Order, TOC, and Structure Extraction](https://openaccess.thecvf.com/content/ICCV2025W/VisionDocs/html/Giovannini_A_Survey_on_Reading_Order_Table_of_Contents_and_Structure_ICCVW_2025_paper.html)
12. [DocLayNet](https://arxiv.org/abs/2206.01062)
13. [M6Doc](https://openaccess.thecvf.com/content/CVPR2023/papers/Cheng_M6Doc_A_Large-Scale_Multi-Format_Multi-Type_Multi-Layout_Multi-Language_Multi-Annotation_Category_Dataset_CVPR_2023_paper.pdf)
14. [OmniDocBench](https://openaccess.thecvf.com/content/CVPR2025/papers/Ouyang_OmniDocBench_Benchmarking_Diverse_PDF_Document_Parsing_with_Comprehensive_Annotations_CVPR_2025_paper.pdf)
15. [A Bounding Box is Worth One Token / LayTextLLM](https://aclanthology.org/2025.findings-acl.379/)
16. [Lost in the Middle](https://aclanthology.org/2024.tacl-1.9/)
17. [When Good OCR Is Not Enough](https://aclanthology.org/2026.acl-industry.60/)
18. [SelectiveNet](https://proceedings.mlr.press/v97/geifman19a.html)
19. [INFaaS](https://www.usenix.org/conference/atc21/presentation/romero)
20. [Serving DNNs like Clockwork](https://www.usenix.org/conference/osdi20/presentation/gujarati)

### 官方框架与行业产品资料

21. [PaddleOCR Formula Recognition](https://www.paddleocr.ai/main/en/version3.x/module_usage/formula_recognition.html)
22. [PaddleOCR PP-StructureV3](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-StructureV3/PP-StructureV3.en.md)
23. [PaddleOCR High-Performance Inference](https://www.paddleocr.ai/v3.1.0/en/version3.x/deployment/high_performance_inference.html)
24. [NVIDIA Triton Dynamic Batcher](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/user_guide/batcher.html)
25. [NVIDIA Triton Model Management](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/user_guide/model_management.html)
26. [NVIDIA Triton Model Analyzer](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/model_analyzer/README.html)
27. [MLPerf Inference](https://docs.mlcommons.org/inference/index_gh/)
28. [Amazon Textract Block](https://docs.aws.amazon.com/textract/latest/APIReference/API_Block.html)
29. [Amazon Textract Async](https://docs.aws.amazon.com/textract/latest/dg/api-async.html)
30. [Google Enterprise Document OCR / Math OCR](https://docs.cloud.google.com/document-ai/docs/enterprise-document-ocr)
31. [Azure Document Intelligence Add-on Capabilities](https://learn.microsoft.com/en-us/azure/ai-services/document-intelligence/concept/add-on-capabilities?view=doc-intel-4.0.0)
32. [Mathpix OCR API](https://docs.mathpix.com/)

## 15. 最终判断

当前最值得继续的不是再加一个公式模型，而是把已有模型置于一条可测、可拒识、可回溯、可按硬件标定的链路中。对 EduGrade 而言：

- **模型层**：印刷中文公式暂定 plus-M 主识别、plus-L 受控回退；手写另设验证项目。
- **文档层**：保留坐标与元素关系，以关系图适配多栏和变化顺序。
- **AI 层**：确定性优先、按题分块、只解歧义、服务端回填 provenance。
- **部署层**：模型缓存持久化；生命周期、batch、并发由目标硬件实测画像选择。
- **质量层**：以题目/答案绑定和选择性风险为最终指标，不以 OCR 字符指标代替业务正确性。

这套结论有论文与官方做法支撑；仍需项目验证的阈值、并发和手写模型选择已明确留在标定阶段，没有被假装成普适答案。
