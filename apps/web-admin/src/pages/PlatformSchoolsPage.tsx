import { useCallback, useEffect, useMemo, useState } from "react";
import { App as AntApp, Button, Form, Input, Modal, Popconfirm, Space, type TableColumnsType } from "antd";
import { Building2, Plus, RefreshCw } from "lucide-react";
import { createTenant, listTenants, updateTenantStatus, type Tenant } from "../api/org";
import { ResponsiveTable } from "../components/ResponsiveTable";
import { StatusTag } from "../components/StatusTag";

interface CreateSchoolValues {
  name: string;
  code: string;
  admin_username: string;
  admin_display_name: string;
  admin_password: string;
}

export function PlatformSchoolsPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<CreateSchoolValues>();
  const [schools, setSchools] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await listTenants();
      setSchools(response.tenants.filter((tenant) => tenant.code !== "platform"));
    } catch {
      message.error("学校列表加载失败");
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    void load();
  }, [load]);

  const changeStatus = useCallback(async (school: Tenant) => {
    const status = school.status === "active" ? "disabled" : "active";
    try {
      const response = await updateTenantStatus(school.id, status);
      setSchools((current) => current.map((item) => item.id === school.id ? response.tenant : item));
      message.success(status === "active" ? "学校已启用" : "学校已停用");
    } catch {
      message.error("状态更新失败");
    }
  }, [message]);

  const submitCreate = useCallback(async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const response = await createTenant({
        name: values.name.trim(),
        code: values.code.trim().toLowerCase(),
        admin_username: values.admin_username.trim(),
        admin_display_name: values.admin_display_name.trim(),
        admin_password: values.admin_password
      });
      setSchools((current) => [response.tenant, ...current]);
      setCreateOpen(false);
      form.resetFields();
      message.success("学校已创建");
    } catch {
      message.error("学校创建失败，请检查学校代码是否重复");
    } finally {
      setSaving(false);
    }
  }, [form, message]);

  const columns = useMemo<TableColumnsType<Tenant>>(() => [
    {
      title: "学校",
      dataIndex: "name",
      key: "name",
      render: (name: string) => <strong>{name}</strong>
    },
    {
      title: "学校代码",
      dataIndex: "code",
      key: "code"
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      render: (status: string) => (
        <StatusTag tone={status === "active" ? "success" : "neutral"}>
          {status === "active" ? "使用中" : "已停用"}
        </StatusTag>
      )
    },
    {
      title: "操作",
      key: "actions",
      render: (_value, school) => (
        <Popconfirm
          title={school.status === "active" ? "停用这所学校？" : "启用这所学校？"}
          description={school.status === "active" ? "停用后该学校账号将无法登录。" : undefined}
          okText="确认"
          cancelText="取消"
          onConfirm={() => void changeStatus(school)}
        >
          <Button danger={school.status === "active"}>
            {school.status === "active" ? "停用" : "启用"}
          </Button>
        </Popconfirm>
      )
    }
  ], [changeStatus]);

  return (
    <div className="page-stack">
      <section className="page-heading">
        <div>
          <h1>学校管理</h1>
          <p>创建和启停学校</p>
        </div>
        <Space>
          <Button icon={<RefreshCw size={16} />} loading={loading} onClick={() => void load()}>刷新</Button>
          <Button type="primary" icon={<Plus size={16} />} onClick={() => setCreateOpen(true)}>新建学校</Button>
        </Space>
      </section>

      <ResponsiveTable<Tenant>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={schools}
        pagination={false}
        locale={{ emptyText: "暂无学校" }}
      />

      <Modal
        title={<Space><Building2 size={18} />新建学校</Space>}
        open={createOpen}
        confirmLoading={saving}
        okText="创建"
        cancelText="取消"
        onOk={() => void submitCreate()}
        onCancel={() => { setCreateOpen(false); form.resetFields(); }}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" requiredMark={false}>
          <Form.Item name="name" label="学校名称" rules={[{ required: true, message: "请输入学校名称" }]}>
            <Input maxLength={128} autoFocus />
          </Form.Item>
          <Form.Item
            name="code"
            label="学校代码"
            extra="用于登录，例如 fuzhou-no1"
            rules={[
              { required: true, message: "请输入学校代码" },
              { pattern: /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/, message: "使用 3–64 位小写字母、数字或连字符" }
            ]}
          >
            <Input maxLength={64} />
          </Form.Item>
          <Form.Item name="admin_display_name" label="管理员姓名" rules={[{ required: true, message: "请输入管理员姓名" }]}>
            <Input maxLength={128} />
          </Form.Item>
          <Form.Item name="admin_username" label="管理员账号" rules={[{ required: true, message: "请输入管理员账号" }]}>
            <Input maxLength={64} autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="admin_password"
            label="初始密码"
            extra="至少 12 位，包含大小写字母、数字和符号"
            rules={[{ required: true, message: "请输入初始密码" }, { min: 12, message: "密码至少 12 位" }]}
          >
            <Input.Password maxLength={72} autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
