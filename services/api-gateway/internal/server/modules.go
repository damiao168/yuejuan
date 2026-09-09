package server

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

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
	"edugrade-enterprise/services/api-gateway/internal/evidence"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/goldpaper"
	"edugrade-enterprise/services/api-gateway/internal/graderdrift"
	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/gradingevaluation"
	"edugrade-enterprise/services/api-gateway/internal/idempotency"
	"edugrade-enterprise/services/api-gateway/internal/imagequality"
	"edugrade-enterprise/services/api-gateway/internal/mathunderstanding"
	"edugrade-enterprise/services/api-gateway/internal/modelcalibration"
	"edugrade-enterprise/services/api-gateway/internal/modelgovernance"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/orchestrator"
	"edugrade-enterprise/services/api-gateway/internal/org"
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

type IdentityStores struct {
	Auth auth.Store
	Org  org.Store
}

type IdentityModule struct {
	AuthStore   auth.Store
	AuthHandler *auth.Handler
	OrgHandler  *org.Handler
}

func NewIdentityModule(cfg config.Config, stores IdentityStores, limiter auth.LoginLimiter) *IdentityModule {
	return &IdentityModule{
		AuthStore: stores.Auth,
		AuthHandler: auth.NewHandler(stores.Auth, cfg.Auth.SessionTTL, auth.HandlerOptions{
			LoginFailureLimit:    cfg.Auth.LoginFailureLimit,
			LoginFailureWindow:   cfg.Auth.LoginFailureWindow,
			RememberedSessionTTL: cfg.Auth.RememberedSessionTTL,
			CookieName:           cfg.Auth.SessionCookieName,
			CookieSecure:         cfg.Auth.SessionCookieSecure,
			LoginLimiter:         limiter,
			TrustedProxyCIDRs:    cfg.Security.TrustedProxyCIDRs,
		}),
		OrgHandler: org.NewHandler(stores.Org, stores.Auth),
	}
}

type ExamPreparationStores struct {
	Exam                   exam.Store
	Paper                  paper.Store
	Files                  files.Store
	Submissions            submission.Store
	Segments               segment.Store
	Assessments            assessment.Store
	DashboardOrganizations dashboard.OrganizationSummaryStore
	DashboardActivities    dashboard.ActivityStore
}

type ExamPreparationModule struct {
	ExamStore          exam.Store
	PaperStore         paper.Store
	FileStore          files.Store
	ObjectStore        files.ObjectStorage
	SubmissionStore    submission.Store
	SegmentStore       segment.Store
	AssessmentStore    assessment.Store
	PaperImportService *paper.DocumentImportService

	ExamHandler       *exam.Handler
	PaperHandler      *paper.Handler
	FileHandler       *files.Handler
	SubmissionHandler *submission.Handler
	SegmentHandler    *segment.Handler
	AssessmentHandler *assessment.Handler
	WorkspaceHandler  *workspace.Handler
	DashboardHandler  *dashboard.Handler

	authStore              auth.Store
	dashboardOrganizations dashboard.OrganizationSummaryStore
	dashboardActivities    dashboard.ActivityStore
}

type ExamPreparationDependencies struct {
	AuthStore      auth.Store
	ObjectStore    files.ObjectStorage
	Reconciliation files.ReconciliationReader
}

func NewExamPreparationModule(cfg config.Config, stores ExamPreparationStores, dependencies ExamPreparationDependencies) *ExamPreparationModule {
	fileHandler := files.NewHandler(stores.Files, dependencies.ObjectStore, dependencies.AuthStore, cfg.Files).WithReconciliationReader(dependencies.Reconciliation)
	segmentHandler := segment.NewHandler(stores.Segments, stores.Paper, stores.Submissions, dependencies.AuthStore, stores.Files, dependencies.ObjectStore)
	paperImportService := paper.NewDocumentImportService(stores.Paper, stores.Files, dependencies.ObjectStore, cfg.AIService.URL, cfg.AIService.Token, cfg.AIService.Timeout)
	return &ExamPreparationModule{
		ExamStore:          stores.Exam,
		PaperStore:         stores.Paper,
		FileStore:          stores.Files,
		ObjectStore:        dependencies.ObjectStore,
		SubmissionStore:    stores.Submissions,
		SegmentStore:       stores.Segments,
		AssessmentStore:    stores.Assessments,
		PaperImportService: paperImportService,
		ExamHandler:        exam.NewHandler(stores.Exam, dependencies.AuthStore),
		PaperHandler:       paper.NewHandler(stores.Paper, dependencies.AuthStore).WithDocumentImport(paperImportService),
		FileHandler:        fileHandler,
		SubmissionHandler:  submission.NewHandler(stores.Submissions, stores.Files, dependencies.AuthStore),
		SegmentHandler:     segmentHandler,
		AssessmentHandler:  assessment.NewHandler(stores.Assessments, dependencies.AuthStore),
		WorkspaceHandler: workspace.NewHandler(workspace.Dependencies{
			Exams: stores.Exam, Papers: stores.Paper, PaperImports: stores.Paper, Submissions: stores.Submissions, Assessments: stores.Assessments,
		}),
		DashboardHandler: dashboard.NewHandler(dashboard.Dependencies{
			Exams: stores.Exam, Submissions: stores.Submissions, Audits: dependencies.AuthStore,
			Organizations: stores.DashboardOrganizations, Activities: stores.DashboardActivities,
		}),
		authStore:              dependencies.AuthStore,
		dashboardOrganizations: stores.DashboardOrganizations,
		dashboardActivities:    stores.DashboardActivities,
	}
}

