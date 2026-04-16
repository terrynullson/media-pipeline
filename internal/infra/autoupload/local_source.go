package autoupload

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	autouploadapp "media-pipeline/internal/app/autoupload"
	"media-pipeline/internal/app/command"
)

type LocalSource struct {
	baseDir    string
	archiveDir string
	minFileAge time.Duration
}

func NewLocalSource(baseDir string, archiveDir string, minFileAge time.Duration) *LocalSource {
	return &LocalSource{
		baseDir:    baseDir,
		archiveDir: archiveDir,
		minFileAge: minFileAge,
	}
}

func (s *LocalSource) FindNext(_ context.Context, nowUTC time.Time) (autouploadapp.Candidate, bool, error) {
	baseDirAbs, err := filepath.Abs(s.baseDir)
	if err != nil {
		return autouploadapp.Candidate{}, false, fmt.Errorf("resolve auto-upload base dir: %w", err)
	}

	info, err := os.Stat(baseDirAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return autouploadapp.Candidate{}, false, nil
		}
		return autouploadapp.Candidate{}, false, fmt.Errorf("stat auto-upload base dir: %w", err)
	}
	if !info.IsDir() {
		return autouploadapp.Candidate{}, false, fmt.Errorf("auto-upload base path is not a directory: %s", s.baseDir)
	}

	candidates := make([]autouploadapp.Candidate, 0)
	err = filepath.WalkDir(baseDirAbs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !command.IsSupportedUploadExtension(entry.Name()) {
			return nil
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if s.minFileAge > 0 && nowUTC.Sub(fileInfo.ModTime().UTC()) < s.minFileAge {
			return nil
		}

		relativePath, err := filepath.Rel(baseDirAbs, path)
		if err != nil {
			return err
		}
		relativePath = filepath.Clean(relativePath)
		if relativePath == "." || strings.HasPrefix(relativePath, "..") {
			return fmt.Errorf("auto-upload candidate escapes base dir: %s", path)
		}

		candidates = append(candidates, autouploadapp.Candidate{
			Name:          entry.Name(),
			RelativePath:  filepath.ToSlash(relativePath),
			SizeBytes:     fileInfo.Size(),
			ModifiedAtUTC: fileInfo.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return autouploadapp.Candidate{}, false, fmt.Errorf("scan auto-upload source: %w", err)
	}
	if len(candidates) == 0 {
		return autouploadapp.Candidate{}, false, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ModifiedAtUTC.Equal(candidates[j].ModifiedAtUTC) {
			return candidates[i].RelativePath < candidates[j].RelativePath
		}
		return candidates[i].ModifiedAtUTC.Before(candidates[j].ModifiedAtUTC)
	})

	return candidates[0], true, nil
}

func (s *LocalSource) Open(_ context.Context, candidate autouploadapp.Candidate) (io.ReadCloser, error) {
	fullPath, err := s.resolveCandidatePath(candidate)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open auto-upload candidate file: %w", err)
	}

	return file, nil
}

func (s *LocalSource) MarkImported(_ context.Context, candidate autouploadapp.Candidate) error {
	sourcePath, err := s.resolveCandidatePath(candidate)
	if err != nil {
		return err
	}

	targetPath, err := s.resolveArchivePath(candidate.RelativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create auto-upload archive directory: %w", err)
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return fmt.Errorf("move imported auto-upload file to archive: %w", err)
	}

	s.cleanupEmptyParents(filepath.Dir(sourcePath))
	return nil
}

func (s *LocalSource) resolveCandidatePath(candidate autouploadapp.Candidate) (string, error) {
	baseDirAbs, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve auto-upload base dir: %w", err)
	}
	fullPath := filepath.Join(baseDirAbs, filepath.FromSlash(candidate.RelativePath))
	fullPathAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve auto-upload candidate path: %w", err)
	}
	if fullPathAbs != baseDirAbs && !strings.HasPrefix(fullPathAbs, baseDirAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("auto-upload candidate path escapes base dir: %s", candidate.RelativePath)
	}
	return fullPathAbs, nil
}

func (s *LocalSource) resolveArchivePath(relativePath string) (string, error) {
	archiveDirAbs, err := filepath.Abs(s.archiveDir)
	if err != nil {
		return "", fmt.Errorf("resolve auto-upload archive dir: %w", err)
	}
	targetPath := filepath.Join(archiveDirAbs, filepath.FromSlash(relativePath))
	targetPathAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve auto-upload archive path: %w", err)
	}
	if targetPathAbs != archiveDirAbs && !strings.HasPrefix(targetPathAbs, archiveDirAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("auto-upload archive path escapes archive dir: %s", relativePath)
	}
	return targetPathAbs, nil
}

func (s *LocalSource) cleanupEmptyParents(startDir string) {
	baseDirAbs, err := filepath.Abs(s.baseDir)
	if err != nil {
		return
	}

	current := startDir
	for current != "" && current != baseDirAbs {
		err := os.Remove(current)
		if err != nil {
			break
		}
		current = filepath.Dir(current)
	}
}
