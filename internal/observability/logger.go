package observability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func NewTextLogger(logPath string) (*slog.Logger, func() error, error) {
	handlerOptions := &slog.HandlerOptions{Level: slog.LevelInfo}
	stdoutLogger := slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
	if strings.TrimSpace(logPath) == "" {
		return stdoutLogger, func() error { return nil }, nil
	}

	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return stdoutLogger, func() error { return nil }, fmt.Errorf("create log directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return stdoutLogger, func() error { return nil }, fmt.Errorf("open log file: %w", err)
	}

	writer := io.MultiWriter(os.Stdout, logFile)
	logger := slog.New(slog.NewTextHandler(writer, handlerOptions))

	return logger, logFile.Close, nil
}
