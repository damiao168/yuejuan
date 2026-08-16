package mathunderstanding

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrInvalidPilotGate = errors.New("invalid math pilot gate evaluation")

type PilotGateEvaluation struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	SubjectCode  string            `json:"subject_code"`
	BenchmarkRef string            `json:"benchmark_ref"`
	Metrics      PilotMetrics      `json:"metrics"`
	Policy       PilotPolicy       `json:"policy"`
	Decision     PilotGateDecision `json:"decision"`
	EvaluatedBy  string            `json:"evaluated_by"`
	CreatedAt    time.Time         `json:"created_at"`
}

type PilotGateStore interface {
	CreatePilotGate(context.Context, string, string, string, PilotMetrics, PilotPolicy, string) (PilotGateEvaluation, error)
	ListPilotGates(context.Context, string, string, int) ([]PilotGateEvaluation, error)
}

type MemoryPilotGateStore struct {
	mu    sync.RWMutex
	items []PilotGateEvaluation
}

func NewMemoryPilotGateStore() *MemoryPilotGateStore { return &MemoryPilotGateStore{} }

func (s *MemoryPilotGateStore) CreatePilotGate(_ context.Context, tenantID, subject, benchmarkRef string, metrics PilotMetrics, policy PilotPolicy, actorID string) (PilotGateEvaluation, error) {
	if err := validatePilotGateInput(tenantID, subject, benchmarkRef, metrics, policy, actorID); err != nil {
		return PilotGateEvaluation{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item := PilotGateEvaluation{ID: "math-pilot-" + time.Now().UTC().Format("20060102150405.000000000"), TenantID: tenantID, SubjectCode: subject, BenchmarkRef: benchmarkRef, Metrics: metrics, Policy: policy, Decision: EvaluatePilotGate(subject, metrics, policy), EvaluatedBy: actorID, CreatedAt: time.Now().UTC()}
	s.items = append(s.items, item)
	return item, nil
}

func (s *MemoryPilotGateStore) ListPilotGates(_ context.Context, tenantID, subject string, limit int) ([]PilotGateEvaluation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]PilotGateEvaluation, 0, limit)
	for i := len(s.items) - 1; i >= 0 && len(out) < limit; i-- {
		if s.items[i].TenantID == tenantID && (subject == "" || s.items[i].SubjectCode == subject) {
			out = append(out, s.items[i])
		}
	}
	return out, nil
}

func validatePilotGateInput(tenantID, subject, benchmarkRef string, metrics PilotMetrics, policy PilotPolicy, actorID string) error {
	if tenantID == "" || actorID == "" || benchmarkRef == "" || !set("mathematics", "physics", "chemistry")[subject] || metrics.SampleCount < 0 || policy.MinimumSamples <= 0 {
		return ErrInvalidPilotGate
	}
	values := []float64{metrics.FormulaExactRate, metrics.ASTExactRate, metrics.SpatialRelationF1, metrics.SolutionGraphEdgeF1, metrics.EquivalencePrecision, metrics.RubricEvidencePrecision, metrics.UnsafeSuggestionRate, metrics.RiskyCaseRecall, policy.MinimumFormulaExact, policy.MinimumASTExact, policy.MinimumSpatialF1, policy.MinimumGraphF1, policy.MinimumEquivalencePrecision, policy.MinimumRubricPrecision, policy.MaximumUnsafeSuggestionRate, policy.MinimumRiskyCaseRecall}
	for _, value := range values {
		if value < 0 || value > 1 {
			return ErrInvalidPilotGate
		}
	}
	return nil
}
