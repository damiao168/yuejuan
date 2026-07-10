package grading_test

import (
	"math"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/paper"
)

func TestEngineGradesSupportedQuestionTypes(t *testing.T) {
	tests := []struct {
		name          string
		kind          string
		standard      any
		equiv         []any
		tolerance     any
		answerText    string
		answerPayload map[string]any
		wantScore     float64
		wantReview    bool
		wantRisk      string
	}{
		{
			name:       "single choice exact",
			kind:       "single_choice",
			standard:   "B",
			answerText: "b",
			wantScore:  5,
		},
		{
			name:       "true false exact",
			kind:       "true_false",
			standard:   true,
			answerText: "正确",
			wantScore:  5,
		},
		{
			name:       "multiple choice wrong option zero",
			kind:       "multiple_choice",
			standard:   []any{"A", "C", "D"},
			answerText: "A,B",
			wantScore:  0,
		},
		{
			name:       "multiple choice partial proportional",
			kind:       "multiple_choice",
			standard:   []any{"A", "C", "D"},
			tolerance:  map[string]any{"allow_partial": true},
			answerText: "A,C",
			wantScore:  10.0 / 3.0 * 2.0,
		},
		{
			name:       "fill blank equivalent normalized",
			kind:       "fill_blank",
			standard:   "F=ma",
			equiv:      []any{"force equals mass times acceleration"},
			tolerance:  map[string]any{"ignore_case": true, "ignore_spaces": true},
			answerText: "Force Equals Mass Times Acceleration",
			wantScore:  5,
		},
		{
			name:       "numeric tolerance with unit",
			kind:       "numeric",
			standard:   map[string]any{"value": 9.8, "unit": "m/s2"},
			tolerance:  map[string]any{"absolute": 0.05, "unit_required": true},
			answerText: "9.81m/s2",
			wantScore:  5,
		},
		{
			name:       "numeric parse failed requires review",
			kind:       "numeric",
			standard:   9.8,
			answerText: "about ten",
			wantScore:  0,
			wantReview: true,
			wantRisk:   "numeric_parse_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := gradingContext(tt.kind, tt.standard, tt.equiv, tt.tolerance, tt.answerText, tt.answerPayload)
			grade, err := grading.NewEngine().Grade(ctx)
			if err != nil {
				t.Fatalf("grade: %v", err)
			}
			if math.Abs(grade.SuggestedScore-tt.wantScore) > 0.0001 {
				t.Fatalf("score got %.4f want %.4f in %#v", grade.SuggestedScore, tt.wantScore, grade)
			}
			if grade.SuggestedScore > grade.MaxScore {
				t.Fatalf("score must not exceed max: %#v", grade)
			}
			if grade.NeedsHumanReview != tt.wantReview {
				t.Fatalf("review got %v want %v in %#v", grade.NeedsHumanReview, tt.wantReview, grade)
			}
			if tt.wantRisk != "" && !contains(grade.RiskFlags, tt.wantRisk) {
				t.Fatalf("missing risk %s in %#v", tt.wantRisk, grade.RiskFlags)
			}
			if grade.GraderType != grading.GraderType || grade.RuleVersion != grading.RuleVersion || grade.Mock {
				t.Fatalf("grade source must be truthful rule-based non-mock: %#v", grade)
			}
		})
	}
}

func TestEngineRejectsUnsupportedQuestionType(t *testing.T) {
	ctx := gradingContext("essay", "expected", nil, nil, "answer", nil)
	if _, err := grading.NewEngine().Grade(ctx); err != grading.ErrUnsupportedQuestionType {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
}

func gradingContext(kind string, standard any, equiv []any, tolerance any, answerText string, payload map[string]any) grading.Context {
	if payload == nil {
		payload = map[string]any{}
	}
	score := 5.0
	if kind == "multiple_choice" {
		score = 10
	}
	return grading.Context{
		SegmentID: "segment-1",
		Question: paper.Question{
			ID:           "question-1",
			TenantID:     tenantID,
			ExamID:       "exam-1",
			QuestionNo:   "Q1",
			QuestionType: kind,
			Score:        score,
		},
		AnswerKey: paper.AnswerKey{
			ID:                "answer-key-1",
			QuestionID:        "question-1",
			AnswerVersion:     "v1",
			StandardAnswer:    standard,
			EquivalentAnswers: equiv,
			Tolerance:         tolerance,
		},
		Answer: grading.SegmentAnswer{
			ID:              "answer-1",
			TenantID:        tenantID,
			AnswerSegmentID: "segment-1",
			AnswerText:      answerText,
			AnswerPayload:   payload,
			Source:          "manual_entry",
		},
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
