# STORY-MATH-04：符号验证服务

状态：Implemented。

- 独立内网 `math-verification-worker` 提供 normalize、equivalence、transition、solve；不暴露到学生网络。
- 只接收受限 AST，显式处理定义域、约束、增根/失根；数值采样只能给反例，不能充当等价证明。
- 子进程、请求体和超时均有边界，超时或不支持表达式统一返回 uncertain。
- Compose 使用 `math` profile 和内部 token，无主机公开端口。
- OCR Worker 的数学运行时已通过内网 token 调用 normalize，并把受限 AST、语法验证状态和不确定原因写入不可变数学产物；服务不可用时任务重试而不是假装验证成功。

边界：SymPy 规则覆盖不等于中国初高中全题型覆盖，几何证明、复杂单位和图表推理仍需人工。
