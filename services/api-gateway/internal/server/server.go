package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/aidisagreement"
	"edugrade-enterprise/services/api-gateway/internal/aieligibility"
	"edugrade-enterprise/services/api-gateway/internal/answergroup"
	"edugrade-enterprise/services/api-gateway/internal/appeal"
	"edugrade-enterprise/services/api-gateway/internal/assessment"
	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/backmark"
	"edugrade-enterprise/services/api-gateway/internal/calibration"
	"edugrade-enterprise/services/api-gateway/internal/capture"
	"edugrade-enterprise/services/api-gateway/internal/captureupload"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/dashboard"
	"edugrade-enterprise/services/api-gateway/internal/db"
	"edugrade-enterprise/services/api-gateway/internal/deps"
	"edugrade-enterprise/services/api-gateway/internal/evidence"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/goldpaper"
	"edugrade-enterprise/services/api-gateway/internal/graderdrift"
	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/gradingevaluation"
	"edugrade-enterprise/services/api-gateway/internal/handlers"
	"edugrade-enterprise/services/api-gateway/internal/idempotency"
	"edugrade-enterprise/services/api-gateway/internal/imagequality"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/middleware"
	"edugrade-enterprise/services/api-gateway/internal/modelcalibration"
	"edugrade-enterprise/services/api-gateway/internal/modelgovernance"
	"edugrade-enterprise/services/api-gateway/internal/observability"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/orchestrator"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/outbox"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/processing"
	"edugrade-enterprise/services/api-gateway/internal/qualitydashboard"
	"edugrade-enterprise/services/api-gateway/internal/regrade"
	"edugrade-enterprise/services/api-gateway/internal/regraderelease"
	"edugrade-enterprise/services/api-gateway/internal/releasegate"
	"edugrade-enterprise/services/api-gateway/internal/report"
	"edugrade-enterprise/services/api-gateway/internal/review"
	"edugrade-enterprise/services/api-gateway/internal/reviewannotation"
	"edugrade-enterprise/services/api-gateway/internal/score"
	"edugrade-enterprise/services/api-gateway/internal/scorerelease"
	"edugrade-enterprise/services/api-gateway/internal/seedquality"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/studentportal"
	"edugrade-enterprise/services/api-gateway/internal/subjective"
	"edugrade-enterprise/services/api-gateway/internal/submission"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
	"edugrade-enterprise/services/api-gateway/internal/workspace"
)

type Server struct {
	handler http.Handler
}

// releaseGatePublisher adapts A20's evidence-producing coordinator to A18's
// narrow publish seam. Evidence stays append-only in A20 and is retrieved
// through its scoped endpoint, not echoed as mutable publish input.
type releaseGatePublisher struct {
	coordinator *releasegate.PublicationCoordinator
}

func (p releaseGatePublisher) PublishPublication(ctx context.Context, tenantID, examID, releaseID, actorID string) (scorerelease.Release, error) {
	release, _, err := p.coordinator.Publish(ctx, tenantID, examID, releaseID, actorID)
	return release, err
}

// regradeBlocker projects only publication-relevant state. A20 does not need
// regrade scores, reviewers, or answer material to decide whether publishing
// must wait for a correction plan.
type regradeBlocker struct{ service *regrade.Service }

func (b regradeBlocker) BlockingRegradeCount(ctx context.Context, tenantID, examID string) (int, error) {
	jobs, err := b.service.List(ctx, tenantID, examID, "")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, job := range jobs {
		// A ready plan is frozen and can safely be materialised as its own
		// successor release. All earlier states are still changing evidence and
		// must block public publication.
		if job.Status != regrade.StatusCancelled && job.Status != regrade.StatusReadyForRelease {
			count++
		}
	}
	return count, nil
}

func New(cfg config.Config, logg *logger.Logger) (*Server, func(), error) {
	checkers := make([]deps.Checker, 0, 5)
	cleanups := make([]func() error, 0, 2)
	metricsRegistry := observability.NewRegistry()

	postgresDB, closePostgres, err := db.OpenPostgres(cfg.Postgres, db.QueryObserver{
		SlowThreshold: cfg.Observability.SlowRequestThreshold,
		Observe:       metricsRegistry.ObserveDatabaseQuery,
		LogSlow: func(ctx context.Context, operation string, duration time.Duration, queryErr error) {
			logg.Warn(ctx, "slow database query observed", map[string]any{
				"event": "slow_database_query", "operation": operation,
				"duration_ms": duration.Milliseconds(), "failed": queryErr != nil,
			})
		},
	})
	if err != nil {
		return nil, nil, err
	}
	authStore := auth.NewPostgresStore(postgresDB)
	orgStore := org.NewPostgresStore(postgresDB)
	examStore := exam.NewPostgresStore(postgresDB)
	paperStore := paper.NewPostgresStore(postgresDB)
	fileStore := files.NewPostgresStore(postgresDB)
	submissionStore := submission.NewPostgresStore(postgresDB)
	imageQualityStore := imagequality.NewPostgresStore(postgresDB)
	workerRuntimeStore := workerruntime.NewPostgresStore(postgresDB)
	ocrStore := ocrpkg.NewPostgresStore(postgresDB)
	ocrQueue := ocrpkg.NewMemoryQueue()
	segmentStore := segment.NewPostgresStore(postgresDB)
	orchestratorStore := orchestrator.NewPostgresStore(postgresDB)
	gradingStore := grading.NewPostgresStore(postgresDB)
	subjectiveStore := subjective.NewPostgresStore(postgresDB)
	evidenceStore := evidence.NewPostgresStore(postgresDB)
	reviewStore := review.NewPostgresStore(postgresDB)
	reviewAnnotationStore := reviewannotation.NewPostgresStore(postgresDB)
	goldPaperStore := goldpaper.NewPostgresStore(postgresDB)
	calibrationStore := calibration.NewPostgresStore(postgresDB)
	answerGroupStore := answergroup.NewPostgresStore(postgresDB, nil, answergroup.DefaultPolicy())
	backmarkStore := backmark.NewPostgresStore(postgresDB)
	regradeStore := regrade.NewPostgresStore(postgresDB)
	graderDriftStore := graderdrift.NewPostgresStore(postgresDB)
	seedQualityStore := seedquality.NewPostgresStore(postgresDB)
	scoreStore := score.NewPostgresStore(postgresDB)
	appealStore := appeal.NewPostgresStore(postgresDB)
	publishedQuestionAppealStore := appeal.NewPublishedQuestionAppealPostgresStore(postgresDB)
	reportStore := report.NewPostgresStore(postgresDB)
	captureStore := capture.NewPostgresStoreWithBarcodeKeyring(postgresDB, capture.BarcodeKeyring{ActiveKeyID: cfg.Barcode.ActiveKeyID, Keys: cfg.Barcode.HMACKeys})
	captureUploadStore := captureupload.NewPostgresStore(postgresDB)
	processingStore := processing.NewPostgresStore(postgresDB)
	modelGovernanceStore := modelgovernance.NewPostgresStore(postgresDB)
	assessmentStore := assessment.NewPostgresStore(postgresDB)
	eligibilityStore := aieligibility.NewPostgresStore(postgresDB)
	gradingEvaluationStore := gradingevaluation.NewPostgresStore(postgresDB)
	modelCalibrationStore := modelcalibration.NewPostgresStore(postgresDB)
	aiDisagreementStore := aidisagreement.NewPostgresStore(postgresDB)
	idempotencyStore := idempotency.NewPostgresStore(postgresDB)
	qualityCalibrationService := calibration.NewService(calibrationStore, goldPaperStore)
	qualitySeedService := seedquality.NewService(seedQualityStore, goldPaperStore, qualityCalibrationService, assessmentStore)
	qualityDriftService := graderdrift.NewService(graderDriftStore, qualitySeedService, qualityCalibrationService)
	qualityDashboardService := qualitydashboard.NewService(qualitydashboard.Sources{
		Questions:   qualitydashboard.NewPostgresQuestionReader(postgresDB),
		Gold:        goldPaperStore,
		Calibration: qualitydashboard.NewPostgresCalibrationReader(postgresDB),
		Seeds:       seedQualityStore,
		Groups:      answerGroupStore,
		Review:      reviewStore,
		Drift:       qualitydashboard.NewDriftReader(qualityDriftService),
		Backmark:    qualitydashboard.NewBackmarkReader(backmark.NewService(backmarkStore)),
	})
	scoreReleaseStore := scorerelease.NewPostgresStore(postgresDB, qualityDashboardService)
	releaseGateStore := releasegate.NewPostgresStore(postgresDB)
	studentPortalStore := studentportal.NewPostgresStore(postgresDB)
	metricsRegistry.SetDatabaseStats(func() observability.DatabaseStats {
		stats := postgresDB.Stats()
		return observability.DatabaseStats{
			OpenConnections: stats.OpenConnections,
			InUse:           stats.InUse,
			Idle:            stats.Idle,
			WaitCount:       stats.WaitCount,
			WaitDuration:    stats.WaitDuration,
		}
	})
	if err := modelGovernanceStore.EnsureLocalBaseline(context.Background(), "", localModelBaseline(cfg)); err != nil {
		_ = closePostgres()
		return nil, nil, err
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Service.Environment), "production") && cfg.AIService.Enabled {
		if err := modelGovernanceStore.ValidateProductionReadiness(
			context.Background(),
			modelgovernance.NewEnvironmentSecretResolver(""),
		); err != nil {
			_ = closePostgres()
			return nil, nil, err
		}
	}
	postgresChecker := deps.NewPostgresChecker(postgresDB)
	checkers = append(checkers, postgresChecker)
	cleanups = append(cleanups, closePostgres)

	redisChecker, closeRedis := deps.NewRedisChecker(cfg.Redis)
	checkers = append(checkers, redisChecker)
	cleanups = append(cleanups, closeRedis)

	minioChecker, err := deps.NewMinIOChecker(cfg.MinIO)
	if err != nil {
		return nil, nil, err
	}
	checkers = append(checkers, minioChecker)
	checkers = append(checkers, deps.NewQdrantChecker(cfg.Qdrant))
	checkers = append(checkers, deps.NewAIServiceChecker(cfg.AIService, cfg.Service.Environment))
	objectStore, err := files.NewMinIOObjectStorage(cfg.MinIO)
	if err != nil {
		return nil, nil, err
	}
	fileReconciler := files.NewReconciler(postgresDB, objectStore)
	if cfg.Files.ReconciliationInterval > 0 {
		reconciliationContext, stopReconciliation := context.WithCancel(context.Background())
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
					run, reconcileErr := fileReconciler.Run(reconciliationContext, files.ReconciliationOptions{
						Bucket: cfg.Files.Bucket, BatchSize: cfg.Files.ReconciliationBatchSize,
						ObjectScanLimit: cfg.Files.ReconciliationObjectLimit,
						StaleAfter:      cfg.Files.ReconciliationStaleAfter, Repair: false,
					})
					if reconcileErr != nil {
						logg.Error(context.Background(), "file reconciliation report failed", map[string]any{
							"event": "file_reconciliation_failed", "error": reconcileErr.Error(),
						})
					} else if run.FindingCount > 0 {
						logg.Warn(context.Background(), "file reconciliation findings detected", map[string]any{
							"event": "file_reconciliation_findings", "run_id": run.ID,
							"finding_count": run.FindingCount, "scanned_assets": run.ScannedAssets,
						})
					}
				}
			}
		}()
		cleanups = append(cleanups, func() error {
			stopReconciliation()
			select {
			case <-reconciliationDone:
			case <-time.After(5 * time.Second):
				return context.DeadlineExceeded
			}
			return nil
		})
	}
	outboxDispatcher := outbox.NewDispatcher(
		outbox.NewPostgresStore(postgresDB),
		outbox.NewLogPublisher(logg),
		outbox.Options{Owner: cfg.Service.Name + "-" + time.Now().UTC().Format("20060102T150405.000000000")},
	)
	outboxContext, stopOutbox := context.WithCancel(context.Background())
	outboxDone := make(chan struct{})
	go func() {
		defer close(outboxDone)
		outboxDispatcher.Run(outboxContext, func(dispatchErr error) {
			logg.Error(context.Background(), "transactional outbox dispatch failed", map[string]any{
				"event": "outbox_dispatch_failed", "error": dispatchErr.Error(),
			})
		})
	}()
	cleanups = append(cleanups, func() error {
		stopOutbox()
		select {
		case <-outboxDone:
		case <-time.After(5 * time.Second):
			return context.DeadlineExceeded
		}
		return nil
	})

	loginLimiter := auth.NewRedisLoginFailureLimiter(redisChecker.Client(), cfg.Auth.LoginFailureLimit, cfg.Auth.LoginFailureWindow)
	router := NewRouterComplete(cfg, logg, checkers, authStore, orgStore, examStore, paperStore, fileStore, objectStore, submissionStore, ocrStore, ocrQueue, segmentStore, imageQualityStore, workerRuntimeStore, orchestratorStore, gradingStore, subjectiveStore, evidenceStore, reviewStore, reviewAnnotationStore, goldPaperStore, calibrationStore, answerGroupStore, backmarkStore, regradeStore, graderDriftStore, seedQualityStore, scoreStore, scoreReleaseStore, releaseGateStore, studentPortalStore, appealStore, publishedQuestionAppealStore, reportStore, captureStore, captureUploadStore, processingStore, modelGovernanceStore, assessmentStore, eligibilityStore, gradingEvaluationStore, modelCalibrationStore, aiDisagreementStore, idempotencyStore, fileReconciler, loginLimiter, metricsRegistry, qualityDashboardService)
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			_ = cleanups[i]()
		}
	}
	return &Server{handler: router}, cleanup, nil
}

