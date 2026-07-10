import { Alert, Button, Form, Input } from "antd";
import { LockKeyhole, LogIn, School } from "lucide-react";

export interface LoginFormValues {
  tenant_code: string;
  username: string;
  password: string;
}

export function LoginPage({
  onLogin,
  loading = false,
  error
}: {
  onLogin: (values: LoginFormValues) => void | Promise<void>;
  loading?: boolean;
  error?: string;
}) {
  return (
    <main className="login-screen">
      <section className="login-panel">
        <div className="login-brand">
          <div className="brand-mark large">E</div>
          <div>
            <h1>EduGrade Enterprise</h1>
            <p>智能阅卷与学情诊断管理后台</p>
          </div>
        </div>
        {error ? <Alert type="error" showIcon message="登录失败" description={error} /> : null}
        <Form
          layout="vertical"
          onFinish={onLogin}
          className="login-form"
        >
          <Form.Item label="租户" name="tenant_code" rules={[{ required: true, message: "请输入租户代码" }]}>
            <Input prefix={<School size={16} />} placeholder="请输入租户代码" autoComplete="organization" />
          </Form.Item>
          <Form.Item label="账号" name="username" rules={[{ required: true, message: "请输入账号" }]}>
            <Input placeholder="请输入账号" autoComplete="username" />
          </Form.Item>
          <Form.Item label="密码" name="password" rules={[{ required: true, message: "请输入密码" }]}>
            <Input.Password prefix={<LockKeyhole size={16} />} placeholder="请输入密码" autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block icon={<LogIn size={17} />} loading={loading}>
            登录
          </Button>
        </Form>
      </section>
    </main>
  );
}
