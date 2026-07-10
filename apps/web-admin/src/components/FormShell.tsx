import type { ReactNode } from "react";
import { Button, Form, Input, Select, Space, Switch } from "antd";
import { Save, RotateCcw } from "lucide-react";

export function FormShell({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <section className="workspace-section form-shell">
      <div className="section-head">
        <div>
          <h2>{title}</h2>
          <p>基础信息</p>
        </div>
      </div>
      <Form layout="vertical" initialValues={{ appeal: true, publish: "after_admin_approval" }}>
        {children ?? (
          <div className="form-grid">
            <Form.Item label="考试名称" name="name" rules={[{ required: true, message: "请输入考试名称" }]}>
              <Input placeholder="高二物理期末考试" />
            </Form.Item>
            <Form.Item label="学科" name="subject" rules={[{ required: true, message: "请选择学科" }]}>
              <Select options={["语文", "数学", "英语", "物理", "化学"].map((item) => ({ label: item, value: item }))} />
            </Form.Item>
            <Form.Item label="阅卷模式" name="mode">
              <Select
                options={[
                  { label: "AI 辅助 + 人工确认", value: "ai_assisted" },
                  { label: "双评 + 仲裁", value: "double_mark" },
                  { label: "仅客观题自动", value: "auto_objective_only" }
                ]}
              />
            </Form.Item>
            <Form.Item label="允许申诉" name="appeal" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="发布策略" name="publish">
              <Select options={[{ label: "管理员审批后发布", value: "after_admin_approval" }]} />
            </Form.Item>
          </div>
        )}
        <Space>
          <Button type="primary" icon={<Save size={16} />}>
            保存
          </Button>
          <Button icon={<RotateCcw size={16} />}>重置</Button>
        </Space>
      </Form>
    </section>
  );
}
