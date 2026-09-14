package server

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/db"
	"edugrade-enterprise/services/api-gateway/internal/deps"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/observability"
	"edugrade-enterprise/services/api-gateway/internal/outbox"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/processing"
)

// Infrastructure owns process-level resources shared by application modules.
// Domain stores, services, and handlers deliberately do not live here.
type Infrastructure struct {
	Config         config.Config
	Logger         *logger.Logger
	DB             *sql.DB
	ObjectStore    files.ReconciliationObjectStorage
	Checkers       []deps.Checker
	Metrics        *observability.Registry
	FileReconciler *files.Reconciler
	LoginGuard     auth.LoginAttemptGuard
	cleanup        []func() error
}

func newInfrastructure(cfg config.Config, logg *logger.Logger) (*Infrastructure, error) {
	infra := &Infrastructure{
		Config:   cfg,
		Logger:   logg,
		Checkers: make([]deps.Checker, 0, 5),
		Metrics:  observability.NewRegistry(),
		cleanup:  make([]func() error, 0, 4),
	}

	postgresDB, closePostgres, err := db.OpenPostgres(cfg.Postgres, db.QueryObserver{
		SlowThreshold: cfg.Observability.SlowRequestThreshold,
		Observe:       infra.Metrics.ObserveDatabaseQuery,
		LogSlow: func(ctx context.Context, operation string, duration time.Duration, queryErr error) {
			logg.Warn(ctx, "slow database query observed", map[string]any{
				"event": "slow_database_query", "operation": operation,
				"duration_ms": duration.Milliseconds(), "failed": queryErr != nil,
			})
		},
	})
	if err != nil {
		return nil, err
	}
	infra.DB = postgresDB
	infra.cleanup = append(infra.cleanup, closePostgres)
	infra.Metrics.SetDatabaseStats(func() observability.DatabaseStats {
		stats := postgresDB.Stats()
		return observability.DatabaseStats{
			OpenConnections: stats.OpenConnections,
			InUse:           stats.InUse,
			Idle:            stats.Idle,
			WaitCount:       stats.WaitCount,
			WaitDuration:    stats.WaitDuration,
		}
	})
	infra.Checkers = append(infra.Checkers, deps.NewPostgresChecker(postgresDB))

	redisChecker, closeRedis := deps.NewRedisChecker(cfg.Redis)
	infra.Checkers = append(infra.Checkers, redisChecker)
	infra.cleanup = append(infra.cleanup, closeRedis)
	infra.LoginGuard = auth.NewRedisLoginAttemptGuard(redisChecker.Client(), cfg.Auth.LoginFailureLimit, cfg.Auth.LoginFailureWindow)

	minioChecker, err := deps.NewMinIOChecker(cfg.MinIO)
	if err != nil {
		infra.Close()
		return nil, err
	}
	infra.Checkers = append(infra.Checkers, minioChecker)
	infra.Checkers = append(infra.Checkers, deps.NewQdrantChecker(cfg.Qdrant))
	infra.Checkers = append(infra.Checkers, deps.NewAIServiceChecker(cfg.AIService, cfg.Service.Environment))

	infra.ObjectStore, err = files.NewMinIOObjectStorage(cfg.MinIO)
	if err != nil {
		infra.Close()
		return nil, err
	}
	infra.FileReconciler = files.NewReconciler(postgresDB, infra.ObjectStore)
	infra.startFileReconciliation()
	infra.startOutboxDispatcher()
	infra.startProcessingProjector()
	return infra, nil
}

