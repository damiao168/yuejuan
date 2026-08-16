# STORY-MATH-02：公式结构与受限 AST

状态：Implemented。

- 公式经词法归一化后进入白名单解析器，支持四则运算、隐式乘法、分式、根式、幂、方程、不等式及批准函数。
- AST 是证据对象，不执行任意代码；未知命令、超界结构和歧义表达式保守返回 unsupported/uncertain。
- 不把 SymPy experimental LaTeX parser 作为生产事实源。

验证：`services/math-verification-worker/tests/test_parser.py`。
