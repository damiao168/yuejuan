# STORY-A21：Student Portal 1.0

状态：Implemented（当前工作树）

## 范围

- 独立 `apps/student-portal` 仅展示学生可访问的已发布考试、总分、题级结果和公开反馈。
- 后端使用专用 Student DTO；不复用管理员、评分任务或模型治理 DTO。

## 接线

- `internal/studentportal` 按当前已发布 Score Release 和学生范围返回安全投影。
- 前端读取学生考试清单与题目详情，并从 A22 提供题目级申诉入口。

## 验证

- Student portal Go 处理器/服务测试、前端 TypeScript 与生产构建已执行。

## 外部边界

- 实际身份联邦、通知、无障碍与学生/家长使用流程尚需学校系统接入验证。