func NewRouter(cfg config.Config, logg *logger.Logger, checkers []deps.Checker, authStore auth.Store, orgStore org.Store, examStore exam.Store, paperStore paper.Store) http.Handler {
	return NewRouterWithFiles(cfg, logg, checkers, authStore, orgStore, examStore, paperStore, files.NewMemoryStore(), files.NewMemoryObjectStorage())
}

func NewRouterWithFiles(cfg config.Config, logg *logger.Logger, checkers []deps.Checker, authStore auth.Store, orgStore org.Store, examStore exam.Store, paperStore paper.Store, fileStore files.Store, objectStore files.ObjectStorage) http.Handler {
	return NewRouterFull(cfg, logg, checkers, authStore, orgStore, examStore, paperStore, fileStore, objectStore, submission.NewMemoryStore())
}

func NewRouterFull(cfg config.Config, logg *logger.Logger, checkers []deps.Checker, authStore auth.Store, orgStore org.Store, examStore exam.Store, paperStore paper.Store, fileStore files.Store, objectStore files.ObjectStorage, submissionStore submission.Store) http.Handler {
	return NewRouterComplete(cfg, logg, checkers, authStore, orgStore, examStore, paperStore, fileStore, objectStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), segment.NewMemoryStore())
}

func NewRouterComplete(cfg config.Config, logg *logger.Logger, checkers []deps.Checker, authStore auth.Store, orgStore org.Store, examStore exam.Store, paperStore paper.Store, fileStore files.Store, objectStore files.ObjectStorage, submissionStore submission.Store, ocrStore ocrpkg.Store, ocrQueue ocrpkg.Queue, segmentStore segment.Store, optionalStores ...any) http.Handler {
	mux := http.NewServeMux()
	h := handlers.New(cfg, checkers)
	var loginLimiter auth.LoginLimiter
	var metricsRegistry *observability.Registry
	var fileReconciliationReader files.ReconciliationReader
	var qualityDashboardService *qualitydashboard.Service
	for _, optionalStore := range optionalStores {
		if limiter, ok := optionalStore.(auth.LoginLimiter); ok && limiter != nil {
			loginLimiter = limiter
		}
		if registry, ok := optionalStore.(*observability.Registry); ok && registry != nil {
			metricsRegistry = registry
		}
		if reader, ok := optionalStore.(files.ReconciliationReader); ok && reader != nil {
			fileReconciliationReader = reader
		}
	}
	if metricsRegistry == nil {
		metricsRegistry = observability.NewRegistry()
	}
	authHandler := auth.NewHandler(authStore, cfg.Auth.SessionTTL, auth.HandlerOptions{
		LoginFailureLimit:    cfg.Auth.LoginFailureLimit,
		LoginFailureWindow:   cfg.Auth.LoginFailureWindow,
		RememberedSessionTTL: cfg.Auth.RememberedSessionTTL,
		CookieName:           cfg.Auth.SessionCookieName,
		CookieSecure:         cfg.Auth.SessionCookieSecure,
		LoginLimiter:         loginLimiter,
		TrustedProxyCIDRs:    cfg.Security.TrustedProxyCIDRs,
	})
	orgHandler := org.NewHandler(orgStore, authStore)
	examHandler := exam.NewHandler(examStore, authStore)
	paperHandler := paper.NewHandler(paperStore, authStore)
	fileHandler := files.NewHandler(fileStore, objectStore, authStore, cfg.Files).WithReconciliationReader(fileReconciliationReader)
	submissionHandler := submission.NewHandler(submissionStore, fileStore, authStore)
	segmentHandler := segment.NewHandler(segmentStore, paperStore, submissionStore, authStore, fileStore, objectStore)
	var imageQualityStore imagequality.Store = imagequality.NewMemoryStore()
	var workerRuntimeStore workerruntime.Store = workerruntime.NewMemoryStore()
	var orchestratorStore orchestrator.Store = orchestrator.NewMemoryStore()
	var gradingStore grading.Store = grading.NewMemoryStore()
	var subjectiveStore subjective.Store = subjective.NewMemoryStore()
	var evidenceStore evidence.Store = evidence.NewMemoryStore()
	var reviewStore review.Store = review.NewMemoryStore()
	var reviewAnnotationStore reviewannotation.Store = reviewannotation.NewMemoryStore()
	var goldPaperStore goldpaper.Store = goldpaper.NewMemoryStore()
	var calibrationStore calibration.Store = calibration.NewMemoryStore()
	var answerGroupStore answergroup.Store = answergroup.NewMemoryStore(nil, answergroup.DefaultPolicy())
	var backmarkStore backmark.Store = backmark.NewMemoryStore()
	var regradeStore regrade.Store = regrade.NewMemoryStore()
	var graderDriftStore graderdrift.Store = graderdrift.NewMemoryStore()
	var seedQualityStore seedquality.Store = seedquality.NewMemoryStore()
	var scoreStore score.Store = score.NewMemoryStore()
	var scoreReleaseStore scorerelease.Store = scorerelease.NewMemoryStore()
	var releaseGateStore releasegate.Store = releasegate.NewMemoryStore()
	var studentPortalStore studentportal.Store = studentportal.NewMemoryStore()
	var appealStore appeal.Store = appeal.NewMemoryStore()
	var publishedQuestionAppealStore appeal.PublishedQuestionAppealStore = appeal.NewPublishedQuestionAppealMemoryStore()
	var reportStore report.Store = report.NewMemoryStore()
	var captureStore capture.Store = capture.NewMemoryStore()
	var captureUploadStore captureupload.Store = captureupload.NewMemoryStore()
	var processingStore processing.Store = processing.NewMemoryStore()
	var modelGovernanceStore modelgovernance.Store = modelgovernance.NewMemoryStore()
	var assessmentStore assessment.Store = assessment.NewMemoryStore()
	var eligibilityStore aieligibility.Store
	var gradingEvaluationStore gradingevaluation.Store = gradingevaluation.NewMemoryStore()
	var modelCalibrationStore modelcalibration.Store = modelcalibration.NewMemoryStore()
	var aiDisagreementStore aidisagreement.Store = aidisagreement.NewMemoryStore()
	var idempotencyStore idempotency.Store = idempotency.NewMemoryStore()
	for _, optionalStore := range optionalStores {
		switch store := optionalStore.(type) {
		case imagequality.Store:
			if store != nil {
				imageQualityStore = store
			}
		case workerruntime.Store:
			if store != nil {
				workerRuntimeStore = store
			}
		case orchestrator.Store:
			if store != nil {
				orchestratorStore = store
			}
		case grading.Store:
			if store != nil {
				gradingStore = store
			}
		case subjective.Store:
			if store != nil {
				subjectiveStore = store
			}
		case evidence.Store:
			if store != nil {
				evidenceStore = store
			}
		case review.Store:
			if store != nil {
				reviewStore = store
			}
		case reviewannotation.Store:
			if store != nil {
				reviewAnnotationStore = store
			}
		case goldpaper.Store:
			if store != nil {
				goldPaperStore = store
			}
		case calibration.Store:
			if store != nil {
				calibrationStore = store
			}
		case answergroup.Store:
			if store != nil {
				answerGroupStore = store
			}
		case backmark.Store:
			if store != nil {
				backmarkStore = store
			}
		case regrade.Store:
			if store != nil {
				regradeStore = store
			}
		case graderdrift.Store:
			if store != nil {
				graderDriftStore = store
			}
		case seedquality.Store:
			if store != nil {
				seedQualityStore = store
			}
		case score.Store:
			if store != nil {
				scoreStore = store
			}
		case scorerelease.Store:
			if store != nil {
				scoreReleaseStore = store
			}
		case releasegate.Store:
			if store != nil {
				releaseGateStore = store
			}
		case studentportal.Store:
			if store != nil {
				studentPortalStore = store
			}
		case appeal.Store:
			if store != nil {
				appealStore = store
			}
		case appeal.PublishedQuestionAppealStore:
			if store != nil {
				publishedQuestionAppealStore = store
			}
		case report.Store:
			if store != nil {
				reportStore = store
			}
		case capture.Store:
			if store != nil {
				captureStore = store
			}
		case captureupload.Store:
			if store != nil {
				captureUploadStore = store
			}
		case processing.Store:
			if store != nil {
				processingStore = store
			}
		case modelgovernance.Store:
			if store != nil {
				modelGovernanceStore = store
			}
		case assessment.Store:
			if store != nil {
				assessmentStore = store
			}
		case aieligibility.Store:
			if store != nil {
				eligibilityStore = store
			}
		case gradingevaluation.Store:
			if store != nil {
				gradingEvaluationStore = store
			}
		case modelcalibration.Store:
			if store != nil {
				modelCalibrationStore = store
			}
		case aidisagreement.Store:
			if store != nil {
				aiDisagreementStore = store
			}
		case idempotency.Store:
			if store != nil {
				idempotencyStore = store
			}
		case *qualitydashboard.Service:
			if store != nil {
				qualityDashboardService = store
			}
		}
	}
	h.WithWorkerRuntimeStore(workerRuntimeStore)
	orchestratorHandler := orchestrator.NewHandler(orchestratorStore, authStore)
	ocrHandler := ocrpkg.NewHandler(ocrStore, ocrQueue, submissionStore, authStore, workerRuntimeStore)
	imageQualityHandler := imagequality.NewHandler(imageQualityStore, submissionStore, fileStore, authStore, workerRuntimeStore).WithCaptureStore(captureStore)
	workerRuntimeHandler := workerruntime.NewHandler(workerRuntimeStore, authStore, workerSourceLeaseRenewer{imageQuality: imageQualityStore})
	captureHandler := capture.NewHandler(captureStore, fileStore, examStore, workerRuntimeStore, authStore)
	var captureUploadHandler *captureupload.Handler
	if lifecycleFiles, ok := fileStore.(files.LifecycleStore); ok {
		captureUploadHandler = captureupload.NewHandler(captureupload.NewService(captureUploadStore, captureStore, lifecycleFiles, objectStore, cfg.Files), authStore)
	}
	processingService := processing.NewService(processingStore, workerRuntimeStore)
	processingHandler := processing.NewHandler(processingService, authStore)
	gradingHandler := grading.NewHandler(gradingStore, grading.NewEngine(), authStore)
	gradingHandler.SetProductionDependencies(workerRuntimeStore, fileStore)
	var subjectiveAdapter subjective.LLMGradingAdapter
	if useRealAIService(cfg) {
		subjectiveAdapter = subjective.NewHTTPAdapter(subjective.HTTPAdapterConfig{
			BaseURL:           cfg.AIService.URL,
			Token:             cfg.AIService.Token,
			Timeout:           cfg.AIService.Timeout,
			MaxRetries:        cfg.AIService.MaxRetries,
			ModelVersion:      cfg.AIService.ModelVersion,
			PromptVersion:     cfg.AIService.PromptVersion,
			MinConfidence:     cfg.AIService.MinConfidence,
			ProviderKey:       cfg.AIService.ProviderKey,
			DeploymentKey:     cfg.AIService.DeploymentKey,
			AdapterType:       cfg.AIService.AdapterType,
			DeploymentRegion:  cfg.AIService.DeploymentRegion,
			CapabilityProfile: cfg.AIService.CapabilityProfile,
		})
	} else if allowMockAI(cfg) {
		subjectiveAdapter = subjective.NewMockLLMAdapter()
	} else {
		subjectiveAdapter = subjective.NewDisabledAdapter("ai_grading_disabled", cfg.AIService.ModelVersion, cfg.AIService.PromptVersion)
	}
	gradingEvaluationService := gradingevaluation.NewService(gradingEvaluationStore)
	modelCalibrationService := modelcalibration.NewService(modelCalibrationStore, modelcalibration.NewEvaluationReader(gradingEvaluationService))
	subjectiveHandler := subjective.NewHandler(subjectiveStore, subjectiveAdapter, authStore).WithWorkerRuntimeStore(workerRuntimeStore).WithEvaluationEvidence(gradingEvaluationService).WithCalibrationEvidence(modelCalibrationService).WithParserQuality(processingService)
	var eligibilityService *aieligibility.Service
	var eligibilityHandler *aieligibility.Handler
	if eligibilityStore != nil {
		eligibilityService = aieligibility.NewService(eligibilityStore)
		eligibilityHandler = aieligibility.NewHandler(eligibilityService)
		subjectiveHandler.WithEligibilityGate(eligibilityService)
	}
	gradingEvaluationHandler := gradingevaluation.NewHandler(gradingEvaluationService)
	modelCalibrationHandler := modelcalibration.NewHandler(modelCalibrationService)
	aiDisagreementService := aidisagreement.NewService(aiDisagreementStore)
	aiDisagreementHandler := aidisagreement.NewHandler(aiDisagreementService)
	evidenceHandler := evidence.NewHandler(evidenceStore, evidence.NewEngine(), authStore)
	calibrationService := calibration.NewService(calibrationStore, goldPaperStore)
	calibrationHandler := calibration.NewHandler(calibrationService, authStore)
	seedQualityService := seedquality.NewService(seedQualityStore, goldPaperStore, calibrationService, assessmentStore)
	seedQualityHandler := seedquality.NewHandler(seedQualityService, authStore)
	graderDriftService := graderdrift.NewService(graderDriftStore, seedQualityService, calibrationService)
	graderDriftHandler := graderdrift.NewHandler(graderDriftService, authStore)
	backmarkService := backmark.NewService(backmarkStore)
	if contextStore, ok := reviewStore.(review.TaskContextStore); ok {
		backmarkService.WithContextSource(contextStore)
	}
	backmarkService.WithTaskSource(reviewStore)
	regradeService := regrade.NewService(regradeStore)
	if contextStore, ok := regradeStore.(regrade.ContextSource); ok {
		regradeService.WithContextSource(contextStore)
	}
	regradeHandler := regrade.NewHandler(regradeService, authStore).WithSegmentImage(segmentHandler.GetImage)
	backmarkHandler := backmark.NewHandler(backmarkService, authStore).WithSegmentImage(segmentHandler.GetImage).WithRegradeService(regradeService)
	var qualityDashboardHandler *qualitydashboard.Handler
	if qualityDashboardService != nil {
		qualityDashboardHandler = qualitydashboard.NewHandler(qualityDashboardService)
	}
	reviewHandler := review.NewHandler(reviewStore, authStore, segmentHandler.GetImage, fileHandler.Download).WithQualificationGate(calibrationService).WithSeedHook(seedQualityService).WithSeedObservationRefresher(graderDriftService).WithAIHumanDisagreementObserver(aiDisagreementService)
	reviewAnnotationHandler := reviewannotation.NewHandler(reviewAnnotationStore, authStore)
	goldPaperHandler := goldpaper.NewHandler(goldPaperStore, authStore)
	answerGroupHandler := answergroup.NewHandlerWithReferences(answerGroupStore, authStore, goldPaperStore)
	scoreHandler := score.NewHandler(scoreStore, authStore)
	scoreReleaseService := scorerelease.NewService(scoreReleaseStore)
	releaseGateService := releasegate.NewService(releaseGateStore, scoreReleaseService).WithRegradeBlockerReader(regradeBlocker{service: regradeService})
	releaseGateHandler := releasegate.NewHandler(releaseGateService, authStore)
	scoreReleaseHandler := scorerelease.NewHandler(scoreReleaseService, authStore).WithPublicationPublisher(releaseGatePublisher{
		coordinator: releasegate.NewPublicationCoordinator(releaseGateService, scoreReleaseService),
	}).WithStudentQuestionImage(segmentHandler.GetImage)
	studentPortalHandler := studentportal.NewHandler(studentportal.NewService(studentPortalStore))
	regradeReleaseHandler := regraderelease.NewHandler(regraderelease.NewService(regradeService, scoreReleaseService))
	appealHandler := appeal.NewHandler(appealStore, authStore)
	publishedQuestionAppealHandler := appeal.NewPublishedQuestionAppealHandler(appeal.NewPublishedQuestionAppealService(publishedQuestionAppealStore), authStore).WithSegmentImage(segmentHandler.GetImage)
	reportHandler := report.NewHandler(reportStore, authStore)
	modelGovernanceHandler := modelgovernance.NewHandler(
		modelGovernanceStore,
		authStore,
		modelgovernance.NewEnvironmentSecretResolver(""),
		localModelBaseline(cfg),
	).WithRuntimePromptSource(modelgovernance.NewHTTPRuntimePromptSource(
		cfg.AIService.URL,
		cfg.AIService.Token,
		cfg.AIService.Timeout,
	))
	assessmentHandler := assessment.NewHandler(assessmentStore, authStore)
	workspaceHandler := workspace.NewHandler(workspace.Dependencies{
		Exams: examStore, Papers: paperStore, Submissions: submissionStore, Reviews: reviewStore, Assessments: assessmentStore, Processing: processingService,
	})
	dashboardHandler := dashboard.NewHandler(dashboard.Dependencies{
		Exams:       examStore,
		Submissions: submissionStore,
		Reviews:     reviewStore,
		Audits:      authStore,
	})
	authenticate := auth.AuthMiddleware(authStore, auth.HandlerOptions{CookieName: cfg.Auth.SessionCookieName})
	environment := strings.ToLower(strings.TrimSpace(cfg.Service.Environment))
	idempotent := idempotency.Middleware(idempotencyStore, idempotency.Options{Enforce: environment == "production" || environment == "staging"})
	requireAuth := func(handler http.Handler) http.Handler {
		return authenticate(idempotent(handler))
	}
	requireOrgManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("org:manage")(handler))
	}
	requireTenantManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("tenant:manage")(handler))
	}
	requireStudentImport := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("student:import")(handler))
	}
	requireExamManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("exam:manage")(handler))
	}
	requireAssessmentRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission("exam:manage", "review:manage", "review:work")(handler))
	}
	requireDashboardRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("exam:manage")(handler))
	}
	requireFileManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("file:manage")(handler))
	}
	requireSubmissionManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("submission:manage")(handler))
	}
	requireCaptureManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("capture:manage")(handler))
	}
	requireOCRManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("ocr:manage")(handler))
	}
	requireSegmentManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("segment:manage")(handler))
	}
	requireSegmentEvidenceRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyRole("platform_admin", "tenant_admin", "school_admin", "page_processing_worker")(
			auth.RequireAnyPermission("segment:manage", "ocr:manage", "grading:manage", "evidence:manage", "review:manage", "arbitration:manage")(handler),
		))
	}
	requireOrchestratorManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("orchestrator:manage")(handler))
	}
	requireGradingManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("grading:manage")(handler))
	}
	requireEvidenceManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("evidence:manage")(handler))
	}
	requireReviewManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("review:manage")(handler))
	}
	requireReviewWork := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission("review:manage", "review:work")(handler))
	}
	requireOriginalReviewImage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyRole("platform_admin", "tenant_admin", "school_admin")(
			auth.RequireAnyPermission("review:manage", "evidence:manage", "tenant:manage")(handler),
		))
	}
	requireArbitrationManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("arbitration:manage")(handler))
	}
	requireArbitrationWork := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission("arbitration:manage", "arbitration:work")(handler))
	}
	requireScoreManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("score:manage")(handler))
	}
	requireRosterManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyRole("platform_admin", "tenant_admin", "school_admin")(
			auth.RequirePermission("score:manage")(handler),
		))
	}
	requireStudentGradeAccess := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission("student:grade:read", "score:manage")(handler))
	}
	requireAppealCreate := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("appeal:create")(handler))
	}
	requireAppealRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("appeal:read")(handler))
	}
	requireAppealManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("appeal:manage")(handler))
	}
	requireAppealWork := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("appeal:work")(handler))
	}
	requireQuestionAppealRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission("student:grade:read", "appeal:read", "appeal:work", "appeal:manage")(handler))
	}
	requireAuditRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("audit:read")(handler))
	}
	requireAuditExport := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("audit:export")(handler))
	}
	requireReportRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("report:read")(handler))
	}
	requireReportExport := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("report:export")(handler))
	}
	requireSystemRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("system:read")(handler))
	}
	requireModelRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("model:read")(handler))
	}
	requireModelProviderManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("model:provider:manage")(handler))
	}
	requireModelPolicyManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("model:policy:manage")(handler))
	}
	requireModelEvaluationManage := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequirePermission("model:evaluation:manage")(handler))
	}
	requireOCRAvailabilityRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission(
			"review:work",
			"review:manage",
			"submission:manage",
			"capture:manage",
			"ocr:manage",
			"grading:manage",
			"system:read",
		)(handler))
	}
	requireWorkerExecute := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission("ocr:manage", "orchestrator:manage")(handler))
	}
	requireWorkerRead := func(handler http.HandlerFunc) http.Handler {
		return requireAuth(auth.RequireAnyPermission("ocr:manage", "orchestrator:manage", "system:read")(handler))
	}
	withScopedExam := func(handler http.HandlerFunc) http.HandlerFunc {
		return auth.RequireScopedResource("exam", "examId")(handler).ServeHTTP
	}
	withWorkerTaskScope := func(handler http.HandlerFunc) http.HandlerFunc {
		return workerruntime.TaskScope(workerRuntimeStore)(handler).ServeHTTP
	}
	withWorkerTaskSource := func(sourceType string, pathParam string, handler http.HandlerFunc) http.HandlerFunc {
		guarded := workerruntime.RequireTaskSource(sourceType, pathParam)(handler)
		return workerruntime.TaskScope(workerRuntimeStore)(guarded).ServeHTTP
	}
	withWorkerTaskPayload := func(pathParam string, payloadKey string, handler http.HandlerFunc) http.HandlerFunc {
		guarded := workerruntime.RequireTaskPayloadValue(pathParam, payloadKey)(handler)
		return workerruntime.TaskScope(workerRuntimeStore)(guarded).ServeHTTP
	}
	withWorkerTaskFile := func(pathParam string, handler http.HandlerFunc) http.HandlerFunc {
		guarded := workerruntime.RequireTaskFile(pathParam)(handler)
		return workerruntime.TaskScope(workerRuntimeStore)(guarded).ServeHTTP
	}

	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /health/live", h.Health)
	mux.HandleFunc("GET /health/ready", h.Ready)
	mux.Handle("GET /metrics", metricsRegistry)
	// /ready remains permission-protected for backward compatibility. New
	// infrastructure probes must use the public, redacted /health/ready route.
	mux.Handle("GET /ready", requireSystemRead(h.Ready))
	mux.Handle("GET /api/v1/system/info", requireSystemRead(h.SystemInfo))
	mux.Handle("GET /api/v1/system/status", requireSystemRead(h.SystemStatus))
	mux.Handle("GET /api/v1/ocr/availability", requireOCRAvailabilityRead(h.OCRAvailability))
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/token", authHandler.TokenLogin)
	mux.Handle("POST /api/v1/auth/logout", requireAuth(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("POST /api/v1/auth/password", requireAuth(http.HandlerFunc(authHandler.ChangePassword)))
	mux.Handle("GET /api/v1/auth/me", requireAuth(http.HandlerFunc(authHandler.Me)))
	mux.Handle("GET /api/v1/auth/sessions", requireAuth(http.HandlerFunc(authHandler.ListSessions)))
	mux.Handle("DELETE /api/v1/auth/sessions/{id}", requireAuth(http.HandlerFunc(authHandler.RevokeSession)))
	mux.Handle("POST /api/v1/auth/logout-all", requireAuth(http.HandlerFunc(authHandler.LogoutAll)))
	mux.Handle("DELETE /api/v1/users/{id}/sessions", requireAuth(auth.RequirePermission("session:revoke")(http.HandlerFunc(authHandler.AdminRevokeUserSessions))))
	dashboard.RegisterRoutes(mux, dashboardHandler, requireDashboardRead)
	mux.Handle("GET /api/v1/ai-grading/status", requireAuth(auth.RequireAnyPermission("grading:manage", "review:work", "review:manage", "model:read", "system:read")(http.HandlerFunc(subjectiveHandler.Availability))))
	mux.Handle("GET /api/v1/users", requireOrgManage(authHandler.ListManagedUsers))
	mux.Handle("POST /api/v1/users", requireOrgManage(authHandler.CreateManagedUser))
	mux.Handle("GET /api/v1/roles", requireOrgManage(authHandler.ListAssignableRoles))
	mux.Handle("GET /api/v1/audit-logs", requireAuditRead(authHandler.ListAudits))
	mux.Handle("POST /api/v1/audit-logs/export", requireAuditExport(authHandler.ExportAudits))
	mux.Handle("GET /api/v1/model-providers", requireModelRead(modelGovernanceHandler.ListProviders))
	mux.Handle("POST /api/v1/model-providers", requireModelProviderManage(modelGovernanceHandler.CreateProvider))
	mux.Handle("PATCH /api/v1/model-providers/{id}/status", requireModelProviderManage(modelGovernanceHandler.UpdateProviderStatus))
	mux.Handle("GET /api/v1/model-deployments", requireModelRead(modelGovernanceHandler.ListDeployments))
	mux.Handle("POST /api/v1/model-deployments", requireModelProviderManage(modelGovernanceHandler.CreateDeployment))
	mux.Handle("PATCH /api/v1/model-deployments/{id}/state", requireModelProviderManage(modelGovernanceHandler.UpdateDeploymentState))
	mux.Handle("GET /api/v1/model-policy", requireModelRead(modelGovernanceHandler.GetPolicy))
	mux.Handle("GET /api/v1/model-prompts/current", requireModelRead(modelGovernanceHandler.GetCurrentPrompt))
	mux.Handle("PUT /api/v1/model-policy", requireModelPolicyManage(modelGovernanceHandler.UpdatePolicy))
	mux.Handle("POST /api/v1/model-secrets/probe", requireModelProviderManage(modelGovernanceHandler.ProbeSecret))
	mux.Handle("GET /api/v1/model-sandbox-approvals", requireModelRead(modelGovernanceHandler.ListSandboxApprovals))
	mux.Handle("POST /api/v1/model-sandbox-approvals", requireModelProviderManage(modelGovernanceHandler.CreateSandboxApproval))
	mux.Handle("POST /api/v1/model-sandbox-approvals/{id}/revoke", requireModelProviderManage(modelGovernanceHandler.RevokeSandboxApproval))
	mux.Handle("GET /api/v1/model-evaluation-runs", requireModelRead(modelGovernanceHandler.ListEvaluationRuns))
	mux.Handle("POST /api/v1/model-evaluation-runs", requireModelEvaluationManage(modelGovernanceHandler.CreateEvaluationRun))
	mux.Handle("POST /api/v1/model-evaluation-runs/{id}/candidates", requireModelEvaluationManage(modelGovernanceHandler.AddEvaluationCandidate))
	mux.Handle("POST /api/v1/model-evaluation-runs/{id}/complete", requireModelEvaluationManage(modelGovernanceHandler.CompleteEvaluationRun))
	mux.Handle("POST /api/v1/model-evaluation-runs/{id}/invalidate", requireModelEvaluationManage(modelGovernanceHandler.InvalidateEvaluationRun))
	mux.Handle("GET /api/v1/model-approvals", requireModelRead(modelGovernanceHandler.ListModelApprovals))
	mux.Handle("POST /api/v1/model-approvals", requireModelEvaluationManage(modelGovernanceHandler.CreateModelApproval))
	mux.Handle("POST /api/v1/model-approvals/{id}/revoke", requireModelEvaluationManage(modelGovernanceHandler.RevokeModelApproval))
	if eligibilityHandler != nil {
		aieligibility.RegisterRoutes(mux, eligibilityHandler, requireModelRead, requireModelPolicyManage)
	}
	gradingevaluation.RegisterRoutes(mux, gradingEvaluationHandler, requireModelEvaluationManage)
	modelcalibration.RegisterRoutes(mux, modelCalibrationHandler, requireModelEvaluationManage)
	aidisagreement.RegisterRoutes(mux, aiDisagreementHandler, requireReviewWork, requireReviewManage)

	mux.Handle("POST /api/v1/tenants", requireTenantManage(orgHandler.CreateTenant))
	mux.Handle("GET /api/v1/tenants", requireAuth(http.HandlerFunc(orgHandler.ListTenants)))
	mux.Handle("PATCH /api/v1/tenants/{id}", requireTenantManage(orgHandler.UpdateTenant))
	mux.Handle("POST /api/v1/schools", requireOrgManage(orgHandler.CreateSchool))
	mux.Handle("GET /api/v1/schools", requireOrgManage(orgHandler.ListSchools))
	mux.Handle("POST /api/v1/grades", requireOrgManage(orgHandler.CreateGrade))
	mux.Handle("GET /api/v1/grades", requireOrgManage(orgHandler.ListGrades))
	mux.Handle("POST /api/v1/classes", requireOrgManage(orgHandler.CreateClass))
	mux.Handle("GET /api/v1/classes", requireOrgManage(orgHandler.ListClasses))
	mux.Handle("POST /api/v1/students", requireOrgManage(orgHandler.CreateStudent))
	mux.Handle("GET /api/v1/students", requireOrgManage(orgHandler.ListStudents))
	mux.Handle("PATCH /api/v1/students/{id}", requireOrgManage(orgHandler.UpdateStudent))
	mux.Handle("POST /api/v1/students/import-csv", requireStudentImport(orgHandler.ImportStudentsCSV))
	mux.Handle("POST /api/v1/classes/{id}/teachers", requireOrgManage(orgHandler.BindTeacherClass))

	mux.Handle("POST /api/v1/exams", requireExamManage(examHandler.CreateExam))
	mux.Handle("GET /api/v1/exams", requireExamManage(examHandler.ListExams))
	mux.Handle("GET /api/v1/exams/{id}", requireExamManage(examHandler.GetExam))
	mux.Handle("PATCH /api/v1/exams/{id}", requireExamManage(examHandler.UpdateExam))
	mux.Handle("POST /api/v1/exams/{id}/archive", requireExamManage(examHandler.Archive))
	mux.Handle("POST /api/v1/exams/{id}/status", requireExamManage(examHandler.UpdateStatus))
	workspace.RegisterRoutes(mux, workspaceHandler, requireDashboardRead)
	mux.Handle("GET /api/v1/assessment/subject-profiles", requireAssessmentRead(assessmentHandler.ListSubjectProfiles))
	mux.Handle("GET /api/v1/assessment/question-archetypes", requireAssessmentRead(assessmentHandler.ListQuestionArchetypes))
	mux.Handle("GET /api/v1/exams/{examId}/questions/{questionId}/assessment-profile", requireAssessmentRead(withScopedExam(assessmentHandler.GetQuestionConfig)))
	mux.Handle("PUT /api/v1/exams/{examId}/questions/{questionId}/assessment-profile", requireExamManage(withScopedExam(assessmentHandler.ConfigureQuestion)))
	mux.Handle("GET /api/v1/exams/{examId}/questions/{questionId}/assessment-snapshot", requireAssessmentRead(withScopedExam(assessmentHandler.GetQuestionSnapshot)))
	mux.Handle("GET /api/v1/exams/{examId}/answer-sheet-templates", requireExamManage(withScopedExam(paperHandler.ListTemplates)))
	mux.Handle("POST /api/v1/exams/{examId}/answer-sheet-templates", requireExamManage(withScopedExam(paperHandler.CreateTemplate)))
	mux.Handle("PATCH /api/v1/answer-sheet-templates/{id}", requireExamManage(paperHandler.UpdateTemplate))
	mux.Handle("POST /api/v1/answer-sheet-templates/{id}/lock", requireExamManage(paperHandler.LockTemplate))
	mux.Handle("POST /api/v1/answer-sheet-templates/{id}/clone", requireExamManage(paperHandler.CloneTemplate))
	mux.Handle("POST /api/v1/answer-sheet-templates/{id}/page-barcodes", requireExamManage(captureHandler.IssueTemplateBarcodes))
	mux.Handle("POST /api/v1/answer-sheet-templates/{id}/student-barcodes", requireExamManage(captureHandler.IssueStudentBarcodes))
	mux.Handle("GET /api/v1/answer-sheet-templates/{id}/print-context", requireExamManage(captureHandler.GetStudentPrintContext))
	mux.Handle("GET /api/v1/answer-sheet-print-batches/{id}/package.pdf", requireExamManage(captureHandler.DownloadStudentPrintPackage))
	mux.Handle("POST /api/v1/answer-sheet-print-sheets/{id}/revoke", requireExamManage(captureHandler.RevokeStudentSheet))
	mux.Handle("POST /api/v1/answer-sheet-print-sheets/{id}/reprint", requireExamManage(captureHandler.ReprintStudentSheet))
	mux.Handle("GET /api/v1/exams/{examId}/readiness", requireExamManage(withScopedExam(paperHandler.GetReadiness)))
	mux.Handle("POST /api/v1/exams/{examId}/readiness/confirm", requireExamManage(withScopedExam(paperHandler.ConfirmReadiness)))
	mux.Handle("POST /api/v1/exams/{examId}/start-collection", requireExamManage(withScopedExam(paperHandler.StartCollection)))

	mux.Handle("POST /api/v1/exams/{examId}/papers", requireExamManage(withScopedExam(paperHandler.CreatePaper)))
	mux.Handle("GET /api/v1/exams/{examId}/papers", requireExamManage(withScopedExam(paperHandler.ListPapers)))
	mux.Handle("POST /api/v1/exams/{examId}/questions", requireExamManage(withScopedExam(paperHandler.CreateQuestion)))
	mux.Handle("GET /api/v1/exams/{examId}/questions", requireExamManage(withScopedExam(paperHandler.ListQuestions)))
	mux.Handle("PATCH /api/v1/questions/{id}", requireExamManage(paperHandler.UpdateQuestion))
	mux.Handle("DELETE /api/v1/questions/{id}", requireExamManage(paperHandler.DeleteQuestion))
	mux.Handle("POST /api/v1/questions/{id}/rubric", requireExamManage(paperHandler.CreateRubric))
	mux.Handle("POST /api/v1/exams/{examId}/validate-paper-config", requireExamManage(withScopedExam(paperHandler.ValidatePaperConfig)))

	mux.Handle("POST /api/v1/files", requireFileManage(withWorkerTaskScope(fileHandler.Upload)))
	mux.Handle("GET /api/v1/files/{id}", requireFileManage(fileHandler.Get))
	mux.Handle("GET /api/v1/files/{id}/download", requireFileManage(withWorkerTaskFile("id", fileHandler.Download)))
	mux.Handle("DELETE /api/v1/files/{id}", requireFileManage(fileHandler.Delete))
	mux.Handle("GET /api/v1/system/file-reconciliation", requireAuth(auth.RequireAnyRole("platform_admin")(http.HandlerFunc(fileHandler.ReconciliationStatus))))

	mux.Handle("POST /api/v1/exams/{examId}/submissions", requireSubmissionManage(withScopedExam(submissionHandler.Create)))
	mux.Handle("GET /api/v1/exams/{examId}/submissions", requireSubmissionManage(withScopedExam(submissionHandler.ListByExam)))
	mux.Handle("GET /api/v1/submissions/{id}", requireSubmissionManage(submissionHandler.Get))
	mux.Handle("POST /api/v1/submissions/{id}/pages", requireSubmissionManage(submissionHandler.AddPage))
	mux.Handle("PUT /api/v1/submissions/{id}/pages/{pageNo}", requireSubmissionManage(submissionHandler.ReplacePage))
	mux.Handle("GET /api/v1/submissions/{id}/pages", requireSubmissionManage(submissionHandler.ListPages))
	mux.Handle("POST /api/v1/submissions/{id}/quality-check", requireSubmissionManage(submissionHandler.QualityCheck))
	mux.Handle("POST /api/v1/submissions/{id}/run-quality-check", requireSubmissionManage(imageQualityHandler.RunQualityCheck))
	mux.Handle("POST /api/v1/submission-pages/{id}/quality-override", requireCaptureManage(imageQualityHandler.OverridePageQuality))
	mux.Handle("POST /api/v1/submissions/{id}/status", requireSubmissionManage(submissionHandler.UpdateStatus))
	mux.Handle("POST /api/v1/exams/{examId}/capture-batches", requireCaptureManage(withScopedExam(captureHandler.CreateBatch)))
	if captureUploadHandler != nil {
		captureupload.RegisterRoutes(mux, captureUploadHandler, requireCaptureManage)
	}
	processing.RegisterRoutes(mux, processingHandler, func(handler http.HandlerFunc) http.Handler {
		return requireCaptureManage(withScopedExam(handler))
	}, requireCaptureManage, requireCaptureManage)
	mux.Handle("GET /api/v1/exams/{examId}/capture-batches", requireCaptureManage(withScopedExam(captureHandler.ListBatches)))
	mux.Handle("GET /api/v1/capture-batches/{id}", requireCaptureManage(captureHandler.GetBatch))
	mux.Handle("GET /api/v1/capture-batches/{id}/matching-queue", requireCaptureManage(captureHandler.GetMatchingQueue))
	mux.Handle("POST /api/v1/capture-batches/{id}/files", requireCaptureManage(captureHandler.RegisterFile))
	mux.Handle("POST /api/v1/capture-batches/{id}/process", requireCaptureManage(captureHandler.ProcessBatch))
	mux.Handle("GET /api/v1/capture-batches/{id}/pages", requireCaptureManage(captureHandler.ListPages))
	mux.Handle("PATCH /api/v1/capture-pages/{id}", requireCaptureManage(captureHandler.UpdatePage))
	mux.Handle("POST /api/v1/capture-pages/{id}/page-match/confirm", requireCaptureManage(captureHandler.ConfirmPageMatch))
	mux.Handle("POST /api/v1/capture-pages/{id}/delete", requireCaptureManage(captureHandler.DeletePage))
	mux.Handle("POST /api/v1/capture-pages/{id}/restore", requireCaptureManage(captureHandler.RestorePage))
	mux.Handle("POST /api/v1/capture-batches/{id}/submissions/split", requireCaptureManage(captureHandler.SplitSubmission))
	mux.Handle("POST /api/v1/capture-batches/{id}/submissions/merge", requireCaptureManage(captureHandler.MergeSubmissions))
	mux.Handle("POST /api/v1/submissions/{id}/student-match/confirm", requireCaptureManage(captureHandler.ConfirmStudentMatch))
	mux.Handle("POST /api/v1/submissions/{id}/student-match/unknown", requireCaptureManage(captureHandler.MarkStudentUnknown))
	mux.Handle("POST /api/v1/capture-batches/{id}/cancel", requireCaptureManage(captureHandler.CancelBatch))
	mux.Handle("POST /api/v1/capture-batches/{id}/reopen", requireCaptureManage(captureHandler.ReopenBatch))
	mux.Handle("POST /api/v1/capture-batches/{id}/complete", requireCaptureManage(captureHandler.CompleteBatch))
	mux.Handle("POST /api/v1/submissions/{id}/process-pages", requireCaptureManage(captureHandler.ProcessSubmissionPages))
	mux.Handle("GET /api/v1/submission-pages/{id}/registration-runs", requireCaptureManage(captureHandler.ListRegistrationRuns))
	mux.Handle("GET /api/v1/submissions/{id}/processing-summary", requireCaptureManage(captureHandler.GetProcessingSummary))
	mux.Handle("POST /api/v1/page-registration-runs/{id}/confirm", requireCaptureManage(captureHandler.ConfirmRegistration))
	mux.Handle("POST /api/v1/page-registration-runs/{id}/retry", requireCaptureManage(captureHandler.RetryRegistration))
	mux.Handle("POST /api/v1/page-registration-runs/{id}/corrections", requireCaptureManage(captureHandler.CreateRegistrationCorrection))
	mux.Handle("GET /api/v1/page-registration-runs/{id}/correction-context", requireCaptureManage(captureHandler.GetRegistrationCorrectionContext))
	mux.Handle("GET /api/v1/page-registration-corrections/{id}", requireCaptureManage(captureHandler.GetRegistrationCorrection))
	mux.Handle("POST /api/v1/page-registration-corrections/{id}/preview", requireCaptureManage(captureHandler.PreviewRegistrationCorrection))
	mux.Handle("POST /api/v1/page-registration-corrections/{id}/apply", requireCaptureManage(captureHandler.ApplyRegistrationCorrection))
	mux.Handle("POST /api/v1/page-registration-corrections/{id}/undo", requireCaptureManage(captureHandler.UndoRegistrationCorrection))

	mux.Handle("POST /api/v1/internal/image-quality/jobs/claim", requireOCRManage(imageQualityHandler.ClaimJobs))
	mux.Handle("POST /api/v1/internal/image-quality/runs/{runId}/normalized-assets", requireOCRManage(withWorkerTaskSource("image_quality_run", "runId", imageQualityHandler.CreateNormalizedAssetSlot)))
	mux.Handle("POST /api/v1/internal/image-quality/runs/{runId}/result", requireOCRManage(withWorkerTaskSource("image_quality_run", "runId", imageQualityHandler.SubmitResult)))
	mux.Handle("POST /api/v1/internal/worker/tasks", requireWorkerExecute(workerRuntimeHandler.CreateTask))
	mux.Handle("POST /api/v1/internal/worker/tasks/claim", requireWorkerExecute(workerRuntimeHandler.Claim))
	mux.Handle("GET /api/v1/internal/worker/tasks/{taskId}", requireWorkerRead(withWorkerTaskScope(workerRuntimeHandler.Get)))
	mux.Handle("POST /api/v1/internal/worker/tasks/{taskId}/heartbeat", requireWorkerExecute(withWorkerTaskScope(workerRuntimeHandler.Heartbeat)))
	mux.Handle("POST /api/v1/internal/worker/tasks/{taskId}/complete", requireWorkerExecute(withWorkerTaskScope(workerRuntimeHandler.Complete)))
	mux.Handle("POST /api/v1/internal/worker/tasks/{taskId}/fail", requireWorkerExecute(withWorkerTaskScope(workerRuntimeHandler.Fail)))
	mux.Handle("POST /api/v1/internal/worker/tasks/{taskId}/cancel", requireWorkerExecute(withWorkerTaskScope(workerRuntimeHandler.Cancel)))
	mux.Handle("POST /api/v1/internal/worker/tasks/{taskId}/requeue", requireWorkerExecute(withWorkerTaskScope(workerRuntimeHandler.Requeue)))
	mux.Handle("POST /api/v1/internal/capture/files/{fileId}/result", requireWorkerExecute(withWorkerTaskSource("capture_file", "fileId", captureHandler.CompleteFile)))
	mux.Handle("POST /api/v1/internal/capture/files/{fileId}/fail", requireWorkerExecute(withWorkerTaskSource("capture_file", "fileId", captureHandler.FailFile)))
	mux.Handle("POST /api/v1/internal/page-registration-runs/{runId}/result", requireWorkerExecute(withWorkerTaskSource("page_registration_run", "runId", captureHandler.CompleteRegistration)))
	mux.Handle("POST /api/v1/internal/page-registration-runs/{runId}/fail", requireWorkerExecute(withWorkerTaskSource("page_registration_run", "runId", captureHandler.FailRegistration)))
	mux.Handle("POST /api/v1/internal/page-registration-corrections/{id}/result", requireWorkerExecute(withWorkerTaskSource("page_registration_correction", "id", captureHandler.CompleteRegistrationCorrection)))
	mux.Handle("POST /api/v1/internal/page-registration-corrections/{id}/failure", requireWorkerExecute(withWorkerTaskSource("page_registration_correction", "id", captureHandler.FailRegistrationCorrection)))
	mux.Handle("GET /api/v1/internal/answer-segments/{id}/image", requireWorkerExecute(withWorkerTaskPayload("id", "answer_segment_id", segmentHandler.GetImage)))
	mux.Handle("POST /api/v1/internal/omr-runs/{runId}/result", requireWorkerExecute(withWorkerTaskSource("omr_run", "runId", gradingHandler.CompleteOMR)))
	mux.Handle("POST /api/v1/internal/omr-runs/{runId}/failure", requireWorkerExecute(withWorkerTaskSource("omr_run", "runId", gradingHandler.FailOMR)))
	mux.Handle("POST /api/v1/internal/subjective-grading/runs/{runId}/result", requireWorkerExecute(withWorkerTaskSource("subjective_grading_run", "runId", subjectiveHandler.CompleteWorker)))
	mux.Handle("POST /api/v1/internal/subjective-grading/runs/{runId}/failure", requireWorkerExecute(withWorkerTaskSource("subjective_grading_run", "runId", subjectiveHandler.FailWorker)))
	mux.Handle("POST /api/v1/internal/subjective-grading/runs/{runId}/execute", requireWorkerExecute(withWorkerTaskSource("subjective_grading_run", "runId", subjectiveHandler.ExecuteWorker)))
	mux.Handle("GET /api/v1/internal/worker/metrics", requireWorkerRead(workerRuntimeHandler.Metrics))

	mux.Handle("POST /api/v1/submissions/{id}/ocr-tasks", requireOCRManage(ocrHandler.CreateTask))
	mux.Handle("GET /api/v1/submissions/{id}/ocr-tasks", requireOCRManage(ocrHandler.ListBySubmission))
	mux.Handle("GET /api/v1/ocr-tasks/pending", requireOCRManage(ocrHandler.ListPending))
	mux.Handle("GET /api/v1/ocr-tasks/{id}", requireOCRManage(ocrHandler.GetTask))
	mux.Handle("GET /api/v1/ocr-tasks/{id}/input", requireOCRManage(withWorkerTaskSource("ocr_task", "id", ocrHandler.GetTaskInput)))
	mux.Handle("POST /api/v1/ocr-tasks/{id}/start", requireOCRManage(withWorkerTaskSource("ocr_task", "id", ocrHandler.StartTask)))
	mux.Handle("POST /api/v1/ocr-tasks/{id}/results", requireOCRManage(withWorkerTaskSource("ocr_task", "id", ocrHandler.CompleteTask)))
	mux.Handle("POST /api/v1/ocr-tasks/{id}/fail", requireOCRManage(withWorkerTaskSource("ocr_task", "id", ocrHandler.FailTask)))

	mux.Handle("POST /api/v1/submissions/{id}/segment-answers", requireSegmentManage(segmentHandler.Generate))
	mux.Handle("GET /api/v1/submissions/{id}/answer-segments", requireSegmentManage(segmentHandler.ListBySubmission))
	mux.Handle("PATCH /api/v1/answer-segments/{id}", requireSegmentManage(segmentHandler.Update))
	mux.Handle("GET /api/v1/answer-segments/{id}/evidence", requireSegmentEvidenceRead(segmentHandler.GetEvidence))
	mux.Handle("GET /api/v1/answer-segments/{id}/image", requireSegmentEvidenceRead(segmentHandler.GetImage))
	mux.Handle("HEAD /api/v1/answer-segments/{id}/image", requireSegmentEvidenceRead(segmentHandler.GetImage))

	mux.Handle("POST /api/v1/orchestrations", requireOrchestratorManage(orchestratorHandler.CreateRun))
	mux.Handle("GET /api/v1/orchestrations/{id}", requireOrchestratorManage(orchestratorHandler.GetRun))
	mux.Handle("GET /api/v1/orchestrations/{id}/tasks", requireOrchestratorManage(orchestratorHandler.ListTasks))
	mux.Handle("POST /api/v1/orchestrations/{id}/tasks", requireOrchestratorManage(orchestratorHandler.CreateTask))
	mux.Handle("POST /api/v1/agent-tasks/{id}/start", requireOrchestratorManage(orchestratorHandler.StartTask))
	mux.Handle("POST /api/v1/agent-tasks/{id}/complete", requireOrchestratorManage(orchestratorHandler.CompleteTask))
	mux.Handle("POST /api/v1/agent-tasks/{id}/fail", requireOrchestratorManage(orchestratorHandler.FailTask))
	mux.Handle("POST /api/v1/agent-tasks/{id}/retry", requireOrchestratorManage(orchestratorHandler.RetryTask))

	mux.Handle("PUT /api/v1/answer-segments/{id}/answer", requireGradingManage(gradingHandler.RecordAnswer))
	mux.Handle("POST /api/v1/answer-segments/{id}/rule-grade", requireGradingManage(gradingHandler.RuleGrade))
	mux.Handle("GET /api/v1/answer-segments/{id}/ai-grades", requireGradingManage(gradingHandler.ListGrades))
	mux.Handle("POST /api/v1/questions/{id}/scoring-rules", requireGradingManage(gradingHandler.CreateScoringRule))
	mux.Handle("GET /api/v1/questions/{id}/scoring-rules", requireGradingManage(gradingHandler.ListScoringRules))
	mux.Handle("PATCH /api/v1/scoring-rules/{id}", requireGradingManage(gradingHandler.UpdateScoringRule))
	mux.Handle("POST /api/v1/scoring-rules/{id}/publish", requireGradingManage(gradingHandler.PublishScoringRule))
	mux.Handle("GET /api/v1/answer-sheet-templates/{id}/omr-calibrations", requireGradingManage(gradingHandler.ListOMRCalibrations))
	mux.Handle("POST /api/v1/answer-sheet-templates/{id}/omr-calibrations", requireGradingManage(gradingHandler.CreateOMRCalibration))
	mux.Handle("GET /api/v1/omr-calibrations/{id}", requireGradingManage(gradingHandler.GetOMRCalibration))
	mux.Handle("POST /api/v1/omr-calibrations/{id}/cases/{caseId}/label", requireGradingManage(gradingHandler.LabelOMRCalibrationCase))
	mux.Handle("POST /api/v1/omr-calibrations/{id}/approve", requireGradingManage(gradingHandler.ApproveOMRCalibration))
	mux.Handle("POST /api/v1/omr-calibrations/{id}/revoke", requireGradingManage(gradingHandler.RevokeOMRCalibration))
	mux.Handle("POST /api/v1/omr-calibrations/{id}/discard", requireGradingManage(gradingHandler.DiscardOMRCalibration))
	mux.Handle("GET /api/v1/exams/{examId}/scoring-readiness", requireGradingManage(withScopedExam(gradingHandler.GetScoringReadiness)))
	mux.Handle("POST /api/v1/exams/{examId}/scoring-runs", requireGradingManage(withScopedExam(gradingHandler.StartScoringRun)))
	mux.Handle("GET /api/v1/exams/{examId}/scoring-summary", requireGradingManage(withScopedExam(gradingHandler.GetScoringSummary)))
	mux.Handle("GET /api/v1/exams/{examId}/automation-results", requireGradingManage(withScopedExam(gradingHandler.GetExamAutomationResults)))
	mux.Handle("GET /api/v1/scoring-runs/{runId}", requireGradingManage(gradingHandler.GetScoringRun))
	mux.Handle("POST /api/v1/scoring-runs/{runId}/cancel", requireGradingManage(gradingHandler.CancelScoringRun))
	mux.Handle("POST /api/v1/scoring-runs/{runId}/retry-failed", requireGradingManage(gradingHandler.RetryFailedScoringRun))
	mux.Handle("POST /api/v1/answer-segments/{id}/reprocess-score", requireGradingManage(gradingHandler.ReprocessSegmentScore))
	mux.Handle("POST /api/v1/answer-segments/{id}/subjective-ai-grade", requireGradingManage(subjectiveHandler.Grade))
	mux.Handle("POST /api/v1/subjective-grading-batches", requireGradingManage(subjectiveHandler.CreateBatch))
	mux.Handle("GET /api/v1/subjective-grading-batches/{batchId}", requireGradingManage(subjectiveHandler.GetBatch))
	mux.Handle("POST /api/v1/subjective-grading-batches/{batchId}/enqueue", requireGradingManage(subjectiveHandler.EnqueueBatch))
	mux.Handle("POST /api/v1/ai-grades/{id}/verify-evidence", requireEvidenceManage(evidenceHandler.Verify))
	mux.Handle("PUT /api/v1/exams/{examId}/double-mark-policy", requireReviewManage(withScopedExam(reviewHandler.SetExamDoubleMarkPolicy)))
	mux.Handle("PUT /api/v1/questions/{id}/double-mark-policy", requireReviewManage(reviewHandler.SetQuestionDoubleMarkPolicy))
	mux.Handle("GET /api/v1/double-mark-policies", requireReviewManage(reviewHandler.ListDoubleMarkPolicies))
	mux.Handle("POST /api/v1/double-mark-sessions", requireReviewManage(reviewHandler.CreateDoubleMarkSession))
	mux.Handle("GET /api/v1/double-mark-sessions", requireReviewManage(reviewHandler.ListDoubleMarkSessions))
	mux.Handle("GET /api/v1/double-mark-sessions/{id}", requireReviewManage(reviewHandler.GetDoubleMarkSession))
	mux.Handle("POST /api/v1/arbitration-tasks", requireArbitrationManage(reviewHandler.CreateArbitrationTask))
	mux.Handle("GET /api/v1/arbitration-tasks", requireArbitrationWork(reviewHandler.ListArbitrationTasks))
	mux.Handle("GET /api/v1/arbitration-tasks/{id}", requireArbitrationWork(reviewHandler.GetArbitrationTask))
	mux.Handle("POST /api/v1/arbitration-tasks/{id}/assign", requireArbitrationManage(reviewHandler.AssignArbitrationTask))
	mux.Handle("POST /api/v1/arbitration-tasks/{id}/submit", requireArbitrationWork(reviewHandler.SubmitArbitration))
	mux.Handle("POST /api/v1/exams/{examId}/finalize", requireScoreManage(withScopedExam(scoreHandler.FinalizeExam)))
	mux.Handle("GET /api/v1/exams/{examId}/grades", requireScoreManage(withScopedExam(scoreHandler.ListExamGrades)))
	mux.Handle("GET /api/v1/exams/{examId}/grades/quality", requireScoreManage(withScopedExam(scoreHandler.CheckQuality)))
	mux.Handle("GET /api/v1/exams/{examId}/roster", requireRosterManage(withScopedExam(scoreHandler.ListRoster)))
	mux.Handle("PUT /api/v1/exams/{examId}/roster/{studentId}/attendance", requireRosterManage(withScopedExam(scoreHandler.SetAttendance)))
	mux.Handle("POST /api/v1/exams/{examId}/confirm-grades", requireScoreManage(withScopedExam(scoreHandler.ConfirmGrades)))
	mux.Handle("POST /api/v1/exams/{examId}/publish", requireScoreManage(withScopedExam(scoreHandler.PublishGrades)))
	scoreReleaseExamManage := func(handler http.HandlerFunc) http.Handler {
		return requireScoreManage(withScopedExam(handler))
	}
	scorerelease.RegisterRoutes(mux, scoreReleaseHandler, scoreReleaseExamManage, requireScoreManage, requireStudentGradeAccess)
	releasegate.RegisterRoutes(mux, releaseGateHandler, scoreReleaseExamManage)
	studentportal.RegisterRoutes(mux, studentPortalHandler, requireStudentGradeAccess)
	mux.Handle("GET /api/v1/exams/{examId}/grades/export", requireScoreManage(withScopedExam(scoreHandler.ExportGrades)))
	mux.Handle("GET /api/v1/students/{studentId}/exams/{examId}/grade", requireStudentGradeAccess(withScopedExam(scoreHandler.GetStudentGrade)))
	mux.Handle("POST /api/v1/appeals", requireAppealCreate(appealHandler.CreateAppeal))
	mux.Handle("GET /api/v1/appeals", requireAppealRead(appealHandler.ListAppeals))
	mux.Handle("GET /api/v1/appeals/statistics", requireAppealManage(appealHandler.Statistics))
	mux.Handle("GET /api/v1/appeals/{id}", requireAppealRead(appealHandler.GetAppeal))
	mux.Handle("POST /api/v1/appeals/{id}/assign", requireAppealManage(appealHandler.AssignAppeal))
	mux.Handle("POST /api/v1/appeals/{id}/recommendation", requireAppealWork(appealHandler.SubmitRecommendation))
	mux.Handle("POST /api/v1/appeals/{id}/review", requireAppealManage(appealHandler.ReviewAppeal))
	mux.Handle("POST /api/v1/appeals/{id}/close", requireAppealManage(appealHandler.CloseAppeal))
	mux.Handle("POST /api/v1/student/exams/{examId}/question-appeals", requireStudentGradeAccess(publishedQuestionAppealHandler.Create))
	mux.Handle("GET /api/v1/student/question-appeals", requireStudentGradeAccess(publishedQuestionAppealHandler.List))
	mux.Handle("GET /api/v1/question-appeals", requireQuestionAppealRead(publishedQuestionAppealHandler.List))
	mux.Handle("GET /api/v1/question-appeals/{id}", requireQuestionAppealRead(publishedQuestionAppealHandler.Get))
	mux.Handle("GET /api/v1/question-appeals/{id}/context", requireQuestionAppealRead(publishedQuestionAppealHandler.Context))
	mux.Handle("GET /api/v1/question-appeals/{id}/answer-image", requireQuestionAppealRead(publishedQuestionAppealHandler.AnswerImage))
	mux.Handle("POST /api/v1/question-appeals/{id}/start-review", requireAppealManage(publishedQuestionAppealHandler.StartReview))
	mux.Handle("POST /api/v1/question-appeals/{id}/decide", requireQuestionAppealRead(publishedQuestionAppealHandler.Decide))
	mux.Handle("POST /api/v1/question-appeals/{id}/resolve", requireQuestionAppealRead(publishedQuestionAppealHandler.Resolve))
	mux.Handle("GET /api/v1/question-appeals/{id}/events", requireQuestionAppealRead(publishedQuestionAppealHandler.Events))
	mux.Handle("GET /api/v1/exams/{examId}/reports/overview", requireReportRead(withScopedExam(reportHandler.Overview)))
	mux.Handle("GET /api/v1/exams/{examId}/reports/classes", requireReportRead(withScopedExam(reportHandler.Classes)))
	mux.Handle("GET /api/v1/exams/{examId}/reports/questions", requireReportRead(withScopedExam(reportHandler.Questions)))
	mux.Handle("GET /api/v1/exams/{examId}/reports/grading-quality", requireReportRead(withScopedExam(reportHandler.GradingQuality)))
	mux.Handle("GET /api/v1/students/{studentId}/reports/{examId}", requireAuth(auth.RequireScopedResource("exam", "examId")(http.HandlerFunc(reportHandler.StudentReport))))
	mux.Handle("POST /api/v1/exams/{examId}/reports/export", requireReportExport(withScopedExam(reportHandler.Export)))
	// Gold sets, answer-group reference cases, Seed observations and drift
	// evidence are quality-management facts.  A grader receives only the
	// current calibration/Seed task through the ordinary review flow; exposing
	// these lists to review:work would reveal reference scores or make dark
	// samples identifiable.
	goldpaper.RegisterRoutes(mux, goldPaperHandler, requireReviewManage, requireReviewManage)
	calibration.RegisterRoutes(mux, calibrationHandler, requireReviewWork, requireReviewManage, requireReviewWork)
	answergroup.RegisterRoutes(mux, answerGroupHandler, requireReviewManage, requireReviewManage)
	seedquality.RegisterRoutes(mux, seedQualityHandler, requireReviewManage, requireReviewManage)
	graderdrift.RegisterRoutes(mux, graderDriftHandler, requireReviewManage, requireReviewManage)
	backmark.RegisterRoutes(mux, backmarkHandler, requireReviewManage, requireReviewWork)
	regrade.RegisterRoutes(mux, regradeHandler, requireReviewManage, requireReviewWork)
	regraderelease.RegisterRoutes(mux, regradeReleaseHandler, requireScoreManage)
	if qualityDashboardHandler != nil {
		qualitydashboard.RegisterRoutes(mux, qualityDashboardHandler, requireReviewManage)
	}
	mux.Handle("POST /api/v1/review-tasks", requireReviewManage(reviewHandler.CreateTask))
	mux.Handle("GET /api/v1/review-tasks", requireReviewWork(reviewHandler.ListTasks))
	mux.Handle("POST /api/v1/review-tasks/next", requireReviewWork(reviewHandler.ClaimNextTask))
	mux.Handle("POST /api/v1/review-tasks/batch-assign", requireReviewManage(reviewHandler.BatchAssignTasks))
	mux.Handle("GET /api/v1/review-tasks/{id}", requireReviewWork(reviewHandler.GetTask))
	mux.Handle("GET /api/v1/review-tasks/{id}/context", requireReviewWork(reviewHandler.GetTaskContext))
	mux.Handle("GET /api/v1/review-tasks/{id}/workspace", requireReviewWork(reviewHandler.GetWorkspace))
	mux.Handle("GET /api/v1/review-tasks/{id}/segment-image", requireReviewWork(reviewHandler.GetWorkspaceSegmentImage))
	mux.Handle("GET /api/v1/review-tasks/{id}/original-image", requireOriginalReviewImage(reviewHandler.GetWorkspaceOriginalImage))
	mux.Handle("POST /api/v1/review-tasks/{id}/renew", requireReviewWork(reviewHandler.RenewTaskClaim))
	mux.Handle("POST /api/v1/review-tasks/{id}/release", requireReviewWork(reviewHandler.ReleaseTaskClaim))
	mux.Handle("POST /api/v1/review-tasks/{id}/assign", requireReviewManage(reviewHandler.AssignTask))
	mux.Handle("POST /api/v1/review-tasks/{id}/submit", requireReviewWork(reviewHandler.SubmitGrade))
	mux.Handle("POST /api/v1/review-tasks/{id}/return", requireReviewWork(reviewHandler.ReturnTask))
	mux.Handle("GET /api/v1/review-tasks/{id}/draft", requireReviewWork(reviewHandler.GetDraft))
	mux.Handle("PUT /api/v1/review-tasks/{id}/draft", requireReviewWork(reviewHandler.SaveDraft))
	mux.Handle("GET /api/v1/review-tasks/{id}/annotations", requireReviewWork(reviewAnnotationHandler.ListAnnotations))
	mux.Handle("POST /api/v1/review-tasks/{id}/annotations", requireReviewWork(reviewAnnotationHandler.CreateAnnotation))
	mux.Handle("GET /api/v1/student/exams/{examId}/questions/{questionId}/annotations", requireStudentGradeAccess(reviewAnnotationHandler.ListStudentQuestionAnnotations))
	mux.Handle("GET /api/v1/review/annotations/{annotationId}", requireReviewWork(reviewAnnotationHandler.GetAnnotation))
	mux.Handle("PUT /api/v1/review/annotations/{annotationId}", requireReviewWork(reviewAnnotationHandler.UpdateAnnotation))
	mux.Handle("DELETE /api/v1/review/annotations/{annotationId}", requireReviewWork(reviewAnnotationHandler.DeleteAnnotation))
	mux.Handle("GET /api/v1/review/comment-templates", requireReviewWork(reviewAnnotationHandler.ListCommentTemplates))
	mux.Handle("POST /api/v1/review/comment-templates", requireReviewWork(reviewAnnotationHandler.CreateCommentTemplate))
	mux.Handle("GET /api/v1/review/comment-templates/{templateId}", requireReviewWork(reviewAnnotationHandler.GetCommentTemplate))
	mux.Handle("PUT /api/v1/review/comment-templates/{templateId}", requireReviewWork(reviewAnnotationHandler.UpdateCommentTemplate))
	mux.Handle("DELETE /api/v1/review/comment-templates/{templateId}", requireReviewWork(reviewAnnotationHandler.DeleteCommentTemplate))
	mux.Handle("POST /api/v1/review/comment-templates/{shortcut}/use", requireReviewWork(reviewAnnotationHandler.UseCommentTemplate))
	mux.HandleFunc("/", h.NotFound)

	return middleware.Chain(
		mux,
		middleware.Recover(logg),
		middleware.RequestID(),
		middleware.AccessLog(logg, cfg.Observability.SlowRequestThreshold),
		metricsRegistry.Middleware(),
		middleware.SecurityHeaders(),
		middleware.CORS(cfg.Security.CORSAllowedOrigins, cfg.Security.CORSAllowedMethods, cfg.Security.CORSAllowedHeaders),
		middleware.BrowserCSRF(cfg.Auth.SessionCookieName),
		middleware.BodyLimit(cfg.Security.MaxRequestBodyBytes, skipGlobalBodyLimit),
	)
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func useRealAIService(cfg config.Config) bool {
	if strings.TrimSpace(cfg.AIService.URL) == "" {
		return false
	}
	environment := strings.ToLower(strings.TrimSpace(cfg.Service.Environment))
	if environment == "production" || environment == "demo" || environment == "staging" {
		return cfg.AIService.Enabled
	}
	return true
}

func allowMockAI(cfg config.Config) bool {
	environment := strings.ToLower(strings.TrimSpace(cfg.Service.Environment))
	switch environment {
	case "", "development", "dev", "test", "local":
		return strings.TrimSpace(cfg.AIService.URL) == ""
	case "demo":
		return cfg.AIService.Enabled && cfg.AIService.AllowMock && strings.TrimSpace(cfg.AIService.URL) == ""
	default:
		return false
	}
}

func localModelBaseline(cfg config.Config) modelgovernance.LocalBaseline {
	return modelgovernance.LocalBaseline{
		ProviderKey:       cfg.AIService.ProviderKey,
		ProviderName:      "Local grading runtime",
		DeploymentKey:     cfg.AIService.DeploymentKey,
		ModelName:         cfg.AIService.ModelVersion,
		ModelVersion:      cfg.AIService.ModelVersion,
		AdapterType:       cfg.AIService.AdapterType,
		Region:            cfg.AIService.DeploymentRegion,
		CapabilityProfile: cfg.AIService.CapabilityProfile,
	}
}

func skipGlobalBodyLimit(r *http.Request) bool {
	return r.Method == http.MethodPost && r.URL.Path == "/api/v1/files"
}
