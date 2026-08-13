package qualitydashboard

import (
	"context"

	"edugrade-enterprise/services/api-gateway/internal/backmark"
	"edugrade-enterprise/services/api-gateway/internal/graderdrift"
)

// NewDriftReader adapts A11's aggregate incident view without granting this
// dashboard access to Seed answers, Gold references or reviewer submissions.
func NewDriftReader(service *graderdrift.Service) DriftReader {
	if service == nil {
		return nil
	}
	return driftReader{service: service}
}

type driftReader struct{ service *graderdrift.Service }

func (r driftReader) GetDriftSummary(ctx context.Context, tenantID, examID, questionID string) (DriftSummary, error) {
	items, err := r.service.ListIncidents(ctx, tenantID, graderdrift.IncidentFilter{
		ExamID: examID, QuestionID: questionID, Limit: 500,
	})
	if err != nil {
		return DriftSummary{}, err
	}
	result := DriftSummary{Incidents: make([]Incident, 0, len(items))}
	for _, item := range items {
		if item.Status != graderdrift.IncidentOpen {
			continue
		}
		if item.Severity == graderdrift.SeverityCritical {
			result.OpenCritical++
		} else {
			result.OpenWarnings++
		}
		result.Incidents = append(result.Incidents, Incident{
			ID: item.ID, GraderRef: item.GraderID, Severity: string(item.Severity),
			Status: string(item.Status), Type: string(item.Type),
		})
	}
	return result, nil
}

// NewBackmarkReader exposes only batch progress and score-difference counts.
// It purposefully omits every original/new score and identity from the
// administrator dashboard projection.
func NewBackmarkReader(service *backmark.Service) BackmarkReader {
	if service == nil {
		return nil
	}
	return backmarkReader{service: service}
}

type backmarkReader struct{ service *backmark.Service }

func (r backmarkReader) GetBackmarkSummary(ctx context.Context, tenantID, examID, questionID string) (BackmarkSummary, error) {
	batches, err := r.service.List(ctx, tenantID, examID, questionID)
	if err != nil {
		return BackmarkSummary{}, err
	}
	result := BackmarkSummary{}
	var compared, corrected int
	for _, batch := range batches {
		if batch.Status != backmark.BatchCompleted && batch.Status != backmark.BatchCancelled {
			result.OpenBatches++
		}
		summary, summaryErr := r.service.Get(ctx, tenantID, batch.ID)
		if summaryErr != nil {
			return BackmarkSummary{}, summaryErr
		}
		for _, item := range summary.Items {
			switch item.Status {
			case backmark.ItemPending, backmark.ItemInProgress:
				result.PendingItems++
			default:
				result.CompletedItems++
			}
			if item.Diff != nil {
				compared++
				if *item.Diff != 0 {
					corrected++
				}
			}
		}
	}
	result.CorrectionRate = ratio(corrected, compared)
	return result, nil
}
