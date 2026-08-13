# STORY-A06：键盘优先、预取、批注与评语模板

状态：Implemented（当前工作树）

## 范围

- 工作台支持高频键盘操作、下一任务预取、草稿恢复与可见的租约/冲突状态。
- `review_annotation` 保存与任务、页、Rubric criterion 绑定的几何批注和文字。
- 评语模板支持个人/学科/学校范围及快捷键；默认私密。

## 接线

- `internal/reviewannotation` 提供批注与模板的持久化服务、乐观修订和管理端接口。
- 管理端 `ReviewAnnotationWorkspace`、`CommentTemplateManager` 与工作台实际组合。
- 学生可见批注仅使用 `student_after_publish` 可见性，并通过发布成绩范围读取；不复用教师私密 DTO。

## 验证

- 批注存储、学生安全 DTO、阅卷上下文和 Web 类型/构建检查已覆盖关键边界。

## 外部边界

- 键位和预取窗口需要由真实阅卷员长时间使用后调整；不把本机交互测试当作效率基线。