func (m *ExamPreparationModule) ConnectOperations(reviews review.Store, processingService *processing.Service) {
	m.WorkspaceHandler = workspace.NewHandler(workspace.Dependencies{
		Exams: m.ExamStore, Papers: m.PaperStore, PaperImports: m.PaperStore, Submissions: m.SubmissionStore,
		Reviews: reviews, Assessments: m.AssessmentStore, Processing: processingService,
	})
	m.DashboardHandler = dashboard.NewHandler(dashboard.Dependencies{
		Exams: m.ExamStore, Submissions: m.SubmissionStore, Reviews: reviews,
		Audits: m.authStore, Organizations: m.dashboardOrganizations, Activities: m.dashboardActivities,
	})
}

type CaptureProcessingStores struct {
	ImageQuality  imagequality.Store
	WorkerRuntime workerruntime.Store
	OCR           ocrpkg.Store
	OCRQueue      ocrpkg.Queue
	Orchestrator  orchestrator.Store
	Capture       capture.Store
	CaptureUpload captureupload.Store
	Processing    processing.Store
}

type CaptureProcessingModule struct {
	WorkerRuntimeStore workerruntime.Store
	CaptureStore       capture.Store
	ProcessingService  *processing.Service

	OrchestratorHandler  *orchestrator.Handler
	OCRHandler           *ocrpkg.Handler
	ImageQualityHandler  *imagequality.Handler
	WorkerRuntimeHandler *workerruntime.Handler
	CaptureHandler       *capture.Handler
	CaptureUploadHandler *captureupload.Handler
	ProcessingHandler    *processing.Handler
}

func NewCaptureProcessingModule(cfg config.Config, stores CaptureProcessingStores, identity *IdentityModule, examModule *ExamPreparationModule) *CaptureProcessingModule {
	module, _ := newCaptureProcessingModule(cfg, stores, identity, examModule, false)
	return module
}

// NewTransactionalCaptureProcessingModule is the production composition root.
// It fails startup unless the image-quality command can run through the atomic
// coordinator, without depending on a concrete store implementation name.
func NewTransactionalCaptureProcessingModule(cfg config.Config, stores CaptureProcessingStores, identity *IdentityModule, examModule *ExamPreparationModule) (*CaptureProcessingModule, error) {
	return newCaptureProcessingModule(cfg, stores, identity, examModule, true)
}

func newCaptureProcessingModule(cfg config.Config, stores CaptureProcessingStores, identity *IdentityModule, examModule *ExamPreparationModule, transactional bool) (*CaptureProcessingModule, error) {
	processingService := processing.NewService(stores.Processing, stores.WorkerRuntime)
	imageQualityHandler := imagequality.NewHandler(stores.ImageQuality, examModule.SubmissionStore, examModule.FileStore, identity.AuthStore, stores.WorkerRuntime).WithCaptureStore(stores.Capture)
	if transactional {
		coordinator, ok := stores.ImageQuality.(imagequality.TransactionalStore)
		if !ok {
			return nil, fmt.Errorf("production image quality store does not provide transactional command coordination")
		}
		var err error
		imageQualityHandler, err = imagequality.NewTransactionalHandler(imagequality.TransactionalHandlerDependencies{
			Coordinator: coordinator, Submissions: examModule.SubmissionStore, Files: examModule.FileStore,
			Audit: identity.AuthStore, Runtime: stores.WorkerRuntime, Captures: stores.Capture,
		})
		if err != nil {
			return nil, err
		}
	}
	module := &CaptureProcessingModule{
		WorkerRuntimeStore:   stores.WorkerRuntime,
		CaptureStore:         stores.Capture,
		ProcessingService:    processingService,
		OrchestratorHandler:  orchestrator.NewHandler(stores.Orchestrator, identity.AuthStore),
		OCRHandler:           ocrpkg.NewHandler(stores.OCR, stores.OCRQueue, examModule.SubmissionStore, identity.AuthStore, stores.WorkerRuntime),
		ImageQualityHandler:  imageQualityHandler,
		WorkerRuntimeHandler: workerruntime.NewHandler(stores.WorkerRuntime, identity.AuthStore, workerSourceLeaseRenewer{imageQuality: stores.ImageQuality}),
		CaptureHandler:       capture.NewHandler(stores.Capture, examModule.FileStore, examModule.ExamStore, stores.WorkerRuntime, identity.AuthStore),
		ProcessingHandler:    processing.NewHandler(processingService, identity.AuthStore),
	}
	if lifecycleFiles, ok := examModule.FileStore.(files.LifecycleStore); ok {
		module.CaptureUploadHandler = captureupload.NewHandler(
			captureupload.NewService(stores.CaptureUpload, stores.Capture, lifecycleFiles, examModule.ObjectStore, cfg.Files),
			identity.AuthStore,
		)
	}
	return module, nil
}