func (i *Infrastructure) startFileReconciliation() {
	cfg := i.Config
	if cfg.Files.ReconciliationInterval <= 0 {
		return
	}
	reconciliationContext, stopReconciliation := context.WithCancel(db.WithTenantMaintenance(context.Background()))
	reconciliationDone := make(chan struct{})
	go func() {
		defer close(reconciliationDone)
		ticker := time.NewTicker(cfg.Files.ReconciliationInterval)
		defer ticker.Stop()
		for {
			select {
			case <-reconciliationContext.Done():
				return
			case <-ticker.C:
				run, err := i.FileReconciler.Run(reconciliationContext, files.ReconciliationOptions{
					Bucket: cfg.Files.Bucket, BatchSize: cfg.Files.ReconciliationBatchSize,
					ObjectScanLimit: cfg.Files.ReconciliationObjectLimit,
					StaleAfter:      cfg.Files.ReconciliationStaleAfter, Repair: false,
				})
				if err != nil {
					i.Logger.Error(context.Background(), "file reconciliation report failed", map[string]any{
						"event": "file_reconciliation_failed", "error": err.Error(),
					})
				} else if run.FindingCount > 0 {
					i.Logger.Warn(context.Background(), "file reconciliation findings detected", map[string]any{
						"event": "file_reconciliation_findings", "run_id": run.ID,
						"finding_count": run.FindingCount, "scanned_assets": run.ScannedAssets,
					})
				}
			}
		}
	}()
	i.cleanup = append(i.cleanup, func() error {
		stopReconciliation()
		select {
		case <-reconciliationDone:
		case <-time.After(5 * time.Second):
			return context.DeadlineExceeded
		}
		return nil
	})
}

func (i *Infrastructure) startOutboxDispatcher() {
	dispatcher := outbox.NewDispatcher(
		outbox.NewPostgresStore(i.DB),
		outbox.NewLogPublisher(i.Logger),
		outbox.Options{Owner: i.Config.Service.Name + "-" + time.Now().UTC().Format("20060102T150405.000000000")},
	)
	dispatchContext, stopDispatch := context.WithCancel(db.WithTenantMaintenance(context.Background()))
	dispatchDone := make(chan struct{})
	go func() {
		defer close(dispatchDone)
		dispatcher.Run(dispatchContext, func(err error) {
			i.Logger.Error(context.Background(), "transactional outbox dispatch failed", map[string]any{
				"event": "outbox_dispatch_failed", "error": err.Error(),
			})
		})
	}()
	i.cleanup = append(i.cleanup, func() error {
		stopDispatch()
		select {
		case <-dispatchDone:
		case <-time.After(5 * time.Second):
			return context.DeadlineExceeded
		}
		return nil
	})
}

func (i *Infrastructure) startProcessingProjector() {
	projector := processing.NewProjector(
		processing.NewPostgresStore(i.DB),
		processing.ProjectorOptions{
			Owner:    i.Config.Service.Name + "-processing-" + time.Now().UTC().Format("20060102T150405.000000000"),
			LeaseTTL: 5 * time.Minute,
		},
	)
	projectContext, stopProject := context.WithCancel(db.WithTenantMaintenance(context.Background()))
	projectDone := make(chan struct{})
	go func() {
		defer close(projectDone)
		projector.Run(projectContext, func(err error) {
			i.Logger.Error(context.Background(), "processing projection refresh failed", map[string]any{
				"event": "processing_projection_failed", "error": err.Error(),
			})
		})
	}()
	i.cleanup = append(i.cleanup, func() error {
		stopProject()
		select {
		case <-projectDone:
		case <-time.After(5 * time.Second):
			return context.DeadlineExceeded
		}
		return nil
	})
}

func (i *Infrastructure) startPaperParseExecutor(executor *paper.ParseTaskExecutor) {
	parseContext, stopParse := context.WithCancel(db.WithTenantMaintenance(context.Background()))
	parseDone := make(chan struct{})
	go func() {
		defer close(parseDone)
		executor.Run(parseContext, func(err error) {
			fields := map[string]any{"event": "paper_parse_failed", "error": err.Error(), "error_code": "paper_parse_failed"}
			var executionErr *paper.ParseExecutionError
			if errors.As(err, &executionErr) {
				fields["import_id"] = executionErr.ImportID
				fields["run_id"] = executionErr.RunID
				fields["generation"] = executionErr.Generation
				fields["task_id"] = executionErr.TaskID
				fields["attempt"] = executionErr.Attempt
				fields["error_code"] = executionErr.ErrorCode
				fields["lease_validation"] = executionErr.LeaseValidation
			}
			i.Logger.Error(context.Background(), "durable paper parse failed", fields)
		})
	}()
	i.cleanup = append(i.cleanup, func() error {
		stopParse()
		select {
		case <-parseDone:
		case <-time.After(5 * time.Second):
			return context.DeadlineExceeded
		}
		return nil
	})
}

func (i *Infrastructure) Close() {
	for index := len(i.cleanup) - 1; index >= 0; index-- {
		_ = i.cleanup[index]()
	}
}
