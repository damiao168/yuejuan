# STORY-MATH-01：公式识别路由

状态：Implemented，真实模型效果待脱敏答卷评测。

- 在现有 OCR Worker 中新增 `RecognitionRouter`，TEXT 继续走通用 OCR，FORMULA 走公式引擎，DIAGRAM/UNKNOWN 保留图像证据并进入人工复核。
- 第一候选为惰性加载的 PP-FormulaNet_plus-M，可配置 plus-L；UniMERNet 只作为受治理 challenger adapter。
- 科目白名单固定为数学、物理、化学。语文、历史、政治、地理等文科和当前未校准的生物不会调用公式模型。
- 测试覆盖白名单、文科不调用公式引擎、公式引擎失败时 fail closed。
- `000100` 在可信答题片段裁剪完成后按冻结题目快照自动写入现有 Worker Runtime；OCR Worker 顺序消费 `math-understanding` 队列，不另建任务系统。
- 运行结果绑定答题片段、冻结快照和裁剪哈希，重复回调复用同一产物；租约失效时不会接受游离结果。

边界：当前按冻结 evidence 类型对整段裁剪做保守路由；复杂图文混排仍需后续区域检测或人工复核。未使用真实脱敏答卷证明 PP-FormulaNet 或 UniMERNet 的准确率。
