package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	transcriptionapp "media-pipeline/internal/app/transcription"
	appworker "media-pipeline/internal/app/worker"
	domaintranscription "media-pipeline/internal/domain/transcription"
	"media-pipeline/internal/infra/config"
	"media-pipeline/internal/infra/db"
	"media-pipeline/internal/infra/db/repositories"
	infraMedia "media-pipeline/internal/infra/media"
	infraRuntime "media-pipeline/internal/infra/runtime"
	infraSummary "media-pipeline/internal/infra/summary"
	infraTranscription "media-pipeline/internal/infra/transcription"
	"media-pipeline/internal/observability"
)

func main() {
	cfg := config.Load()
	logger, closeLog, err := observability.NewTextLogger(filepath.Join("data", "logs", "worker.log"))
	if err != nil {
		logger.Error("configure worker logger", slog.Any("error", err))
		os.Exit(1)
	}
	defer closeLog()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	if err := db.RunMigrations(sqlDB, migrationsPath); err != nil {
		logger.Error("run migrations", slog.Any("error", err), slog.String("path", migrationsPath))
		os.Exit(1)
	}

	jobRepo := repositories.NewJobRepository(sqlDB)
	mediaRepo := repositories.NewMediaRepository(sqlDB)
	transcriptRepo := repositories.NewTranscriptRepository(sqlDB)
	triggerRuleRepo := repositories.NewTriggerRuleRepository(sqlDB)
	triggerEventRepo := repositories.NewTriggerEventRepository(sqlDB)
	triggerScreenshotRepo := repositories.NewTriggerScreenshotRepository(sqlDB)
	summaryRepo := repositories.NewSummaryRepository(sqlDB)
	profileRepo := repositories.NewTranscriptionProfileRepository(sqlDB)
	profileService := transcriptionapp.NewService(profileRepo, domaintranscription.DefaultProfile(cfg.TranscribeLanguage))
	audioExtractor := infraMedia.NewFFmpegExtractor(cfg.FFmpegBinary)
	previewGenerator := infraMedia.NewFFmpegPreviewGenerator(cfg.FFmpegBinary)
	audioDurationReader := infraMedia.NewWAVDurationReader()
	screenshotExtractor := infraMedia.NewFFmpegScreenshotExtractor(cfg.FFmpegBinary)
	summarizer := infraSummary.NewSimpleSummarizer()
	transcribeScriptPath, err := infraRuntime.ResolvePath(cfg.TranscribeScript)
	if err != nil {
		logger.Error("resolve transcribe script path", slog.Any("error", err), slog.String("path", cfg.TranscribeScript))
		os.Exit(1)
	}
	transcriber := infraTranscription.NewPythonTranscriber(cfg.PythonBinary, transcribeScriptPath, logger)

	processor := appworker.NewProcessor(
		jobRepo,
		mediaRepo,
		transcriptRepo,
		triggerRuleRepo,
		triggerEventRepo,
		triggerScreenshotRepo,
		summaryRepo,
		audioExtractor,
		previewGenerator,
		audioDurationReader,
		screenshotExtractor,
		transcriber,
		summarizer,
		profileService,
		cfg.UploadDir,
		cfg.AudioDir,
		cfg.PreviewDir,
		cfg.ScreenshotsDir,
		cfg.FFmpegTimeout(),
		cfg.PreviewTimeout(),
		cfg.ScreenshotTimeout(),
		cfg.TranscribeTimeout(),
		logger,
	)
	runner := appworker.NewRunner(processor, cfg.WorkerPollInterval(), logger)

	logger.Info("starting worker",
		slog.String("db_path", cfg.DBPath),
		slog.String("upload_dir", cfg.UploadDir),
		slog.String("audio_dir", cfg.AudioDir),
		slog.String("preview_dir", cfg.PreviewDir),
		slog.String("screenshots_dir", cfg.ScreenshotsDir),
		slog.String("ffmpeg_binary", cfg.FFmpegBinary),
		slog.String("python_binary", cfg.PythonBinary),
		slog.String("transcribe_script", transcribeScriptPath),
		slog.Duration("poll_interval", cfg.WorkerPollInterval()),
		slog.Duration("ffmpeg_timeout", cfg.FFmpegTimeout()),
		slog.Duration("preview_timeout", cfg.PreviewTimeout()),
		slog.Duration("screenshot_timeout", cfg.ScreenshotTimeout()),
		slog.Duration("transcribe_base_timeout", cfg.TranscribeTimeout()),
	)

	if err := runner.Run(ctx); err != nil {
		logger.Error("worker stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}
