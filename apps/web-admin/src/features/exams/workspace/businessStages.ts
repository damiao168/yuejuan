import type { ExamWorkspaceProjection } from "../../../api/workspace";

export type ExamBusinessStage = "preparation" | "capture" | "grading" | "results";

const stageOrder: ExamBusinessStage[] = ["preparation", "capture", "grading", "results"];

export const sectionBusinessStage: Record<string, ExamBusinessStage> = {
  overview: "preparation",
  students: "preparation",
  paper: "preparation",
  questions: "preparation",
  template: "preparation",
  settings: "preparation",
  capture: "capture",
  processing: "capture",
  grading: "grading",
  quality: "grading",
  scores: "results",
  appeals: "results",
  reports: "results"
};

function lifecycleBusinessStage(stage: string): ExamBusinessStage {
  if (stage === "capture") return "capture";
  if (stage === "grading" || stage === "quality") return "grading";
  if (stage === "results") return "results";
  return "preparation";
}

export function examBusinessStages(data: ExamWorkspaceProjection) {
  const current = lifecycleBusinessStage(data.stage);
  const currentIndex = stageOrder.indexOf(current);
  const progress = new Map(data.stage_progress.map((item) => [item.stage, item.summary]));
  return [
    { key: "preparation", label: "考试准备", action_route: `/exams/${encodeURIComponent(data.exam_id)}/settings`, summary: progress.get("prepare") },
    { key: "capture", label: "答卷导入", action_route: `/exams/${encodeURIComponent(data.exam_id)}/capture`, summary: progress.get("capture") },
    { key: "grading", label: "阅卷", action_route: `/exams/${encodeURIComponent(data.exam_id)}/grading`, summary: data.stage === "quality" ? progress.get("quality") : progress.get("grading") },
    { key: "results", label: "成绩", action_route: `/exams/${encodeURIComponent(data.exam_id)}/scores`, summary: progress.get("results") }
  ].map((stage, index) => ({ ...stage, state: index < currentIndex ? "completed" : index === currentIndex ? "current" : "pending" }));
}

export function currentExamBusinessStage(data: ExamWorkspaceProjection) {
  return examBusinessStages(data).find((stage) => stage.state === "current");
}
