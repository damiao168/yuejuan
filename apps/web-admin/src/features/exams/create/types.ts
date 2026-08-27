export type GradingChoice = "auto_objective_only" | "ai_assisted" | "human_review_required" | "double_mark" | "blind_double_mark";

export interface CreateExamDraft {
  schoolId: string;
  name: string;
  examType: string;
  gradeId: string;
  classIds: string[];
  subject: string;
  totalScore: number;
  gradingMode: GradingChoice;
  publishPolicy: string;
  appealEnabled: boolean;
}
