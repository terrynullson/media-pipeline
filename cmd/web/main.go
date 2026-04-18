package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	appsettingsapp "media-pipeline/internal/app/appsettings"
	"media-pipeline/internal/app/command"
	mediaapp "media-pipeline/internal/app/media"
	transcriptionapp "media-pipeline/internal/app/transcription"
	triggerapp "media-pipeline/internal/app/trigger"
	appsettings "media-pipeline/internal/domain/appsettings"
	domaintranscription "media-pipeline/internal/domain/transcription"
	"media-pipeline/internal/infra/config"
	"media-pipeline/internal/infra/db"
	"media-pipeline/internal/infra/db/repositories"
	inframedia "media-pipeline/internal/infra/media"
	infraRuntime "media-pipeline/internal/infra/runtime"
	"media-pipeline/internal/infra/storage"
	"media-pipeline/internal/observability"
	httptransport "media-pipeline/internal/transport/http"
	"media-pipeline/internal/transport/http/handlers"
)

func main() {
	observability.RegisterMetrics()

	cfg := config.Load()
	logger, closeLog, err := observability.NewTextLogger(filepath.Join("data", "logs", "web.log"))
	if err != nil {
		logger.Error("configure web logger", slog.Any("error", err))
		os.Exit(1)
	}
	defer closeLog()

	if check := infraRuntime.CheckWebDependencies(cfg); !check.OK() {
		for _, e := range check.Errors {
			logger.Error("startup check failed", slog.String("error", e))
		}
		for _, w := range check.Warnings {
			logger.Warn("startup warning", slog.String("warning", w))
		}
		os.Exit(1)
	}

	sqlDB, err := db.OpenSQLite(cfg.DBPath)
	if err != nil {
		logger.Error("open database", slog.Any("error", err), slog.String("db_path", cfg.DBPath))
		os.Exit(1)
	}
	defer sqlDB.Close()

	migrationsPath, err := infraRuntime.ResolvePath("internal/infra/db/migrations")
	if err != nil {
		logger.Error("resolve migrations path", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("starting migrations", slog.String("path", migrationsPath))
	if err = db.RunMigrations(sqlDB, migrationsPath); err != nil {
		logger.Error("run migrations", slog.Any("error", err), slog.String("path", migrationsPath))
		os.Exit(1)
	}
	logger.Info("migrations completed", slog.String("path", migrationsPath))

	mediaRepo := repositories.NewMediaRepository(sqlDB)
	jobRepo := repositories.NewJobRepository(sqlDB)
	transcriptRepo := repositories.NewTranscriptRepository(sqlDB)
	triggerRuleRepo := repositories.NewTriggerRuleRepository(sqlDB)
	triggerEventRepo := repositories.NewTriggerEventRepository(sqlDB)
	triggerScreenshotRepo := repositories.NewTriggerScreenshotRepository(sqlDB)
	summaryRepo := repositories.NewSummaryRepository(sqlDB)
	profileRepo := repositories.NewTranscriptionProfileRepository(sqlDB)
	runtimeSettingsRepo := repositories.NewRuntimeSettingsRepository(sqlDB)
	cancelRequestRepo := repositories.NewMediaCancelRequestRepository(sqlDB)
	fileStorage := storage.NewLocalStorage(cfg.UploadDir)
	audioStorage := storage.NewLocalStorage(cfg.AudioDir)
	previewStorage := storage.NewLocalStorage(cfg.PreviewDir)
	screenshotStorage := storage.NewLocalStorage(cfg.ScreenshotsDir)
	audioDurationReader := inframedia.NewWAVDurationReader()
	profileService := transcriptionapp.NewService(profileRepo, domaintranscription.DefaultProfile(cfg.TranscribeLanguage))
	runtimeSettingsSvc := appsettingsapp.NewService(runtimeSettingsRepo, appsettings.Settings{
		AutoUploadMinAgeSec: cfg.AutoUploadMinAgeSec,
		PreviewTimeoutSec:   cfg.PreviewTimeoutSec,
		MaxUploadSizeMB:     cfg.MaxUploadSizeMB,
	})
	triggerRuleService := triggerapp.NewService(triggerRuleRepo)
	transcriptViewUC := mediaapp.NewTranscriptViewUseCase(
		mediaRepo,
		transcriptRepo,
		triggerEventRepo,
		triggerScreenshotRepo,
		summaryRepo,
		jobRepo,
		cfg.UploadDir,
		audioDurationReader,
		cfg.AudioDir,
		cfg.PreviewDir,
		cfg.TranscribeTimeout(),
	)
	requestSummaryUC := mediaapp.NewRequestSummaryUseCase(mediaRepo, transcriptRepo, jobRepo)
	deleteMediaUC := mediaapp.NewDeleteMediaUseCase(mediaRepo, jobRepo, cancelRequestRepo, triggerScreenshotRepo, fileStorage, audioStorage, previewStorage, screenshotStorage, logger)
	retryJobUC := mediaapp.NewRetryJobUseCase(mediaRepo, jobRepo, jobRepo, mediaRepo, logger)
	historicalETA := mediaapp.NewHistoricalEstimator(jobRepo)

	uploadUC := command.NewUploadMediaUseCase(mediaRepo, jobRepo, fileStorage, cfg.MaxUploadSizeBytes(), logger)
	templatesDir, err := infraRuntime.ResolvePath("internal/transport/http/views/templates")
	if err != nil {
		logger.Error("resolve templates path", slog.Any("error", err))
		os.Exit(1)
	}
	uploadHandler, err := handlers.NewUploadHandler(
		uploadUC,
		profileService,
		runtimeSettingsSvc,
		triggerRuleService,
		transcriptViewUC,
		requestSummaryUC,
		deleteMediaUC,
		retryJobUC,
		jobRepo,
		historicalETA,
		templatesDir,
		cfg.MaxUploadSizeBytes(),
		logger,
	)
	if err != nil {
		logger.Error("create upload handler", slog.Any("error", err))
		os.Exit(1)
	}
	mediaStatusUC := mediaapp.NewMediaStatusUseCase(mediaRepo, transcriptRepo, jobRepo)
	machineAPIHandler := handlers.NewMachineAPIHandler(mediaStatusUC, transcriptViewUC, logger)
	workerStatusUC := mediaapp.NewWorkerStatusUseCase(jobRepo)
	workerStatusHandler := handlers.NewWorkerStatusHandler(workerStatusUC, logger)
	triggerPreviewUC := mediaapp.NewTriggerPreviewUseCase(transcriptRepo)
	triggerRuleHandler := handlers.NewTriggerRuleHandler(triggerRuleService, uploadHandler, logger).
		WithPreviewService(triggerPreviewUC)

	staticPath, err := infraRuntime.ResolvePath("web/static")
	if err != nil {
		logger.Error("resolve static path", slog.Any("error", err))
		os.Exit(1)
	}
	frontendV1DistPath, frontendV1Err := infraRuntime.ResolvePath("frontend_v1/dist")
	if frontendV1Err != nil {
		frontendV1DistPath = ""
	}
	router := httptransport.NewRouter(logger, uploadHandler, machineAPIHandler, triggerRuleHandler, workerStatusHandler, staticPath, cfg.UploadDir, cfg.AudioDir, cfg.PreviewDir, cfg.ScreenshotsDir, cfg.MediaAccessToken, cfg.HTTPRequestTimeout(), cfg.UploadRateLimitPerMinute, frontendV1DistPath)

	// Signal-aware context: SIGINT (Ctrl-C) and SIGTERM (systemd / Docker stop)
	// both trigger a clean drain of in-flight requests before exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := ":" + cfg.AppPort
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
		// Guard against Slowloris: limit time to read request headers.
		// Per-request body/response timeouts are applied by the router middleware.
		ReadHeaderTimeout: 30 * time.Second,
	}

	logger.Info("starting web server",
		slog.String("addr", addr),
		slog.String("db_path", cfg.DBPath),
		slog.String("upload_dir", cfg.UploadDir),
		slog.String("preview_dir", cfg.PreviewDir),
		slog.Int64("max_upload_bytes", cfg.MaxUploadSizeBytes()),
	)

	// ListenAndServe blocks, so run it in a goroutine and report startup failures.
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	// Wait for a shutdown signal or a fatal server error.
	select {
	case err := <-serveErr:
		logger.Error("listen and serve", slog.Any("error", err), slog.String("addr", addr))
		os.Exit(1)
	case <-ctx.Done():
		stop() // release signal resources immediately
	}

	logger.Info("shutdown signal received, draining active connections")

	// Give in-flight requests up to 30 s to finish before forcibly closing.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("web server stopped cleanly")
}
