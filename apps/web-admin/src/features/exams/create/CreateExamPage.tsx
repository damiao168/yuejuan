import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, App } from "antd";
import { createExam } from "../../../api/exams";
import { listClasses, listGrades, listSchools, listStudents, type Grade, type School, type SchoolClass, type Student } from "../../../api/org";
import type { SessionUser } from "../../../auth/session";
import { ErrorState, LoadingState } from "../../../components/PageState";
import { initialCreateExamDraft } from "./createExamDraft";
import { CreateExamWizard } from "./CreateExamWizard";
import type { CreateExamDraft } from "./types";

async function listAllStudents() {
  const students: Student[] = [];
  let cursor = "";
  for (let page = 0; page < 50; page += 1) {
    const result = await listStudents({ limit: 200, cursor: cursor || undefined });
    students.push(...result.students);
    if (!result.has_more || !result.next_cursor) break;
    cursor = result.next_cursor;
  }
  return students;
}

function validationMessage(step: number, draft: CreateExamDraft) {
  if (step === 0 && !draft.schoolId) return "请选择考试所属学校";
  if (step === 0 && !draft.name.trim()) return "考试名称不能为空";
  if (step === 0 && !draft.subject) return "请选择考试科目";
  if (step === 0 && draft.totalScore <= 0) return "满分必须大于 0";
  if (step === 1 && !draft.gradeId) return "请选择考试年级";
  if (step === 1 && draft.classIds.length === 0) return "请至少选择一个参考班级";
  if (step === 2 && !draft.gradingMode) return "请选择阅卷方式";
  return "";
}

export function CreateExamPage({ user, onNavigate }: { user: SessionUser; onNavigate: (path: string) => void }) {
  const { message } = App.useApp();
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<CreateExamDraft>(() => initialCreateExamDraft());
  const [schools, setSchools] = useState<School[]>([]);
  const [grades, setGrades] = useState<Grade[]>([]);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [students, setStudents] = useState<Student[]>([]);
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
      const school = activeSchools.find((item) => item.name === user.school) ?? (activeSchools.length === 1 ? activeSchools[0] : undefined);
      const [gradeResult, classResult, studentResult] = await Promise.all([listGrades(), listClasses(), listAllStudents()]);
      const activeGrades = gradeResult.grades.filter((grade) => grade.status === "active" && activeSchools.some((item) => item.id === grade.school_id));
      const defaultGrade = school ? activeGrades.find((grade) => grade.school_id === school.id) : undefined;
      setSchools(activeSchools);
      setGrades(activeGrades);
      setClasses(classResult.classes.filter((item) => activeSchools.some((schoolItem) => schoolItem.id === item.school_id)));
      setStudents(studentResult.filter((item) => activeSchools.some((schoolItem) => schoolItem.id === item.school_id) && item.status === "active"));
      setDraft(initialCreateExamDraft(school?.id ?? "", defaultGrade));
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "考试创建信息加载失败");
    } finally {
      setLoading(false);
    }
  }, [user.school]);

  useEffect(() => { void load(); }, [load]);

  const visibleGrades = useMemo(() => grades.filter((grade) => grade.school_id === draft.schoolId), [draft.schoolId, grades]);
  const visibleClasses = useMemo(() => classes.filter((item) => item.school_id === draft.schoolId), [classes, draft.schoolId]);

  const studentCountByClass = useMemo(() => {
    const counts = new Map<string, number>();
    students.forEach((student) => counts.set(student.class_id, (counts.get(student.class_id) ?? 0) + 1));
    return counts;
  }, [students]);

  const changeStep = (next: number) => {
    if (next > step) {
      const invalid = validationMessage(step, draft);
      if (invalid) { message.warning(invalid); return; }
    }
    setStep(Math.max(0, Math.min(3, next)));
  };

  const submit = async () => {
    const invalid = validationMessage(0, draft) || validationMessage(1, draft) || validationMessage(2, draft);
    if (invalid) { message.warning(invalid); return; }
    setSubmitting(true);
    try {
      const result = await createExam({
        school_id: draft.schoolId,
        name: draft.name.trim(),
        subject: draft.subject,
        exam_type: draft.examType,
        total_score: draft.totalScore,
        grading_mode: draft.gradingMode,
        appeal_enabled: draft.appealEnabled,
        publish_policy: draft.publishPolicy,
        class_ids: draft.classIds
      });
      message.success("考试已创建，接下来完成开考检查");
      onNavigate(`/exams/${encodeURIComponent(result.exam.id)}/settings`);
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
      <CreateExamWizard step={step} draft={draft} schools={schools} grades={visibleGrades} classes={visibleClasses} studentCountByClass={studentCountByClass} submitting={submitting} onChange={(patch) => setDraft((current) => ({ ...current, ...patch }))} onStepChange={changeStep} onSubmit={() => void submit()} onCancel={() => onNavigate("/exams")} />
    </div>
  );
}
