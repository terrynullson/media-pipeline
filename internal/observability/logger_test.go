package observability

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewTextLoggerWritesRussianMessageToLogFile(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "worker.log")
	logger, closeLog, err := NewTextLogger(logPath)
	if err != nil {
		t.Fatalf("NewTextLogger() error = %v", err)
	}
	t.Cleanup(func() {
		if err := closeLog(); err != nil {
			t.Fatalf("closeLog() error = %v", err)
		}
	})

	expected := "Истекло время ожидания распознавания текста."
	logger.Error("pipeline step failed", slog.String("error_message", expected))

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("log content = %q, want to contain %q", string(content), expected)
	}
}
