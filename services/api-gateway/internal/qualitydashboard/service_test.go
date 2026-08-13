package qualitydashboard

import (
	"context"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/review"
)

type staticQuestions []Question

func (s staticQuestions) ListQualityQuestions(_ context.Context, _, _ string) ([]Question, error) {
	return append([]Question(nil), s...), nil
}

type staticCalibration struct{ value CalibrationSummary }

func (s staticCalibration) GetCalibrationSummary(context.Context, string, string, string) (CalibrationSummary, error) {
	return s.value, nil
}

type staticDrift struct{ value DriftSummary }

func (s staticDrift) GetDriftSummary(context.Context, string, string, string) (DriftSummary, error) {
	return s.value, nil
}

func TestR3OpenCriticalIncidentBlocksQualityGate(t *testing.T) {
	service := NewService(Sources{
		Questions:   staticQuestions{{ID: "question-1", QuestionNo: "Q1", RiskTier: "R3", ArchetypeCode: "extended_response", MaxScore: 10}},
		Calibration: staticCalibration{value: CalibrationSummary{Configured: true, Completed: 1, Passed: 1, PassRate: ratio(1, 1)}},
		Drift:       staticDrift{value: DriftSummary{OpenCritical: 1, Incidents: []Incident{{ID: "incident-1", GraderRef: "grader-1", Severity: "critical", Status: "open", Type: "severe_seed_failure"}}}},
	})
	dashboard, err := service.Get(context.Background(), "tenant-1", "exam-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if dashboard.Gate != GateBlocked {
		t.Fatalf("gate=%s, want blocked", dashboard.Gate)
	}
	if len(dashboard.Blocking) != 1 || dashboard.Blocking[0].IncidentRef != "incident-1" {
		t.Fatalf("blocking=%#v", dashboard.Blocking)
	}
}

func TestQuestionWarningsContainOwnerAndResolution(t *testing.T) {
	service := NewService(Sources{Questions: staticQuestions{{ID: "question-1", QuestionNo: "Q1", RiskTier: "R2"}}})
	dashboard, err := service.Get(context.Background(), "tenant-1", "exam-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if dashboard.Gate != GateWarning || len(dashboard.Warnings) == 0 {
		t.Fatalf("dashboard=%#v", dashboard)
	}
	for _, item := range dashboard.Warnings {
		if item.Owner == "" || item.Resolution == "" || item.QuestionNo != "Q1" {
			t.Fatalf("finding lacks operational context: %#v", item)
		}
	}
}

func TestHumanAgreementFiltersAtStoreBoundary(t *testing.T) {
	store := &filteringReviewStore{items: []review.DoubleMarkSession{
		{ID: "wanted", TenantID: "tenant-1", ExamID: "exam-1", QuestionID: "question-1", Status: "completed", ScoreDifference: floatPointer(0.5), Threshold: 1},
		{ID: "other", TenantID: "tenant-1", ExamID: "other-exam", QuestionID: "other-question", Status: "completed", ScoreDifference: floatPointer(2), Threshold: 1},
	}}
	service := NewService(Sources{Review: store})
	agreement, err := service.humanAgreement(context.Background(), "tenant-1", "exam-1", "question-1")
	if err != nil {
		t.Fatalf("humanAgreement: %v", err)
	}
	if agreement.SampleSize != 1 || agreement.WithinRule.Numerator != 1 {
		t.Fatalf("agreement=%#v, want only the target question session", agreement)
	}
	if store.lastFilter.ExamID != "exam-1" || store.lastFilter.QuestionID != "question-1" {
		t.Fatalf("filter=%#v, want exam/question constrained store query", store.lastFilter)
	}
}

func floatPointer(value float64) *float64 { return &value }

type filteringReviewStore struct {
	review.Store
	items      []review.DoubleMarkSession
	lastFilter review.DoubleMarkSessionFilter
}

func (s *filteringReviewStore) ListDoubleMarkSessions(_ context.Context, tenantID string, filter review.DoubleMarkSessionFilter) ([]review.DoubleMarkSession, error) {
	s.lastFilter = filter
	result := make([]review.DoubleMarkSession, 0, len(s.items))
	for _, item := range s.items {
		if item.TenantID == tenantID && item.ExamID == filter.ExamID && item.QuestionID == filter.QuestionID {
			result = append(result, item)
		}
	}
	return result, nil
}
