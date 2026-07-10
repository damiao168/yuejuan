package imagequality_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/imagequality"
)

const tenantID = "tenant-1"

func TestCreateRunsForCurrentSubmissionPages(t *testing.T) {
	store := imagequality.NewMemoryStore()
	runs, err := store.CreateRuns(context.Background(), tenantID, imagequality.CreateRunsInput{
		SubmissionID: "submission-1",
		Profile:      imagequality.DefaultProfile(),
		Pages: []imagequality.PageSource{
			{
				SubmissionPageID:  "page-1",
				PageNo:            1,
				SourceFileAssetID: "file-original-1",
				SourceSHA256:      "source-hash-1",
				DownloadURL:       "/api/v1/files/file-original-1/download",
			},
		},
	})
	if err != nil {
		t.Fatalf("create runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one run, got %d", len(runs))
	}
	run := runs[0]
	if run.ProcessingStatus != imagequality.ProcessingPending {
		t.Fatalf("expected pending processing status, got %s", run.ProcessingStatus)
	}
	if run.QualityStatus != "" {
		t.Fatalf("new run should not have quality conclusion, got %s", run.QualityStatus)
	}
	if run.SourceFileAssetID != "file-original-1" || run.SourceSHA256 != "source-hash-1" {
		t.Fatalf("run did not preserve source file identity: %#v", run)
	}
	if run.ProfileName != "opencv-default" || run.ProfileVersion != "v1" || run.ProfileConfigHash == "" {
		t.Fatalf("run did not preserve immutable profile: %#v", run)
	}
}

func TestClaimUsesLeaseAndSkipsAlreadyClaimedRuns(t *testing.T) {
	store := imagequality.NewMemoryStore()
	runs, err := store.CreateRuns(context.Background(), tenantID, imagequality.CreateRunsInput{
		SubmissionID: "submission-1",
		Profile:      imagequality.DefaultProfile(),
		Pages: []imagequality.PageSource{{
			SubmissionPageID:  "page-1",
			PageNo:            1,
			SourceFileAssetID: "file-original-1",
			SourceSHA256:      "source-hash-1",
			DownloadURL:       "/api/v1/files/file-original-1/download",
		}},
	})
	if err != nil {
		t.Fatalf("create runs: %v", err)
	}

	first, err := store.Claim(context.Background(), tenantID, imagequality.ClaimInput{WorkerInstanceID: "worker-a", Limit: 1, LeaseSeconds: 300})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	second, err := store.Claim(context.Background(), tenantID, imagequality.ClaimInput{WorkerInstanceID: "worker-b", Limit: 1, LeaseSeconds: 300})
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(first) != 1 || len(second) != 0 {
		t.Fatalf("expected first claim only, got first=%d second=%d", len(first), len(second))
	}
	if first[0].RunID != runs[0].ID || first[0].LeaseToken == "" || first[0].AttemptNo != 1 {
		t.Fatalf("claim did not return run lease details: %#v", first[0])
	}
	if !first[0].LeaseExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("lease expiry should be in the future: %s", first[0].LeaseExpiresAt)
	}
}

func TestClaimCanRecoverExpiredLease(t *testing.T) {
	store := imagequality.NewMemoryStore()
	_, err := store.CreateRuns(context.Background(), tenantID, imagequality.CreateRunsInput{
		SubmissionID: "submission-1",
		Profile:      imagequality.DefaultProfile(),
		Pages: []imagequality.PageSource{{
			SubmissionPageID:  "page-1",
			PageNo:            1,
			SourceFileAssetID: "file-original-1",
			SourceSHA256:      "source-hash-1",
			DownloadURL:       "/api/v1/files/file-original-1/download",
		}},
	})
	if err != nil {
		t.Fatalf("create runs: %v", err)
	}
	claimed, err := store.Claim(context.Background(), tenantID, imagequality.ClaimInput{WorkerInstanceID: "worker-a", Limit: 1, LeaseSeconds: 1})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %#v", err, claimed)
	}
	store.ForceExpireLeaseForTest(claimed[0].RunID)

	reclaimed, err := store.Claim(context.Background(), tenantID, imagequality.ClaimInput{WorkerInstanceID: "worker-b", Limit: 1, LeaseSeconds: 300})
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if len(reclaimed) != 1 || reclaimed[0].AttemptNo != 2 || reclaimed[0].LeaseToken == claimed[0].LeaseToken {
		t.Fatalf("expected second attempt with new lease, got %#v", reclaimed)
	}
}

