import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App } from "antd";
import { createExamSession } from "../../../api/exams";
import { listExamTemplates, type ExamTemplate } from "../../../api/examTemplates";
import { listClasses, listGrades, listSchools, type Grade, type School, type SchoolClass } from "../../../api/org";
import type { SessionUser } from "../../../auth/session";
import { ErrorState, LoadingState } from "../../../components/PageState";
import { initialCreateExamDraft, subjectScore } from "./createExamDraft";
import { CreateExamWizard } from "./CreateExamWizard";
import type { CreateExamDraft } from "./types";

function validationMessage(step: number, draft: CreateExamDraft) {
  if (step === 0 && !draft.schoolId) return "请选择考试所属学校";
  if (step === 0 && !draft.gradeId) return "请选择考试年级";
  if (step === 0 && !draft.name.trim()) return "考试名称不能为空";
  if (step === 0 && !draft.examType) return "请选择考试类型";
  if (step === 0 && draft.classIds.length === 0) return "请至少选择一个参考班级";
  if (step === 1 && !draft.templateId) return "请选择考试方案或完全自定义";
  if (step === 1 && draft.subjects.length === 0) return "请至少选择一个考试科目";
  if (step === 2) {
    for (const subject of draft.subjects) {
      if (subject.totalScore <= 0 || subject.durationMinutes <= 0 || !subject.sections.length) return "请完整设置每个科目的满分、时长和试卷分区";
      if (subject.candidateRule === "subject_selected_classes" && !subject.classIds.length) return "单独指定范围的科目必须选择参考班级";
      if (Math.abs(subjectScore(subject) - subject.totalScore) > .001) return "每个科目的题目分值合计必须等于科目满分";
    }
  }
  if (step === 3 && !draft.gradingMode) return "请选择阅卷方式";
  return "";
}