type AIFoundationStores struct {
	Eligibility       aieligibility.Store
	GradingEvaluation gradingevaluation.Store
	ModelCalibration  modelcalibration.Store
	Disagreement      aidisagreement.Store
}

type AIFoundation struct {
	EligibilityService       *aieligibility.Service
	GradingEvaluationService *gradingevaluation.Service
	ModelCalibrationService  *modelcalibration.Service
	DisagreementService      *aidisagreement.Service

	EligibilityHandler       *aieligibility.Handler
	GradingEvaluationHandler *gradingevaluation.Handler
	ModelCalibrationHandler  *modelcalibration.Handler
	DisagreementHandler      *aidisagreement.Handler
}

func NewAIFoundation(stores AIFoundationStores) *AIFoundation {
	gradingEvaluationService := gradingevaluation.NewService(stores.GradingEvaluation)
	modelCalibrationService := modelcalibration.NewService(stores.ModelCalibration, modelcalibration.NewEvaluationReader(gradingEvaluationService))
	disagreementService := aidisagreement.NewService(stores.Disagreement)
	module := &AIFoundation{
		GradingEvaluationService: gradingEvaluationService,
		ModelCalibrationService:  modelCalibrationService,
		DisagreementService:      disagreementService,
		GradingEvaluationHandler: gradingevaluation.NewHandler(gradingEvaluationService),
		ModelCalibrationHandler:  modelcalibration.NewHandler(modelCalibrationService),
		DisagreementHandler:      aidisagreement.NewHandler(disagreementService),
	}
	if stores.Eligibility != nil {
		module.EligibilityService = aieligibility.NewService(stores.Eligibility)
		module.EligibilityHandler = aieligibility.NewHandler(module.EligibilityService)
	}
	return module
}

type GradingQualityStores struct {
	Grading          grading.Store
	Subjective       subjective.Store
	Evidence         evidence.Store
	Review           review.Store
	ReviewAnnotation reviewannotation.Store
	GoldPaper        goldpaper.Store
	Calibration      calibration.Store
	AnswerGroup      answergroup.Store
	Backmark         backmark.Store
	Regrade          regrade.Store
	GraderDrift      graderdrift.Store
	SeedQuality      seedquality.Store
	QualityDashboard *qualitydashboard.Service
}

type GradingQualityModule struct {
	ReviewStore             review.Store
	RegradeService          *regrade.Service
	QualityDashboardService *qualitydashboard.Service

	GradingHandler          *grading.Handler
	SubjectiveHandler       *subjective.Handler
	EvidenceHandler         *evidence.Handler
	ReviewHandler           *review.Handler
	ReviewAnnotationHandler *reviewannotation.Handler
	GoldPaperHandler        *goldpaper.Handler
	CalibrationHandler      *calibration.Handler
	AnswerGroupHandler      *answergroup.Handler
	BackmarkHandler         *backmark.Handler
	RegradeHandler          *regrade.Handler
	GraderDriftHandler      *graderdrift.Handler
	SeedQualityHandler      *seedquality.Handler
	QualityDashboardHandler *qualitydashboard.Handler
}

type GradingQualityDependencies struct {
	DB       *sql.DB
	Identity *IdentityModule
	Exam     *ExamPreparationModule
	Capture  *CaptureProcessingModule
	AI       *AIFoundation
}

