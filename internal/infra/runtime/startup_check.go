package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"media-pipeline/internal/infra/config"
	"media-pipeline/internal/infra/db"
)

// StartupCheckResult holds errors and warnings from startup dependency checks.
type StartupCheckResult struct {
	Errors   []string
	Warnings []string
}

// OK returns true when there are no errors (warnings are acceptable).
func (r StartupCheckResult) OK() bool { return len(r.Errors) == 0 }

// CheckWorkerDependencies verifies that all worker runtime dependencies are
// present and usable. Call this immediately after loading config; exit with
// code 1 if result.OK() == false.
func CheckWorkerDependencies(cfg config.Config) StartupCheckResult {
	var result StartupCheckResult

	// Binaries
	checkBinary(&result, cfg.FFmpegBinary, "FFMPEG_BINARY")
	checkBinary(&result, cfg.PythonBinary, "PYTHON_BINARY")

	// Script file
	if cfg.TranscribeScript != "" {
		if _, err := os.Stat(cfg.TranscribeScript); os.IsNotExist(err) {
			result.Errors = append(result.Errors,
				fmt.Sprintf("TRANSCRIBE_SCRIPT not found: %s", cfg.TranscribeScript))
		}
	} else {
		result.Errors = append(result.Errors, "TRANSCRIBE_SCRIPT is not set")
	}

	// Writable directories (create if absent)
	for _, dir := range []struct {
		path    string
		envName string
	}{
		{cfg.UploadDir, "UPLOAD_DIR"},
		{cfg.AudioDir, "AUDIO_DIR"},
		{cfg.ScreenshotsDir, "SCREENSHOTS_DIR"},
		{cfg.PreviewDir, "PREVIEW_DIR"},
	} {
		checkWritableDir(&result, dir.path, dir.envName)
	}

	// PostgreSQL reachability
	checkPostgres(&result, cfg)

	return result
}

// CheckWebDependencies verifies dependencies needed by the web server.
func CheckWebDependencies(cfg config.Config) StartupCheckResult {
	var result StartupCheckResult

	checkWritableDir(&result, cfg.UploadDir, "UPLOAD_DIR")
	checkPostgres(&result, cfg)

	return result
}

// checkPostgres builds the DSN, opens a short-lived database/sql connection,
// and pings it. Failures are reported as errors. The DSN is never logged
// in plaintext — only host/port/db/sslmode via SafeDSN().
func checkPostgres(result *StartupCheckResult, cfg config.Config) {
	dsn, err := cfg.BuildDatabaseURL()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("database config: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sqlDB, err := db.Open(ctx, db.Options{
		DSN:          dsn,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
		PingTimeout:  3 * time.Second,
	})
	if err != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("postgres unreachable (%s): %v", cfg.SafeDSN(), err))
		return
	}
	_ = sqlDB.Close()
}

// checkBinary appends an error if the named binary cannot be found in PATH or
// directly on disk.
func checkBinary(result *StartupCheckResult, binary, envName string) {
	if binary == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("%s is not set", envName))
		return
	}
	if _, err := exec.LookPath(binary); err != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("%s binary not found or not executable: %s (%v)", envName, binary, err))
	}
}

// checkWritableDir ensures dir exists (creating it if necessary) and is writable.
func checkWritableDir(result *StartupCheckResult, dir, label string) {
	if dir == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("%s directory path is empty", label))
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("%s directory cannot be created (%s): %v", label, dir, err))
		return
	}
	// Probe write access by creating a temp file.
	f, err := os.CreateTemp(dir, ".startup-check-*")
	if err != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("%s directory is not writable (%s): %v", label, dir, err))
		return
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
}
