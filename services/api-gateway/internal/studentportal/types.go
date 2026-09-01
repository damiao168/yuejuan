// Package studentportal owns student-facing discovery only.  Score facts and
// question feedback remain owned by the immutable student-safe DTOs in
// scorerelease, so this package cannot accidentally introduce a mutable or
// administrator-shaped result representation.
package studentportal

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid student portal input")
	ErrNotFound     = errors.New("student portal resource not found")
)

// PublishedExam is deliberately only the metadata a student needs to choose
// a released result. It omits release identifiers, classmates, rank, quality
// signals and any scoring provenance.
type PublishedExam struct {
	ExamID         string    `json:"exam_id"`
	Name           string    `json:"name"`
	Subject        string    `json:"subject"`
	ExamType       string    `json:"exam_type,omitempty"`
	ReleaseVersion int       `json:"release_version"`
	PublishedAt    time.Time `json:"published_at"`
	TotalScore     float64   `json:"total_score,omitempty"`
	MaxScore       float64   `json:"max_score,omitempty"`
}

type Store interface {
	ListPublishedExams(context.Context, string, string) ([]PublishedExam, error)
}
