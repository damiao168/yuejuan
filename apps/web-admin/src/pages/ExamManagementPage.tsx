import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  App,
  Button,
  Descriptions,
  Dropdown,
  Drawer,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
  type MenuProps,
  type SelectProps,
  type TableColumnsType
} from "antd";
import { Archive, Eye, FileClock, LayoutDashboard, MoreHorizontal, Pencil, Plus, RefreshCw, Save, Search } from "lucide-react";
import { ApiClientError } from "../api/client";
import type { SessionUser } from "../auth/session";
import {
  archiveExam,
  createExam,
  getExam,
  listExams,
  updateExam,
  updateExamStatus,
  type Exam,
  type ExamPayload
} from "../api/exams";
import { listClasses, listGrades, listSchools, type Grade, type School, type SchoolClass } from "../api/org";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { ResponsiveTable } from "../components/ResponsiveTable";
import { StatusTag } from "../components/StatusTag";
import { examStatusLabels, examStatusTone, examSubjectOptions } from "../constants/examStatus";
import type { ProductExperience } from "../router/experience";
import { hashQueryParam } from "../router/query";

const statusFlow = ["draft", "configured", "ready", "collecting", "grading", "reviewing", "finalized", "published", "archived"];

const advanceLabels: Record<string, string> = {
  draft: "开始配置",
  collecting: "结束采集，进入阅卷",
  grading: "进入复核",
  reviewing: "定稿成绩"
};

const subjectOptions = examSubjectOptions;

const examTypeOptions = [
  { label: "正式考试", value: "formal_exam" },
  { label: "校内考试", value: "school_exam" },
  { label: "联考", value: "joint_exam" },
  { label: "模拟考试", value: "mock_exam" },
  { label: "阶段测验", value: "quiz" },
  { label: "作业", value: "homework" }
];

const gradingModeOptions = [
  { label: "仅客观题自动", value: "auto_objective_only" },
  { label: "AI 辅助 + 人工确认", value: "ai_assisted" },
  { label: "必须人工复核", value: "human_review_required" },
  { label: "双评", value: "double_mark" },
  { label: "盲双评", value: "blind_double_mark" }
];

const publishPolicyOptions = [
  { label: "管理员审批后发布", value: "after_admin_approval" },
  { label: "阅卷完成后手动发布", value: "manual_publish" },
  { label: "成绩确认后自动发布", value: "after_grade_confirmation" }
];

interface ExamFormValues {
  school_id: string;
  name: string;
  subject: string;
  exam_type: string;
  total_score: number;
  grading_mode: string;
  appeal_enabled: boolean;
  publish_policy: string;
  class_ids: string[];
}

interface Filters {
  search: string;
  schoolId: string;
  subject: string;
  gradeId: string;
  status: string;
  examType: string;
}

function labelFrom(options: { label: string; value: string }[], value: string) {
  return options.find((item) => item.value === value)?.label ?? value;
}

function nextStatus(status: string) {
  if (status === "configured" || status === "ready" || status === "finalized") {
    return null;
  }
  const index = statusFlow.indexOf(status);
  return index >= 0 && index < statusFlow.length - 1 ? statusFlow[index + 1] : null;
}

function isLocked(status: string) {
  return status === "published" || status === "archived";
}

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    console.error(`考试操作失败：${error.status} ${error.code}`, error);
    return error.message || "操作失败，请稍后重试";
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return "操作失败，请稍后重试";
}

