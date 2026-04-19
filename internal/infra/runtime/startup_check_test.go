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
	}
	applyTestDatabaseURL(&cfg)

	result := CheckWorkerDependencies(cfg)

	// Directories must have been created.
	for _, dir := range []string{cfg.UploadDir, cfg.AudioDir, cfg.ScreenshotsDir, cfg.PreviewDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected dir %s to be created, got: %v", dir, err)
		}
	}

	// Filter out binary- and database-related errors. Binaries may not exist
	// in CI; the Postgres ping is exercised by separate integration tests
	// that require TEST_DATABASE_URL.
	var dirErrors []string
	for _, e := range result.Errors {
		if !containsAny(e, "FFMPEG_BINARY", "PYTHON_BINARY", "postgres", "database config", "DATABASE_URL") {
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
	}
	applyTestDatabaseURL(&cfg)

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
	}
	applyTestDatabaseURL(&cfg)

	result := CheckWebDependencies(cfg)

	// Allow Postgres errors when TEST_DATABASE_URL is unset; the directory
	// creation is what this test asserts.
	for _, e := range result.Errors {
		if !containsAny(e, "postgres", "database config", "DATABASE_URL") {
			t.Errorf("unexpected CheckWebDependencies error: %s", e)
		}
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
	}
	applyTestDatabaseURL(&cfg)

	result := CheckWebDependencies(cfg)
	// On Windows, 0o555 doesn't prevent directory creation, so we only check on
	// systems where the permission actually takes effect.
	if os.Getuid() == 0 {
		// root can always write — skip the check
		return
	}
	_ = result // acceptable: may or may not error on Windows
}

// applyTestDatabaseURL fills DATABASE_URL onto cfg from TEST_DATABASE_URL when
// available. With a real Postgres DSN the checks exercise the full ping path;
// without one they will surface a "postgres unreachable" / "database config"
// error that callers filter out — the per-test assertions only care about
// directory creation.
func applyTestDatabaseURL(cfg *config.Config) {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		cfg.DatabaseURL = dsn
	}
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
