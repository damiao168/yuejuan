import { apiClient } from "./client";

export interface DashboardStatistics {
  active_exam_count: number;
  collecting_exam_count: number;
  pending_review_question_count: number;
  pending_review_submission_count: number;
  pending_arbitration_count: number;
  pending_arbitration_submission_count: number;
  failed_submission_count: number;
  unmatched_submission_count: number;
  quality_issue_submission_count: number;
  finalized_exam_count: number;
}

export interface DashboardOrganizationStatistics {
  active_student_count: number;
  grade_count: number;
  class_count: number;
  teacher_count: number;
  grader_count: number;
  empty_class_count: number;
  unassigned_teacher_count: number;
}

export interface DashboardBlockingIssue {
  code: string;
  label: string;
  count: number;
  unit: string;
  impact: string;
  action: string;
  drilldown_path: string;
}

export interface DashboardActiveExam {
  id: string;
  name: string;
  subject: string;
  status: string;
  submission_count: number;
  failed_count: number;
  quality_issue_count: number;
  unmatched_count: number;
  created_at: string;
}

export interface DashboardActivity {
  id: string;
  action: string;
  target_type: string;
  target_id?: string;
  reason?: string;
  created_at: string;
  drilldown_path?: string;
}

export interface DashboardSummary {
  scope: {
    tenant_id: string;
    school_id?: string;
  };
  updated_at: string;
  organization_statistics: DashboardOrganizationStatistics;
  statistics: DashboardStatistics;
  blocking_issues: DashboardBlockingIssue[];
  active_exams: DashboardActiveExam[];
  recent_activities: DashboardActivity[];
  warnings: string[];
}

export function getDashboardSummary() {
  return apiClient.request<DashboardSummary>("/api/v1/dashboard/summary");
}
