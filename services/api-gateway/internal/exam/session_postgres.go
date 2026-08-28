package exam

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func (s *PostgresStore) CreateExamSession(ctx context.Context, scope auth.AccessScope, createdBy string, input CreateSessionInput) (ExamSession, error) {
	if !scopeAllowsRequestedClasses(scope, input.SchoolID, input.ClassIDs) || !scopeAllowsRequestedGrade(scope, input.GradeID) {
		return ExamSession{}, ErrScopeForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ExamSession{}, err
	}
	defer tx.Rollback()

	appealEnabled := true
	if input.AppealEnabled != nil {
		appealEnabled = *input.AppealEnabled
	}
	templateVersion, err := requireExamTemplate(ctx, tx, scope, input.TemplateID)
	if err != nil {
		return ExamSession{}, err
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO exam_session (tenant_id, school_id, grade_id, exam_template_id, exam_template_version, name, exam_type, grading_mode, appeal_enabled, publish_policy, created_by)
SELECT $1, s.id, g.id, NULLIF($4,'')::uuid, NULLIF($5,0), $6, $7, $8, $9, $10, $11
FROM school s
JOIN grade g ON g.tenant_id = s.tenant_id AND g.school_id = s.id
WHERE s.tenant_id = $1 AND s.id::text = $2 AND g.id::text = $3
  AND s.deleted_at IS NULL AND g.deleted_at IS NULL AND g.status = 'active'
RETURNING id::text, tenant_id::text, school_id::text, grade_id::text, COALESCE(exam_template_id::text,''), COALESCE(exam_template_version,0), name, exam_type,
          status, grading_mode, appeal_enabled, publish_policy, created_by::text,
          revision, created_at, updated_at
`, scope.TenantID, input.SchoolID, input.GradeID, input.TemplateID, templateVersion, input.Name, input.ExamType, input.GradingMode, appealEnabled, input.PublishPolicy, createdBy)
	var session ExamSession
	if err := row.Scan(&session.ID, &session.TenantID, &session.SchoolID, &session.GradeID, &session.TemplateID, &session.TemplateVersion, &session.Name, &session.ExamType, &session.Status, &session.GradingMode, &session.AppealEnabled, &session.PublishPolicy, &session.CreatedBy, &session.Revision, &session.CreatedAt, &session.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExamSession{}, ErrInvalidInput
		}
		return ExamSession{}, err
	}

	for _, subject := range input.Subjects {
		classIDs := subject.ClassIDs
		if len(classIDs) == 0 {
			classIDs = input.ClassIDs
		}
		if !scopeAllowsRequestedClasses(scope, input.SchoolID, classIDs) {
			return ExamSession{}, ErrScopeForbidden
		}
		if err := requireSessionClasses(ctx, tx, scope.TenantID, input.SchoolID, input.GradeID, classIDs); err != nil {
			return ExamSession{}, err
		}
		candidateRule := subject.CandidateRule
		if candidateRule == "" {
			candidateRule = "all_selected_classes"
		}
		examName := fmt.Sprintf("%s · %s", input.Name, subjectDisplayName(subject.Subject))
		examRow := tx.QueryRowContext(ctx, `
INSERT INTO exam (tenant_id, school_id, exam_session_id, name, subject, exam_type, total_score, status,
                  grading_mode, appeal_enabled, publish_policy, created_by, duration_minutes, candidate_rule)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft', $8, $9, $10, $11, $12, $13)
RETURNING id::text, tenant_id::text, school_id::text, name, subject, exam_type, total_score::float8,
          status, grading_mode, appeal_enabled, publish_policy, created_by::text, revision, created_at, updated_at
`, scope.TenantID, input.SchoolID, session.ID, examName, subject.Subject, input.ExamType, subject.TotalScore, input.GradingMode, appealEnabled, input.PublishPolicy, createdBy, subject.DurationMinutes, candidateRule)
		var child Exam
		if err := scanExam(examRow, &child); err != nil {
			return ExamSession{}, err
		}
		if err := s.replaceClasses(ctx, tx, scope, child.ID, classIDs); err != nil {
			return ExamSession{}, err
		}
		if err := s.refreshExamCandidateSnapshot(ctx, tx, scope.TenantID, child.ID); err != nil {
			return ExamSession{}, err
		}
		child.ClassIDs = cloneStrings(classIDs)
		questionOrder := 1
		for sectionOrder, section := range subject.Sections {
			var sectionID string
			if err := tx.QueryRowContext(ctx, `
INSERT INTO exam_blueprint_section (tenant_id, exam_id, title, question_type, question_count, score_per_question, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id::text
`, scope.TenantID, child.ID, section.Title, section.QuestionType, section.QuestionCount, section.ScorePerQuestion, sectionOrder+1).Scan(&sectionID); err != nil {
				return ExamSession{}, err
			}
			for index := 0; index < section.QuestionCount; index++ {
				if _, err := tx.ExecContext(ctx, `
INSERT INTO question (tenant_id, exam_id, question_no, question_type, score, stem, knowledge_points, sort_order, status)
VALUES ($1, $2, $3, $4, $5, '', '[]', $6, 'draft')
`, scope.TenantID, child.ID, fmt.Sprintf("%d", questionOrder), section.QuestionType, section.ScorePerQuestion, questionOrder); err != nil {
					return ExamSession{}, err
				}
				questionOrder++
			}
		}
		session.Exams = append(session.Exams, child)
	}
	if err := tx.Commit(); err != nil {
		return ExamSession{}, err
	}
	return session, nil
}

func scopeAllowsRequestedGrade(scope auth.AccessScope, gradeID string) bool {
	return scope.TenantWide || len(scope.GradeIDs) == 0 || scope.AllowsGrade(gradeID)
}

func requireSessionClasses(ctx context.Context, tx *sql.Tx, tenantID, schoolID, gradeID string, classIDs []string) error {
	for _, classID := range classIDs {
		var exists int
		err := tx.QueryRowContext(ctx, `
SELECT 1 FROM school_class
WHERE tenant_id = $1 AND id::text = $2 AND school_id::text = $3 AND grade_id::text = $4
  AND status = 'active' AND deleted_at IS NULL
`, tenantID, classID, schoolID, gradeID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidInput
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func subjectDisplayName(subject string) string {
	labels := map[string]string{"chinese": "语文", "math": "数学", "mathematics": "数学", "english": "英语", "physics": "物理", "chemistry": "化学", "biology": "生物", "history": "历史", "geography": "地理", "politics": "政治", "ethics_politics": "政治"}
	if label := labels[subject]; label != "" {
		return label
	}
	return subject
}