export function CreateExamPage({ user, onNavigate }: { user: SessionUser; onNavigate: (path: string) => void }) {
  const { message } = App.useApp();
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<CreateExamDraft>(() => initialCreateExamDraft());
  const [schools, setSchools] = useState<School[]>([]);
  const [grades, setGrades] = useState<Grade[]>([]);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [templates, setTemplates] = useState<ExamTemplate[]>([]);
  const [savedAt, setSavedAt] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const schoolResult = await listSchools();
      const activeSchools = schoolResult.schools.filter((school) => school.status === "active");
      if (!activeSchools.length) throw new Error("当前账号范围内没有可用学校，请先完成组织设置");
      const organizationScope = user.organizationScope;
      const scopedSchools = organizationScope.resolved && !organizationScope.tenantWide
        ? activeSchools.filter((item) => organizationScope.schoolIds.includes(item.id))
        : activeSchools;
      if (!scopedSchools.length) throw new Error("当前身份尚未分配可管理的学校，请联系学校管理员");
      const school = scopedSchools.find((item) => item.name === user.school) ?? (scopedSchools.length === 1 ? scopedSchools[0] : undefined);
      const [gradeResult, classResult, templateResult] = await Promise.all([listGrades(), listClasses(), listExamTemplates()]);
      const activeGrades = gradeResult.grades.filter((grade) => grade.status === "active" && scopedSchools.some((item) => item.id === grade.school_id));
      const scopedGrades = organizationScope.resolved && !organizationScope.tenantWide && organizationScope.gradeIds.length
        ? activeGrades.filter((grade) => organizationScope.gradeIds.includes(grade.id))
        : activeGrades;
      const defaultGrade = school ? scopedGrades.find((grade) => grade.school_id === school.id) : scopedGrades[0];
      const activeClasses = classResult.classes.filter((item) => scopedGrades.some((grade) => grade.id === item.grade_id));
      const scopedClasses = organizationScope.resolved && !organizationScope.tenantWide && !organizationScope.gradeIds.length && organizationScope.classIds.length
        ? activeClasses.filter((item) => organizationScope.classIds.includes(item.id))
        : activeClasses;
      setSchools(scopedSchools);
      setGrades(scopedGrades);
      setClasses(scopedClasses);
      setTemplates(templateResult.exam_templates);
      const fallback = initialCreateExamDraft(school?.id ?? "", defaultGrade);
      try {
        const stored = localStorage.getItem(`exam-create-draft:${user.tenant}:${user.id}`);
        const restored = stored ? JSON.parse(stored) as CreateExamDraft : null;
        setDraft(restored && scopedSchools.some((item) => item.id === restored.schoolId) && scopedGrades.some((item) => item.id === restored.gradeId)
          ? { ...fallback, ...restored, templateId: restored.templateId ?? "" }
          : fallback);
      } catch {
        setDraft(fallback);
      }
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "考试创建信息加载失败");
    } finally {
      setLoading(false);
    }
  }, [user.id, user.school, user.tenant]);

  useEffect(() => { void load(); }, [load]);

  const visibleGrades = useMemo(() => grades.filter((grade) => grade.school_id === draft.schoolId), [draft.schoolId, grades]);
  const visibleClasses = useMemo(() => classes.filter((item) => item.school_id === draft.schoolId), [classes, draft.schoolId]);

  useEffect(() => {
    if (loading) return;
    const timer = window.setTimeout(() => {
      localStorage.setItem(`exam-create-draft:${user.tenant}:${user.id}`, JSON.stringify(draft));
      setSavedAt(new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }));
    }, 450);
    return () => window.clearTimeout(timer);
  }, [draft, loading, user.id, user.tenant]);

  const changeStep = (next: number) => {
    if (next > step) {
      const invalid = validationMessage(step, draft);
      if (invalid) { message.warning(invalid); return; }
    }
    setStep(Math.max(0, Math.min(3, next)));
  };

  const submit = async () => {
    const invalid = validationMessage(0, draft) || validationMessage(1, draft) || validationMessage(2, draft) || validationMessage(3, draft);
    if (invalid) { message.warning(invalid); return; }
    setSubmitting(true);
    try {
      const result = await createExamSession({
        school_id: draft.schoolId,
        grade_id: draft.gradeId,
        template_id: draft.templateId === "custom" ? undefined : draft.templateId,
        name: draft.name.trim(),
        exam_type: draft.examType,
        grading_mode: draft.gradingMode,
        appeal_enabled: draft.appealEnabled,
        publish_policy: draft.publishPolicy,
        class_ids: draft.classIds,
        subjects: draft.subjects.map((subject) => ({ subject: subject.subject, total_score: subject.totalScore, duration_minutes: subject.durationMinutes, candidate_rule: subject.candidateRule, class_ids: subject.candidateRule === "subject_selected_classes" ? subject.classIds : [], sections: subject.sections.map((section) => ({ title: section.title.trim(), question_type: section.questionType, question_count: section.questionCount, score_per_question: section.scorePerQuestion })) }))
      });
      localStorage.removeItem(`exam-create-draft:${user.tenant}:${user.id}`);
      const firstExam = result.exam_session.exams[0];
      message.success(`已创建 ${result.exam_session.exams.length} 个科目工作区`);
      onNavigate(firstExam ? `/exams/${encodeURIComponent(firstExam.id)}/settings` : "/exams");
    } catch (submitError) {
      message.error(submitError instanceof Error ? submitError.message : "考试创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return <LoadingState label="正在准备考试创建流程" />;
  if (error) return <ErrorState message={error} onRetry={() => void load()} />;
  return (
    <div className="create-exam-page">
      {!grades.length ? <Alert type="warning" showIcon message="当前学校还没有可用年级" description="请先到成员管理建立年级和班级。" /> : null}
      <CreateExamWizard step={step} draft={draft} schools={schools} grades={visibleGrades} classes={visibleClasses} templates={templates} scopeLocked={user.organizationScope.resolved && !user.organizationScope.tenantWide} savedAt={savedAt} submitting={submitting} onChange={(patch) => setDraft((current) => ({ ...current, ...patch }))} onStepChange={changeStep} onSubmit={() => void submit()} onCancel={() => onNavigate("/exams")} />
    </div>
  );
}
