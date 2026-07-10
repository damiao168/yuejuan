import { useCallback, useEffect, useMemo, useState } from "react";
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
  Table,
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
  deleteQuestion,
  listPapers,
  listQuestions,
  registerPaperFromFile,
  updateQuestion,
  uploadFile,
  validatePaperConfig,
  type PaperVersion,
  type Question,
  type QuestionPayload,
  type RubricPoint,
  type ValidationResult
} from "../api/papers";
import { EmptyState, ErrorState, LoadingState } from "../components/PageState";
import { StatusTag } from "../components/StatusTag";
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
  { label: "提交审批", value: "pending_review" },
  { label: "已批准", value: "approved" },
  { label: "锁定", value: "locked" }
];

interface QuestionFormValues {
  exam_paper_id?: string;
  question_no: string;
  question_type: string;
  score: number;
  stem: string;
  knowledge_points: string[];
  answer_area_json: string;
  sort_order: number;
  standard_answer: string;
  equivalent_answers: string[];
  tolerance_json: string;
}

function labelFrom(options: { label: string; value: string }[], value?: string) {
  return options.find((item) => item.value === value)?.label ?? value ?? "-";
}

function formatError(error: unknown) {
  if (error instanceof ApiClientError) {
    return `${error.status} ${error.code}: ${error.message}`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "未知错误";
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

function parseObjectJSON(value: string, label: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(value || "{}") as unknown;
    if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
      throw new Error(`${label}必须是 JSON 对象`);
    }
    return parsed as Record<string, unknown>;
  } catch (error) {
    if (error instanceof Error) {
      throw new Error(`${label}解析失败：${error.message}`);
    }
    throw new Error(`${label}解析失败`);
  }
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

export function PaperRubricPage({ canManage }: { canManage: boolean }) {
  const { message, modal } = App.useApp();
  const [form] = Form.useForm<QuestionFormValues>();
  const hasSession = true;
  const canWrite = canManage && hasSession;
  const [exams, setExams] = useState<Exam[]>([]);
  const [selectedExamId, setSelectedExamId] = useState("");
  const [papers, setPapers] = useState<PaperVersion[]>([]);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [selectedQuestionId, setSelectedQuestionId] = useState<string | null>(null);
  const [editorMode, setEditorMode] = useState<"create" | "edit">("create");
  const [loading, setLoading] = useState(true);
  const [configLoading, setConfigLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [configError, setConfigError] = useState<string | null>(null);
  const [savingQuestion, setSavingQuestion] = useState(false);
  const [savingRubric, setSavingRubric] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [validation, setValidation] = useState<ValidationResult | null>(null);
  const [rubricStatus, setRubricStatus] = useState("draft");
  const [rubricPoints, setRubricPoints] = useState<RubricPoint[]>([]);
  const [deductionsJson, setDeductionsJson] = useState("[]");
  const [examplesJson, setExamplesJson] = useState("[]");

  const selectedExam = useMemo(() => exams.find((exam) => exam.id === selectedExamId), [exams, selectedExamId]);
  const selectedQuestion = useMemo(
    () => (editorMode === "edit" ? questions.find((question) => question.id === selectedQuestionId) ?? null : null),
    [editorMode, questions, selectedQuestionId]
  );
  const selectedLocked = selectedQuestion?.rubric?.status === "locked";
  const questionDisabled = !canWrite || selectedLocked;

  const loadExams = useCallback(async () => {
    setLoading(true);
    setError(null);
    if (!hasSession) {
      setExams([]);
      setSelectedExamId("");
      setError("当前没有有效登录会话，无法调用真实后端 API。");
      setLoading(false);
      return;
    }
    try {
      const result = await listExams();
      setExams(result.exams);
      setSelectedExamId((current) => current || result.exams[0]?.id || "");
    } catch (currentError) {
      setError(formatError(currentError));
    } finally {
      setLoading(false);
    }
  }, [hasSession]);

  const loadConfig = useCallback(async (examId: string) => {
    if (!examId || !hasSession) {
      setPapers([]);
      setQuestions([]);
      return;
    }
    setConfigLoading(true);
    setConfigError(null);
    try {
      const [paperResult, questionResult] = await Promise.all([listPapers(examId), listQuestions(examId)]);
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
      setConfigError(formatError(currentError));
    } finally {
      setConfigLoading(false);
    }
  }, [hasSession]);

  useEffect(() => {
    void loadExams();
  }, [loadExams]);

  useEffect(() => {
    void loadConfig(selectedExamId);
  }, [loadConfig, selectedExamId]);

  useEffect(() => {
    if (selectedQuestion) {
      form.setFieldsValue({
        exam_paper_id: selectedQuestion.exam_paper_id,
        question_no: selectedQuestion.question_no,
        question_type: selectedQuestion.question_type,
        score: selectedQuestion.score,
        stem: selectedQuestion.stem ?? "",
        knowledge_points: selectedQuestion.knowledge_points,
        answer_area_json: jsonText(selectedQuestion.answer_area, "{\n  \"page\": 1,\n  \"x\": 0,\n  \"y\": 0,\n  \"w\": 0,\n  \"h\": 0\n}"),
        sort_order: selectedQuestion.sort_order,
        standard_answer: String(selectedQuestion.answer_key?.standard_answer ?? ""),
        equivalent_answers: (selectedQuestion.answer_key?.equivalent_answers ?? []).map((item) => String(item)),
        tolerance_json: jsonText(selectedQuestion.answer_key?.tolerance, "{}")
      });
      setRubricStatus(selectedQuestion.rubric?.status ?? "draft");
      setRubricPoints(selectedQuestion.rubric?.points ?? []);
      setDeductionsJson(jsonText(selectedQuestion.rubric?.deductions, "[]"));
      setExamplesJson(jsonText(selectedQuestion.rubric?.examples, "[]"));
      return;
    }
    form.setFieldsValue({
      exam_paper_id: papers[0]?.id,
      question_no: "",
      question_type: "short_answer",
      score: 10,
      stem: "",
      knowledge_points: [],
      answer_area_json: "{\n  \"page\": 1,\n  \"x\": 0,\n  \"y\": 0,\n  \"w\": 0,\n  \"h\": 0\n}",
      sort_order: questions.length + 1,
      standard_answer: "",
      equivalent_answers: [],
      tolerance_json: "{}"
    });
    setRubricStatus("draft");
    setRubricPoints([{ id: "p1", description: "", score: 10, required: true }]);
    setDeductionsJson("[]");
    setExamplesJson("[]");
  }, [form, papers, questions.length, selectedQuestion]);

  const paperColumns: TableColumnsType<PaperVersion> = [
    { title: "版本", dataIndex: "version_no", width: 72, render: (value: number) => `v${value}` },
    { title: "文件名", render: (_, paper) => paper.file.original_name || paper.file_asset_id },
    { title: "类型", render: (_, paper) => paper.file.content_type || "-" },
    { title: "大小", render: (_, paper) => (paper.file.size_bytes ? `${Math.round(paper.file.size_bytes / 1024)} KB` : "-") },
    { title: "状态", dataIndex: "status", width: 100, render: (value: string) => <StatusTag tone="processing">{value}</StatusTag> }
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
    if (!selectedExam || !hasSession) {
      message.error("请先选择考试并配置真实后端访问令牌");
      return;
    }
    setUploading(true);
    try {
      const upload = await uploadFile(file, { owner_type: "exam", owner_id: selectedExam.id, exam_id: selectedExam.id, school_id: selectedExam.school_id });
      await registerPaperFromFile(selectedExam.id, upload.file.id);
      message.success("试卷文件已上传并登记");
      await loadConfig(selectedExam.id);
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
    let answerArea: Record<string, unknown>;
    let tolerance: Record<string, unknown>;
    try {
      answerArea = parseObjectJSON(values.answer_area_json, "答题区域 JSON");
      tolerance = parseObjectJSON(values.tolerance_json, "容差 JSON");
    } catch (currentError) {
      message.error(formatError(currentError));
      return;
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
      message.error("Rubric 采分点总分必须等于题目分值");
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
      message.success(rubricStatus === "locked" ? "Rubric 已锁定" : "Rubric 新版本已提交");
      await loadConfig(selectedExam.id);
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
      message.success(result.result.valid ? "试卷配置校验通过" : "试卷配置存在问题");
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
            <StatusTag tone="success">真实 API</StatusTag>
          </Space>
          <p>上传试卷、配置题目、维护标准答案和 Rubric。</p>
        </div>
        <Space wrap>
          <Button icon={<RefreshCw size={16} />} onClick={() => void loadExams()} loading={loading}>
            刷新
          </Button>
          <Button icon={<CheckCircle2 size={16} />} disabled={!selectedExam || !hasSession} onClick={() => void runValidation()}>
            完整性检查
          </Button>
        </Space>
      </section>

      {!hasSession ? (
        <Alert
          type="warning"
          showIcon
          message="未检测到真实后端访问令牌"
            description="维护试卷文件、题目结构、标准答案和评分细则。"
        />
      ) : null}

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
            <Button icon={<FileUp size={16} />} disabled={!canWrite || !selectedExam} loading={uploading}>
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
          <EmptyState title="暂无考试" description="后端没有返回可配置试卷的考试。" />
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
            <Table<PaperVersion>
              rowKey="id"
              dataSource={papers}
              columns={paperColumns}
              size="middle"
              pagination={false}
              scroll={{ x: "max-content" }}
              locale={{ emptyText: <EmptyState title="暂无试卷文件" description="上传后会显示真实试卷版本。" /> }}
            />
          </section>

          {validation ? (
            <Alert
              type={validation.valid ? "success" : "warning"}
              showIcon
              message={validation.valid ? "试卷配置完整" : "试卷配置存在问题"}
              description={
                validation.valid ? "后端校验未发现配置问题。" : validation.issues.map((issue) => `${issue.code}: ${issue.message}`).join("；")
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
                  disabled={!canWrite}
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
                      <StatusTag tone={rubricTone(question.rubric?.status)}>{question.rubric?.status ?? "无 Rubric"}</StatusTag>
                    </div>
                  </List.Item>
                )}
              />
            </aside>

            <section className="workspace-section question-editor">
              <div className="section-head">
                <div>
                  <h2>{editorMode === "edit" ? "题目配置" : "新建题目"}</h2>
                  <p>{selectedLocked ? "Rubric 已锁定，题目和 Rubric 不可编辑。" : "保存题目修改会生成新的答案版本。"}</p>
                </div>
                <Space>
                  {selectedLocked ? <StatusTag tone="success">locked</StatusTag> : null}
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
                      options={papers.map((paper) => ({ label: `v${paper.version_no} ${paper.file.original_name || paper.file_asset_id}`, value: paper.id }))}
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
                <Form.Item label="答题区域 JSON" name="answer_area_json" rules={[{ required: true, message: "请输入答题区域 JSON" }]}>
                  <Input.TextArea rows={5} spellCheck={false} />
                </Form.Item>
                <Form.Item label="标准答案" name="standard_answer" rules={[{ required: true, message: "请输入标准答案" }]}>
                  <Input.TextArea rows={3} />
                </Form.Item>
                <Form.Item label="等价答案" name="equivalent_answers">
                  <Select mode="tags" placeholder="输入后回车" />
                </Form.Item>
                <Form.Item label="容差 JSON" name="tolerance_json">
                  <Input.TextArea rows={3} spellCheck={false} />
                </Form.Item>
              </Form>

              <div className="rubric-editor">
                <div className="section-head">
                  <div>
                    <h2>Rubric</h2>
                    <p>
                      采分点合计 {pointTotal(rubricPoints)} / {selectedQuestion?.score ?? "未保存题目"} 分
                    </p>
                  </div>
                  <Space>
                    <Select value={rubricStatus} options={rubricStatusOptions} disabled={questionDisabled} onChange={setRubricStatus} className="rubric-status-select" />
                    <Button icon={<Plus size={16} />} disabled={questionDisabled} onClick={addPoint}>
                      采分点
                    </Button>
                    <Button type="primary" icon={rubricStatus === "locked" ? <LockKeyhole size={16} /> : <Save size={16} />} disabled={questionDisabled || !selectedQuestion || scoreMismatch} loading={savingRubric} onClick={() => void saveRubric()}>
                      提交 Rubric
                    </Button>
                  </Space>
                </div>
                {scoreMismatch ? <Alert type="error" showIcon message="Rubric 分值不匹配" description="采分点总分必须等于题目分值，当前不会提交后端。" /> : null}
                <Table<RubricPoint> rowKey="id" dataSource={rubricPoints} columns={rubricColumns} pagination={false} scroll={{ x: "max-content" }} size="middle" />
                <div className="form-grid rubric-json-grid">
                  <label>
                    <span>扣分点 JSON</span>
                    <Input.TextArea value={deductionsJson} disabled={questionDisabled} rows={4} spellCheck={false} onChange={(event) => setDeductionsJson(event.target.value)} />
                  </label>
                  <label>
                    <span>样例答案 JSON</span>
                    <Input.TextArea value={examplesJson} disabled={questionDisabled} rows={4} spellCheck={false} onChange={(event) => setExamplesJson(event.target.value)} />
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
