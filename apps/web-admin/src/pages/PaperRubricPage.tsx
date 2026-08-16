import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  App,
  Button,
  Form,
  Input,
  InputNumber,
  List,
  Select,
  Space,
  Switch,
  Upload,
  type TableColumnsType,
  type UploadProps
} from "antd";
import { CheckCircle2, FileUp, LockKeyhole, Plus, RefreshCw, Save, Trash2 } from "lucide-react";
import { ApiClientError } from "../api/client";
import { listExams, type Exam } from "../api/exams";
import {
  createQuestion,
  createRubric,
  createScoringRule,
  deleteQuestion,
  listPapers,
  listQuestions,
  listScoringRules,
  publishScoringRule,
  registerPaperFromFile,
  updateQuestion,
  updateScoringRule,
  uploadFile,
  validatePaperConfig,
  type PaperVersion,
  type Question,
  type QuestionPayload,
  type RubricEvidenceRequirement,
  type RubricPoint,
  type ScoringRule,
  type ValidationResult
} from "../api/papers";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { ResponsiveTable } from "../components/ResponsiveTable";
import { StatusTag } from "../components/StatusTag";
import { AssessmentProfileEditor } from "../components/features/assessment/AssessmentProfileEditor";
import { isFormulaEvidenceSubject } from "../features/grading/workbench/mathEvidenceSubjects";
import type { StatusTone } from "../types";

const questionTypeOptions = [
  { label: "单选题", value: "single_choice" },
  { label: "多选题", value: "multiple_choice" },
  { label: "判断题", value: "true_false" },
  { label: "填空题", value: "fill_blank" },
  { label: "数值题", value: "numeric" },
  { label: "公式题", value: "formula" },
  { label: "简答题", value: "short_answer" },
  { label: "计算题", value: "calculation" },
  { label: "作文", value: "essay" },
  { label: "论述题", value: "discussion" },
  { label: "编程题", value: "coding" }
];

const rubricStatusOptions = [
  { label: "草稿", value: "draft" },
  { label: "待审批", value: "pending_review" },
  { label: "已批准", value: "approved" },
  { label: "锁定", value: "locked" }
];

const paperStatusMeta: Record<string, { label: string; tone: StatusTone }> = {
  registered: { label: "已登记", tone: "success" },
  processing: { label: "处理中", tone: "processing" },
  failed: { label: "处理失败", tone: "danger" }
};

const toleranceQuestionTypes = ["numeric", "formula", "calculation"];

const formulaEvidenceQuestionTypes = new Set(["formula", "calculation", "numeric"]);

const formulaEvidenceOptions: { label: string; value: RubricEvidenceRequirement["type"] }[] = [
  { label: "关键等价变形成立", value: "valid_transformation" },
  { label: "最终结果经核验", value: "final_result" },
  { label: "出现指定概念", value: "concept" },
  { label: "单位正确", value: "unit" },
  { label: "定义域／取值条件", value: "domain" }
];

interface QuestionFormValues {
  exam_paper_id?: string;
  question_no: string;
  question_type: string;
  score: number;
  stem: string;
  knowledge_points: string[];
  answer_area_page: number;
  answer_area_x: number;
  answer_area_y: number;
  answer_area_w: number;
  answer_area_h: number;
  sort_order: number;
  standard_answer: string;
  equivalent_answers: string[];
  tolerance_absolute?: number | null;
  tolerance_relative?: number | null;
}

function labelFrom(options: { label: string; value: string }[], value?: string) {
  return options.find((item) => item.value === value)?.label ?? value ?? "-";
}

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    console.error("请求失败", error.status, error.code, error.message);
    return error.message || "操作失败，请稍后重试";
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return "操作失败，请稍后重试";
}

function rubricStatusLabel(status?: string) {
  if (!status) {
    return "未配置";
  }
  return rubricStatusOptions.find((item) => item.value === status)?.label ?? "未知状态";
}

function rubricTone(status?: string): StatusTone {
  if (status === "locked" || status === "approved") {
    return "success";
  }
  if (status === "pending_review") {
    return "warning";
  }
  return "neutral";
}

function jsonText(value: unknown, fallback: string) {
  if (value === undefined || value === null) {
    return fallback;
  }
  return JSON.stringify(value, null, 2);
}

function parseArrayJSON(value: string, label: string): unknown[] {
  try {
    const parsed = JSON.parse(value || "[]") as unknown;
    if (!Array.isArray(parsed)) {
      throw new Error(`${label}必须是 JSON 数组`);
    }
    return parsed;
  } catch (error) {
    if (error instanceof Error) {
      throw new Error(`${label}解析失败：${error.message}`);
    }
    throw new Error(`${label}解析失败`);
  }
}

function pointTotal(points: RubricPoint[]) {
  return points.reduce((sum, point) => sum + (Number(point.score) || 0), 0);
}

function formulaEvidenceEnabled(subject: string | undefined, questionType: string | undefined) {
  return isFormulaEvidenceSubject(subject) && formulaEvidenceQuestionTypes.has(questionType ?? "");
}

