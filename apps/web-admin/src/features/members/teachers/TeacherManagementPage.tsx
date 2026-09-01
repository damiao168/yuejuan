import { useCallback, useEffect, useMemo, useState } from "react";
import { App, Button, Form, Input, Modal, Select } from "antd";
import type { TableColumnsType } from "antd";
import { Plus, RefreshCw } from "lucide-react";
import { getUserErrorMessage } from "../../../api/client";
import { listClasses, listGrades, listSchools, type Grade, type School, type SchoolClass } from "../../../api/org";
import { createManagedUser, listAssignableRoles, listManagedUsers, type AssignableRole, type ManagedUser } from "../../../api/users";
import { ErrorState, LoadingState } from "../../../components/PageState";
import { ResponsiveTable } from "../../../components/ResponsiveTable";
import { StatusTag } from "../../../components/StatusTag";

const roleLabels: Record<string, string> = {
  tenant_admin: "机构管理员",
  school_admin: "学校管理员",
  teacher: "学科教师",
  grader: "阅卷教师",
  arbitrator: "仲裁/复核教师"
};

export function TeacherManagementPage() {
  const { message } = App.useApp();
  const [users, setUsers] = useState<ManagedUser[]>([]);
  const [roles, setRoles] = useState<AssignableRole[]>([]);
  const [grades, setGrades] = useState<Grade[]>([]);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [schools, setSchools] = useState<School[]>([]);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [form] = Form.useForm();
  const selectedRole = Form.useWatch("role_code", form);

  const load = useCallback(async () => {
    setLoading(true); setError("");
    try {
      const [userResult, roleResult, schoolResult, gradeResult, classResult] = await Promise.all([listManagedUsers({ limit: 200 }), listAssignableRoles(), listSchools(), listGrades(), listClasses()]);
      setUsers(userResult.users.filter((user) => user.roles.some((role) => roleLabels[role])));
      setRoles(roleResult.roles.filter((role) => ["teacher", "grader", "arbitrator"].includes(role.code)));
      setSchools(schoolResult.schools);
      setGrades(gradeResult.grades);
      setClasses(classResult.classes);
    } catch (loadError) { setError(getUserErrorMessage(loadError, "教师与阅卷人员加载失败")); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);

  const visible = useMemo(() => users.filter((user) => `${user.display_name} ${user.username}`.toLowerCase().includes(query.trim().toLowerCase())), [query, users]);
  const columns: TableColumnsType<ManagedUser> = [
    { title: "姓名", dataIndex: "display_name", width: 160 },
    { title: "登录账号", dataIndex: "username", width: 180 },
    { title: "业务角色", dataIndex: "roles", render: (items: string[]) => items.map((item) => roleLabels[item]).filter(Boolean).join("、") || "-" },
    { title: "状态", dataIndex: "status", width: 100, render: (status: string) => <StatusTag tone={status === "active" ? "success" : "neutral"}>{status === "active" ? "启用" : "停用"}</StatusTag> }
  ];

  async function submit(values: { username: string; display_name: string; password: string; role_code: string; school_id: string; class_ids?: string[] }) {
    setSaving(true);
    try {
      const selectedClasses = classes.filter((item) => (values.class_ids ?? []).includes(item.id));
      const schoolIds = [...new Set(selectedClasses.map((item) => item.school_id))];
      await createManagedUser({ username: values.username, display_name: values.display_name, password: values.password, role_code: values.role_code, school_id: schoolIds.length === 1 ? schoolIds[0] : values.school_id, class_ids: values.role_code === "teacher" ? values.class_ids ?? [] : [] });
      message.success(values.class_ids?.length ? "人员账号已创建并绑定班级" : "人员账号已创建");
      setOpen(false); await load();
    } catch (saveError) { message.error(getUserErrorMessage(saveError, "创建人员账号失败")); }
    finally { setSaving(false); }
  }

  if (loading && !users.length) return <LoadingState label="正在加载教师与阅卷人员" />;
  if (error && !users.length) return <ErrorState message={error} onRetry={() => void load()} />;
  return <div className="member-management-page">
    <section className="member-management-heading"><div><h1>教师与阅卷人员</h1><p>维护教师账号及其学校业务角色，不直接暴露技术权限代码。</p></div><div><Button icon={<RefreshCw size={15} />} loading={loading} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<Plus size={15} />} onClick={() => setOpen(true)}>新增人员</Button></div></section>
    <section className="member-table-section"><div className="member-management-filters"><Input.Search allowClear value={query} placeholder="搜索姓名或账号" onChange={(event) => setQuery(event.target.value)} /><span>共 {visible.length} 人</span></div><ResponsiveTable rowKey="id" size="small" columns={columns} dataSource={visible} pagination={{ pageSize: 20 }} scroll={{ x: 930 }} /></section>
    <Modal title="新增教师或阅卷人员" open={open} footer={null} destroyOnHidden onCancel={() => setOpen(false)}><Form form={form} layout="vertical" onFinish={(values) => void submit(values)}><div className="form-grid compact-form-grid"><Form.Item name="display_name" label="姓名" rules={[{ required: true, message: "请输入姓名" }]}><Input /></Form.Item><Form.Item name="username" label="登录账号" rules={[{ required: true, message: "请输入登录账号" }]}><Input autoComplete="off" /></Form.Item><Form.Item name="role_code" label="业务角色" rules={[{ required: true, message: "请选择业务角色" }]}><Select options={roles.map((role) => ({ value: role.code, label: roleLabels[role.code] ?? role.name }))} /></Form.Item><Form.Item name="school_id" label="所属学校" rules={[{ required: true, message: "请选择所属学校" }]}><Select options={schools.map((school) => ({ value: school.id, label: school.name }))} /></Form.Item><Form.Item name="password" label="初始密码" extra="至少 12 位，并包含大小写字母、数字和符号。" rules={[{ required: true, message: "请输入初始密码" }]}><Input.Password autoComplete="new-password" /></Form.Item></div>{selectedRole === "teacher" ? <Form.Item name="class_ids" label="关联班级（可选）"><Select mode="multiple" optionFilterProp="label" options={classes.map((item) => ({ value: item.id, label: `${grades.find((grade) => grade.id === item.grade_id)?.name ?? "未分年级"} · ${item.name}` }))} /></Form.Item> : selectedRole === "grader" ? <p>阅卷权限由阅卷任务分配决定。</p> : selectedRole === "arbitrator" ? <p>仲裁权限由仲裁任务分配决定。</p> : null}<Button type="primary" htmlType="submit" loading={saving}>创建人员</Button></Form></Modal>
  </div>;
}
