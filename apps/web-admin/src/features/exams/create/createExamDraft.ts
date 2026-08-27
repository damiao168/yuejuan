import type { Grade } from "../../../api/org";
import type { CreateExamDraft } from "./types";

export function initialCreateExamDraft(schoolId = "", grade?: Grade): CreateExamDraft {
  const gradePrefix = grade ? `${grade.academic_year}学年${grade.name}` : "";
  return {
    schoolId,
    name: gradePrefix ? `${gradePrefix}期中考试` : "",
    examType: "midterm_exam",
    gradeId: grade?.id ?? "",
    classIds: [],
    subject: "math",
    totalScore: 150,
    gradingMode: "ai_assisted",
    publishPolicy: "after_admin_approval",
    appealEnabled: true
  };
}
