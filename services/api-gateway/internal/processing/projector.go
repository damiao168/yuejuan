package processing

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ProjectorOptions struct {
	Owner        string
	LeaseTTL     time.Duration
	PollInterval time.Duration
	BaseBackoff  time.Duration
	MaxBackoff   time.Duration
}

type Projector struct {
	store   ProjectionStore
	options ProjectorOptions
}

func NewProjector(store ProjectionStore, options ProjectorOptions) *Projector {
	if options.Owner == "" {
		options.Owner = fmt.Sprintf("processing-projector-%d", time.Now().UTC().UnixNano())
	}
	if options.LeaseTTL <= 0 {
		options.LeaseTTL = 30 * time.Second
	}
	if options.PollInterval <= 0 {
		options.PollInterval = time.Second
	}
	if options.BaseBackoff <= 0 {
		options.BaseBackoff = time.Second
	}
	if options.MaxBackoff <= 0 {
		options.MaxBackoff = 5 * time.Minute
	}
	return &Projector{store: store, options: options}
}

func (p *Projector) RunOnce(ctx context.Context) (bool, error) {
	if p == nil || p.store == nil {
		return false, ErrInvalidInput
	}
	refresh, ok, err := p.store.ClaimProjection(ctx, p.options.Owner, p.options.LeaseTTL)
	if err != nil || !ok {
		return false, err
	}
	if err = p.store.ApplyProjection(ctx, p.options.Owner, refresh); err != nil {
		markErr := p.store.FailProjection(ctx, p.options.Owner, refresh, err.Error(), p.backoff(refresh.AttemptCount))
		return true, errors.Join(err, markErr)
	}
	return true, nil
}

func (p *Projector) Run(ctx context.Context, onError func(error)) {
	for {
		worked, err := p.RunOnce(ctx)
		if err != nil && onError != nil && !errors.Is(err, context.Canceled) {
			onError(err)
		}
		if ctx.Err() != nil {
			return
		}
		if worked {
			continue
		}
		timer := time.NewTimer(p.options.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (p *Projector) backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := p.options.BaseBackoff
	for index := 1; index < attempt && delay < p.options.MaxBackoff; index++ {
		delay *= 2
		if delay >= p.options.MaxBackoff {
			return p.options.MaxBackoff
		}
	}
	return delay
}
