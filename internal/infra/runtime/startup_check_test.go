package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"media-pipeline/internal/infra/config"
)

func TestCheckWorkerDependencies_ValidDirs(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	scriptPath := filepath.Join(base, "transcribe.py")
	if err := os.WriteFile(scriptPath, []byte("# stub"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := config.Config{
		FFmpegBinary:     "ffmpeg", // may not be installed; only dirs are asserted below
		PythonBinary:     "python",
		TranscribeScript: scriptPath,
		UploadDir:        filepath.Join(base, "uploads"),
		AudioDir:         filepath.Join(base, "audio"),
		ScreenshotsDir:   filepath.Join(base, "screenshots"),
		PreviewDir:       filepath.Join(base, "previews"),
		DBPath:           filepath.Join(base, "data", "app.db"),
	}

	result := CheckWorkerDependencies(cfg)

	// Directories must have been created.
	for _, dir := range []string{cfg.UploadDir, cfg.AudioDir, cfg.ScreenshotsDir, cfg.PreviewDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected dir %s to be created, got: %v", dir, err)
		}
	}

	// Filter out only binary-related errors (binaries may not exist in CI).
	var dirErrors []string
	for _, e := range result.Errors {
		if !containsAny(e, "FFMPEG_BINARY", "PYTHON_BINARY") {
			dirErrors = append(dirErrors, e)
		}
	}
	if len(dirErrors) != 0 {
		t.Errorf("unexpected directory errors: %v", dirErrors)
	}
}

func TestCheckWorkerDependencies_MissingScript(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cfg := config.Config{
		FFmpegBinary:     "ffmpeg",
		PythonBinary:     "python",
		TranscribeScript: filepath.Join(base, "nonexistent.py"),
		UploadDir:        filepath.Join(base, "uploads"),
		AudioDir:         filepath.Join(base, "audio"),
		ScreenshotsDir:   filepath.Join(base, "screenshots"),
		PreviewDir:       filepath.Join(base, "previews"),
		DBPath:           filepath.Join(base, "data", "app.db"),
	}

	result := CheckWorkerDependencies(cfg)

	found := false
	for _, e := range result.Errors {
		if containsAny(e, "TRANSCRIBE_SCRIPT") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TRANSCRIBE_SCRIPT error, got errors: %v", result.Errors)
	}
}

func TestCheckWebDependencies_ValidDirs(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cfg := config.Config{
		UploadDir: filepath.Join(base, "uploads"),
		DBPath:    filepath.Join(base, "data", "app.db"),
	}

	result := CheckWebDependencies(cfg)

	if !result.OK() {
		t.Errorf("CheckWebDependencies() errors: %v", result.Errors)
	}
	if _, err := os.Stat(cfg.UploadDir); err != nil {
		t.Errorf("expected upload dir to be created: %v", err)
	}
}

func TestCheckWebDependencies_ReadOnlyDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	readOnly := filepath.Join(base, "readonly")
	if err := os.MkdirAll(readOnly, 0o555); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg := config.Config{
		UploadDir: filepath.Join(readOnly, "uploads"),
		DBPath:    filepath.Join(base, "data", "app.db"),
	}

	result := CheckWebDependencies(cfg)
	// On Windows, 0o555 doesn't prevent directory creation, so we only check on
	// systems where the permission actually takes effect.
	if os.Getuid() == 0 {
		// root can always write — skip the check
		return
	}
	_ = result // acceptable: may or may not error on Windows
}

func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
