package modelcalibration

import (
	"context"

	"edugrade-enterprise/services/api-gateway/internal/gradingevaluation"
)

// NewEvaluationReader adapts the A16 service without exposing its persistence
// internals. Calibration can therefore only consume completed, tenant-scoped
// aligned observations that A16 has already accepted.
func NewEvaluationReader(service *gradingevaluation.Service) EvaluationReader {
	return evaluationReader{service: service}
}

type evaluationReader struct{ service *gradingevaluation.Service }

func (r evaluationReader) GetRun(ctx context.Context, tenantID, runID string) (EvaluationRun, error) {
	item, err := r.service.GetRun(ctx, tenantID, runID)
	if err != nil {
		return EvaluationRun{}, err
	}
	return EvaluationRun{ID: item.ID, ModelReference: item.ModelReference, PromptVersion: item.PromptVersion, RubricVersion: item.RubricVersion, Status: string(item.Status)}, nil
}

func (r evaluationReader) ListObservations(ctx context.Context, tenantID, runID string) ([]EvaluationObservation, error) {
	items, err := r.service.ListObservations(ctx, tenantID, runID)
	if err != nil {
		return nil, err
	}
	result := make([]EvaluationObservation, 0, len(items))
	for _, item := range items {
		result = append(result, EvaluationObservation{ResponseKey: item.ResponseKey, Subject: item.Subject, Archetype: item.Archetype,
			OCRQuality: item.OCRQuality, ScoreBand: item.ReferenceScoreBand, ReferenceScore: item.ReferenceScore,
			ModelScore: item.ModelScore, MaxScore: item.MaxScore})
	}
	return result, nil
}
