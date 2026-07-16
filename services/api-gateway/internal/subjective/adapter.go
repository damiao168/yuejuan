package subjective

import (
	"context"

	"edugrade-enterprise/services/api-gateway/internal/grading"
)

type MockLLMAdapter struct{}

func NewMockLLMAdapter() *MockLLMAdapter {
	return &MockLLMAdapter{}
}

func (a *MockLLMAdapter) Name() string {
	return "mock_llm_adapter"
}

func (a *MockLLMAdapter) Grade(_ context.Context, input AdapterInput) (AdapterOutput, error) {
	return AdapterOutput{
		RequestID:      input.RequestID,
		SuggestedScore: 0,
		Confidence:     0.5,
		MatchedPoints:  []grading.PointResult{},
		MissingPoints: []grading.PointResult{
			{Code: "mock_not_scored", Label: "mock adapter does not perform real subjective scoring", Score: input.Question.Score},
		},
		Evidence: []grading.Evidence{
			{Type: "mock", AnswerSegment: "", AnswerText: "", StandardAnswer: "", Rule: "mock output; not real model evidence"},
		},
		RiskFlags:        []string{"mock_llm_output", "low_model_confidence"},
		NeedsHumanReview: true,
		StudentFeedback:  "MOCK: subjective AI feedback is not available from a real model in this environment.",
		TeacherNote:      "MOCK LLM adapter returned a fixed low-confidence placeholder. Human review is required.",
		ModelVersion:     input.ModelPolicy.ModelVersion,
		PromptVersion:    input.ModelPolicy.PromptVersion,
		RubricVersion:    input.Rubric.Version,
		DeliveryMode:     "teacher_review",
		Telemetry: AdapterTelemetry{
			Adapter:         a.Name(),
			Attempts:        1,
			PriorErrorCodes: []string{},
		},
		RawOutput: map[string]any{
			"adapter":        a.Name(),
			"mock":           true,
			"model_version":  input.ModelPolicy.ModelVersion,
			"prompt_version": input.ModelPolicy.PromptVersion,
		},
		Mock: true,
	}, nil
}