func NewGradingQualityModule(cfg config.Config, stores GradingQualityStores, dependencies GradingQualityDependencies) *GradingQualityModule {
	gradingHandler := grading.NewHandler(stores.Grading, grading.NewEngine(), dependencies.Identity.AuthStore)
	gradingHandler.SetProductionDependencies(dependencies.Capture.WorkerRuntimeStore, dependencies.Exam.FileStore)

	subjectiveHandler := subjective.NewHandler(stores.Subjective, newSubjectiveAdapter(cfg), dependencies.Identity.AuthStore).
		WithWorkerRuntimeStore(dependencies.Capture.WorkerRuntimeStore).
		WithEvaluationEvidence(dependencies.AI.GradingEvaluationService).
		WithCalibrationEvidence(dependencies.AI.ModelCalibrationService).
		WithParserQuality(dependencies.Capture.ProcessingService)
	if dependencies.AI.EligibilityService != nil {
		subjectiveHandler.WithEligibilityGate(dependencies.AI.EligibilityService)
	}

	calibrationService := calibration.NewService(stores.Calibration, stores.GoldPaper)
	seedQualityService := seedquality.NewService(stores.SeedQuality, stores.GoldPaper, calibrationService, dependencies.Exam.AssessmentStore)
	graderDriftService := graderdrift.NewService(stores.GraderDrift, seedQualityService, calibrationService)
	backmarkService := backmark.NewService(stores.Backmark)
	if contextStore, ok := stores.Review.(review.TaskContextStore); ok {
		backmarkService.WithContextSource(contextStore)
	}
	backmarkService.WithTaskSource(stores.Review)
	regradeService := regrade.NewService(stores.Regrade)
	if contextStore, ok := stores.Regrade.(regrade.ContextSource); ok {
		regradeService.WithContextSource(contextStore)
	}
	regradeHandler := regrade.NewHandler(regradeService, dependencies.Identity.AuthStore).WithSegmentImage(dependencies.Exam.SegmentHandler.GetImage)
	backmarkHandler := backmark.NewHandler(backmarkService, dependencies.Identity.AuthStore).
		WithSegmentImage(dependencies.Exam.SegmentHandler.GetImage).
		WithRegradeService(regradeService)

	qualityDashboardService := stores.QualityDashboard
	if qualityDashboardService == nil && dependencies.DB != nil {
		qualityDashboardService = newQualityDashboardService(dependencies.DB, stores, graderDriftService, backmarkService)
	}

	module := &GradingQualityModule{
		ReviewStore:             stores.Review,
		RegradeService:          regradeService,
		QualityDashboardService: qualityDashboardService,
		GradingHandler:          gradingHandler,
		SubjectiveHandler:       subjectiveHandler,
		EvidenceHandler:         evidence.NewHandler(stores.Evidence, evidence.NewEngine(), dependencies.Identity.AuthStore),
		ReviewHandler: review.NewHandler(
			stores.Review, dependencies.Identity.AuthStore, dependencies.Exam.SegmentHandler.GetImage, dependencies.Exam.FileHandler.Download,
		).WithQualificationGate(calibrationService).
			WithSeedHook(seedQualityService).
			WithSeedObservationRefresher(graderDriftService).
			WithAIHumanDisagreementObserver(dependencies.AI.DisagreementService),
		ReviewAnnotationHandler: reviewannotation.NewHandler(stores.ReviewAnnotation, dependencies.Identity.AuthStore),
		GoldPaperHandler:        goldpaper.NewHandler(stores.GoldPaper, dependencies.Identity.AuthStore),
		CalibrationHandler:      calibration.NewHandler(calibrationService, dependencies.Identity.AuthStore),
		AnswerGroupHandler:      answergroup.NewHandlerWithReferences(stores.AnswerGroup, dependencies.Identity.AuthStore, stores.GoldPaper),
		BackmarkHandler:         backmarkHandler,
		RegradeHandler:          regradeHandler,
		GraderDriftHandler:      graderdrift.NewHandler(graderDriftService, dependencies.Identity.AuthStore),
		SeedQualityHandler:      seedquality.NewHandler(seedQualityService, dependencies.Identity.AuthStore),
	}
	if qualityDashboardService != nil {
		module.QualityDashboardHandler = qualitydashboard.NewHandler(qualityDashboardService)
	}
	return module
}

func newQualityDashboardService(db *sql.DB, stores GradingQualityStores, graderDriftService *graderdrift.Service, backmarkService *backmark.Service) *qualitydashboard.Service {
	return qualitydashboard.NewService(qualitydashboard.Sources{
		Questions:   qualitydashboard.NewPostgresQuestionReader(db),
		Gold:        stores.GoldPaper,
		Calibration: qualitydashboard.NewPostgresCalibrationReader(db),
		Seeds:       stores.SeedQuality,
		Groups:      stores.AnswerGroup,
		Review:      stores.Review,
		Drift:       qualitydashboard.NewDriftReader(graderDriftService),
		Backmark:    qualitydashboard.NewBackmarkReader(backmarkService),
	})
}

