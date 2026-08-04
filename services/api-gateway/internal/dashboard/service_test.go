package dashboard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/review"
	"edugrade-enterprise/services/api-gateway/internal/submission"
)

func TestSummaryUsesEveryTenantExamAndKeepsUnitsSeparate(t *testing.T) {
	const tenantID = "tenant-school"
	exams := exam.NewMemoryStore()
	submissions := submission.NewMemoryStore()
	reviews := review.NewMemoryStore()
	audits := auth.NewMemoryStore()
	now := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
	tenantScope := auth.AccessScope{TenantID: tenantID, ActorID: "admin-1", TenantWide: true, SchoolIDs: []string{"school-1"}}

	for index := 0; index < 12; index++ {
		item, err := exams.CreateExam(context.Background(), tenantScope, "admin-1", exam.CreateInput{
			SchoolID: "school-1", Name: fmt.Sprintf("考试 %02d", index+1), Subject: "math",
			ExamType: "school_exam", TotalScore: 100, GradingMode: "human_review_required",
			PublishPolicy: "manual",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := submissions.Create(context.Background(), tenantID, item.ID, "admin-1", submission.CreateSubmissionInput{
			SourceType: "scanner_upload", ExpectedPageCount: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	other, err := exams.CreateExam(context.Background(), auth.AccessScope{TenantID: "other-tenant", ActorID: "admin-2", TenantWide: true}, "admin-2", exam.CreateInput{
		SchoolID: "other-school", Name: "其他租户考试", Subject: "math",
		ExamType: "school_exam", TotalScore: 100, GradingMode: "human_review_required",
		PublishPolicy: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := submissions.Create(context.Background(), "other-tenant", other.ID, "admin-2", submission.CreateSubmissionInput{SourceType: "scanner_upload"}); err != nil {
		t.Fatal(err)
	}

	service := NewService(Dependencies{Exams: exams, Submissions: submissions, Reviews: reviews, Audits: audits, Now: func() time.Time { return now }})
	summary, err := service.Summary(context.Background(), auth.User{
		ID: "admin-1", TenantID: tenantID, Roles: []string{"school_admin"},
		DataScope: map[string]any{"school_admin": map[string]any{"school_id": "school-1"}},
	}, tenantScope)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Statistics.ActiveExamCount != 12 || len(summary.ActiveExams) != 12 {
		t.Fatalf("dashboard must aggregate all 12 exams, got statistics=%d rows=%d", summary.Statistics.ActiveExamCount, len(summary.ActiveExams))
	}
	if summary.Statistics.UnmatchedSubmissionCount != 12 {
		t.Fatalf("expected 12 unmatched answer sheets, got %d", summary.Statistics.UnmatchedSubmissionCount)
	}
	if summary.Statistics.PendingReviewQuestionCount != 0 || summary.Statistics.PendingReviewSubmissionCount != 0 {
		t.Fatalf("question and answer-sheet counts must stay independent: %#v", summary.Statistics)
	}
	if summary.Scope.TenantID != tenantID || summary.Scope.SchoolID != "school-1" || !summary.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected scope or timestamp: %#v", summary)
	}
}

func TestPlatformAdminCannotReadSchoolDashboard(t *testing.T) {
	service := NewService(Dependencies{
		Exams: exam.NewMemoryStore(), Submissions: submission.NewMemoryStore(),
		Reviews: review.NewMemoryStore(), Audits: auth.NewMemoryStore(),
	})
	if _, err := service.Summary(context.Background(), auth.User{TenantID: auth.PlatformTenantID, Roles: []string{"platform_admin"}}, auth.AccessScope{TenantID: auth.PlatformTenantID, IsPlatform: true}); err == nil {
		t.Fatal("platform administrator must not enter a school dashboard")
	}
}
