package exam

import (
	"context"
	"errors"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestCreateExamSessionRejectsGradeOutsideResolvedScope(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.CreateExamSession(context.Background(), auth.AccessScope{
		TenantID: "tenant-1", SchoolIDs: []string{"school-1"},
		GradeIDs: []string{"grade-allowed"}, ClassIDs: []string{"class-1"},
	}, "user-1", CreateSessionInput{
		SchoolID: "school-1", GradeID: "grade-forbidden", Name: "越权考试",
		ExamType: "midterm_exam", GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval",
		ClassIDs: []string{"class-1"}, Subjects: []SessionSubjectInput{{
			Subject: "math", TotalScore: 100, DurationMinutes: 90,
			Sections: []BlueprintSectionInput{{Title: "全卷", QuestionType: "short_answer", QuestionCount: 10, ScorePerQuestion: 10}},
		}},
	})
	if !errors.Is(err, ErrScopeForbidden) {
		t.Fatalf("expected grade scope rejection, got %v", err)
	}
}
