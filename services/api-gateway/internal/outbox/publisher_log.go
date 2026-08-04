package outbox

import (
	"context"

	"edugrade-enterprise/services/api-gateway/internal/logger"
)

// LogPublisher is the built-in durable sink. External log/alert pipelines can
// consume these structured records, while the full payload remains in Postgres.
type LogPublisher struct{ log *logger.Logger }

func NewLogPublisher(log *logger.Logger) *LogPublisher { return &LogPublisher{log: log} }

func (p *LogPublisher) Publish(ctx context.Context, event Event) error {
	p.log.Info(ctx, "transactional outbox event", map[string]any{
		"event":           "outbox_event",
		"outbox_event_id": event.ID,
		"tenant_id":       event.TenantID,
		"aggregate_type":  event.AggregateType,
		"aggregate_id":    event.AggregateID,
		"event_type":      event.EventType,
		"occurred_at":     event.OccurredAt,
		"attempt_count":   event.AttemptCount,
	})
	return nil
}
