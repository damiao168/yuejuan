package studentportal

import (
	"context"
	"database/sql"
)

type PostgresStore struct{ db *sql.DB }

func NewPostgresStore(db *sql.DB) *PostgresStore { return &PostgresStore{db: db} }

func (s *PostgresStore) ListPublishedExams(ctx context.Context, tenantID, studentID string) ([]PublishedExam, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT exam_id::text, exam_name, subject, release_version, published_at
FROM student_published_exam
WHERE tenant_id = $1 AND student_id = $2::uuid
ORDER BY published_at DESC, exam_id
`, tenantID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PublishedExam{}
	for rows.Next() {
		var item PublishedExam
		if err := rows.Scan(&item.ExamID, &item.Name, &item.Subject, &item.ReleaseVersion, &item.PublishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
