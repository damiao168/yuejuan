package reviewannotation

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound         = errors.New("review annotation resource not found")
	ErrInvalidInput     = errors.New("invalid review annotation input")
	ErrRevisionConflict = errors.New("review annotation revision conflict")
	ErrShortcutConflict = errors.New("review comment template shortcut conflict")
)

type Visibility string

const (
	VisibilityPrivate             Visibility = "private"
	VisibilityStudentAfterPublish Visibility = "student_after_publish"
)

func (v Visibility) Valid() bool {
	return v == VisibilityPrivate || v == VisibilityStudentAfterPublish
}

type AnnotationType string

const (
	AnnotationNote      AnnotationType = "note"
	AnnotationHighlight AnnotationType = "highlight"
	AnnotationRectangle AnnotationType = "rectangle"
	AnnotationFreehand  AnnotationType = "freehand"
)

func (kind AnnotationType) Valid() bool {
	switch kind {
	case AnnotationNote, AnnotationHighlight, AnnotationRectangle, AnnotationFreehand:
		return true
	default:
		return false
	}
}

// ImageGeometry is always measured against the orientation-corrected original
// page image. Values are normalized to [0,1], so annotations survive rendition
// and viewport size changes without storing display pixels.
type ImageGeometry struct {
	CoordinateSpace string  `json:"coordinate_space"`
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Width           float64 `json:"width"`
	Height          float64 `json:"height"`
}

const CanonicalImageNormalized = "canonical_image_normalized"

func (geometry ImageGeometry) Valid() bool {
	return geometry.CoordinateSpace == CanonicalImageNormalized &&
		geometry.X >= 0 && geometry.X <= 1 && geometry.Y >= 0 && geometry.Y <= 1 &&
		geometry.Width >= 0 && geometry.Height >= 0 &&
		geometry.X+geometry.Width <= 1 && geometry.Y+geometry.Height <= 1
}

type Annotation struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	ReviewTaskID     string         `json:"review_task_id"`
	AnswerSegmentID  string         `json:"answer_segment_id"`
	SubmissionPageID string         `json:"submission_page_id"`
	Type             AnnotationType `json:"type"`
	Geometry         ImageGeometry  `json:"geometry"`
	Payload          map[string]any `json:"payload"`
	Content          string         `json:"content"`
	Visibility       Visibility     `json:"visibility"`
	Revision         int64          `json:"revision"`
	CreatedBy        string         `json:"created_by"`
	UpdatedBy        string         `json:"updated_by"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// StudentAnnotation intentionally has no tenant, actor, revision, payload, or
// visibility field. In particular, a private annotation cannot be represented
// by this DTO and filtering happens before conversion.
type StudentAnnotation struct {
	ID               string         `json:"id"`
	AnswerSegmentID  string         `json:"answer_segment_id"`
	SubmissionPageID string         `json:"submission_page_id"`
	Type             AnnotationType `json:"type"`
	Geometry         ImageGeometry  `json:"geometry"`
	Content          string         `json:"content"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

func StudentAnnotations(items []Annotation) []StudentAnnotation {
	out := make([]StudentAnnotation, 0, len(items))
	for _, item := range items {
		if item.Visibility != VisibilityStudentAfterPublish {
			continue
		}
		out = append(out, StudentAnnotation{
			ID: item.ID, AnswerSegmentID: item.AnswerSegmentID,
			SubmissionPageID: item.SubmissionPageID, Type: item.Type,
			Geometry: item.Geometry, Content: item.Content,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	}
	return out
}

type CreateAnnotationInput struct {
	Type       AnnotationType `json:"type"`
	Geometry   ImageGeometry  `json:"geometry"`
	Payload    map[string]any `json:"payload"`
	Content    string         `json:"content"`
	Visibility Visibility     `json:"visibility"`
}

type UpdateAnnotationInput struct {
	Type             AnnotationType `json:"type"`
	Geometry         ImageGeometry  `json:"geometry"`
	Payload          map[string]any `json:"payload"`
	Content          string         `json:"content"`
	Visibility       Visibility     `json:"visibility"`
	ExpectedRevision int64          `json:"expected_revision"`
}

type DeleteInput struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type CommentTemplate struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	OwnerID    string    `json:"owner_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Shortcut   string    `json:"shortcut"`
	UsageCount int64     `json:"usage_count"`
	Revision   int64     `json:"revision"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateCommentTemplateInput struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Shortcut string `json:"shortcut"`
}

type UpdateCommentTemplateInput struct {
	Title            string `json:"title"`
	Content          string `json:"content"`
	Shortcut         string `json:"shortcut"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type Store interface {
	CreateAnnotation(context.Context, string, string, string, CreateAnnotationInput) (Annotation, error)
	ListAnnotations(context.Context, string, string) ([]Annotation, error)
	GetAnnotation(context.Context, string, string) (Annotation, error)
	UpdateAnnotation(context.Context, string, string, string, UpdateAnnotationInput) (Annotation, error)
	DeleteAnnotation(context.Context, string, string, string, int64) error
	ListStudentAnnotations(context.Context, string, string) ([]StudentAnnotation, error)
	// ListStudentQuestionAnnotations resolves the student's answer only through
	// the current immutable published release. It deliberately takes the
	// student and question scope rather than a caller-supplied submission ID so
	// a student cannot enumerate another student's annotations.
	ListStudentQuestionAnnotations(context.Context, string, string, string, string) ([]StudentAnnotation, error)

	CreateCommentTemplate(context.Context, string, string, CreateCommentTemplateInput) (CommentTemplate, error)
	ListCommentTemplates(context.Context, string, string) ([]CommentTemplate, error)
	GetCommentTemplate(context.Context, string, string, string) (CommentTemplate, error)
	UpdateCommentTemplate(context.Context, string, string, string, UpdateCommentTemplateInput) (CommentTemplate, error)
	DeleteCommentTemplate(context.Context, string, string, string, int64) error
	UseCommentTemplate(context.Context, string, string, string) (CommentTemplate, error)
}

func normalizeAnnotationInput(input CreateAnnotationInput) CreateAnnotationInput {
	input.Content = strings.TrimSpace(input.Content)
	if input.Geometry.CoordinateSpace == "" {
		input.Geometry.CoordinateSpace = CanonicalImageNormalized
	}
	if input.Visibility == "" {
		input.Visibility = VisibilityPrivate
	}
	if input.Payload == nil {
		input.Payload = map[string]any{}
	}
	return input
}

func validateAnnotationInput(input CreateAnnotationInput) error {
	if !input.Type.Valid() || !input.Geometry.Valid() || !input.Visibility.Valid() || len(input.Content) > 4000 {
		return ErrInvalidInput
	}
	return nil
}

func normalizeTemplate(title, content, shortcut string) (string, string, string) {
	return strings.TrimSpace(title), strings.TrimSpace(content), strings.ToLower(strings.TrimSpace(shortcut))
}

func validateTemplate(title, content, shortcut string) error {
	if title == "" || len(title) > 120 || content == "" || len(content) > 4000 || len(shortcut) == 0 || len(shortcut) > 32 {
		return ErrInvalidInput
	}
	for index, value := range shortcut {
		valid := value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || (index > 0 && (value == '.' || value == '_' || value == '-'))
		if !valid {
			return ErrInvalidInput
		}
	}
	return nil
}

func clonePayload(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}