func TestCompleteResultIsIdempotentAndRejectsChangedPayload(t *testing.T) {
	store := imagequality.NewMemoryStore()
	_, err := store.CreateRuns(context.Background(), tenantID, imagequality.CreateRunsInput{
		SubmissionID: "submission-1",
		Profile:      imagequality.DefaultProfile(),
		Pages: []imagequality.PageSource{{
			SubmissionPageID:  "page-1",
			PageNo:            1,
			SourceFileAssetID: "file-original-1",
			SourceSHA256:      "source-hash-1",
			DownloadURL:       "/api/v1/files/file-original-1/download",
		}},
	})
	if err != nil {
		t.Fatalf("create runs: %v", err)
	}
	claimed, err := store.Claim(context.Background(), tenantID, imagequality.ClaimInput{WorkerInstanceID: "worker-a", Limit: 1, LeaseSeconds: 300})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %#v", err, claimed)
	}
	input := imagequality.ResultInput{
		LeaseToken:            claimed[0].LeaseToken,
		AttemptNo:             claimed[0].AttemptNo,
		ResultVersion:         "result-v1",
		DurationMS:            740,
		ProcessingStatus:      imagequality.ProcessingCompleted,
		QualityStatus:         imagequality.QualityPassed,
		NormalizedFileAssetID: "file-normalized-1",
		QualityReport: map[string]any{
			"metric_schema_version": "image-quality-metrics-v1",
			"metrics": map[string]any{
				"sharpness_score": 0.91,
			},
		},
		QualityIssues:          []imagequality.Issue{},
		NormalizationTransform: map[string]any{"source_to_normalized_matrix": [][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}},
	}

	completed, err := store.CompleteRun(context.Background(), tenantID, claimed[0].RunID, input)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	duplicate, err := store.CompleteRun(context.Background(), tenantID, claimed[0].RunID, input)
	if err != nil {
		t.Fatalf("duplicate complete should be idempotent: %v", err)
	}
	if duplicate.ID != completed.ID || duplicate.CompletedAt == nil {
		t.Fatalf("duplicate should return existing completed run: %#v", duplicate)
	}
	stale := input
	stale.LeaseToken = "stale-token"
	_, err = store.CompleteRun(context.Background(), tenantID, claimed[0].RunID, stale)
	if !errors.Is(err, imagequality.ErrLeaseMismatch) {
		t.Fatalf("duplicate completion with stale lease must be rejected, got %v", err)
	}

	changed := input
	changed.QualityStatus = imagequality.QualityReview
	_, err = store.CompleteRun(context.Background(), tenantID, claimed[0].RunID, changed)
	if !errors.Is(err, imagequality.ErrConflict) {
		t.Fatalf("expected conflict for changed result payload, got %v", err)
	}
}

func TestCompleteRunRejectsExpiredOrStaleLease(t *testing.T) {
	store := imagequality.NewMemoryStore()
	_, err := store.CreateRuns(context.Background(), tenantID, imagequality.CreateRunsInput{
		SubmissionID: "submission-1",
		Profile:      imagequality.DefaultProfile(),
		Pages: []imagequality.PageSource{{
			SubmissionPageID:  "page-1",
			PageNo:            1,
			SourceFileAssetID: "file-original-1",
			SourceSHA256:      "source-hash-1",
			DownloadURL:       "/api/v1/files/file-original-1/download",
		}},
	})
	if err != nil {
		t.Fatalf("create runs: %v", err)
	}
	claimed, err := store.Claim(context.Background(), tenantID, imagequality.ClaimInput{WorkerInstanceID: "worker-a", Limit: 1, LeaseSeconds: 300})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %#v", err, claimed)
	}
	store.ForceExpireLeaseForTest(claimed[0].RunID)
	_, err = store.CompleteRun(context.Background(), tenantID, claimed[0].RunID, imagequality.ResultInput{
		LeaseToken:            claimed[0].LeaseToken,
		AttemptNo:             claimed[0].AttemptNo,
		ResultVersion:         "result-v1",
		ProcessingStatus:      imagequality.ProcessingCompleted,
		QualityStatus:         imagequality.QualityPassed,
		NormalizedFileAssetID: "file-normalized-1",
	})
	if !errors.Is(err, imagequality.ErrLeaseExpired) {
		t.Fatalf("expected expired lease error, got %v", err)
	}
}
