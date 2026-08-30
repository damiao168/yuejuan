package paper

import "testing"

func TestReadinessDoesNotRequireUniqueAnswerForEssayWithLockedRubric(t *testing.T) {
	question := Question{
		ID: "q1", QuestionNo: "1", QuestionType: "essay", Score: 20,
		Rubric: &Rubric{Status: "locked", MaxScore: 20, Points: []RubricPoint{{ID: "p1", Description: "内容", Score: 20}}},
	}
	result := buildReadiness(20, 1, 1, []Paper{{ID: "paper"}}, []Question{question}, nil)
	for _, check := range result.Checks {
		if check.Code == "answer_keys" && !check.Passed {
			t.Fatalf("essay was incorrectly required to have a unique standard answer: %#v", check)
		}
	}
}

func TestReadinessUsesAssessmentArchetypeOverLegacyQuestionType(t *testing.T) {
	question := Question{
		ID: "q1", QuestionNo: "1", QuestionType: "short_answer", AssessmentArchetype: "extended_response", Score: 20,
		Rubric: &Rubric{Status: "locked", MaxScore: 20, Points: []RubricPoint{{ID: "p1", Description: "内容", Score: 20}}},
	}
	result := buildReadiness(20, 1, 1, []Paper{{ID: "paper"}}, []Question{question}, nil)
	for _, check := range result.Checks {
		if check.Code == "answer_keys" && !check.Passed {
			t.Fatalf("extended_response profile was incorrectly required to have a unique answer: %#v", check)
		}
	}
}