func newSubjectiveAdapter(cfg config.Config) subjective.LLMGradingAdapter {
	if useRealAIService(cfg) {
		return subjective.NewHTTPAdapter(subjective.HTTPAdapterConfig{
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
	}
	if allowMockAI(cfg) {
		return subjective.NewMockLLMAdapter()
	}
	return subjective.NewDisabledAdapter("ai_grading_disabled", cfg.AIService.ModelVersion, cfg.AIService.PromptVersion)
}

type ReleaseStores struct {
	Score                   score.Store
	ScoreRelease            scorerelease.Store
	ReleaseGate             releasegate.Store
	StudentPortal           studentportal.Store
	Appeal                  appeal.Store
	PublishedQuestionAppeal appeal.PublishedQuestionAppealStore
	Report                  report.Store
}

type ReleaseModule struct {
	ScoreHandler                   *score.Handler
	ScoreReleaseHandler            *scorerelease.Handler
	ReleaseGateHandler             *releasegate.Handler
	StudentPortalHandler           *studentportal.Handler
	RegradeReleaseHandler          *regraderelease.Handler
	AppealHandler                  *appeal.Handler
	PublishedQuestionAppealHandler *appeal.PublishedQuestionAppealHandler
	ReportHandler                  *report.Handler
}

func NewReleaseModule(stores ReleaseStores, identity *IdentityModule, examModule *ExamPreparationModule, gradingQuality *GradingQualityModule) *ReleaseModule {
	scoreReleaseService := scorerelease.NewService(stores.ScoreRelease)
	releaseGateService := releasegate.NewService(stores.ReleaseGate, scoreReleaseService).
		WithRegradeBlockerReader(regradeBlocker{service: gradingQuality.RegradeService})
	return &ReleaseModule{
		ScoreHandler: score.NewHandler(stores.Score, identity.AuthStore),
		ScoreReleaseHandler: scorerelease.NewHandler(scoreReleaseService, identity.AuthStore).
			WithPublicationPublisher(releaseGatePublisher{
				coordinator: releasegate.NewPublicationCoordinator(releaseGateService, scoreReleaseService),
			}).
			WithStudentQuestionImage(examModule.SegmentHandler.GetImage).
			WithStudentPaperPageImage(examModule.SegmentHandler.GetPageImage),
		ReleaseGateHandler:   releasegate.NewHandler(releaseGateService, identity.AuthStore),
		StudentPortalHandler: studentportal.NewHandler(studentportal.NewService(stores.StudentPortal)),
		RegradeReleaseHandler: regraderelease.NewHandler(
			regraderelease.NewService(gradingQuality.RegradeService, scoreReleaseService),
		),
		AppealHandler: appeal.NewHandler(stores.Appeal, identity.AuthStore),
		PublishedQuestionAppealHandler: appeal.NewPublishedQuestionAppealHandler(
			appeal.NewPublishedQuestionAppealService(stores.PublishedQuestionAppeal), identity.AuthStore,
		).WithSegmentImage(examModule.SegmentHandler.GetImage),
		ReportHandler: report.NewHandler(stores.Report, identity.AuthStore),
	}
}

type AIGovernanceStores struct {
	ModelGovernance   modelgovernance.Store
	MathUnderstanding mathunderstanding.Store
	MathCorrections   mathunderstanding.CorrectionStore
	MathPilotGates    mathunderstanding.PilotGateStore
}

type AIGovernanceModule struct {
	modelStore               modelgovernance.Store
	Foundation               *AIFoundation
	ModelGovernanceHandler   *modelgovernance.Handler
	MathUnderstandingHandler *mathunderstanding.Handler
}

func NewAIGovernanceModule(cfg config.Config, stores AIGovernanceStores, identity *IdentityModule, captureModule *CaptureProcessingModule, gradingQuality *GradingQualityModule, foundation *AIFoundation) *AIGovernanceModule {
	return &AIGovernanceModule{
		modelStore: stores.ModelGovernance,
		Foundation: foundation,
		ModelGovernanceHandler: modelgovernance.NewHandler(
			stores.ModelGovernance,
			identity.AuthStore,
			modelgovernance.NewEnvironmentSecretResolver(""),
			localModelBaseline(cfg),
		).WithRuntimePromptSource(modelgovernance.NewHTTPRuntimePromptSource(
			cfg.AIService.URL,
			cfg.AIService.Token,
			cfg.AIService.Timeout,
		)),
		MathUnderstandingHandler: mathunderstanding.NewHandler(
			stores.MathUnderstanding, stores.MathCorrections, stores.MathPilotGates, gradingQuality.ReviewStore, identity.AuthStore,
		).WithRuntime(captureModule.WorkerRuntimeStore),
	}
}

type ApplicationModules struct {
	Identity     *IdentityModule
	Exam         *ExamPreparationModule
	Capture      *CaptureProcessingModule
	Grading      *GradingQualityModule
	Release      *ReleaseModule
	AIGovernance *AIGovernanceModule
	Idempotency  idempotency.Store
}

type ApplicationStores struct {
	Identity     IdentityStores
	Exam         ExamPreparationStores
	Capture      CaptureProcessingStores
	AIFoundation AIFoundationStores
	Grading      GradingQualityStores
	Release      ReleaseStores
	AIGovernance AIGovernanceStores
	Idempotency  idempotency.Store
}

func NewMemoryApplicationStores() ApplicationStores {
	mathStore := mathunderstanding.NewMemoryStore()
	stores := ApplicationStores{
		Identity: IdentityStores{Auth: auth.NewMemoryStore(), Org: org.NewMemoryStore()},
		Exam: ExamPreparationStores{
			Exam: exam.NewMemoryStore(), Paper: paper.NewMemoryStore(), Files: files.NewMemoryStore(),
			Submissions: submission.NewMemoryStore(), Segments: segment.NewMemoryStore(), Assessments: assessment.NewMemoryStore(),
		},
		Capture: CaptureProcessingStores{
			ImageQuality: imagequality.NewMemoryStore(), WorkerRuntime: workerruntime.NewMemoryStore(),
			OCR: ocrpkg.NewMemoryStore(), OCRQueue: ocrpkg.NewMemoryQueue(), Orchestrator: orchestrator.NewMemoryStore(),
			Capture: capture.NewMemoryStore(), CaptureUpload: captureupload.NewMemoryStore(), Processing: processing.NewMemoryStore(),
		},
		AIFoundation: AIFoundationStores{
			GradingEvaluation: gradingevaluation.NewMemoryStore(), ModelCalibration: modelcalibration.NewMemoryStore(),
			Disagreement: aidisagreement.NewMemoryStore(),
		},
		Grading: GradingQualityStores{
			Grading: grading.NewMemoryStore(), Subjective: subjective.NewMemoryStore(), Evidence: evidence.NewMemoryStore(),
			Review: review.NewMemoryStore(), ReviewAnnotation: reviewannotation.NewMemoryStore(),
			GoldPaper: goldpaper.NewMemoryStore(), Calibration: calibration.NewMemoryStore(),
			AnswerGroup: answergroup.NewMemoryStore(nil, answergroup.DefaultPolicy()), Backmark: backmark.NewMemoryStore(),
			Regrade: regrade.NewMemoryStore(), GraderDrift: graderdrift.NewMemoryStore(), SeedQuality: seedquality.NewMemoryStore(),
		},
		Release: ReleaseStores{
			Score: score.NewMemoryStore(), ScoreRelease: scorerelease.NewMemoryStore(), ReleaseGate: releasegate.NewMemoryStore(),
			StudentPortal: studentportal.NewMemoryStore(), Appeal: appeal.NewMemoryStore(),
			PublishedQuestionAppeal: appeal.NewPublishedQuestionAppealMemoryStore(), Report: report.NewMemoryStore(),
		},
		AIGovernance: AIGovernanceStores{
			ModelGovernance: modelgovernance.NewMemoryStore(), MathUnderstanding: mathStore,
			MathCorrections: mathunderstanding.NewMemoryCorrectionStore(mathStore), MathPilotGates: mathunderstanding.NewMemoryPilotGateStore(),
		},
		Idempotency: idempotency.NewMemoryStore(),
	}
	return stores
}

// NewPostgresApplicationStores is the single production persistence graph.
// Integration tests use this factory too, so adding a new application store
// cannot silently leave the production-style suite backed by memory.
func NewPostgresApplicationStores(infra *Infrastructure) (ApplicationStores, error) {
	assessmentStore := assessment.NewPostgresStore(infra.DB)
	gradingStores := GradingQualityStores{
		Grading:          grading.NewPostgresStore(infra.DB),
		Subjective:       subjective.NewPostgresStore(infra.DB),
		Evidence:         evidence.NewPostgresStore(infra.DB),
		Review:           review.NewPostgresStore(infra.DB),
		ReviewAnnotation: reviewannotation.NewPostgresStore(infra.DB),
		GoldPaper:        goldpaper.NewPostgresStore(infra.DB),
		Calibration:      calibration.NewPostgresStore(infra.DB),
		AnswerGroup:      answergroup.NewPostgresStore(infra.DB, nil, answergroup.DefaultPolicy()),
		Backmark:         backmark.NewPostgresStore(infra.DB),
		Regrade:          regrade.NewPostgresStore(infra.DB),
		GraderDrift:      graderdrift.NewPostgresStore(infra.DB),
		SeedQuality:      seedquality.NewPostgresStore(infra.DB),
	}
	gradingStores.QualityDashboard = newPostgresQualityDashboard(infra.DB, gradingStores, assessmentStore)

	credentialCipher, err := modelgovernance.NewCredentialCipher(infra.Config.ModelSecrets.MasterKey)
	if err != nil {
		return ApplicationStores{}, err
	}
	governanceStore := modelgovernance.NewPostgresStore(infra.DB, credentialCipher)
	if err := governanceStore.EnsureLocalBaseline(context.Background(), "", localModelBaseline(infra.Config)); err != nil {
		return ApplicationStores{}, err
	}
	if strings.EqualFold(strings.TrimSpace(infra.Config.Service.Environment), "production") && infra.Config.AIService.Enabled {
		if err := governanceStore.ValidateProductionReadiness(context.Background(), modelgovernance.NewEnvironmentSecretResolver("")); err != nil {
			return ApplicationStores{}, err
		}
	}
	mathStore := mathunderstanding.NewPostgresStore(infra.DB)

	stores := ApplicationStores{
		Identity: IdentityStores{Auth: auth.NewPostgresStore(infra.DB), Org: org.NewPostgresStore(infra.DB)},
		Exam: ExamPreparationStores{
			Exam: exam.NewPostgresStore(infra.DB), Paper: paper.NewPostgresStore(infra.DB), Files: files.NewPostgresStore(infra.DB),
			Submissions: submission.NewPostgresStore(infra.DB), Segments: segment.NewPostgresStore(infra.DB), Assessments: assessmentStore,
			DashboardOrganizations: dashboard.NewPostgresOrganizationSummaryStore(infra.DB), DashboardActivities: dashboard.NewPostgresActivityStore(infra.DB),
		},
		Capture: CaptureProcessingStores{
			ImageQuality: imagequality.NewPostgresStore(infra.DB), WorkerRuntime: workerruntime.NewPostgresStore(infra.DB),
			OCR: ocrpkg.NewPostgresStore(infra.DB), Orchestrator: orchestrator.NewPostgresStore(infra.DB),
			Capture: capture.NewPostgresStoreWithBarcodeKeyring(infra.DB, capture.BarcodeKeyring{
				ActiveKeyID: infra.Config.Barcode.ActiveKeyID, Keys: infra.Config.Barcode.HMACKeys,
			}),
			CaptureUpload: captureupload.NewPostgresStore(infra.DB), Processing: processing.NewPostgresStore(infra.DB),
		},
		AIFoundation: AIFoundationStores{
			Eligibility: aieligibility.NewPostgresStore(infra.DB), GradingEvaluation: gradingevaluation.NewPostgresStore(infra.DB),
			ModelCalibration: modelcalibration.NewPostgresStore(infra.DB), Disagreement: aidisagreement.NewPostgresStore(infra.DB),
		},
		Grading: gradingStores,
		Release: ReleaseStores{
			Score: score.NewPostgresStore(infra.DB), ScoreRelease: scorerelease.NewPostgresStore(infra.DB, gradingStores.QualityDashboard),
			ReleaseGate: releasegate.NewPostgresStore(infra.DB), StudentPortal: studentportal.NewPostgresStore(infra.DB),
			Appeal: appeal.NewPostgresStore(infra.DB), PublishedQuestionAppeal: appeal.NewPublishedQuestionAppealPostgresStore(infra.DB),
			Report: report.NewPostgresStore(infra.DB),
		},
		AIGovernance: AIGovernanceStores{
			ModelGovernance: governanceStore, MathUnderstanding: mathStore,
			MathCorrections: mathunderstanding.NewPostgresCorrectionStore(infra.DB, mathStore), MathPilotGates: mathunderstanding.NewPostgresPilotGateStore(infra.DB),
		},
		Idempotency: idempotency.NewPostgresStore(infra.DB),
	}
	if err := validatePostgresStoreGraph(stores); err != nil {
		return ApplicationStores{}, err
	}
	return stores, nil
}

func validatePostgresStoreGraph(stores ApplicationStores) error {
	return validatePostgresStoreValue(reflect.ValueOf(stores), "stores")
}

func validatePostgresStoreValue(value reflect.Value, path string) error {
	typeOfValue := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		name := path + "." + typeOfValue.Field(index).Name
		if name == "stores.Capture.OCRQueue" {
			// PostgreSQL worker runtime is the durable OCR queue. This legacy
			// seam is used only by the memory router when no runtime is present.
			continue
		}
		switch field.Kind() {
		case reflect.Struct:
			if err := validatePostgresStoreValue(field, name); err != nil {
				return err
			}
		case reflect.Interface, reflect.Pointer:
			for field.Kind() == reflect.Interface && !field.IsNil() {
				field = field.Elem()
			}
			if field.Kind() != reflect.Pointer && field.Kind() != reflect.Interface {
				continue
			}
			if field.IsNil() {
				return fmt.Errorf("production application store %s is not configured", name)
			}
		}
	}
	return nil
}

func newPostgresQualityDashboard(db *sql.DB, stores GradingQualityStores, assessments assessment.Store) *qualitydashboard.Service {
	calibrationService := calibration.NewService(stores.Calibration, stores.GoldPaper)
	seedQualityService := seedquality.NewService(stores.SeedQuality, stores.GoldPaper, calibrationService, assessments)
	graderDriftService := graderdrift.NewService(stores.GraderDrift, seedQualityService, calibrationService)
	backmarkService := backmark.NewService(stores.Backmark)
	if contextStore, ok := stores.Review.(review.TaskContextStore); ok {
		backmarkService.WithContextSource(contextStore)
	}
	backmarkService.WithTaskSource(stores.Review)
	return newQualityDashboardService(db, stores, graderDriftService, backmarkService)
}

type ApplicationDependencies struct {
	Config         config.Config
	ObjectStore    files.ObjectStorage
	Reconciliation files.ReconciliationReader
	LoginLimiter   auth.LoginLimiter
	DB             *sql.DB
}

func NewApplicationModules(dependencies ApplicationDependencies, stores ApplicationStores) ApplicationModules {
	modules, _ := newApplicationModules(dependencies, stores, false)
	return modules
}

func NewTransactionalApplicationModules(dependencies ApplicationDependencies, stores ApplicationStores) (ApplicationModules, error) {
	if err := validatePostgresStoreGraph(stores); err != nil {
		return ApplicationModules{}, err
	}
	return newApplicationModules(dependencies, stores, true)
}

func newApplicationModules(dependencies ApplicationDependencies, stores ApplicationStores, transactional bool) (ApplicationModules, error) {
	identity := NewIdentityModule(dependencies.Config, stores.Identity, dependencies.LoginLimiter)
	examModule := NewExamPreparationModule(dependencies.Config, stores.Exam, ExamPreparationDependencies{
		AuthStore: identity.AuthStore, ObjectStore: dependencies.ObjectStore, Reconciliation: dependencies.Reconciliation,
	})
	var captureModule *CaptureProcessingModule
	var err error
	if transactional {
		captureModule, err = NewTransactionalCaptureProcessingModule(dependencies.Config, stores.Capture, identity, examModule)
	} else {
		captureModule = NewCaptureProcessingModule(dependencies.Config, stores.Capture, identity, examModule)
	}
	if err != nil {
		return ApplicationModules{}, err
	}
	aiFoundation := NewAIFoundation(stores.AIFoundation)
	gradingQuality := NewGradingQualityModule(dependencies.Config, stores.Grading, GradingQualityDependencies{
		DB: dependencies.DB, Identity: identity, Exam: examModule, Capture: captureModule, AI: aiFoundation,
	})
	examModule.ConnectOperations(gradingQuality.ReviewStore, captureModule.ProcessingService)
	releaseModule := NewReleaseModule(stores.Release, identity, examModule, gradingQuality)
	aiGovernance := NewAIGovernanceModule(dependencies.Config, stores.AIGovernance, identity, captureModule, gradingQuality, aiFoundation)
	connectSchoolDocumentModels(examModule, stores.AIGovernance.ModelGovernance)
	return ApplicationModules{
		Identity: identity, Exam: examModule, Capture: captureModule, Grading: gradingQuality,
		Release: releaseModule, AIGovernance: aiGovernance, Idempotency: stores.Idempotency,
	}, nil
}

func NewPostgresApplicationModules(infra *Infrastructure) (ApplicationModules, error) {
	stores, err := NewPostgresApplicationStores(infra)
	if err != nil {
		return ApplicationModules{}, err
	}
	return NewTransactionalApplicationModules(ApplicationDependencies{
		Config: infra.Config, ObjectStore: infra.ObjectStore, Reconciliation: infra.FileReconciler,
		LoginLimiter: infra.LoginLimiter, DB: infra.DB,
	}, stores)
}

func connectSchoolDocumentModels(examModule *ExamPreparationModule, store modelgovernance.Store) {
	managed, ok := store.(modelgovernance.ManagedAPIConfigStore)
	if !ok {
		return
	}
	examModule.PaperHandler.WithDocumentModelResolver(func(ctx context.Context, tenantID string) (*paper.DocumentModelConfig, error) {
		connection, err := modelgovernance.ResolveDefaultManagedAPI(ctx, managed, tenantID)
		if err != nil || connection == nil {
			return nil, err
		}
		return &paper.DocumentModelConfig{
			AdapterType: connection.Config.AdapterType, BaseURL: connection.Config.BaseURL,
			APIKey: connection.APIKey, ModelName: connection.Config.ModelName, ModelVersion: connection.Config.ModelVersion,
		}, nil
	})
}
