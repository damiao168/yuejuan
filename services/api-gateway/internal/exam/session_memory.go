package exam

import (
	"context"
	"fmt"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func (s *MemoryStore) CreateExamSession(ctx context.Context, scope auth.AccessScope, createdBy string, input CreateSessionInput) (ExamSession, error) {
	if !scopeAllowsRequestedClasses(scope, input.SchoolID, input.ClassIDs) || !scopeAllowsRequestedGrade(scope, input.GradeID) {
		return ExamSession{}, ErrScopeForbidden
	}
	now := time.Now().UTC()
	appealEnabled := true
	if input.AppealEnabled != nil {
		appealEnabled = *input.AppealEnabled
	}
	templateVersion := 0
	if input.TemplateID != "" {
		template, err := s.GetExamTemplate(ctx, scope, input.TemplateID)
		if err != nil {
			return ExamSession{}, ErrInvalidInput
		}
		templateVersion = template.Version
	}
	session := ExamSession{
		ID: fmt.Sprintf("exam-session-%d", now.UnixNano()), TenantID: scope.TenantID,
		SchoolID: input.SchoolID, GradeID: input.GradeID, TemplateID: input.TemplateID, TemplateVersion: templateVersion, Name: input.Name,
		ExamType: input.ExamType, Status: "draft", GradingMode: input.GradingMode,
		AppealEnabled: appealEnabled, PublishPolicy: input.PublishPolicy, CreatedBy: createdBy,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	for _, subject := range input.Subjects {
		classes := subject.ClassIDs
		if len(classes) == 0 {
			classes = input.ClassIDs
		}
		child, err := s.CreateExam(ctx, scope, createdBy, CreateInput{
			SchoolID: input.SchoolID, Name: fmt.Sprintf("%s · %s", input.Name, subjectDisplayName(subject.Subject)),
			Subject: subject.Subject, ExamType: input.ExamType, TotalScore: subject.TotalScore,
			GradingMode: input.GradingMode, AppealEnabled: input.AppealEnabled,
			PublishPolicy: input.PublishPolicy, ClassIDs: classes,
		})
		if err != nil {
			return ExamSession{}, err
		}
		session.Exams = append(session.Exams, child)
	}
	return session, nil
}
