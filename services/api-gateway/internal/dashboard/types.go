package dashboard

import "time"

type Scope struct {
	TenantID string `json:"tenant_id"`
	SchoolID string `json:"school_id,omitempty"`
}

type Statistics struct {
	ActiveExamCount                   int `json:"active_exam_count"`
	CollectingExamCount               int `json:"collecting_exam_count"`
	PendingReviewQuestionCount        int `json:"pending_review_question_count"`
	PendingReviewSubmissionCount      int `json:"pending_review_submission_count"`
	PendingArbitrationCount           int `json:"pending_arbitration_count"`
	PendingArbitrationSubmissionCount int `json:"pending_arbitration_submission_count"`
	FailedSubmissionCount             int `json:"failed_submission_count"`
	UnmatchedSubmissionCount          int `json:"unmatched_submission_count"`
	QualityIssueSubmissionCount       int `json:"quality_issue_submission_count"`
	FinalizedExamCount                int `json:"finalized_exam_count"`
}

type OrganizationStatistics struct {
	ActiveStudentCount     int `json:"active_student_count"`
	GradeCount             int `json:"grade_count"`
	ClassCount             int `json:"class_count"`
	TeacherCount           int `json:"teacher_count"`
	GraderCount            int `json:"grader_count"`
	EmptyClassCount        int `json:"empty_class_count"`
	UnassignedTeacherCount int `json:"unassigned_teacher_count"`
}

type BlockingIssue struct {
	Code          string `json:"code"`
	Label         string `json:"label"`
	Count         int    `json:"count"`
	Unit          string `json:"unit"`
	Impact        string `json:"impact"`
	Action        string `json:"action"`
	DrilldownPath string `json:"drilldown_path"`
}

type ActiveExam struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Subject           string    `json:"subject"`
	Status            string    `json:"status"`
	SubmissionCount   int       `json:"submission_count"`
	FailedCount       int       `json:"failed_count"`
	QualityIssueCount int       `json:"quality_issue_count"`
	UnmatchedCount    int       `json:"unmatched_count"`
	CreatedAt         time.Time `json:"created_at"`
}

type RecentActivity struct {
	ID            string    `json:"id"`
	Action        string    `json:"action"`
	TargetType    string    `json:"target_type"`
	TargetID      string    `json:"target_id,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	DrilldownPath string    `json:"drilldown_path,omitempty"`
}

type Summary struct {
	Scope                  Scope                  `json:"scope"`
	UpdatedAt              time.Time              `json:"updated_at"`
	OrganizationStatistics OrganizationStatistics `json:"organization_statistics"`
	Statistics             Statistics             `json:"statistics"`
	BlockingIssues         []BlockingIssue        `json:"blocking_issues"`
	ActiveExams            []ActiveExam           `json:"active_exams"`
	RecentActivities       []RecentActivity       `json:"recent_activities"`
	Warnings               []string               `json:"warnings"`
}
