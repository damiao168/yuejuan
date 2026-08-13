package releasegate

import (
	"context"
	"errors"
	"fmt"

	"edugrade-enterprise/services/api-gateway/internal/scorerelease"
)

// ScoreReleasePublisher is intentionally the narrow A18 publish operation.
// The coordinator never sees score facts and cannot publish a mutable result.
type ScoreReleasePublisher interface {
	Publish(context.Context, string, string, string) (scorerelease.Release, error)
}

// PublicationCoordinator is the A20 composition point for the existing A18
// endpoint. The host must first resolve releaseID -> examID under the same
// tenant scope, then call this coordinator instead of Store.Publish directly.
// A18's Postgres publish transaction still recomputes its own source gate;
// therefore a concurrent score change can only fail closed, never publish on
// stale evidence.
type PublicationCoordinator struct {
	gates    *Service
	releases ScoreReleasePublisher
}

func NewPublicationCoordinator(gates *Service, releases ScoreReleasePublisher) *PublicationCoordinator {
	return &PublicationCoordinator{gates: gates, releases: releases}
}

func (c *PublicationCoordinator) Publish(ctx context.Context, tenantID, examID, releaseID, actorID string) (scorerelease.Release, Evidence, error) {
	if c == nil || c.gates == nil || c.releases == nil || !validIDs(tenantID, examID, releaseID, actorID) {
		return scorerelease.Release{}, Evidence{}, ErrInvalidInput
	}
	evidence, err := c.gates.RecheckForPublish(ctx, tenantID, examID, releaseID, actorID)
	if err != nil {
		if errors.Is(err, ErrGateBlocked) {
			// Surface A18's common sentinel so its HTTP boundary can return a
			// precise conflict while A20 keeps its append-only evidence.
			return scorerelease.Release{}, evidence, fmt.Errorf("%w: %v", scorerelease.ErrGateBlocked, err)
		}
		return scorerelease.Release{}, evidence, err
	}
	release, err := c.releases.Publish(ctx, tenantID, releaseID, actorID)
	if err != nil {
		return scorerelease.Release{}, evidence, err
	}
	return release, evidence, nil
}
