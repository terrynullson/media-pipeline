package main

import (
	"log/slog"
	"net/http"
	"os"

	"media-pipeline/internal/app/command"
	"media-pipeline/internal/infra/config"
	"media-pipeline/internal/infra/db"
	"media-pipeline/internal/infra/db/repositories"
	infraRuntime "media-pipeline/internal/infra/runtime"
	"media-pipeline/internal/infra/storage"
	httptransport "media-pipeline/internal/transport/http"
	"media-pipeline/internal/transport/http/handlers"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

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
	fileStorage := storage.NewLocalStorage(cfg.UploadDir)

	uploadUC := command.NewUploadMediaUseCase(mediaRepo, jobRepo, fileStorage, cfg.MaxUploadSizeBytes(), logger)
	templatePath, err := infraRuntime.ResolvePath("internal/transport/http/views/templates/index.html")
	if err != nil {
		logger.Error("resolve template path", slog.Any("error", err))
		os.Exit(1)
	}
	uploadHandler, err := handlers.NewUploadHandler(
		uploadUC,
		templatePath,
		cfg.MaxUploadSizeBytes(),
		logger,
	)
	if err != nil {
		logger.Error("create upload handler", slog.Any("error", err))
		os.Exit(1)
	}

	staticPath, err := infraRuntime.ResolvePath("web/static")
	if err != nil {
		logger.Error("resolve static path", slog.Any("error", err))
		os.Exit(1)
	}
	router := httptransport.NewRouter(logger, uploadHandler, staticPath)

	addr := ":" + cfg.AppPort
	logger.Info("starting web server",
		slog.String("addr", addr),
		slog.String("db_path", cfg.DBPath),
		slog.String("upload_dir", cfg.UploadDir),
		slog.Int64("max_upload_bytes", cfg.MaxUploadSizeBytes()),
	)
	if err = http.ListenAndServe(addr, router); err != nil {
		logger.Error("listen and serve", slog.Any("error", err), slog.String("addr", addr))
		os.Exit(1)
	}
}
