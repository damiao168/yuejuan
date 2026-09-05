import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  App as AntApp,
  Button,
  Drawer,
  Empty,
  Form,
  Input,
  Select,
  Space,
  Switch,
  Tooltip,
  type TableColumnsType
} from "antd";
import { Building2, CheckCircle2, KeyRound, Pencil, Plus, RefreshCw, ShieldCheck, Unplug, Zap } from "lucide-react";
import {
  createManagedModelAPIConfig,
  listManagedModelAPIConfigs,
  probeManagedModelAPIConfig,
  updateManagedModelAPIConfig,
  type ManagedAdapterType,
  type ManagedModelAPIConfig
} from "../api/modelApiConfig";
import { listTenants, type Tenant } from "../api/org";
import { ResponsiveTable } from "../components/ResponsiveTable";
import { StatusTag } from "../components/StatusTag";

type SupplierPreset = "aliyun" | "deepseek" | "openai" | "custom";

interface ConfigFormValues {
  supplier: SupplierPreset;
  provider_key: string;
  display_name: string;
  adapter_type: ManagedAdapterType;
  base_url: string;
  api_key: string;
  model_name: string;
  model_version: string;
  region: string;
  enabled: boolean;
  is_default: boolean;
}

const supplierOptions = [
  { value: "aliyun", label: "阿里云百炼" },
  { value: "deepseek", label: "DeepSeek" },
  { value: "openai", label: "OpenAI" },
  { value: "custom", label: "其他兼容接口" }
];

const supplierDefaults: Record<SupplierPreset, Partial<ConfigFormValues>> = {
  aliyun: { provider_key: "aliyun", display_name: "阿里云百炼", adapter_type: "openai_compatible", region: "cn" },
  deepseek: { provider_key: "deepseek", display_name: "DeepSeek", adapter_type: "openai_compatible", region: "global" },
  openai: { provider_key: "openai", display_name: "OpenAI", adapter_type: "openai_compatible", region: "global" },
  custom: { provider_key: "custom", display_name: "自定义供应商", adapter_type: "openai_compatible", region: "global" }
};

function formatDateTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false
  }).format(date);
}

function presetForConfig(config: ManagedModelAPIConfig): SupplierPreset {
  if (config.provider_key === "aliyun") return "aliyun";
  if (config.provider_key === "deepseek") return "deepseek";
  if (config.provider_key === "openai") return "openai";
  return "custom";
}

async function loadAllSchoolTenants() {
  const schools: Tenant[] = [];
  let cursor = "";
  for (let page = 0; page < 20; page += 1) {
    const response = await listTenants({ limit: 100, cursor: cursor || undefined });
    schools.push(...response.tenants.filter((tenant) => tenant.code !== "platform"));
    if (!response.has_more || !response.next_cursor) break;
    cursor = response.next_cursor;
  }
  return schools;
}