export function PaperRubricPage({
  canManage,
  canManageAssessment,
  initialExamId = "",
  onExamChanged
}: {
  canManage: boolean;
  canManageAssessment: boolean;
  initialExamId?: string;
  onExamChanged?: () => void;
}) {
  const { message, modal } = App.useApp();
  const [form] = Form.useForm<QuestionFormValues>();
  const watchedQuestionType = Form.useWatch("question_type", form);
  const [exams, setExams] = useState<Exam[]>([]);
  const [selectedExamId, setSelectedExamId] = useState(initialExamId);
  const [papers, setPapers] = useState<PaperVersion[]>([]);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [selectedQuestionId, setSelectedQuestionId] = useState<string | null>(null);
  const [editorMode, setEditorMode] = useState<"create" | "edit">("create");
  const [loading, setLoading] = useState(true);
  const [configLoading, setConfigLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [configError, setConfigError] = useState<string | null>(null);
  const configRequestRef = useRef(0);
  const [savingQuestion, setSavingQuestion] = useState(false);
  const [savingRubric, setSavingRubric] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [validation, setValidation] = useState<ValidationResult | null>(null);
  const [rubricStatus, setRubricStatus] = useState("draft");
  const [rubricPoints, setRubricPoints] = useState<RubricPoint[]>([]);
  const [deductionsJson, setDeductionsJson] = useState("");
  const [examplesJson, setExamplesJson] = useState("");
  const [scoringRules, setScoringRules] = useState<ScoringRule[]>([]);
  const [scoringRuleConfig, setScoringRuleConfig] = useState<Record<string, unknown>>({});
  const [savingScoringRule, setSavingScoringRule] = useState(false);

  const selectedExam = useMemo(() => exams.find((exam) => exam.id === selectedExamId), [exams, selectedExamId]);
  const selectedQuestion = useMemo(
    () => (editorMode === "edit" ? questions.find((question) => question.id === selectedQuestionId) ?? null : null),
    [editorMode, questions, selectedQuestionId]
  );
  const selectedLocked = selectedQuestion?.rubric?.status === "locked";
  const questionDisabled = !canManage || selectedLocked;
  const showTolerance = toleranceQuestionTypes.includes(watchedQuestionType ?? "");
  const showFormulaEvidence = formulaEvidenceEnabled(selectedExam?.subject, selectedQuestion?.question_type);
  const objectiveRuleType = selectedQuestion && ["single_choice", "true_false", "multiple_choice", "fill_blank", "numeric"].includes(selectedQuestion.question_type) ? selectedQuestion.question_type : "";
  const draftScoringRule = scoringRules.find((rule) => rule.status === "draft");
  const publishedScoringRule = scoringRules.find((rule) => rule.status === "published");

  const loadExams = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await listExams();
      setExams(result.exams);
      setSelectedExamId((current) => {
        if (initialExamId && result.exams.some((exam) => exam.id === initialExamId)) return initialExamId;
        return result.exams.some((exam) => exam.id === current) ? current : result.exams[0]?.id || "";
      });
    } catch (currentError) {
      setError(formatError(currentError));
    } finally {
      setLoading(false);
    }
  }, [initialExamId]);

  const loadConfig = useCallback(async (examId: string) => {
    const requestId = ++configRequestRef.current;
    if (!examId) {
      setPapers([]);
      setQuestions([]);
      setConfigLoading(false);
      return;
    }
    setConfigLoading(true);
    setConfigError(null);
    try {
      const [paperResult, questionResult] = await Promise.all([listPapers(examId), listQuestions(examId)]);
      if (requestId !== configRequestRef.current) return;
      setPapers(paperResult.papers);
      setQuestions(questionResult.questions);
      setValidation(null);
      setEditorMode((current) => {
        if (questionResult.questions.length === 0) {
          return "create";
        }
        return current === "create" ? "create" : "edit";
      });
      setSelectedQuestionId((current) => {
        if (questionResult.questions.length === 0) {
          return null;
        }
        return current && questionResult.questions.some((question) => question.id === current) ? current : questionResult.questions[0].id;
      });
    } catch (currentError) {
      if (requestId !== configRequestRef.current) return;
      setConfigError(formatError(currentError));
    } finally {
      if (requestId === configRequestRef.current) setConfigLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadExams();
  }, [loadExams]);

  useEffect(() => {
    if (initialExamId) setSelectedExamId(initialExamId);
  }, [initialExamId]);

  useEffect(() => {
    void loadConfig(selectedExamId);
  }, [loadConfig, selectedExamId]);

  useEffect(() => {
    if (selectedQuestion) {
      const answerArea = selectedQuestion.answer_area ?? {};
      const rawTolerance = selectedQuestion.answer_key?.tolerance;
      const tolerance: Record<string, unknown> =
        rawTolerance && typeof rawTolerance === "object" && !Array.isArray(rawTolerance) ? (rawTolerance as Record<string, unknown>) : {};
      form.setFieldsValue({
        exam_paper_id: selectedQuestion.exam_paper_id,
        question_no: selectedQuestion.question_no,
        question_type: selectedQuestion.question_type,
        score: selectedQuestion.score,
        stem: selectedQuestion.stem ?? "",
        knowledge_points: selectedQuestion.knowledge_points,
        answer_area_page: Number(answerArea.page ?? 1),
        answer_area_x: Number(answerArea.x ?? 0),
        answer_area_y: Number(answerArea.y ?? 0),
        answer_area_w: Number(answerArea.w ?? 0),
        answer_area_h: Number(answerArea.h ?? 0),
        sort_order: selectedQuestion.sort_order,
        standard_answer: String(selectedQuestion.answer_key?.standard_answer ?? ""),
        equivalent_answers: (selectedQuestion.answer_key?.equivalent_answers ?? []).map((item) => String(item)),
        tolerance_absolute: tolerance.absolute === undefined || tolerance.absolute === null ? undefined : Number(tolerance.absolute),
        tolerance_relative:
          tolerance.relative === undefined || tolerance.relative === null ? undefined : Number((Number(tolerance.relative) * 100).toFixed(6))
      });
      setRubricStatus(selectedQuestion.rubric?.status ?? "draft");
      setRubricPoints(selectedQuestion.rubric?.points ?? []);
      setDeductionsJson(jsonText(selectedQuestion.rubric?.deductions, ""));
      setExamplesJson(jsonText(selectedQuestion.rubric?.examples, ""));
      return;
    }
    form.setFieldsValue({
      exam_paper_id: papers[0]?.id,
      question_no: "",
      question_type: "short_answer",
      score: 10,
      stem: "",
      knowledge_points: [],
      answer_area_page: 1,
      answer_area_x: 0,
      answer_area_y: 0,
      answer_area_w: 0,
      answer_area_h: 0,
      sort_order: questions.length + 1,
      standard_answer: "",
      equivalent_answers: [],
      tolerance_absolute: undefined,
      tolerance_relative: undefined
    });
    setRubricStatus("draft");
    setRubricPoints([{ id: "p1", description: "", score: 10, required: true }]);
    setDeductionsJson("");
    setExamplesJson("");
  }, [form, papers, questions.length, selectedQuestion]);

  useEffect(() => {
    let active = true;
    async function loadRules() {
      if (!selectedQuestion || !objectiveRuleType) {
        setScoringRules([]);
        setScoringRuleConfig({});
        return;
      }
      try {
        const result = await listScoringRules(selectedQuestion.id);
        if (!active) return;
        setScoringRules(result.scoring_rules);
        const editable = result.scoring_rules.find((rule) => rule.status === "draft") ?? result.scoring_rules.find((rule) => rule.status === "published");
        setScoringRuleConfig(editable?.config ?? {});
      } catch (currentError) {
        if (active) message.error(formatError(currentError));
      }
    }
    void loadRules();
    return () => { active = false; };
  }, [message, objectiveRuleType, selectedQuestion]);

  function setRuleConfig(key: string, value: unknown) {
    setScoringRuleConfig((current) => ({ ...current, [key]: value }));
  }

  async function saveScoringRule(publish: boolean) {
    if (!selectedQuestion || !objectiveRuleType) return;
    setSavingScoringRule(true);
    try {
      const saved = draftScoringRule
        ? (await updateScoringRule(draftScoringRule.id, scoringRuleConfig, draftScoringRule.revision)).scoring_rule
        : (await createScoringRule(selectedQuestion.id, objectiveRuleType, scoringRuleConfig)).scoring_rule;
      if (publish) {
        await publishScoringRule(saved.id);
        message.success("评分规则已发布并锁定版本");
      } else {
        message.success("评分规则草稿已保存");
      }
      const result = await listScoringRules(selectedQuestion.id);
      setScoringRules(result.scoring_rules);
      const editable = result.scoring_rules.find((rule) => rule.status === "draft") ?? result.scoring_rules.find((rule) => rule.status === "published");
      setScoringRuleConfig(editable?.config ?? {});
      onExamChanged?.();
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setSavingScoringRule(false);
    }
  }

  const paperColumns: TableColumnsType<PaperVersion> = [
    { title: "版本", dataIndex: "version_no", width: 72, render: (value: number) => `v${value}` },
    { title: "文件名", render: (_, paper) => paper.file.original_name || "未命名文件" },
    { title: "类型", render: (_, paper) => paper.file.content_type || "-" },
    { title: "大小", render: (_, paper) => (paper.file.size_bytes ? `${Math.round(paper.file.size_bytes / 1024)} KB` : "-") },
    {
      title: "状态",
      dataIndex: "status",
      width: 100,
      render: (value: string) => {
        const meta = paperStatusMeta[value] ?? { label: "未知状态", tone: "neutral" as StatusTone };
        return (
          <span title={value}>
            <StatusTag tone={meta.tone}>{meta.label}</StatusTag>
          </span>
        );
      }
    }
  ];

  const rubricColumns: TableColumnsType<RubricPoint> = [
    {
      title: "采分点",
      dataIndex: "description",
      render: (_, point, index) => (
        <Input
          value={point.description}
          disabled={questionDisabled}
          placeholder="采分点描述"
          onChange={(event) => updatePoint(index, { description: event.target.value })}
        />
      )
    },
    {
      title: "分值",
      dataIndex: "score",
      width: 120,
      render: (_, point, index) => (
        <InputNumber
          value={point.score}
          disabled={questionDisabled}
          min={0}
          precision={1}
          className="full-width-control"
          onChange={(value) => updatePoint(index, { score: Number(value ?? 0) })}
        />
      )
    },
    {
      title: "必需",
      dataIndex: "required",
      width: 90,
      render: (_, point, index) => (
        <Switch checked={point.required} disabled={questionDisabled} onChange={(checked) => updatePoint(index, { required: checked })} />
      )
    },
    ...(showFormulaEvidence
      ? [
          {
            title: "识别证据（仅供教师参考）",
            width: 380,
            render: (_: unknown, point: RubricPoint, index: number) => {
              const requirements = point.evidence_requirements ?? [];
              return (
                <Space direction="vertical" size={6} className="full-width-control">
                  {requirements.map((requirement, requirementIndex) => (
                    <Space key={`${requirement.type}-${requirementIndex}`} wrap size={6}>
                      <Select
                        value={requirement.type}
                        options={formulaEvidenceOptions}
                        disabled={questionDisabled}
                        className="rubric-evidence-type-select"
                        onChange={(type: RubricEvidenceRequirement["type"]) =>
                          updateEvidenceRequirement(index, requirementIndex, {
                            type,
                            target: type === "concept" || type === "unit" || type === "domain" ? requirement.target ?? "" : undefined,
                            minimum: undefined,
                            children: undefined
                          })
                        }
                      />
                      {requirement.type === "concept" || requirement.type === "unit" || requirement.type === "domain" ? (
                        <Input
                          value={requirement.target}
                          disabled={questionDisabled}
                          placeholder={requirement.type === "concept" ? "例如：配方法" : requirement.type === "unit" ? "例如：m/s" : "例如：x ≥ 0"}
                          className="rubric-evidence-target-input"
                          onChange={(event) => updateEvidenceRequirement(index, requirementIndex, { target: event.target.value })}
                        />
                      ) : null}
                      <Button
                        type="text"
                        danger
                        size="small"
                        aria-label="删除识别证据"
                        disabled={questionDisabled}
                        icon={<Trash2 size={14} />}
                        onClick={() => removeEvidenceRequirement(index, requirementIndex)}
                      />
                    </Space>
                  ))}
                  <Button
                    type="link"
                    size="small"
                    icon={<Plus size={14} />}
                    disabled={questionDisabled}
                    onClick={() => addEvidenceRequirement(index)}
                  >
                    添加识别证据
                  </Button>
                </Space>
              );
            }
          } satisfies TableColumnsType<RubricPoint>[number]
        ]
      : []),
    {
      title: "操作",
      width: 90,
      render: (_, __, index) => (
        <Button danger size="small" icon={<Trash2 size={14} />} disabled={questionDisabled || rubricPoints.length <= 1} onClick={() => removePoint(index)} />
      )
    }
  ];

  function updatePoint(index: number, patch: Partial<RubricPoint>) {
    setRubricPoints((current) => current.map((point, currentIndex) => (currentIndex === index ? { ...point, ...patch } : point)));
  }

  function addEvidenceRequirement(pointIndex: number) {
    updatePoint(pointIndex, {
      evidence_requirements: [
        ...(rubricPoints[pointIndex]?.evidence_requirements ?? []),
        { type: "valid_transformation" }
      ]
    });
  }

  function updateEvidenceRequirement(pointIndex: number, requirementIndex: number, patch: Partial<RubricEvidenceRequirement>) {
    const requirements = rubricPoints[pointIndex]?.evidence_requirements ?? [];
    updatePoint(pointIndex, {
      evidence_requirements: requirements.map((requirement, index) => (index === requirementIndex ? { ...requirement, ...patch } : requirement))
    });
  }

  function removeEvidenceRequirement(pointIndex: number, requirementIndex: number) {
    const requirements = rubricPoints[pointIndex]?.evidence_requirements ?? [];
    updatePoint(pointIndex, { evidence_requirements: requirements.filter((_, index) => index !== requirementIndex) });
  }

  function removePoint(index: number) {
    setRubricPoints((current) => current.filter((_, currentIndex) => currentIndex !== index));
  }

  function addPoint() {
    setRubricPoints((current) => [...current, { id: `p${current.length + 1}`, description: "", score: 0, required: false }]);
  }

  const uploadProps: UploadProps = {
    showUploadList: false,
    beforeUpload: (file) => {
      void handleUpload(file);
      return false;
    }
  };

  async function handleUpload(file: File) {
    if (!selectedExam) {
      message.error("请先在上方选择考试，再上传试卷文件");
      return;
    }
    setUploading(true);
    try {
      const upload = await uploadFile(file, { owner_type: "exam", owner_id: selectedExam.id, exam_id: selectedExam.id, school_id: selectedExam.school_id });
      await registerPaperFromFile(selectedExam.id, upload.file.id);
      message.success("试卷文件已上传并登记");
      await loadConfig(selectedExam.id);
      onExamChanged?.();
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setUploading(false);
    }
  }

  async function saveQuestion() {
    if (!selectedExam) {
      message.error("请先选择考试");
      return;
    }
    const values = await form.validateFields();
    const answerAreaBase = editorMode === "edit" ? selectedQuestion?.answer_area ?? {} : {};
    const answerArea: Record<string, unknown> = {
      ...answerAreaBase,
      page: values.answer_area_page,
      x: values.answer_area_x,
      y: values.answer_area_y,
      w: values.answer_area_w,
      h: values.answer_area_h
    };
    const rawTolerance = editorMode === "edit" ? selectedQuestion?.answer_key?.tolerance : undefined;
    const tolerance: Record<string, unknown> =
      rawTolerance && typeof rawTolerance === "object" && !Array.isArray(rawTolerance) ? { ...(rawTolerance as Record<string, unknown>) } : {};
    if (toleranceQuestionTypes.includes(values.question_type)) {
      if (values.tolerance_absolute === undefined || values.tolerance_absolute === null) {
        delete tolerance.absolute;
      } else {
        tolerance.absolute = values.tolerance_absolute;
      }
      if (values.tolerance_relative === undefined || values.tolerance_relative === null) {
        delete tolerance.relative;
      } else {
        tolerance.relative = Number((values.tolerance_relative / 100).toFixed(8));
      }
    }
    const payload: QuestionPayload = {
      exam_paper_id: values.exam_paper_id,
      question_no: values.question_no,
      question_type: values.question_type,
      score: values.score,
      stem: values.stem,
      knowledge_points: values.knowledge_points ?? [],
      answer_area: answerArea,
      sort_order: values.sort_order,
      answer_key: {
        standard_answer: values.standard_answer,
        equivalent_answers: values.equivalent_answers ?? [],
        tolerance
      }
    };
    setSavingQuestion(true);
    try {
      if (editorMode === "edit" && selectedQuestion) {
        await updateQuestion(selectedQuestion.id, payload);
        message.success("题目已更新，标准答案将产生新版本");
      } else {
        await createQuestion(selectedExam.id, payload);
        message.success("题目已创建");
      }
      await loadConfig(selectedExam.id);
      onExamChanged?.();
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setSavingQuestion(false);
    }
  }

  function confirmDeleteQuestion() {
    if (!selectedQuestion || !selectedExam) {
      return;
    }
    modal.confirm({
      title: "删除题目",
      content: `确认删除 ${selectedQuestion.question_no}？`,
      okText: "删除",
      okButtonProps: { danger: true },
      cancelText: "取消",
      onOk: async () => {
        await deleteQuestion(selectedQuestion.id);
        message.success("题目已删除");
        setEditorMode("create");
        setSelectedQuestionId(null);
        await loadConfig(selectedExam.id);
        onExamChanged?.();
      }
    });
  }

  async function saveRubric() {
    if (!selectedQuestion || !selectedExam) {
      message.error("请先选择已保存的题目");
      return;
    }
    const total = pointTotal(rubricPoints);
    if (Math.abs(total - selectedQuestion.score) > 0.0001) {
      message.error("评分细则采分点总分必须等于题目分值");
      return;
    }
    let deductions: unknown[];
    let examples: unknown[];
    try {
      deductions = parseArrayJSON(deductionsJson, "扣分点");
      examples = parseArrayJSON(examplesJson, "样例答案");
    } catch (currentError) {
      message.error(formatError(currentError));
      return;
    }
    setSavingRubric(true);
    try {
      await createRubric(selectedQuestion.id, {
        status: rubricStatus,
        max_score: selectedQuestion.score,
        points: rubricPoints,
        deductions,
        examples
      });
      message.success(rubricStatus === "locked" ? "评分细则已锁定" : "评分细则新版本已提交");
      await loadConfig(selectedExam.id);
      onExamChanged?.();
    } catch (currentError) {
      message.error(formatError(currentError));
    } finally {
      setSavingRubric(false);
    }
  }

  async function runValidation() {
    if (!selectedExam) {
      return;
    }
    try {
      const result = await validatePaperConfig(selectedExam.id);
      setValidation(result.result);
      if (result.result.valid) {
        message.success("试卷配置检查通过");
      } else {
        message.warning("试卷配置存在问题，请查看下方详情");
      }
    } catch (currentError) {
      message.error(formatError(currentError));
    }
  }

  const scoreMismatch = selectedQuestion ? Math.abs(pointTotal(rubricPoints) - selectedQuestion.score) > 0.0001 : false;
  const editorVisible = !loading && !error && Boolean(selectedExam) && !configLoading && !configError;

  return (
    <div className="page-stack">
      {!editorVisible ? <Form form={form} component={false} /> : null}
      <section className="page-heading">
        <div>
          <Space>
            <h1>试卷管理</h1>
          </Space>
          <p>上传试卷、配置题目、维护标准答案和评分细则。</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} onClick={() => void loadExams()} loading={loading}>
            刷新
          </Button>
          <Button icon={<CheckCircle2 size={16} />} disabled={!selectedExam} onClick={() => void runValidation()}>
            完整性检查
          </Button>
        </Space>
      </section>

      <section className="workspace-section filter-panel">
        <div className="paper-topline">
          <Select
            className="exam-picker"
            placeholder="选择考试"
            value={selectedExamId || undefined}
            options={exams.map((exam) => ({ label: exam.name, value: exam.id }))}
            onChange={(value) => setSelectedExamId(value)}
            loading={loading}
          />
          <Upload {...uploadProps}>
            <Button icon={<FileUp size={16} />} disabled={!canManage || !selectedExam} loading={uploading}>
              上传试卷文件
            </Button>
          </Upload>
          {selectedExam ? <span className="muted">当前考试总分：{selectedExam.total_score}</span> : null}
        </div>
      </section>

      {loading ? (
        <section className="workspace-section">
          <LoadingState label="正在读取考试列表" />
        </section>
      ) : error ? (
        <ErrorState message={error} onRetry={() => void loadExams()} />
      ) : !selectedExam ? (
        <section className="workspace-section">
          <EmptyState title="暂无考试" description="还没有可配置的考试。请先在「考试管理」中创建考试。" />
        </section>
      ) : configLoading ? (
        <section className="workspace-section">
          <LoadingState label="正在读取试卷与题目配置" />
        </section>
      ) : configError ? (
        <ErrorState message={configError} onRetry={() => void loadConfig(selectedExam.id)} />
      ) : (
        <>
          <section className="workspace-section">
            <div className="section-head">
              <div>
                <h2>试卷文件</h2>
                <p>{papers.length} 个版本</p>
              </div>
            </div>
            <ResponsiveTable<PaperVersion>
              rowKey="id"
              dataSource={papers}
              columns={paperColumns}
              size="middle"
              pagination={false}
              locale={{ emptyText: <EmptyState title="暂无试卷文件" description="上传试卷文件后，版本记录会显示在这里。" /> }}
            />
          </section>

          {validation ? (
            <Alert
              type={validation.valid ? "success" : "warning"}
              showIcon
              message={validation.valid ? "试卷配置完整" : "试卷配置存在问题"}
              description={
                validation.valid ? (
                  "检查完成，未发现配置问题。"
                ) : (
                  <ul className="validation-issue-list">
                    {validation.issues.map((issue, index) => (
                      <li key={`${issue.code}-${index}`} title={issue.code}>
                        {issue.message}
                      </li>
                    ))}
                  </ul>
                )
              }
            />
          ) : null}

          <section className="paper-workbench">
            <aside className="workspace-section question-list-panel">
              <div className="section-head">
                <div>
                  <h2>题目列表</h2>
                  <p>{questions.length} 道题</p>
                </div>
                <Button
                  size="small"
                  icon={<Plus size={14} />}
                  disabled={!canManage}
                  onClick={() => {
                    setEditorMode("create");
                    setSelectedQuestionId(null);
                  }}
                >
                  新建
                </Button>
              </div>
              <List
                className="question-list"
                dataSource={questions}
                locale={{ emptyText: <EmptyState title="暂无题目" description="先新建题目配置。" /> }}
                renderItem={(question) => (
                  <List.Item
                    className={selectedQuestionId === question.id ? "question-list-item active" : "question-list-item"}
                    onClick={() => {
                      setEditorMode("edit");
                      setSelectedQuestionId(question.id);
                    }}
                  >
                    <div>
                      <strong>{question.question_no}</strong>
                      <span>{labelFrom(questionTypeOptions, question.question_type)}</span>
                    </div>
                    <div>
                      <span>{question.score} 分</span>
                      <span title={question.rubric?.status}>
                        <StatusTag tone={rubricTone(question.rubric?.status)}>{rubricStatusLabel(question.rubric?.status)}</StatusTag>
                      </span>
                    </div>
                  </List.Item>
                )}
              />
            </aside>

            <section className="workspace-section question-editor">
              <div className="section-head">
                <div>
                  <h2>{editorMode === "edit" ? "题目配置" : "新建题目"}</h2>
                  <p>{selectedLocked ? "评分细则已锁定，题目和细则不可编辑。" : "保存题目修改会生成新的答案版本。"}</p>
                </div>
                <Space>
                  {selectedLocked ? <StatusTag tone="success">已锁定</StatusTag> : null}
                  {editorMode === "edit" ? (
                    <Button danger icon={<Trash2 size={16} />} disabled={questionDisabled} onClick={confirmDeleteQuestion}>
                      删除
                    </Button>
                  ) : null}
                  <Button type="primary" icon={<Save size={16} />} disabled={questionDisabled} loading={savingQuestion} onClick={() => void saveQuestion()}>
                    保存题目
                  </Button>
                </Space>
              </div>

              <Form form={form} layout="vertical" disabled={questionDisabled}>
                <div className="form-grid">
                  <Form.Item label="关联试卷版本" name="exam_paper_id">
                    <Select
                      allowClear
                      options={papers.map((paper) => ({ label: `v${paper.version_no} ${paper.file.original_name || "未命名文件"}`, value: paper.id }))}
                      placeholder="可不关联"
                    />
                  </Form.Item>
                  <Form.Item label="题号" name="question_no" rules={[{ required: true, message: "请输入题号" }]}>
                    <Input placeholder="Q1" />
                  </Form.Item>
                  <Form.Item label="题型" name="question_type" rules={[{ required: true, message: "请选择题型" }]}>
                    <Select options={questionTypeOptions} />
                  </Form.Item>
                  <Form.Item label="分值" name="score" rules={[{ required: true, message: "请输入分值" }]}>
                    <InputNumber min={0.5} precision={1} className="full-width-control" />
                  </Form.Item>
                  <Form.Item label="排序" name="sort_order" rules={[{ required: true, message: "请输入排序" }]}>
                    <InputNumber min={1} precision={0} className="full-width-control" />
                  </Form.Item>
                  <Form.Item label="知识点" name="knowledge_points">
                    <Select mode="tags" placeholder="输入后回车" />
                  </Form.Item>
                </div>
                <Form.Item label="题干" name="stem">
                  <Input.TextArea rows={3} />
                </Form.Item>
                <Form.Item label="答题区域位置" required>
                  <div className="form-grid">
                    <Form.Item label="所在页码" name="answer_area_page" rules={[{ required: true, message: "请输入所在页码" }]}>
                      <InputNumber min={1} precision={0} className="full-width-control" />
                    </Form.Item>
                    <Form.Item label="左边距" name="answer_area_x" rules={[{ required: true, message: "请输入左边距" }]}>
                      <InputNumber min={0} className="full-width-control" />
                    </Form.Item>
                    <Form.Item label="上边距" name="answer_area_y" rules={[{ required: true, message: "请输入上边距" }]}>
                      <InputNumber min={0} className="full-width-control" />
                    </Form.Item>
                    <Form.Item label="宽度" name="answer_area_w" rules={[{ required: true, message: "请输入宽度" }]}>
                      <InputNumber min={0} className="full-width-control" />
                    </Form.Item>
                    <Form.Item label="高度" name="answer_area_h" rules={[{ required: true, message: "请输入高度" }]}>
                      <InputNumber min={0} className="full-width-control" />
                    </Form.Item>
                  </div>
                  <p className="muted">也可以在「答题卡模板」中拖拽框选，更直观。</p>
                </Form.Item>
                <Form.Item label="标准答案" name="standard_answer" rules={[{ required: true, message: "请输入标准答案" }]}>
                  <Input.TextArea rows={3} />
                </Form.Item>
                <Form.Item label="等价答案" name="equivalent_answers">
                  <Select mode="tags" placeholder="输入后回车" />
                </Form.Item>
                {showTolerance ? (
                  <Form.Item label="答案误差范围（选填）">
                    <div className="form-grid">
                      <Form.Item label="允许绝对误差" name="tolerance_absolute">
                        <InputNumber min={0} className="full-width-control" />
                      </Form.Item>
                      <Form.Item label="允许相对误差（%）" name="tolerance_relative">
                        <InputNumber min={0} className="full-width-control" />
                      </Form.Item>
                    </div>
                  </Form.Item>
                ) : null}
              </Form>

              <AssessmentProfileEditor
                examId={selectedExam.id}
                examSubject={selectedExam.subject}
                examStatus={selectedExam.status}
                question={selectedQuestion}
                canManage={canManageAssessment}
                onChanged={onExamChanged}
              />

              {selectedQuestion && objectiveRuleType ? <section className="scoring-rule-editor">
                <div className="section-head">
                  <div>
                    <h2>客观题评分规则</h2>
                    <p>规则发布后不可修改；再次调整会创建新版本。</p>
                  </div>
                  <Space>
                    {publishedScoringRule ? <StatusTag tone="success">{`已发布 v${publishedScoringRule.version}`}</StatusTag> : <StatusTag tone="warning">未发布</StatusTag>}
                    <Button icon={<Save size={16} />} disabled={!canManage} loading={savingScoringRule} onClick={() => void saveScoringRule(false)}>保存草稿</Button>
                    <Button type="primary" icon={<LockKeyhole size={16} />} disabled={!canManage} loading={savingScoringRule} onClick={() => void saveScoringRule(true)}>发布规则</Button>
                  </Space>
                </div>
                {objectiveRuleType === "multiple_choice" ? <div className="scoring-rule-grid">
                  <label><span>允许少选得分</span><Switch checked={Boolean(scoringRuleConfig.allow_partial)} onChange={(value) => setRuleConfig("allow_partial", value)} /></label>
                  <label><span>每个正确选项分值</span><InputNumber min={0} max={selectedQuestion.score} precision={2} value={Number(scoringRuleConfig.score_per_correct_option ?? 0)} onChange={(value) => setRuleConfig("score_per_correct_option", Number(value ?? 0))} /></label>
                  <label><span>每个错误选项扣分</span><InputNumber min={0} max={selectedQuestion.score} precision={2} value={Number(scoringRuleConfig.wrong_option_penalty ?? selectedQuestion.score)} onChange={(value) => setRuleConfig("wrong_option_penalty", Number(value ?? 0))} /></label>
                  <label><span>最低得分</span><InputNumber min={0} max={selectedQuestion.score} precision={2} value={Number(scoringRuleConfig.minimum_score ?? 0)} onChange={(value) => setRuleConfig("minimum_score", Number(value ?? 0))} /></label>
                </div> : null}
                {objectiveRuleType === "fill_blank" ? <div className="scoring-rule-grid">
                  <label><span>忽略大小写</span><Switch checked={Boolean(scoringRuleConfig.ignore_case)} onChange={(value) => setRuleConfig("ignore_case", value)} /></label>
                  <label><span>忽略空格</span><Switch checked={Boolean(scoringRuleConfig.ignore_spaces)} onChange={(value) => setRuleConfig("ignore_spaces", value)} /></label>
                  <label><span>忽略标点</span><Switch checked={Boolean(scoringRuleConfig.ignore_punctuation)} onChange={(value) => setRuleConfig("ignore_punctuation", value)} /></label>
                </div> : null}
                {objectiveRuleType === "numeric" ? <div className="scoring-rule-grid">
                  <label><span>绝对误差</span><InputNumber min={0} precision={6} value={Number(scoringRuleConfig.absolute ?? 0)} onChange={(value) => setRuleConfig("absolute", Number(value ?? 0))} /></label>
                  <label><span>相对误差</span><InputNumber min={0} max={1} step={0.001} precision={6} value={Number(scoringRuleConfig.relative ?? 0)} onChange={(value) => setRuleConfig("relative", Number(value ?? 0))} /></label>
                  <label><span>单位必须填写</span><Switch checked={Boolean(scoringRuleConfig.unit_required)} onChange={(value) => setRuleConfig("unit_required", value)} /></label>
                </div> : null}
                {["single_choice", "true_false"].includes(objectiveRuleType) ? <Alert type="info" showIcon message="使用标准答案精确判定" description="空白、多涂、擦除或识别把握不足的答卷不会自动判零分，将转入人工确认。" /> : null}
              </section> : null}

              <div className="rubric-editor">
                <div className="section-head">
                  <div>
                    <h2>评分细则</h2>
                    <p>
                      {selectedQuestion ? `采分点合计 ${pointTotal(rubricPoints)} / ${selectedQuestion.score} 分` : "先保存题目，再配置评分细则"}
                    </p>
                  </div>
                  <Space>
                    <span className="muted">提交为</span>
                    <Select value={rubricStatus} options={rubricStatusOptions} disabled={questionDisabled} onChange={setRubricStatus} className="rubric-status-select" />
                    <Button icon={<Plus size={16} />} disabled={questionDisabled} onClick={addPoint}>
                      添加采分点
                    </Button>
                    <Button type="primary" icon={rubricStatus === "locked" ? <LockKeyhole size={16} /> : <Save size={16} />} disabled={questionDisabled || !selectedQuestion || scoreMismatch} loading={savingRubric} onClick={() => void saveRubric()}>
                      {rubricStatus === "locked" ? "锁定评分细则" : rubricStatus === "pending_review" ? "提交审批" : "保存评分细则"}
                    </Button>
                  </Space>
                </div>
                {scoreMismatch ? <Alert type="error" showIcon message="评分细则分值不匹配" description="采分点总分必须等于题目分值，调整后才能保存。" /> : null}
                {showFormulaEvidence ? (
                  <Alert
                    type="info"
                    showIcon
                    message="公式与步骤证据只作阅卷提示"
                    description="系统只会为数学、物理、化学的公式类题目生成这些证据；不会自动给分，最终分数仍由规则或教师确认。"
                  />
                ) : null}
                <ResponsiveTable<RubricPoint> rowKey="id" dataSource={rubricPoints} columns={rubricColumns} pagination={false} size="middle" />
                <div className="form-grid rubric-json-grid">
                  <label>
                    <span>扣分点（JSON 格式，选填）</span>
                    <Input.TextArea
                      value={deductionsJson}
                      disabled={questionDisabled}
                      rows={4}
                      spellCheck={false}
                      placeholder={'示例：[{"description": "缺少必要步骤", "score": -2}]'}
                      onChange={(event) => setDeductionsJson(event.target.value)}
                    />
                  </label>
                  <label>
                    <span>样例答案（JSON 格式，选填）</span>
                    <Input.TextArea
                      value={examplesJson}
                      disabled={questionDisabled}
                      rows={4}
                      spellCheck={false}
                      placeholder={'示例：["满分示例答案", "部分得分示例答案"]'}
                      onChange={(event) => setExamplesJson(event.target.value)}
                    />
                  </label>
                </div>
              </div>
            </section>
          </section>
        </>
      )}
    </div>
  );
}
