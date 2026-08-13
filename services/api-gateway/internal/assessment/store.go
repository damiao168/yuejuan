package assessment

import "context"

type Store interface {
	ListSubjectProfiles(ctx context.Context, tenantID string, stage EducationStage, subject SubjectCode) ([]SubjectProfile, error)
	ListQuestionArchetypes(ctx context.Context) ([]QuestionArchetype, error)
	GetQuestionConfig(ctx context.Context, tenantID string, examID string, questionID string) (QuestionAssessmentConfig, error)
	ConfigureQuestion(ctx context.Context, tenantID string, examID string, questionID string, input ConfigureQuestionInput) (QuestionAssessmentConfig, error)
	FreezeQuestionSnapshot(ctx context.Context, tenantID string, examID string, questionID string) (ExamQuestionSnapshot, error)
	GetQuestionSnapshot(ctx context.Context, tenantID string, examID string, questionID string) (ExamQuestionSnapshot, error)
	GetExamAssessmentSummary(ctx context.Context, tenantID string, examID string) (ExamAssessmentSummary, error)
	CreateScoringEvidence(ctx context.Context, tenantID string, input CreateScoringEvidenceInput) (ScoringEvidence, error)
	ListScoringEvidence(ctx context.Context, tenantID string, submissionID string, questionID string) ([]ScoringEvidence, error)
}
