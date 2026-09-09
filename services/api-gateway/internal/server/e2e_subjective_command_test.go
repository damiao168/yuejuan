package server

import (
	"context"
	"edugrade-enterprise/services/api-gateway/internal/subjective"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
	"errors"
	"github.com/google/uuid"
	"os"
	"testing"
	"time"
)

func TestSubjectiveCommandRecoveryWithPostgresTestDatabase(t *testing.T) {
	dsn := os.Getenv("EDUGRADE_E2E_DATABASE_URL")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin"})
	router := e2ePostgresRouter(db)
	token := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	fixture := e2eCreateStory056AcceptanceFixture(t, db, router, token, suffix)
	e2eSeedStory056AcceptanceAnswers(t, db, fixture, suffix)
	ctx := context.Background()
	store := subjective.NewPostgresStore(db)
	var segmentID, questionID string
	if err := db.QueryRow(`SELECT seg.id::text,seg.question_id::text FROM answer_segment seg JOIN submission sub ON sub.id=seg.submission_id WHERE sub.exam_id=$1::uuid LIMIT 1`, fixture.ExamID).Scan(&segmentID, &questionID); err != nil {
		t.Fatal(err)
	}
	input := subjective.CreateBatchInput{IdempotencyKey: "subjective-" + suffix, SegmentIDs: []string{segmentID}}
	type outcome struct {
		batch subjective.GradingBatch
		err   error
	}
	gate := make(chan struct{})
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-gate
			b, e := store.CreateBatch(ctx, fixture.TenantID, fixture.AdminID, input)
			results <- outcome{b, e}
		}()
	}
	close(gate)
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || a.batch.ID != b.batch.ID {
		t.Fatalf("concurrent batch: %+v %+v", a, b)
	}
	bad := input
	bad.SegmentIDs = []string{"00000000-0000-0000-0000-000000000001"}
	if _, err := store.CreateBatch(ctx, fixture.TenantID, fixture.AdminID, bad); !errors.Is(err, subjective.ErrIdempotencyConflict) {
		t.Fatalf("changed request: %v", err)
	}
	plan := subjective.BatchEnqueuePlan{CommandID: "enqueue:" + a.batch.ID, BatchID: a.batch.ID, Runs: []subjective.CreateRunInput{{BatchID: a.batch.ID, AnswerSegmentID: segmentID, QuestionID: questionID, AnswerVersion: "v1", RubricVersion: "v1", ModelVersion: "model-v1", PromptVersion: "prompt-v1", MinConfidence: .8, RequestID: "subjective-frozen-" + suffix}}}
	saved, err := store.SaveEnqueuePlan(ctx, fixture.TenantID, fixture.AdminID, plan)
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.GetOrCreateRun(ctx, fixture.TenantID, fixture.AdminID, saved.Runs[0])
	if err != nil {
		t.Fatal(err)
	}
	// Simulate interruption after creating the run, before creating its worker task.
	restarted := subjective.NewPostgresStore(db)
	plan.Runs = append([]subjective.CreateRunInput(nil), plan.Runs...)
	plan.Runs[0].RequestID = "changed-request"
	plan.Runs[0].ModelVersion = "model-v2"
	replay, err := restarted.SaveEnqueuePlan(ctx, fixture.TenantID, fixture.AdminID, plan)
	if err != nil || replay.Runs[0] != saved.Runs[0] {
		t.Fatalf("snapshot replay: %+v %v", replay, err)
	}
	repeated, err := restarted.GetOrCreateRun(ctx, fixture.TenantID, fixture.AdminID, replay.Runs[0])
	if err != nil || repeated.ID != run.ID {
		t.Fatalf("run replay: %+v %v", repeated, err)
	}
	runtime := workerruntime.NewPostgresStore(db)
	taskInput := workerruntime.CreateTaskInput{TaskType: "ai_grade", QueueName: "subjective-grading", SourceType: "subjective_grading_run", SourceID: run.ID, Payload: map[string]any{"run_id": run.ID}, PayloadSchemaVersion: "subjective-grade-v1", IdempotencyKey: saved.Runs[0].RequestID, DedupeKey: "subjective:" + a.batch.ID + ":" + segmentID, MaxAttempts: 3, RetryBackoffSeconds: 30}
	task, err := runtime.CreateTask(ctx, fixture.TenantID, fixture.AdminID, taskInput)
	if err != nil {
		t.Fatal(err)
	}
	again, err := runtime.CreateTask(ctx, fixture.TenantID, fixture.AdminID, taskInput)
	if err != nil || again.ID != task.ID {
		t.Fatalf("task replay: %+v %v", again, err)
	}
	if err = store.CompleteEnqueuePlan(ctx, fixture.TenantID, fixture.AdminID, a.batch.ID); err != nil {
		t.Fatal(err)
	}
	recovered, err := restarted.GetEnqueuePlan(ctx, fixture.TenantID, fixture.AdminID, a.batch.ID)
	if err != nil || !recovered.Completed {
		t.Fatalf("receipt: %+v %v", recovered, err)
	}
	if _, err = db.Exec(`UPDATE subjective_grading_batch SET deleted_at=now() WHERE id=$1::uuid`, a.batch.ID); err != nil {
		t.Fatal(err)
	}
	receipt, err := restarted.RecoverBatchCommand(ctx, fixture.TenantID, fixture.AdminID, input.IdempotencyKey)
	if err != nil || receipt.Batch == nil || receipt.Batch.ID != a.batch.ID {
		t.Fatalf("deleted batch receipt: %+v %v", receipt, err)
	}
	for name, identity := range map[string][2]string{
		"other tenant": {uuid.NewString(), fixture.AdminID},
		"other actor":  {fixture.TenantID, uuid.NewString()},
	} {
		isolated, err := restarted.RecoverBatchCommand(ctx, identity[0], identity[1], input.IdempotencyKey)
		if err != nil || isolated.Status != "not_accepted" || isolated.Batch != nil {
			t.Fatalf("%s batch command scope: %+v %v", name, isolated, err)
		}
	}
	if _, err := restarted.GetEnqueuePlan(ctx, fixture.TenantID, uuid.NewString(), a.batch.ID); !errors.Is(err, subjective.ErrNotFound) {
		t.Fatalf("other actor recovered enqueue plan: %v", err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM subjective_grading_run WHERE tenant_id=$1::uuid AND batch_id=$2::uuid`, fixture.TenantID, a.batch.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("run count %d: %v", count, err)
	}
}
