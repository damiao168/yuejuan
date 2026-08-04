package handlers

import (
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

func TestBuildOCRWorkerServiceStatusOnline(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	status := buildWorkerServiceStatus(workerruntime.Metrics{
		Workers: []workerruntime.WorkerHeartbeat{{
			WorkerService: "ocr-worker", WorkerInstanceID: "ocr-1", QueueName: "ocr", LastSeenAt: now.Add(-5 * time.Second),
		}},
		Queues: []workerruntime.QueueMetrics{{QueueName: "ocr", Queued: 3, Leased: 1, Running: 2, DeadLetter: 1, FailedLastHour: 2}},
	}, "ocr_worker", "ocr-worker", "ocr", now, 30*time.Second)

	if status.Status != "ok" || status.Availability != "online" || !status.AutomationAvailable || status.FreshInstances != 1 || status.StaleInstances != 0 {
		t.Fatalf("unexpected online worker status: %#v", status)
	}
	if status.QueuedTasks != 3 || status.InFlightTasks != 3 || status.DeadLetterTasks != 1 || status.FailedLastHour != 2 {
		t.Fatalf("worker queue impact metrics missing: %#v", status)
	}
}

func TestBuildOCRWorkerServiceStatusStale(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	status := buildWorkerServiceStatus(workerruntime.Metrics{
		Workers: []workerruntime.WorkerHeartbeat{{
			WorkerService: "ocr-worker", WorkerInstanceID: "ocr-1", QueueName: "ocr", LastSeenAt: now.Add(-31 * time.Second),
		}},
	}, "ocr_worker", "ocr-worker", "ocr", now, 30*time.Second)

	if status.Status != "error" || status.Availability != "stale" || status.AutomationAvailable || status.StaleInstances != 1 || status.ImpactCode != "ocr_automation_unavailable" {
		t.Fatalf("unexpected stale worker status: %#v", status)
	}
}

func TestBuildOCRWorkerServiceStatusNotConfiguredWhenIdle(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	status := buildWorkerServiceStatus(workerruntime.Metrics{}, "ocr_worker", "ocr-worker", "ocr", now, 30*time.Second)

	if status.Status != "not_configured" || status.Availability != "not_configured" || status.AutomationAvailable || status.LastSeenAt != "" {
		t.Fatalf("unexpected idle worker status: %#v", status)
	}
}

func TestBuildOCRWorkerServiceStatusUnavailableWithQueuedWork(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	status := buildWorkerServiceStatus(workerruntime.Metrics{
		Queues: []workerruntime.QueueMetrics{{QueueName: "ocr", Queued: 2}},
	}, "ocr_worker", "ocr-worker", "ocr", now, 30*time.Second)

	if status.Status != "error" || status.Availability != "unavailable" || status.AutomationAvailable || status.QueuedTasks != 2 {
		t.Fatalf("unexpected unavailable worker status: %#v", status)
	}
}
