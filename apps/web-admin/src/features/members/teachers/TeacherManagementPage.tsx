import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App, Button, Form, Input, Modal, Select, Tabs } from "antd";
import type { TableColumnsType } from "antd";
import { Plus, RefreshCw } from "lucide-react";
import { getUserErrorMessage } from "../../../api/client";
import { listClasses, listGrades, listSchools, type Grade, type School, type SchoolClass } from "../../../api/org";
import {
  createManagedUser,
  listAssignableRoles,
  listManagedUsers,
  updateManagedUserStatus,
  type AssignableRole,
  type ManagedUser
} from "../../../api/users";
import type { SessionUser } from "../../../auth/session";
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
const teacherRoles = ["teacher", "grader", "arbitrator"];

type MemberView = "teachers" | "administrators";
type CreateMode = "teacher" | "administrator";

export function TeacherManagementPage({ currentUser }: { currentUser: SessionUser }) {
  const { message, modal } = App.useApp();
  const [users, setUsers] = useState<ManagedUser[]>([]);
  const [roles, setRoles] = useState<AssignableRole[]>([]);
  const [grades, setGrades] = useState<Grade[]>([]);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [schools, setSchools] = useState<School[]>([]);
  const [view, setView] = useState<MemberView>("teachers");
  const [query, setQuery] = useState("");
  const [createMode, setCreateMode] = useState<CreateMode>("teacher");
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [updatingUserID, setUpdatingUserID] = useState("");
  const [error, setError] = useState("");
  const [form] = Form.useForm();
  const selectedRole = Form.useWatch("role_code", form);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [userResult, roleResult, schoolResult, gradeResult, classResult] = await Promise.all([
        listManagedUsers({ limit: 200 }),
        listAssignableRoles(),
        listSchools(),
        listGrades(),
        listClasses()
      ]);
      setUsers(userResult.users.filter((user) => user.roles.some((role) => teacherRoles.includes(role) || role === "school_admin")));
      setRoles(roleResult.roles);
      setSchools(schoolResult.schools);
      setGrades(gradeResult.grades);
      setClasses(classResult.classes);
    } catch (loadError) {
      setError(getUserErrorMessage(loadError, "人员名单加载失败"));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  const teacherUsers = useMemo(() => users.filter((user) => !user.roles.includes("school_admin") && user.roles.some((role) => teacherRoles.includes(role))), [users]);
  const administratorUsers = useMemo(() => users.filter((user) => user.roles.includes("school_admin")), [users]);
  const activeUsers = view === "teachers" ? teacherUsers : administratorUsers;
  const visible = useMemo(() => activeUsers.filter((user) => `${user.display_name} ${user.username}`.toLowerCase().includes(query.trim().toLowerCase())), [activeUsers, query]);
  const assignableRoleCodes = useMemo(() => new Set(roles.map((role) => role.code)), [roles]);
  const canCreateAdministrator = assignableRoleCodes.has("school_admin");
  const schoolNames = useMemo(() => new Map(schools.map((school) => [school.id, school.name])), [schools]);
  const schoolsNeedingBackup = useMemo(() => schools.map((school) => ({
    name: school.name,
    count: administratorUsers.filter((user) => user.school_id === school.id && user.status === "active").length
  })).filter((item) => item.count < 2), [administratorUsers, schools]);
  const administratorNotice = `${schoolsNeedingBackup.length
    ? `${schoolsNeedingBackup.map((item) => `${item.name}当前有 ${item.count} 位`).join("；")}。建议至少保留 2 位启用的学校管理员，系统不允许停用最后一位。`
    : "当前每所学校均有至少 2 位启用的学校管理员，系统不允许停用最后一位。"}${canCreateAdministrator ? "" : " 如需新增、停用或恢复管理员，请联系机构管理员。"}`;

  function canUpdateStatus(user: ManagedUser) {
    return user.id !== currentUser.id && user.roles.length > 0 && user.roles.every((role) => assignableRoleCodes.has(role));
  }

  const teacherColumns: TableColumnsType<ManagedUser> = [
    { title: "姓名", dataIndex: "display_name", width: 160 },
    { title: "登录账号", dataIndex: "username", width: 180 },
    { title: "工作职责", dataIndex: "roles", render: (items: string[]) => items.map((item) => roleLabels[item]).filter(Boolean).join("、") || "-" },
    { title: "状态", dataIndex: "status", width: 100, render: (status: string) => <StatusTag tone={status === "active" ? "success" : "neutral"}>{status === "active" ? "启用" : "停用"}</StatusTag> },
    { title: "操作", key: "actions", width: 90, render: (_, item) => canUpdateStatus(item) ? <Button type="link" danger={item.status === "active"} loading={updatingUserID === item.id} onClick={() => confirmStatusChange(item)}>{item.status === "active" ? "停用" : "恢复"}</Button> : "-" }
  ];
  const administratorColumns: TableColumnsType<ManagedUser> = [
    { title: "姓名", dataIndex: "display_name", width: 160 },
    { title: "登录账号", dataIndex: "username", width: 190 },
    { title: "所属学校", dataIndex: "school_id", render: (schoolID?: string) => schoolID ? schoolNames.get(schoolID) ?? "学校信息已变更" : "未标明学校" },
    { title: "状态", dataIndex: "status", width: 100, render: (status: string) => <StatusTag tone={status === "active" ? "success" : "neutral"}>{status === "active" ? "启用" : "停用"}</StatusTag> },
    { title: "操作", key: "actions", width: 90, render: (_, item) => canUpdateStatus(item) ? <Button type="link" danger={item.status === "active"} loading={updatingUserID === item.id} onClick={() => confirmStatusChange(item)}>{item.status === "active" ? "停用" : "恢复"}</Button> : "-" }
  ];

  function openCreate(nextMode: CreateMode) {
    setCreateMode(nextMode);
    form.resetFields();
    if (nextMode === "administrator") form.setFieldValue("role_code", "school_admin");
    setOpen(true);
  }

  async function submit(values: { username: string; display_name: string; password: string; role_code: string; school_id: string; class_ids?: string[] }) {
    setSaving(true);
    try {
      const selectedClasses = classes.filter((item) => (values.class_ids ?? []).includes(item.id));
      const schoolIds = [...new Set(selectedClasses.map((item) => item.school_id))];
      await createManagedUser({
        username: values.username,
        display_name: values.display_name,
        password: values.password,
        role_code: createMode === "administrator" ? "school_admin" : values.role_code,
        school_id: schoolIds.length === 1 ? schoolIds[0] : values.school_id,
        class_ids: values.role_code === "teacher" ? values.class_ids ?? [] : []
      });
      message.success(createMode === "administrator" ? "学校管理员已添加" : values.class_ids?.length ? "教师账号已创建并关联班级" : "教师账号已创建");
      setOpen(false);
      await load();
    } catch (saveError) {
      message.error(getUserErrorMessage(saveError, createMode === "administrator" ? "添加学校管理员失败" : "创建教师账号失败"));
    } finally {
      setSaving(false);
    }
  }

  function confirmStatusChange(user: ManagedUser) {
    const disabling = user.status === "active";
    modal.confirm({
      title: disabling ? `停用${user.display_name || user.username}？` : `恢复${user.display_name || user.username}？`,
      content: disabling ? "停用后，该账号将无法登录；以后可以随时恢复。" : "恢复后，该账号可以重新登录。",
      okText: disabling ? "确认停用" : "确认恢复",
      okButtonProps: { danger: disabling },
      cancelText: "取消",
      onOk: async () => {
        setUpdatingUserID(user.id);
        try {
          const result = await updateManagedUserStatus(user.id, disabling ? "disabled" : "active");
          setUsers((current) => current.map((item) => item.id === result.user.id ? result.user : item));
          message.success(disabling ? "账号已停用" : "账号已恢复");
        } catch (updateError) {
          message.error(getUserErrorMessage(updateError, disabling ? "账号暂时无法停用" : "账号暂时无法恢复"));
          throw updateError;
        } finally {
          setUpdatingUserID("");
        }
      }
    });
  }

  if (loading && !users.length) return <LoadingState label="正在加载人员名单" />;
  if (error && !users.length) return <ErrorState message={error} onRetry={() => void load()} />;
  const isTeacherView = view === "teachers";
  return <div className="member-management-page">
    <section className="member-management-heading">
      <div><h1>{isTeacherView ? "阅卷教师" : "管理员账号"}</h1><p>{isTeacherView ? "管理参与教学、阅卷和复核工作的教师账号。" : "每所学校建议至少配置 2 位管理员，便于工作交接和应急接替。"}</p></div>
      <div><Button icon={<RefreshCw size={15} />} loading={loading} onClick={() => void load()}>刷新</Button>{isTeacherView ? <Button type="primary" icon={<Plus size={15} />} onClick={() => openCreate("teacher")}>新增教师</Button> : canCreateAdministrator ? <Button type="primary" icon={<Plus size={15} />} onClick={() => openCreate("administrator")}>新增管理员</Button> : null}</div>
    </section>
    <section className="member-table-section">
      <Tabs activeKey={view} onChange={(key) => { setView(key as MemberView); setQuery(""); }} items={[
        { key: "teachers", label: `阅卷教师（${teacherUsers.length}）` },
        { key: "administrators", label: `管理员账号（${administratorUsers.length}）` }
      ]} />
      {!isTeacherView ? <Alert className="member-administrator-alert" showIcon type={schoolsNeedingBackup.length ? "warning" : "success"} message={schoolsNeedingBackup.length ? "管理员配置需要完善" : "管理员配置正常"} description={administratorNotice} /> : null}
      <div className="member-management-filters"><Input.Search allowClear value={query} placeholder="搜索姓名或账号" onChange={(event) => setQuery(event.target.value)} /><span>共 {visible.length} 人</span></div>
      <ResponsiveTable rowKey="id" size="small" columns={isTeacherView ? teacherColumns : administratorColumns} dataSource={visible} pagination={{ pageSize: 20 }} scroll={{ x: 930 }} />
    </section>
    <Modal title={createMode === "administrator" ? "新增学校管理员" : "新增教师"} open={open} footer={null} destroyOnHidden onCancel={() => setOpen(false)}>
      <Form form={form} layout="vertical" onFinish={(values) => void submit(values)}>
        <div className="form-grid compact-form-grid">
          <Form.Item name="display_name" label="姓名" rules={[{ required: true, message: "请输入姓名" }]}><Input /></Form.Item>
          <Form.Item name="username" label="登录账号" rules={[{ required: true, message: "请输入登录账号" }]}><Input autoComplete="off" /></Form.Item>
          {createMode === "teacher" ? <Form.Item name="role_code" label="工作职责" rules={[{ required: true, message: "请选择工作职责" }]}><Select options={roles.filter((role) => teacherRoles.includes(role.code)).map((role) => ({ value: role.code, label: roleLabels[role.code] ?? role.name }))} /></Form.Item> : <Form.Item name="role_code" hidden><Input /></Form.Item>}
          <Form.Item name="school_id" label="所属学校" rules={[{ required: true, message: "请选择所属学校" }]}><Select options={schools.map((school) => ({ value: school.id, label: school.name }))} /></Form.Item>
          <Form.Item name="password" label="初始密码" extra="至少 12 位，并包含大小写字母、数字和符号。" rules={[{ required: true, message: "请输入初始密码" }]}><Input.Password autoComplete="new-password" /></Form.Item>
        </div>
        {createMode === "teacher" && selectedRole === "teacher" ? <Form.Item name="class_ids" label="关联班级（可选）"><Select mode="multiple" optionFilterProp="label" options={classes.map((item) => ({ value: item.id, label: `${grades.find((grade) => grade.id === item.grade_id)?.name ?? "未分年级"} · ${item.name}` }))} /></Form.Item> : createMode === "teacher" && selectedRole === "grader" ? <p>阅卷范围由具体阅卷任务决定。</p> : createMode === "teacher" && selectedRole === "arbitrator" ? <p>复核范围由具体复核任务决定。</p> : null}
        <Button type="primary" htmlType="submit" loading={saving}>{createMode === "administrator" ? "添加管理员" : "创建教师"}</Button>
      </Form>
    </Modal>
  </div>;
}
