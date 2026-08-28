import { examTypeOptions } from "../../../constants/examCatalog";
export { examTypeOptions } from "../../../constants/examCatalog";

export const gradingPresentation = {
  ai_assisted: { title: "智能辅助阅卷", description: "系统自动完成高置信度评分，需要确认的题目交给教师。" },
  auto_objective_only: { title: "仅客观题自动评分", description: "客观题自动完成，主观题由教师评分。" },
  human_review_required: { title: "全部人工确认", description: "系统提供识别辅助，主观题评分均由教师确认。" },
  double_mark: { title: "双评", description: "两位教师独立评分。" },
  blind_double_mark: { title: "盲双评", description: "两位教师独立评分，互不可见对方结果。" }
} as const;

export const publishPolicyOptions = [
  { label: "阅卷完成并由管理员确认后发布", value: "after_admin_approval" }
];

export function examTypeLabel(value: string) {
  return examTypeOptions.find((item) => item.value === value)?.label ?? value;
}

export function gradingLabel(value: string) {
  return gradingPresentation[value as keyof typeof gradingPresentation]?.title ?? value;
}

export function publishPolicyLabel(value: string) {
  return publishPolicyOptions.find((item) => item.value === value)?.label ?? value;
}
