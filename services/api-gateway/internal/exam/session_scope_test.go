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

func TestMemoryExamSessionReplaysStableCommand(t *testing.T) {
	store := NewMemoryStore()
	scope := auth.AccessScope{TenantID: "tenant-1", TenantWide: true}
	input := CreateSessionInput{
		SchoolID: "school-1", GradeID: "grade-1", Name: "恢复考试", CommandID: "stable-command-1",
		ExamType: "unit_test", GradingMode: "ai_assisted", PublishPolicy: "manual_after_confirmation",
		ClassIDs: []string{"class-1"}, Subjects: []SessionSubjectInput{{
			Subject: "math", TotalScore: 100, DurationMinutes: 90,
			Sections: []BlueprintSectionInput{{Title: "全卷", QuestionType: "short_answer", QuestionCount: 10, ScorePerQuestion: 10}},
		}},
	}
	first, err := store.CreateExamSession(context.Background(), scope, "user-1", input)
	if err != nil {
		t.Fatalf("first command: %v", err)
	}
	second, err := store.CreateExamSession(context.Background(), scope, "user-1", input)
	if err != nil || second.ID != first.ID || len(second.Exams) != 1 || second.Exams[0].ID != first.Exams[0].ID {
		t.Fatalf("stable command duplicated session: first=%#v second=%#v err=%v", first, second, err)
	}
}
