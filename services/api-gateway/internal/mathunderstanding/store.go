package mathunderstanding

import "context"

type Store interface {
	CreateArtifact(ctx context.Context, tenantID string, input CreateArtifactInput) (Artifact, error)
	GetLatestArtifact(ctx context.Context, tenantID string, answerSegmentID string) (Artifact, error)
	GetArtifact(ctx context.Context, tenantID string, artifactID string) (Artifact, error)
}
