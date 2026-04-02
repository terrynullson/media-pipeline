package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	appworker "media-pipeline/internal/app/worker"
	"media-pipeline/internal/infra/config"
	"media-pipeline/internal/infra/db"
	"media-pipeline/internal/infra/db/repositories"
	infraMedia "media-pipeline/internal/infra/media"
	infraRuntime "media-pipeline/internal/infra/runtime"
	infraTranscription "media-pipeline/internal/infra/transcription"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

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
	audioExtractor := infraMedia.NewFFmpegExtractor(cfg.FFmpegBinary)
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
		audioExtractor,
		transcriber,
		cfg.UploadDir,
		cfg.AudioDir,
		cfg.FFmpegTimeout(),
		cfg.TranscribeTimeout(),
		cfg.TranscribeLanguage,
		logger,
	)
	runner := appworker.NewRunner(processor, cfg.WorkerPollInterval(), logger)

	logger.Info("starting worker",
		slog.String("db_path", cfg.DBPath),
		slog.String("upload_dir", cfg.UploadDir),
		slog.String("audio_dir", cfg.AudioDir),
		slog.String("ffmpeg_binary", cfg.FFmpegBinary),
		slog.String("python_binary", cfg.PythonBinary),
		slog.String("transcribe_script", transcribeScriptPath),
		slog.Duration("poll_interval", cfg.WorkerPollInterval()),
		slog.Duration("ffmpeg_timeout", cfg.FFmpegTimeout()),
		slog.Duration("transcribe_timeout", cfg.TranscribeTimeout()),
	)

	if err := runner.Run(ctx); err != nil {
		logger.Error("worker stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}
