package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"media-pipeline/internal/infra/config"
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

	// DB directory must exist (or be creatable)
	if cfg.DBPath != "" {
		checkWritableDir(&result, filepath.Dir(cfg.DBPath), "DB_PATH (directory)")
	}

	return result
}

// CheckWebDependencies verifies dependencies needed by the web server.
func CheckWebDependencies(cfg config.Config) StartupCheckResult {
	var result StartupCheckResult

	if cfg.DBPath != "" {
		checkWritableDir(&result, filepath.Dir(cfg.DBPath), "DB_PATH (directory)")
	}
	checkWritableDir(&result, cfg.UploadDir, "UPLOAD_DIR")

	return result
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
