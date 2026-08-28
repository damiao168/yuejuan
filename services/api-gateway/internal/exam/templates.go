package exam

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
)

type ExamTemplateSection struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	QuestionType     string  `json:"question_type"`
	QuestionCount    int     `json:"question_count"`
	ScorePerQuestion float64 `json:"score_per_question"`
	SortOrder        int     `json:"sort_order"`
}

type ExamTemplateSubject struct {
	ID              string                `json:"id"`
	Subject         string                `json:"subject"`
	TotalScore      float64               `json:"total_score"`
	DurationMinutes int                   `json:"duration_minutes"`
	CandidateRule   string                `json:"candidate_rule"`
	SortOrder       int                   `json:"sort_order"`
	Sections        []ExamTemplateSection `json:"sections"`
}

type ExamTemplate struct {
	ID             string                `json:"id"`
	TenantID       string                `json:"tenant_id"`
	SchoolID       string                `json:"school_id,omitempty"`
	Code           string                `json:"code"`
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	EducationStage string                `json:"education_stage"`
	ExamType       string                `json:"exam_type"`
	Region         string                `json:"region,omitempty"`
	Curriculum     string                `json:"curriculum,omitempty"`
	Version        int                   `json:"version"`
	Source         string                `json:"source"`
	Recommended    bool                  `json:"recommended"`
	Subjects       []ExamTemplateSubject `json:"subjects"`
}

type ExamTemplateFilter struct {
	EducationStage string
	ExamType       string
}

type ExamTemplateStore interface {
	ListExamTemplates(ctx context.Context, scope auth.AccessScope, filter ExamTemplateFilter) ([]ExamTemplate, error)
	GetExamTemplate(ctx context.Context, scope auth.AccessScope, id string) (ExamTemplate, error)
}

func (h *Handler) ListExamTemplates(w http.ResponseWriter, r *http.Request) {
	scope, ok := mustAccessScope(w, r)
	if !ok {
		return
	}
	stage := strings.TrimSpace(r.URL.Query().Get("education_stage"))
	if stage != "" && stage != "junior" && stage != "senior" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_education_stage", "education_stage must be junior or senior")
		return
	}
	store, ok := h.store.(ExamTemplateStore)
	if !ok {
		httpx.Error(w, r, http.StatusServiceUnavailable, "exam_template_unavailable", "考试方案服务未配置")
		return
	}
	templates, err := store.ListExamTemplates(r.Context(), scope, ExamTemplateFilter{EducationStage: stage, ExamType: strings.TrimSpace(r.URL.Query().Get("exam_type"))})
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "exam_template_list_failed", "考试方案加载失败")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exam_templates": templates})
}

func templateByID(templates []ExamTemplate, id string) (ExamTemplate, error) {
	for _, template := range templates {
		if template.ID == id {
			return template, nil
		}
	}
	return ExamTemplate{}, errors.New("exam template not found")
}