export function PlatformModelConfigPage() {
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<ConfigFormValues>();
  const [schools, setSchools] = useState<Tenant[]>([]);
  const [schoolLoading, setSchoolLoading] = useState(true);
  const [selectedTenantID, setSelectedTenantID] = useState("");
  const [configs, setConfigs] = useState<ManagedModelAPIConfig[]>([]);
  const [configLoading, setConfigLoading] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<ManagedModelAPIConfig | null>(null);
  const [saving, setSaving] = useState(false);
  const [probingID, setProbingID] = useState("");
  const configRequestRef = useRef(0);

  const selectedSchool = useMemo(
    () => schools.find((school) => school.id === selectedTenantID),
    [schools, selectedTenantID]
  );

  const loadSchools = useCallback(async () => {
    setSchoolLoading(true);
    try {
      const items = await loadAllSchoolTenants();
      setSchools(items);
      setSelectedTenantID((current) => current && items.some((school) => school.id === current)
        ? current
        : items.find((school) => school.status === "active")?.id ?? items[0]?.id ?? "");
    } catch {
      message.error("学校列表加载失败");
    } finally {
      setSchoolLoading(false);
    }
  }, [message]);

  const loadConfigs = useCallback(async (tenantID: string) => {
    const requestID = ++configRequestRef.current;
    if (!tenantID) {
      setConfigs([]);
      setConfigLoading(false);
      return;
    }
    setConfigLoading(true);
    try {
      const response = await listManagedModelAPIConfigs(tenantID);
      if (requestID !== configRequestRef.current) return;
      setConfigs(response.configs);
    } catch {
      if (requestID !== configRequestRef.current) return;
      setConfigs([]);
      message.error("模型 API 配置加载失败");
    } finally {
      if (requestID === configRequestRef.current) setConfigLoading(false);
    }
  }, [message]);

  useEffect(() => { void loadSchools(); }, [loadSchools]);
  useEffect(() => {
    setConfigs([]);
    void loadConfigs(selectedTenantID);
    return () => { configRequestRef.current += 1; };
  }, [loadConfigs, selectedTenantID]);

  const openCreate = () => {
    setEditing(null);
    form.setFieldsValue({
      supplier: "custom",
      provider_key: "custom",
      display_name: "自定义供应商",
      adapter_type: "openai_compatible",
      base_url: "",
      api_key: "",
      model_name: "",
      model_version: "",
      region: "global",
      enabled: true,
      is_default: configs.length === 0
    });
    setDrawerOpen(true);
  };

  const openEdit = useCallback((config: ManagedModelAPIConfig) => {
    setEditing(config);
    form.setFieldsValue({
      supplier: presetForConfig(config),
      provider_key: config.provider_key,
      display_name: config.display_name,
      adapter_type: config.adapter_type,
      base_url: config.base_url,
      api_key: "",
      model_name: config.model_name,
      model_version: config.model_version,
      region: config.region,
      enabled: config.status === "active",
      is_default: config.is_default
    });
    setDrawerOpen(true);
  }, [form]);

  const changeSupplier = (supplier: SupplierPreset) => {
    if (editing) return;
    form.setFieldsValue({ ...supplierDefaults[supplier], supplier, base_url: "", api_key: "", model_name: "", model_version: "" });
  };

  const save = async () => {
    if (!selectedTenantID || saving) return;
    let values: ConfigFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      if (editing) {
        const response = await updateManagedModelAPIConfig(editing.id, selectedTenantID, {
          display_name: values.display_name.trim(),
          adapter_type: values.adapter_type,
          base_url: values.base_url.trim(),
          api_key: values.api_key.trim(),
          model_name: values.model_name.trim(),
          model_version: values.model_version.trim(),
          region: values.region.trim(),
          status: values.enabled ? "active" : "disabled",
          is_default: values.is_default
        });
        setConfigs((current) => current.map((item) => item.id === response.config.id
          ? response.config
          : response.config.is_default ? { ...item, is_default: false } : item));
        message.success(values.api_key ? "配置和密钥已更新" : "配置已更新");
      } else {
        const response = await createManagedModelAPIConfig({
          tenant_id: selectedTenantID,
          provider_key: values.provider_key.trim().toLowerCase(),
          display_name: values.display_name.trim(),
          adapter_type: values.adapter_type,
          base_url: values.base_url.trim(),
          api_key: values.api_key.trim(),
          model_name: values.model_name.trim(),
          model_version: values.model_version.trim(),
          region: values.region.trim(),
          status: values.enabled ? "active" : "disabled",
          is_default: values.is_default
        });
        setConfigs((current) => [response.config, ...current.map((item) => response.config.is_default ? { ...item, is_default: false } : item)]);
        message.success(`已为${selectedSchool?.name ?? "学校"}分配模型 API`);
      }
      setDrawerOpen(false);
      form.resetFields();
    } catch {
      message.error(editing ? "配置更新失败" : "配置保存失败，请检查供应商标识是否重复");
    } finally {
      setSaving(false);
    }
  };

  const probe = useCallback(async (config: ManagedModelAPIConfig) => {
    setProbingID(config.id);
    try {
      const response = await probeManagedModelAPIConfig(config.id, selectedTenantID);
      setConfigs((current) => current.map((item) => item.id === response.config.id ? response.config : item));
      message.success(`${config.display_name}：${response.result.message}`);
    } catch {
      await loadConfigs(selectedTenantID);
      message.error(`${config.display_name}连接失败，请检查接口地址、密钥和模型权限`);
    } finally {
      setProbingID("");
    }
  }, [loadConfigs, message, selectedTenantID]);

  const columns = useMemo<TableColumnsType<ManagedModelAPIConfig>>(() => [
    {
      title: "供应商与模型",
      key: "identity",
      render: (_value, config) => (
        <div className="platform-model-identity">
          <span className={config.status === "active" ? "active" : "disabled"}><Zap size={16} /></span>
          <div>
            <strong>{config.display_name}</strong>
            <small>{config.model_name} · {config.model_version}</small>
          </div>
        </div>
      )
    },
    {
      title: "接口",
      dataIndex: "base_url",
      key: "base_url",
      render: (baseURL: string, config) => (
        <div className="platform-model-endpoint">
          <span>{baseURL}</span>
          <small>{config.adapter_type === "openai_compatible" ? "OpenAI 兼容协议" : "DashScope 原生协议"}</small>
        </div>
      )
    },
    {
      title: "密钥",
      key: "credential",
      width: 130,
      render: (_value, config) => (
        <div className="platform-model-secret">
          <KeyRound size={14} />
          <span>{config.credential_hint || "已配置"}</span>
        </div>
      )
    },
    {
      title: "连接状态",
      key: "probe",
      width: 170,
      render: (_value, config) => (
        <div className="platform-model-probe">
          <StatusTag tone={config.last_test_status === "success" ? "success" : config.last_test_status === "failed" ? "danger" : "neutral"}>
            {config.last_test_status === "success" ? "连接正常" : config.last_test_status === "failed" ? "连接失败" : "尚未测试"}
          </StatusTag>
          <small>{formatDateTime(config.last_tested_at)}</small>
        </div>
      )
    },
    {
      title: "分配",
      key: "assignment",
      width: 120,
      render: (_value, config) => config.is_default
        ? <StatusTag tone="info">当前默认</StatusTag>
        : <StatusTag tone={config.status === "active" ? "success" : "neutral"}>{config.status === "active" ? "可用" : "已停用"}</StatusTag>
    },
    {
      title: "操作",
      key: "actions",
      width: 190,
      render: (_value, config) => (
        <Space size={6}>
          <Button size="small" icon={<Zap size={14} />} loading={probingID === config.id} onClick={() => void probe(config)}>测试连接</Button>
          <Tooltip title="编辑配置或更换密钥">
            <Button size="small" type="text" icon={<Pencil size={14} />} aria-label={`编辑${config.display_name}`} onClick={() => openEdit(config)} />
          </Tooltip>
        </Space>
      )
    }
  ], [openEdit, probe, probingID]);

  const activeCount = configs.filter((config) => config.status === "active").length;
  const healthyCount = configs.filter((config) => config.last_test_status === "success").length;

  return (
    <div className="platform-model-page">
      <section className="page-heading platform-model-heading">
        <div>
          <span className="platform-model-kicker"><ShieldCheck size={15} /> 平台级密钥托管</span>
          <h1>模型配置</h1>
          <p>选择学校，为其分配独立的第三方模型 API、模型和密钥；默认 API 用于考试资料解析。</p>
        </div>
        <Space>
          <Button icon={<RefreshCw size={16} />} loading={schoolLoading || configLoading} onClick={() => { void loadSchools(); void loadConfigs(selectedTenantID); }}>刷新</Button>
          <Button type="primary" icon={<Plus size={16} />} disabled={!selectedTenantID || configLoading} onClick={openCreate}>添加 API 配置</Button>
        </Space>
      </section>

      <section className="platform-model-schoolbar">
        <div className="platform-model-school-select">
          <label htmlFor="platform-model-school">配置学校</label>
          <Select
            id="platform-model-school"
            showSearch
            optionFilterProp="label"
            loading={schoolLoading}
            disabled={drawerOpen || saving || Boolean(probingID)}
            value={selectedTenantID || undefined}
            placeholder="选择一所学校"
            options={schools.map((school) => ({ value: school.id, label: `${school.name} · ${school.code}`, disabled: school.status !== "active" }))}
            onChange={(tenantID) => {
              configRequestRef.current += 1;
              setConfigs([]);
              setConfigLoading(true);
              setSelectedTenantID(tenantID);
            }}
          />
        </div>
        <div className="platform-model-school-summary">
          <span><Building2 size={15} /> {selectedSchool?.name ?? "未选择学校"}</span>
          <span><Zap size={15} /> {activeCount} 个可用配置</span>
          <span><CheckCircle2 size={15} /> {healthyCount} 个连接正常</span>
        </div>
      </section>

      <Alert
        className="platform-model-security-note"
        type="info"
        showIcon
        message="密钥加密保存，学校配置相互隔离。设置默认 API 后，该校考试资料中的文字将发送到指定供应商解析；未设置默认 API 时使用本地模型。"
      />

      <section className="platform-model-table-shell">
        {selectedTenantID ? (
          <ResponsiveTable<ManagedModelAPIConfig>
            rowKey="id"
            loading={configLoading}
            columns={columns}
            dataSource={configs}
            pagination={false}
            locale={{ emptyText: <Empty image={<Unplug size={38} />} description="这所学校还没有第三方模型 API" /> }}
          />
        ) : (
          <Empty description="请先选择学校" />
        )}
      </section>

      <Drawer
        title={editing ? `编辑 ${editing.display_name}` : `为${selectedSchool?.name ?? "学校"}添加 API`}
        width={560}
        open={drawerOpen}
        closable={!saving}
        maskClosable={!saving}
        keyboard={!saving}
        onClose={() => { if (!saving) { setDrawerOpen(false); form.resetFields(); } }}
        extra={<Space><Button disabled={saving} onClick={() => setDrawerOpen(false)}>取消</Button><Button type="primary" loading={saving} onClick={() => void save()}>保存配置</Button></Space>}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" requiredMark={false} className="platform-model-form">
          <div className="platform-model-form-grid">
            <Form.Item name="supplier" label="供应商" rules={[{ required: true, message: "请选择供应商" }]}>
              <Select options={supplierOptions} disabled={Boolean(editing)} onChange={changeSupplier} />
            </Form.Item>
            <Form.Item name="provider_key" label="供应商标识" rules={[{ required: true }, { pattern: /^[a-z0-9][a-z0-9._-]{0,127}$/, message: "使用小写字母、数字、点、下划线或连字符" }]}>
              <Input disabled={Boolean(editing)} maxLength={128} />
            </Form.Item>
          </div>
          <Form.Item name="display_name" label="显示名称" rules={[{ required: true, message: "请输入显示名称" }]}>
            <Input maxLength={128} />
          </Form.Item>
          <Form.Item name="adapter_type" label="接口协议" rules={[{ required: true }]}>
            <Select options={[
              { value: "openai_compatible", label: "OpenAI 兼容协议" },
              { value: "dashscope_native", label: "DashScope 原生协议" }
            ]} />
          </Form.Item>
          <Form.Item
            name="base_url"
            label="Base URL"
            extra="填写到 API 版本层级。兼容协议测试 /models；DashScope 原生协议发送一条最小测试请求。"
            rules={[
              { required: true, message: "请输入接口地址" },
              { pattern: /^https:\/\/[^\s]+$/i, message: "第三方接口必须使用 HTTPS" }
            ]}
          >
            <Input placeholder="https://api.example.com/v1" autoComplete="url" />
          </Form.Item>
          <Form.Item
            name="api_key"
            label={editing ? "API Key（留空表示不更换）" : "API Key"}
            rules={editing ? [{ min: 16, message: "密钥至少 16 个字符" }] : [{ required: true, message: "请输入 API Key" }, { min: 16, message: "密钥至少 16 个字符" }]}
          >
            <Input.Password prefix={<KeyRound size={15} />} maxLength={1024} autoComplete="new-password" placeholder={editing ? `当前 ${editing.credential_hint ?? "已配置"}` : "输入供应商密钥"} />
          </Form.Item>
          <div className="platform-model-form-grid">
            <Form.Item name="model_name" label="模型名称" rules={[{ required: true, message: "请输入模型名称" }]}>
              <Input placeholder="用于请求的模型名" maxLength={256} />
            </Form.Item>
            <Form.Item name="model_version" label="模型版本" rules={[{ required: true, message: "请输入模型版本" }]}>
              <Input placeholder="用于审计的固定版本" maxLength={256} />
            </Form.Item>
          </div>
          <Form.Item name="region" label="服务区域" rules={[{ required: true, message: "请输入服务区域" }]}>
            <Input placeholder="cn / global / cn-beijing" maxLength={128} />
          </Form.Item>
          <div className="platform-model-switches">
            <Form.Item name="enabled" label="启用配置" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="停用" />
            </Form.Item>
            <Form.Item name="is_default" label="设为该校默认 API" valuePropName="checked">
              <Switch checkedChildren="默认" unCheckedChildren="备用" />
            </Form.Item>
          </div>
        </Form>
      </Drawer>
    </div>
  );
}
