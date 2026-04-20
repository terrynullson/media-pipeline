package autoupload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	autouploadapp "media-pipeline/internal/app/autoupload"
)

func TestLocalSourceFindNextReturnsOldestSupportedStableFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	archiveDir := t.TempDir()
	oldFile := filepath.Join(baseDir, "2026-02-10", "Recorder_1_25.02.10_09.00.00.00.mp4")
	newFile := filepath.Join(baseDir, "2026-02-10", "Recorder_1_25.02.10_10.00.00.00.mp4")
	ignoredFile := filepath.Join(baseDir, "2026-02-10", "notes.txt")
	mustWriteFile(t, oldFile, []byte("old"))
	mustWriteFile(t, newFile, []byte("new"))
	mustWriteFile(t, ignoredFile, []byte("ignored"))

	nowUTC := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	source := NewLocalSource(baseDir, archiveDir, time.Minute)

	if _, ok, err := source.FindNext(context.Background(), nowUTC); err != nil {
		t.Fatalf("FindNext(first pass) error = %v", err)
	} else if ok {
		t.Fatal("FindNext(first pass) ok = true, want false until file stability is observed")
	}

	candidate, ok, err := source.FindNext(context.Background(), nowUTC.Add(11*time.Second))
	if err != nil {
		t.Fatalf("FindNext(second pass) error = %v", err)
	}
	if !ok {
		t.Fatal("FindNext(second pass) ok = false, want true")
	}
	if candidate.RelativePath != "2026-02-10/Recorder_1_25.02.10_09.00.00.00.mp4" {
		t.Fatalf("RelativePath = %q, want oldest mp4", candidate.RelativePath)
	}
}

func TestLocalSourceFindNextWaitsForQuietPeriodAfterFileGrowth(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	archiveDir := t.TempDir()
	filePath := filepath.Join(baseDir, "2026-02-10", "Recorder_1_25.02.10_09.00.00.00.mp4")
	mustWriteFile(t, filePath, []byte("demo"))

	nowUTC := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	source := NewLocalSource(baseDir, archiveDir, time.Minute)

	if _, ok, err := source.FindNext(context.Background(), nowUTC); err != nil {
		t.Fatalf("FindNext(first pass) error = %v", err)
	} else if ok {
		t.Fatal("FindNext(first pass) ok = true, want false")
	}

	if _, ok, err := source.FindNext(context.Background(), nowUTC.Add(11*time.Second)); err != nil {
		t.Fatalf("FindNext(stable pass) error = %v", err)
	} else if !ok {
		t.Fatal("FindNext(stable pass) ok = false, want true")
	}

	if err := os.WriteFile(filePath, []byte("demo-demo"), 0o644); err != nil {
		t.Fatalf("WriteFile(grow) error = %v", err)
	}
	growthTime := nowUTC.Add(20 * time.Second)
	if err := os.Chtimes(filePath, growthTime, growthTime); err != nil {
		t.Fatalf("Chtimes(grow) error = %v", err)
	}

	if _, ok, err := source.FindNext(context.Background(), growthTime); err != nil {
		t.Fatalf("FindNext(growth pass) error = %v", err)
	} else if ok {
		t.Fatal("FindNext(growth pass) ok = true, want false right after growth")
	}

	if _, ok, err := source.FindNext(context.Background(), growthTime.Add(59*time.Second)); err != nil {
		t.Fatalf("FindNext(before quiet period) error = %v", err)
	} else if ok {
		t.Fatal("FindNext(before quiet period) ok = true, want false")
	}

	candidate, ok, err := source.FindNext(context.Background(), growthTime.Add(61*time.Second))
	if err != nil {
		t.Fatalf("FindNext(after quiet period) error = %v", err)
	}
	if !ok {
		t.Fatal("FindNext(after quiet period) ok = false, want true")
	}
	if candidate.RelativePath != "2026-02-10/Recorder_1_25.02.10_09.00.00.00.mp4" {
		t.Fatalf("RelativePath = %q, want updated file", candidate.RelativePath)
	}
}

func TestLocalSourceMarkImportedMovesFileToArchive(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	archiveDir := t.TempDir()
	filePath := filepath.Join(baseDir, "2026-02-10", "Recorder_1_25.02.10_09.00.00.00.mp4")
	mustWriteFile(t, filePath, []byte("demo"))

	source := NewLocalSource(baseDir, archiveDir, time.Minute)
	candidate := mustCandidateFromPath(t, baseDir, filePath)

	if err := source.MarkImported(context.Background(), candidate); err != nil {
		t.Fatalf("MarkImported() error = %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("source file still exists, stat err = %v", err)
	}

	archivedPath := filepath.Join(archiveDir, "2026-02-10", "Recorder_1_25.02.10_09.00.00.00.mp4")
	if _, err := os.Stat(archivedPath); err != nil {
		t.Fatalf("archived file missing: %v", err)
	}
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func mustCandidateFromPath(t *testing.T, baseDir string, path string) autouploadapp.Candidate {
	t.Helper()

	relativePath, err := filepath.Rel(baseDir, path)
	if err != nil {
		t.Fatalf("filepath.Rel() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}

	return autouploadapp.Candidate{
		Name:          filepath.Base(path),
		RelativePath:  filepath.ToSlash(relativePath),
		SizeBytes:     info.Size(),
		ModifiedAtUTC: info.ModTime().UTC(),
	}
}
