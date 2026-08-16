# STORY-MATH-00：数学答卷理解契约与基准

状态：Implemented（契约与基准底座；识别效果尚未验证）

## 目标

在既有 `answer_segment`、冻结题目/Rubric 和人工阅卷链路之间建立唯一的数学理解工件，避免后续 MATH Story 各自发明 JSON。工件只提供证据，不产生或发布最终分数。

## 复用边界

- EXISTING：答题区域原图、裁片哈希、题目快照、OCR、Rubric、评分证据及人工复核。
- EXTEND：以 `answer_segment_id + exam_question_snapshot_id` 绑定数学理解版本。
- NEW：`MathAnswerBlock`、`FormulaArtifact`、受限 `FormulaAST`、`SpatialRelation`、`SolutionGraph`、`MathVerification` 和 `RubricEvidence`。
- OUT OF SCOPE：HMER/多模态模型接入、自动最终评分、教师纠错 UI、真实学校效果结论。

## 实现

- Go 领域契约执行枚举、归一化坐标、置信度、引用完整性和 DAG 无环校验；AST 只允许白名单节点并限制深度与规模。
- PostgreSQL `math_understanding_artifact` 使用复合租户外键绑定答题区域和冻结题目快照；内容版本不可变，更新只允许旧版本退出 current。
- `lab/evals/math-recognition` 提供确定性 MathBench v1，覆盖公式、AST、符号、空间关系、步骤、图边和 Rubric 证据指标。
- 内置单条合成 fixture 仅验证 runner 可复现，不代表模型效果或试点门槛。

## 验证

```text
go test ./internal/mathunderstanding -count=1
python -m pytest lab/evals/math-recognition/test_math_bench.py -q
python lab/evals/math-recognition/run_math_bench.py ...
npm run check:story052
```

## 后续

MATH-01 才接公式/文本/图形路由；所有无法可靠分类或解析的区域必须进入人工复核，不能把 smoke fixture 的满分指标当作模型质量。

## 2026-08-16 合成评测集扩充（synthetic-v2）

- `generate_synthetic_fixtures.py` 确定性生成 54 个新样本（共 55 条），覆盖规划矩阵 25 个类别（分数/根式/指数/下标/方程组/不等式/绝对值/三角/几何符号/向量/极限/求和/积分/矩阵/中文混排/横纵步骤/两栏/补写/划除/草稿/低质量/多方法/求解链），16 种识别缺陷循环注入以保证各指标有命中与缺失。
- runner 新增 `--dataset` 溯源参数，报告 JSON 顶层带 `dataset` 字段；测试扩为 8 项，含类别覆盖、生成器零漂移与 synthetic 标注强制断言。
- `reports/synthetic-v2.json` 为合成 harness 自校验基线，不是真实模型证据；真实 MathBench（1000+ 公式 crop、500+ 完整答案）仍待脱敏答卷。
