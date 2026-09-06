package paper

import (
	"testing"
	"time"
)

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

func TestReadinessConfigurationHashIsStableWhenDefaultArchetypeIsMaterialized(t *testing.T) {
	implicit := Question{
		ID: "q1", QuestionNo: "1", QuestionType: "single_choice", Score: 1,
		AnswerKey: &AnswerKey{StandardAnswer: "A"},
	}
	explicit := implicit
	explicit.AssessmentArchetype = "selected_response"

	beforeConfirmation := buildReadiness(1, 1, 1, []Paper{{ID: "paper"}}, []Question{implicit}, nil)
	afterFreezeTrigger := buildReadiness(1, 1, 1, []Paper{{ID: "paper"}}, []Question{explicit}, nil)
	if beforeConfirmation.ConfigurationHash != afterFreezeTrigger.ConfigurationHash {
		t.Fatalf("materializing the effective archetype changed readiness hash: before=%s after=%s", beforeConfirmation.ConfigurationHash, afterFreezeTrigger.ConfigurationHash)
	}
}

func TestReadinessConfigurationHashIgnoresPersistenceMetadata(t *testing.T) {
	papers, questions, templates := readinessHashFixture()
	baseline := buildReadinessForScope(1, []string{"class-1"}, []string{"student-1"}, papers, questions, templates)

	papers[0].File.OriginalName = "renamed.pdf"
	papers[0].File.StorageBucket = "archive"
	papers[0].File.StorageKey = "moved/paper.pdf"
	questions[0].PaperImportID = "import-2"
	questions[0].PaperImportCandidateID = "candidate-2"
	questions[0].PaperImportSourceRefs = []PaperImportSourceRef{{SourceID: "source-2"}}
	questions[0].AnswerKey.ID = "answer-row-2"
	questions[0].AnswerKey.AnswerVersion = "99"
	questions[0].AnswerKey.PaperImportID = "import-2"
	templates[0].Name = "renamed template"
	templates[0].Revision = 99
	templates[0].ContentHash = "persistence-hash"
	templates[0].CreatedBy = "operator-2"
	templates[0].LockedBy = "operator-3"
	now := time.Now().UTC()
	templates[0].LockedAt = &now
	templates[0].CreatedAt = now
	templates[0].UpdatedAt = now
	templates[0].Layout.Pages[0].QuestionRegions[0].ID = "region-row-2"
	templates[0].Layout.Pages[0].QuestionRegions[0].SuggestionConfidence = 0.42
	templates[0].Layout.Pages[0].QuestionRegions[0].SuggestionSource = "different-detector"

	changed := buildReadinessForScope(1, []string{"class-1"}, []string{"student-1"}, papers, questions, templates)
	if baseline.ConfigurationHash != changed.ConfigurationHash {
		t.Fatalf("persistence metadata changed readiness hash: before=%s after=%s", baseline.ConfigurationHash, changed.ConfigurationHash)
	}
}

func TestReadinessConfigurationHashTracksCandidateIdentityNotOnlyCount(t *testing.T) {
	papers, questions, templates := readinessHashFixture()
	first := buildReadinessForScope(1, []string{"class-1"}, []string{"student-1"}, papers, questions, templates)
	second := buildReadinessForScope(1, []string{"class-1"}, []string{"student-2"}, papers, questions, templates)
	if first.ConfigurationHash == second.ConfigurationHash {
		t.Fatal("different candidate identities produced the same readiness hash")
	}
}

func TestReadinessConfigurationHashTracksSemanticChanges(t *testing.T) {
	papers, questions, templates := readinessHashFixture()
	baseline := buildReadinessForScope(1, []string{"class-1"}, []string{"student-1"}, papers, questions, templates)

	questions[0].AnswerKey.StandardAnswer = "B"
	changedAnswer := buildReadinessForScope(1, []string{"class-1"}, []string{"student-1"}, papers, questions, templates)
	if baseline.ConfigurationHash == changedAnswer.ConfigurationHash {
		t.Fatal("answer change did not invalidate readiness hash")
	}

	questions[0].AnswerKey.StandardAnswer = "A"
	templates[0].Layout.Pages[0].QuestionRegions[0].X = 0.25
	changedLayout := buildReadinessForScope(1, []string{"class-1"}, []string{"student-1"}, papers, questions, templates)
	if baseline.ConfigurationHash == changedLayout.ConfigurationHash {
		t.Fatal("layout change did not invalidate readiness hash")
	}
}

func TestReadinessConfigurationHashCanonicalizesCollectionOrder(t *testing.T) {
	papers, questions, templates := readinessHashFixture()
	secondQuestion := questions[0]
	secondQuestion.ID = "question-2"
	secondQuestion.QuestionNo = "2"
	secondQuestion.SortOrder = 2
	templates[0].Layout.Pages[0].QuestionRegions = append(templates[0].Layout.Pages[0].QuestionRegions, LayoutRegion{QuestionID: "question-2", X: 0.1, Y: 0.3, Width: 0.8, Height: 0.2})

	first := buildReadinessForScope(2, []string{"class-2", "class-1"}, []string{"student-2", "student-1"}, papers, []Question{secondQuestion, questions[0]}, templates)
	templates[0].Layout.Pages[0].QuestionRegions[0], templates[0].Layout.Pages[0].QuestionRegions[1] = templates[0].Layout.Pages[0].QuestionRegions[1], templates[0].Layout.Pages[0].QuestionRegions[0]
	second := buildReadinessForScope(2, []string{"class-1", "class-2"}, []string{"student-1", "student-2"}, papers, []Question{questions[0], secondQuestion}, templates)
	if first.ConfigurationHash != second.ConfigurationHash {
		t.Fatalf("collection order changed readiness hash: first=%s second=%s", first.ConfigurationHash, second.ConfigurationHash)
	}
}

func readinessHashFixture() ([]Paper, []Question, []AnswerSheetTemplate) {
	papers := []Paper{{
		ID: "paper-1", ExamID: "exam-1", FileAssetID: "file-1", VersionNo: 1, Status: "active",
		File: FileAssetInput{OriginalName: "paper.pdf", HashSHA256: "sha-1", StorageBucket: "primary", StorageKey: "paper.pdf"},
	}}
	questions := []Question{{
		ID: "question-1", ExamID: "exam-1", ExamPaperID: "paper-1", QuestionNo: "1", QuestionType: "single_choice",
		Score: 1, Stem: "Choose", KnowledgePoints: []string{"b", "a"}, SortOrder: 1,
		AnswerKey: &AnswerKey{ID: "answer-row-1", QuestionID: "question-1", AnswerVersion: "1", StandardAnswer: "A"},
	}}
	templates := []AnswerSheetTemplate{{
		ID: "template-1", ExamID: "exam-1", ExamPaperID: "paper-1", VersionNo: 1, Revision: 1,
		Name: "template", Status: "locked", PageCount: 1, ContentHash: "stored-hash", CreatedBy: "operator-1", LockedBy: "operator-1",
		Layout: TemplateLayout{Pages: []TemplatePage{{
			PageNo: 1, Width: 1000, Height: 1400,
			QuestionRegions: []LayoutRegion{{ID: "region-row-1", QuestionID: "question-1", X: 0.1, Y: 0.1, Width: 0.8, Height: 0.2}},
		}}},
	}}
	return papers, questions, templates
}
