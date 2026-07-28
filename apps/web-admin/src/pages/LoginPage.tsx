import { Alert, Button, Checkbox, Form, Input } from "antd";
import { LockKeyhole, LogIn, School } from "lucide-react";
import { isWithinUtf8ByteLimit, LOGIN_FIELD_LIMITS } from "../auth/rememberedLogin";

export interface LoginFormValues {
  tenant_code: string;
  username: string;
  password: string;
  remember_password: boolean;
}

function byteLimitRule(limit: number, message: string) {
  return {
    validator: (_: unknown, value?: string) => (
      !value || isWithinUtf8ByteLimit(value, limit)
        ? Promise.resolve()
        : Promise.reject(new Error(message))
    )
  };
}

export function LoginPage({
  onLogin,
  onForgetRemembered,
  initialValues,
  loading = false,
  error
}: {
  onLogin: (values: LoginFormValues) => void | Promise<void>;
  onForgetRemembered?: () => void;
  initialValues?: Partial<LoginFormValues>;
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
            <p>智能阅卷与学情诊断平台</p>
          </div>
        </div>
        {error ? <Alert type="error" showIcon message="登录失败" description={error} /> : null}
        <Form
          layout="vertical"
          onFinish={onLogin}
          className="login-form"
          initialValues={{ remember_password: false, ...initialValues }}
        >
          <Form.Item
            label="学校代码"
            name="tenant_code"
            rules={[
              { required: true, message: "请输入学校代码" },
              byteLimitRule(LOGIN_FIELD_LIMITS.tenant_code, "学校代码过长")
            ]}
          >
            <Input
              prefix={<School size={16} />}
              placeholder="请输入学校代码（由管理员提供）"
              autoComplete="organization"
              maxLength={LOGIN_FIELD_LIMITS.tenant_code}
            />
          </Form.Item>
          <Form.Item
            label="账号"
            name="username"
            rules={[
              { required: true, message: "请输入账号" },
              byteLimitRule(LOGIN_FIELD_LIMITS.username, "账号过长")
            ]}
          >
            <Input placeholder="请输入账号" autoComplete="username" maxLength={LOGIN_FIELD_LIMITS.username} />
          </Form.Item>
          <Form.Item
            label="密码"
            name="password"
            rules={[
              { required: true, message: "请输入密码" },
              byteLimitRule(LOGIN_FIELD_LIMITS.password, "密码过长")
            ]}
          >
            <Input.Password
              prefix={<LockKeyhole size={16} />}
              placeholder="请输入密码"
              autoComplete="current-password"
              maxLength={LOGIN_FIELD_LIMITS.password}
            />
          </Form.Item>
          <div className="login-options">
            <Form.Item name="remember_password" valuePropName="checked" noStyle>
              <Checkbox onChange={(event) => {
                if (!event.target.checked) {
                  onForgetRemembered?.();
                }
              }}>
                保存密码，下次自动登录
              </Checkbox>
            </Form.Item>
            <span>仅限个人设备</span>
          </div>
          <Button type="primary" htmlType="submit" block icon={<LogIn size={17} />} loading={loading}>
            登录
          </Button>
        </Form>
      </section>
    </main>
  );
}
