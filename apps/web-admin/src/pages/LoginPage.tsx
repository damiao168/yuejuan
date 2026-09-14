import { Alert, Button, Checkbox, Form, Input } from "antd";
import { LockKeyhole, LogIn, School } from "lucide-react";
import { isWithinUtf8ByteLimit, LOGIN_FIELD_LIMITS } from "../auth/loginSecurity";

export interface LoginFormValues {
  tenant_code?: string;
  tenant_hint?: string;
  identifier: string;
  password: string;
  remember_device: boolean;
  public_device: boolean;
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
  loading = false,
  error
}: {
  onLogin: (values: LoginFormValues) => void | Promise<void>;
  loading?: boolean;
  error?: string;
}) {
  const tenantHint = tenantHintFromLocation();
  const [form] = Form.useForm<LoginFormValues>();
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
          form={form}
          layout="vertical"
          onFinish={onLogin}
          className="login-form"
          initialValues={{ remember_device: false, public_device: false, tenant_hint: tenantHint }}
        >
          {tenantHint ? (
            <>
              <div className="login-tenant-context"><School size={16} /><span>学校入口：{tenantHint}</span></div>
              <Form.Item name="tenant_hint" hidden><Input /></Form.Item>
            </>
          ) : <Form.Item
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
          </Form.Item>}
          <Form.Item
            label="手机号 / 教职工号"
            name="identifier"
            rules={[
              { required: true, message: "请输入手机号、教职工号或原登录账号" },
              byteLimitRule(LOGIN_FIELD_LIMITS.identifier, "登录标识过长")
            ]}
          >
            <Input placeholder="手机号、教职工号或原登录账号" autoComplete="username" maxLength={LOGIN_FIELD_LIMITS.identifier} />
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
            />
          </Form.Item>
          <div className="login-options">
            <Form.Item name="remember_device" valuePropName="checked" noStyle>
              <Checkbox onChange={(event) => { if (event.target.checked) form.setFieldValue("public_device", false); }}>
                这是我的常用电脑，30 天内减少验证
              </Checkbox>
            </Form.Item>
            <Form.Item name="public_device" valuePropName="checked" noStyle>
              <Checkbox onChange={(event) => { if (event.target.checked) form.setFieldValue("remember_device", false); }}>
                这是学校公共电脑
              </Checkbox>
            </Form.Item>
          </div>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.public_device !== current.public_device}>
            {({ getFieldValue }) => getFieldValue("public_device")
              ? <Alert className="login-public-device-alert" type="warning" showIcon message="公共电脑会话最长 4 小时；页面会持续提示，离开前请点退出。" />
              : null}
          </Form.Item>
          <Button type="primary" htmlType="submit" block icon={<LogIn size={17} />} loading={loading}>
            登录
          </Button>
          <p className="login-recovery-help">忘记密码？请联系学校管理员核对身份并生成一次性恢复链接。</p>
        </Form>
      </section>
    </main>
  );
}

function tenantHintFromLocation(): string | undefined {
  if (typeof window === "undefined") return undefined;
  const queryHint = new URLSearchParams(window.location.search).get("tenant")?.trim();
  const pathHint = /^\/school\/([^/]+)\/?$/.exec(window.location.pathname)?.[1];
  let decodedPathHint = "";
  try {
    decodedPathHint = pathHint ? decodeURIComponent(pathHint) : "";
  } catch {
    return undefined;
  }
  const candidate = queryHint || decodedPathHint;
  return /^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(candidate) ? candidate : undefined;
}