function formatTime(value?: string) {
  if (!value) {
    return "暂无记录";
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString("zh-CN", { hour12: false });
}

export function ExamManagementPage({ mode, canManage, currentUser, onOpenWorkspace }: { mode: ProductExperience; canManage: boolean; currentUser: SessionUser; onOpenWorkspace: (examId: string) => void }) {
  const { message, modal } = App.useApp();
  const [form] = Form.useForm<ExamFormValues>();
  const watchedSchoolId = Form.useWatch("school_id", form);
  const watchedGradingMode = Form.useWatch("grading_mode", form);
  const [filters, setFilters] = useState<Filters>(() => {
    const status = hashQueryParam("status");
    return { search: "", schoolId: "", subject: "", gradeId: "", status: status === "active" || statusFlow.includes(status) ? status : "", examType: "" };
  });
  const [exams, setExams] = useState<Exam[]>([]);
  const [schools, setSchools] = useState<School[]>([]);
  const [grades, setGrades] = useState<Grade[]>([]);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [drawer, setDrawer] = useState<{ mode: "create" | "edit"; exam?: Exam } | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailExam, setDetailExam] = useState<Exam | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [actioningId, setActioningId] = useState<string | null>(null);
  const canWrite = canManage;
  const teacherMode = mode === "teacher";

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const examResult = await listExams({ status: filters.status && filters.status !== "active" ? filters.status : undefined, school_id: teacherMode ? undefined : filters.schoolId || undefined });
      setExams(examResult.exams);
      if (teacherMode) {
        setSchools([]);
        setGrades([]);
        setClasses([]);
      } else {
        const [schoolResult, gradeResult, classResult] = await Promise.all([
          listSchools(),
          listGrades(filters.schoolId || undefined),
          listClasses()
        ]);
        setSchools(schoolResult.schools);
        setGrades(gradeResult.grades);
        setClasses(classResult.classes);
      }
    } catch (currentError) {
      setError(formatError(currentError));
    } finally {
      setLoading(false);
    }
  }, [filters.schoolId, filters.status, teacherMode]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useEffect(() => {
    if (!drawer) {
      return;
    }
    if (drawer.mode === "create") {
      form.setFieldsValue({
        appeal_enabled: true,
        publish_policy: "after_admin_approval",
        grading_mode: "ai_assisted",
        total_score: 100,
        class_ids: []
      });
      return;
    }
    if (drawer.exam) {
      form.setFieldsValue({
        school_id: drawer.exam.school_id,
        name: drawer.exam.name,
        subject: drawer.exam.subject,
        exam_type: drawer.exam.exam_type,
        total_score: drawer.exam.total_score,
        grading_mode: drawer.exam.grading_mode,
        appeal_enabled: drawer.exam.appeal_enabled,
        publish_policy: drawer.exam.publish_policy,
        class_ids: drawer.exam.class_ids
      });
    }
  }, [drawer, form]);

  const schoolById = useMemo(() => new Map(schools.map((school) => [school.id, school])), [schools]);
  const gradeById = useMemo(() => new Map(grades.map((grade) => [grade.id, grade])), [grades]);
  const classById = useMemo(() => new Map(classes.map((schoolClass) => [schoolClass.id, schoolClass])), [classes]);

  const gradeOptions = useMemo(() => grades.map((grade) => ({ label: grade.name, value: grade.id })), [grades]);
  const schoolOptions = useMemo(() => schools.map((school) => ({ label: school.name, value: school.id })), [schools]);

  const classOptions = useMemo<SelectProps["options"]>(() => {
    const availableClasses = classes.filter((schoolClass) => !watchedSchoolId || schoolClass.school_id === watchedSchoolId);
    const grouped = grades
      .map((grade) => ({
        label: grade.name,
        options: availableClasses
          .filter((schoolClass) => schoolClass.grade_id === grade.id)
          .map((schoolClass) => ({ label: `${schoolClass.name} (${schoolClass.code})`, value: schoolClass.id }))
      }))
      .filter((group) => group.options.length > 0);
    const ungrouped = availableClasses
      .filter((schoolClass) => !gradeById.has(schoolClass.grade_id))
      .map((schoolClass) => ({ label: `${schoolClass.name} (${schoolClass.code})`, value: schoolClass.id }));
    return ungrouped.length > 0 ? [...grouped, { label: "未关联年级", options: ungrouped }] : grouped;
  }, [classes, gradeById, grades, watchedSchoolId]);

  const gradeNamesForExam = useCallback(
    (exam: Exam) => {
      const gradeNames = new Set<string>();
      for (const classId of exam.class_ids) {
        const schoolClass = classById.get(classId);
        const grade = schoolClass ? gradeById.get(schoolClass.grade_id) : undefined;
        if (grade) {
          gradeNames.add(grade.name);
        }
      }
      return gradeNames.size > 0 ? Array.from(gradeNames).join("、") : "未关联年级";
    },
    [classById, gradeById]
  );

  const filteredExams = useMemo(() => {
    const keyword = filters.search.trim().toLowerCase();
    return exams.filter((exam) => {
      const keywordMatched =
        !keyword ||
        exam.name.toLowerCase().includes(keyword) ||
        labelFrom(subjectOptions, exam.subject).toLowerCase().includes(keyword);
      const statusMatched = filters.status !== "active" || !["published", "archived"].includes(exam.status);
      const subjectMatched = !filters.subject || exam.subject === filters.subject;
      const typeMatched = !filters.examType || exam.exam_type === filters.examType;
      const gradeMatched =
        !filters.gradeId ||
        exam.class_ids.some((classId) => {
          const schoolClass = classById.get(classId);
          return schoolClass?.grade_id === filters.gradeId;
        });
      return keywordMatched && statusMatched && subjectMatched && typeMatched && gradeMatched;
    });
  }, [classById, exams, filters.examType, filters.gradeId, filters.search, filters.subject]);

  const openDetail = async (exam: Exam) => {
    setDetailOpen(true);
    setDetailExam(null);
    setDetailError(null);
    setDetailLoading(true);
    try {
      const result = await getExam(exam.id);
      setDetailExam(result.exam);
    } catch (currentError) {
      setDetailError(formatError(currentError));
    } finally {
      setDetailLoading(false);
    }
  };

  const submitForm = async () => {
    const values = await form.validateFields();
    const payload: ExamPayload = {
      school_id: values.school_id,
      name: values.name,
      subject: values.subject,
      exam_type: values.exam_type,
      total_score: values.total_score,
      grading_mode: values.grading_mode,
      appeal_enabled: values.appeal_enabled,
      publish_policy: values.publish_policy,
      class_ids: values.class_ids
    };
    setSubmitting(true);
    try {
      if (drawer?.mode === "edit" && drawer.exam) {
        await updateExam(drawer.exam.id, { ...payload, expected_revision: drawer.exam.revision });
        message.success("考试已更新");
      } else {
        await createExam(payload);
        message.success("考试已创建");
      }
      setDrawer(null);
      await loadData();
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setSubmitting(false);
    }
  };

  const changeStatus = (exam: Exam, status: string) => {
    modal.confirm({
      title: "确认推进考试状态",
      content: `将“${exam.name}”推进到“${examStatusLabels[status] ?? "下一阶段"}”。`,
      okText: "确认",
      cancelText: "取消",
      onOk: async () => {
        setActioningId(exam.id);
        try {
          await updateExamStatus(exam.id, status, exam.revision);
          message.success("状态已更新");
          await loadData();
        } catch (currentError) {
          message.error(formatError(currentError));
        } finally {
          setActioningId(null);
        }
      }
    });
  };

  const archive = (exam: Exam) => {
    modal.confirm({
      title: "确认归档考试",
      content: `归档后“${exam.name}”将不能继续编辑核心配置。`,
      okText: "归档",
      okButtonProps: { danger: true },
      cancelText: "取消",
      onOk: async () => {
        setActioningId(exam.id);
        try {
          await archiveExam(exam.id, exam.revision);
          message.success("考试已归档");
          await loadData();
        } catch (currentError) {
          message.error(formatError(currentError));
        } finally {
          setActioningId(null);
        }
      }
    });
  };

  const columns: TableColumnsType<Exam> = [
    {
      title: "考试",
      dataIndex: "name",
      width: 260,
      render: (value: string, exam) => (
        <div className="exam-name-cell">
          <button type="button" onClick={() => onOpenWorkspace(exam.id)}>{value}</button>
          <span>{labelFrom(examTypeOptions, exam.exam_type)}</span>
        </div>
      )
    },
    {
      title: "学科与范围",
      width: 175,
      render: (_, exam) => (
        <div className="exam-scope-cell">
          <strong>{labelFrom(subjectOptions, exam.subject)} · {gradeNamesForExam(exam)}</strong>
          <span>{exam.class_ids.length} 个班级</span>
        </div>
      )
    },
    { title: "总分", dataIndex: "total_score", width: 65, align: "right" },
    { title: "状态", dataIndex: "status", width: 105, render: (value: string) => <StatusTag tone={examStatusTone(value)}>{examStatusLabels[value] ?? "未知状态"}</StatusTag> },
    { title: "阅卷模式", dataIndex: "grading_mode", width: 165, render: (value: string) => <span className="exam-mode-text">{labelFrom(gradingModeOptions, value)}</span> },
    {
      title: "创建信息",
      width: 165,
      render: (_, exam) => (
        <div className="exam-created-cell">
          <strong>{exam.created_by === currentUser.id ? currentUser.name : "本校管理员"}</strong>
          <span>{formatTime(exam.created_at)}</span>
        </div>
      )
    },
    {
      title: "操作",
      width: 250,
      render: (_, exam) => {
        const next = nextStatus(exam.status);
        const locked = isLocked(exam.status);
        const moreItems: MenuProps["items"] = [
          { key: "detail", label: "查看详情", icon: <Eye size={14} /> },
          ...(canWrite && next && next !== "archived"
            ? [{ key: "advance", label: advanceLabels[exam.status] ?? `推进到${examStatusLabels[next] ?? "下一阶段"}` }]
            : []),
          ...(canWrite ? [{ key: "edit", label: "编辑考试", icon: <Pencil size={14} />, disabled: locked }] : []),
          ...(canWrite && exam.status !== "archived" ? [{ key: "archive", label: "归档考试", icon: <Archive size={14} />, danger: true }] : [])
        ];
        return (
          <Space className="table-actions exam-table-actions" size={6}>
            <Button size="small" type="primary" ghost icon={<LayoutDashboard size={14} />} onClick={() => onOpenWorkspace(exam.id)}>
              工作区
            </Button>
            <Dropdown
              trigger={["click"]}
              menu={{
                items: moreItems,
                onClick: ({ key }) => {
                  if (key === "detail") void openDetail(exam);
                  if (key === "advance" && next) changeStatus(exam, next);
                  if (key === "edit") setDrawer({ mode: "edit", exam });
                  if (key === "archive") archive(exam);
                }
              }}
            >
              <Button size="small" loading={actioningId === exam.id} aria-label={`${exam.name} 更多操作`} icon={<MoreHorizontal size={15} />} />
            </Dropdown>
          </Space>
        );
      }
    }
  ];

  const formDisabled = !canWrite || (drawer?.mode === "edit" && drawer.exam ? isLocked(drawer.exam.status) : false);

  return (
    <div className="page-stack">
      <section className="page-heading">
        <div>
          <h1>{teacherMode ? "我的考试" : "考试管理"}</h1>
          <p>{teacherMode ? "查看已授权考试，并进入试卷、阅卷、成绩和学情工作区。" : "创建考试、维护班级范围、推进考试状态和进入后续配置流程。"}</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} onClick={() => void loadData()} loading={loading}>
            刷新
          </Button>
          {canWrite ? <Button type="primary" icon={<Plus size={16} />} onClick={() => setDrawer({ mode: "create" })}>
            新建考试
          </Button> : null}
        </Space>
      </section>

      <section className="workspace-section filter-panel">
        <div className="filter-grid exam-filter-primary">
          <Input
            prefix={<Search size={16} />}
            placeholder="搜索考试名称或学科"
            value={filters.search}
            onChange={(event) => setFilters((current) => ({ ...current, search: event.target.value }))}
          />
          <Select
            placeholder="状态"
            allowClear
            value={filters.status || undefined}
            options={[
              { label: "进行中", value: "active" },
              ...statusFlow.map((status) => ({ label: examStatusLabels[status] ?? "未知状态", value: status }))
            ]}
            onChange={(value) => setFilters((current) => ({ ...current, status: value ?? "" }))}
          />
        </div>
        {!teacherMode ? <details className="advanced-filter-disclosure">
          <summary>更多筛选</summary>
          <div className="filter-grid">
            <Select placeholder="学校" allowClear value={filters.schoolId || undefined} options={schoolOptions} onChange={(value) => setFilters((current) => ({ ...current, schoolId: value ?? "", gradeId: "" }))} />
            <Select placeholder="学科" allowClear value={filters.subject || undefined} options={subjectOptions} onChange={(value) => setFilters((current) => ({ ...current, subject: value ?? "" }))} />
            <Select placeholder="年级" allowClear value={filters.gradeId || undefined} options={gradeOptions} onChange={(value) => setFilters((current) => ({ ...current, gradeId: value ?? "" }))} />
            <Select placeholder="考试类型" allowClear value={filters.examType || undefined} options={examTypeOptions} onChange={(value) => setFilters((current) => ({ ...current, examType: value ?? "" }))} />
          </div>
        </details> : null}
      </section>

      {loading ? (
        <section className="workspace-section">
          <LoadingState label="正在加载考试列表" />
        </section>
      ) : error ? (
        <ErrorState message={error} onRetry={() => void loadData()} />
      ) : (
        <section className="workspace-section">
          <div className="section-head">
            <div>
              <h2>{teacherMode ? "已授权考试" : "考试列表"}</h2>
            </div>
          </div>
          <ResponsiveTable<Exam>
            rowKey="id"
            dataSource={filteredExams}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false, showTotal: (total) => `共 ${total} 场考试` }}
            locale={{ emptyText: <EmptyState title="暂无考试" description={canWrite ? "没有符合条件的考试。试试调整筛选条件，或点击右上角“新建考试”。" : "没有符合条件的考试，请联系管理员为你授权。"} /> }}
            size="middle"
            className="exam-management-table"
          />
        </section>
      )}

      <Drawer
        title={drawer?.mode === "edit" ? "编辑考试" : "新建考试"}
        open={Boolean(drawer)}
        onClose={() => setDrawer(null)}
        width={720}
        destroyOnClose
        extra={
          <Space>
            <Button onClick={() => form.resetFields()}>重置</Button>
            <Button type="primary" icon={<Save size={16} />} loading={submitting} disabled={formDisabled} onClick={() => void submitForm()}>
              保存
            </Button>
          </Space>
        }
      >
        {formDisabled && drawer?.mode === "edit" ? (
          <Alert type="info" showIcon message="当前考试状态不允许编辑核心配置" className="drawer-alert" />
        ) : null}
        <Form form={form} layout="vertical" disabled={formDisabled} preserve={false}>
          <div className="form-grid">
            <Form.Item label="考试名称" name="name" rules={[{ required: true, message: "请输入考试名称" }]}>
              <Input placeholder="高二物理期末考试" />
            </Form.Item>
            <Form.Item label="学校" name="school_id" rules={[{ required: true, message: "请选择学校" }]}>
              <Select options={schoolOptions} placeholder="选择学校" />
            </Form.Item>
            <Form.Item label="学科" name="subject" rules={[{ required: true, message: "请选择学科" }]}>
              <Select options={subjectOptions} placeholder="选择学科" />
            </Form.Item>
            <Form.Item label="考试类型" name="exam_type" rules={[{ required: true, message: "请选择考试类型" }]}>
              <Select options={examTypeOptions} placeholder="选择考试类型" />
            </Form.Item>
            <Form.Item label="总分" name="total_score" rules={[{ required: true, message: "请输入总分" }]}>
              <InputNumber min={1} max={1000} precision={1} className="full-width-control" />
            </Form.Item>
            <Form.Item
              label="阅卷模式"
              name="grading_mode"
              rules={[{ required: true, message: "请选择阅卷模式" }]}
              extra={watchedGradingMode === "double_mark" || watchedGradingMode === "blind_double_mark" ? "双评需在“阅卷”环节为题目逐题开启后才会生效。" : undefined}
            >
              <Select options={gradingModeOptions} placeholder="选择阅卷模式" />
            </Form.Item>
            <Form.Item label="成绩发布策略" name="publish_policy" rules={[{ required: true, message: "请选择发布策略" }]}>
              <Select options={publishPolicyOptions} placeholder="选择发布策略" />
            </Form.Item>
            <Form.Item label="允许申诉" name="appeal_enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
          </div>
          <Form.Item
            label="选择班级"
            name="class_ids"
            rules={[
              { required: true, message: "请选择至少一个班级" },
              { type: "array", min: 1, message: "请选择至少一个班级" }
            ]}
          >
            <Select mode="multiple" options={classOptions} placeholder="按年级选择班级" optionFilterProp="label" />
          </Form.Item>
        </Form>
      </Drawer>

      <Drawer title="考试详情" open={detailOpen} onClose={() => setDetailOpen(false)} width={680}>
        {detailLoading ? (
          <LoadingState label="正在读取考试详情" />
        ) : detailError ? (
          <ErrorState message={detailError} />
        ) : detailExam ? (
          <div className="detail-stack">
            <Descriptions bordered column={1} size="small">
              <Descriptions.Item label="考试名称">{detailExam.name}</Descriptions.Item>
              <Descriptions.Item label="学校">{schoolById.get(detailExam.school_id)?.name ?? "本校"}</Descriptions.Item>
              <Descriptions.Item label="学科">{labelFrom(subjectOptions, detailExam.subject)}</Descriptions.Item>
              <Descriptions.Item label="考试类型">{labelFrom(examTypeOptions, detailExam.exam_type)}</Descriptions.Item>
              <Descriptions.Item label="年级">{gradeNamesForExam(detailExam)}</Descriptions.Item>
              <Descriptions.Item label="班级数量">{detailExam.class_ids.length}</Descriptions.Item>
              <Descriptions.Item label="总分">{detailExam.total_score}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusTag tone={examStatusTone(detailExam.status)}>{examStatusLabels[detailExam.status] ?? "未知状态"}</StatusTag>
              </Descriptions.Item>
              <Descriptions.Item label="阅卷模式">{labelFrom(gradingModeOptions, detailExam.grading_mode)}</Descriptions.Item>
              <Descriptions.Item label="允许申诉">{detailExam.appeal_enabled ? "是" : "否"}</Descriptions.Item>
              <Descriptions.Item label="成绩发布策略">{labelFrom(publishPolicyOptions, detailExam.publish_policy)}</Descriptions.Item>
              <Descriptions.Item label="创建人">{detailExam.created_by === currentUser.id ? currentUser.name : "本校管理员"}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatTime(detailExam.created_at)}</Descriptions.Item>
            </Descriptions>

            {!teacherMode ? (
              <Button icon={<FileClock size={16} />} onClick={() => (window.location.hash = "/audit")}>
                查看操作日志
              </Button>
            ) : null}
          </div>
        ) : null}
      </Drawer>
    </div>
  );
}
