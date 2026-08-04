import { useCallback, useEffect, useState } from "react";
import { App, Button, Form, Input, List, Popconfirm, Tag } from "antd";
import { KeyRound, LogOut, RefreshCw, Smartphone } from "lucide-react";
import { changePassword, listSessions, logoutAll, revokeSession, type DeviceSession } from "../api/auth";
import { ErrorState, LoadingState } from "../components/PageState";

interface PasswordFormValues {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

function sessionTypeLabel(type: DeviceSession["session_type"]) {
  if (type === "remembered_device") return "保持登录";
  if (type === "desktop_device") return "桌面客户端";
  if (type === "service") return "后台服务";
  return "普通会话";
}

export function SessionManagementPage({ onLoggedOut }: { onLoggedOut: () => void }) {
  const { message } = App.useApp();
  const [form] = Form.useForm<PasswordFormValues>();
  const [sessions, setSessions] = useState<DeviceSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [actioning, setActioning] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      setSessions((await listSessions()).sessions);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "登录设备加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const revoke = async (session: DeviceSession) => {
    setActioning(session.id);
    try {
      await revokeSession(session.id);
      if (session.current) {
        onLoggedOut();
        return;
      }
      setSessions((current) => current.filter((item) => item.id !== session.id));
      message.success("该设备已退出");
    } catch (actionError) {
      message.error(actionError instanceof Error ? actionError.message : "退出设备失败");
    } finally {
      setActioning(undefined);
    }
  };

  const revokeAll = async () => {
    setActioning("all");
    try {
      await logoutAll();
      onLoggedOut();
    } catch (actionError) {
      message.error(actionError instanceof Error ? actionError.message : "退出全部设备失败");
    } finally {
      setActioning(undefined);
    }
  };

  const submitPassword = async (values: PasswordFormValues) => {
    setActioning("password");
    try {
      await changePassword({ current_password: values.currentPassword, new_password: values.newPassword });
      form.resetFields();
      message.success("密码已更新，请重新登录");
      onLoggedOut();
    } catch (actionError) {
      message.error(actionError instanceof Error ? actionError.message : "修改密码失败");
    } finally {
      setActioning(undefined);
    }
  };

  if (loading && sessions.length === 0) return <LoadingState label="正在读取登录设备" />;
  if (error && sessions.length === 0) return <ErrorState message={error} onRetry={() => void load()} />;

  return (
    <div className="page-stack session-management-page">
      <section className="page-heading">
        <div>
          <h1>账户安全</h1>
          <p>管理登录设备和密码。浏览器只保存安全会话，不保存你的密码。</p>
        </div>
        <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load()}>刷新</Button>
      </section>

      <section className="workspace-section">
        <div className="section-heading compact">
          <div>
            <h2>登录设备</h2>
            <p>退出不再使用的设备，降低账号被冒用的风险。</p>
          </div>
        </div>
        <List
          dataSource={sessions}
          locale={{ emptyText: "当前没有其他有效会话" }}
          renderItem={(session) => (
            <List.Item
              actions={[
                <Popconfirm
                  key="revoke"
                  title={session.current ? "退出当前设备？" : "让该设备退出？"}
                  description="被撤销的会话需要重新输入密码登录。"
                  onConfirm={() => void revoke(session)}
                >
                  <Button danger loading={actioning === session.id}>退出</Button>
                </Popconfirm>
              ]}
            >
              <List.Item.Meta
                avatar={<Smartphone size={22} />}
                title={<span>{session.device_name || "未命名设备"} {session.current ? <Tag color="blue">当前设备</Tag> : null}</span>}
                description={`${sessionTypeLabel(session.session_type)} · 最近使用 ${new Date(session.last_seen_at).toLocaleString("zh-CN")} · 有效至 ${new Date(session.expires_at).toLocaleString("zh-CN")}`}
              />
            </List.Item>
          )}
        />
        <Popconfirm
          title="退出全部设备？"
          description="包括当前设备在内的全部登录会话都会失效。"
          onConfirm={() => void revokeAll()}
        >
          <Button danger icon={<LogOut size={16} />} loading={actioning === "all"}>退出全部设备</Button>
        </Popconfirm>
      </section>

      <section className="workspace-section">
        <div className="section-heading compact">
          <div>
            <h2>修改密码</h2>
            <p>修改后会自动退出所有设备。</p>
          </div>
        </div>
        <Form<PasswordFormValues> form={form} layout="vertical" className="account-password-form" onFinish={(values) => void submitPassword(values)}>
          <Form.Item name="currentPassword" label="当前密码" rules={[{ required: true, message: "请输入当前密码" }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label="新密码"
            extra="至少 12 个字符，包含大小写字母、数字和符号。"
            rules={[{ required: true, message: "请输入新密码" }, { min: 12, message: "新密码至少 12 个字符" }]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label="确认新密码"
            dependencies={["newPassword"]}
            rules={[
              { required: true, message: "请再次输入新密码" },
              ({ getFieldValue }) => ({ validator: (_, value) => !value || getFieldValue("newPassword") === value ? Promise.resolve() : Promise.reject(new Error("两次输入的密码不一致")) })
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" icon={<KeyRound size={16} />} loading={actioning === "password"}>更新密码</Button>
        </Form>
      </section>
    </div>
  );
}
