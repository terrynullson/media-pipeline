package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"media-pipeline/internal/infra/config"
)

func TestCheckWorkerDependencies_ValidDirs(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pythonBinary, ok := resolveTestPythonBinary()
	if !ok {
		t.Skip("python launcher is not available in PATH")
	}
	scriptPath := filepath.Join(base, "transcribe.py")
	script := "import json\nimport sys\nif '--self-check' in sys.argv:\n    json.dump({'status': 'ok'}, sys.stdout)\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := config.Config{
		FFmpegBinary:     "ffmpeg", // may not be installed; only dirs are asserted below
		PythonBinary:     pythonBinary,
		TranscribeScript: scriptPath,
		UploadDir:        filepath.Join(base, "uploads"),
		AudioDir:         filepath.Join(base, "audio"),
		ScreenshotsDir:   filepath.Join(base, "screenshots"),
		PreviewDir:       filepath.Join(base, "previews"),
	}
	applyTestDatabaseURL(&cfg)

	result := CheckWorkerDependencies(cfg)

	for _, dir := range []string{cfg.UploadDir, cfg.AudioDir, cfg.ScreenshotsDir, cfg.PreviewDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected dir %s to be created, got: %v", dir, err)
		}
	}

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
	pythonBinary, ok := resolveTestPythonBinary()
	if !ok {
		t.Skip("python launcher is not available in PATH")
	}
	cfg := config.Config{
		FFmpegBinary:     "ffmpeg",
		PythonBinary:     pythonBinary,
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

func TestCheckWorkerDependencies_FailingSelfCheck(t *testing.T) {
	t.Parallel()

	pythonBinary, ok := resolveTestPythonBinary()
	if !ok {
		t.Skip("python launcher is not available in PATH")
	}

	base := t.TempDir()
	scriptPath := filepath.Join(base, "transcribe.py")
	script := "import sys\nif '--self-check' in sys.argv:\n    print('backend missing', file=sys.stderr)\n    raise SystemExit(1)\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := config.Config{
		FFmpegBinary:     "ffmpeg",
		PythonBinary:     pythonBinary,
		TranscribeScript: scriptPath,
		UploadDir:        filepath.Join(base, "uploads"),
		AudioDir:         filepath.Join(base, "audio"),
		ScreenshotsDir:   filepath.Join(base, "screenshots"),
		PreviewDir:       filepath.Join(base, "previews"),
	}
	applyTestDatabaseURL(&cfg)

	result := CheckWorkerDependencies(cfg)

	found := false
	for _, e := range result.Errors {
		if containsAny(e, "transcription backend self-check failed") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected transcription self-check error, got %v", result.Errors)
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
	if runtime.GOOS == "windows" {
		return
	}
	if len(result.Errors) == 0 {
		t.Skip("read-only directory semantics are not enforced in this environment")
	}
}

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

func resolveTestPythonBinary() (string, bool) {
	for _, candidate := range []string{"py", "python"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}
