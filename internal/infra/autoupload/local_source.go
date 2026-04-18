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
	"sync"
	"time"

	autouploadapp "media-pipeline/internal/app/autoupload"
	"media-pipeline/internal/app/command"
	appsettings "media-pipeline/internal/domain/appsettings"
)

const defaultStabilityProbeInterval = 10 * time.Second

type LocalSource struct {
	baseDir                string
	archiveDir             string
	minFileAge             time.Duration
	stabilityProbeInterval time.Duration
	provider               AutoUploadMinAgeProvider

	mu           sync.Mutex
	observations map[string]fileObservation
}

type fileObservation struct {
	SizeBytes     int64
	ModifiedAtUTC time.Time
	FirstSeenAt   time.Time
	LastGrowthAt  time.Time
}

type AutoUploadMinAgeProvider interface {
	GetCurrent(ctx context.Context) (appsettings.Settings, error)
}

func NewLocalSource(baseDir string, archiveDir string, minFileAge time.Duration) *LocalSource {
	return &LocalSource{
		baseDir:                baseDir,
		archiveDir:             archiveDir,
		minFileAge:             minFileAge,
		stabilityProbeInterval: defaultStabilityProbeInterval,
		observations:           make(map[string]fileObservation),
	}
}

func (s *LocalSource) WithMinAgeProvider(provider AutoUploadMinAgeProvider) *LocalSource {
	s.provider = provider
	return s
}

func (s *LocalSource) FindNext(ctx context.Context, nowUTC time.Time) (autouploadapp.Candidate, bool, error) {
	minFileAge := s.minFileAge
	if s.provider != nil {
		settings, err := s.provider.GetCurrent(ctx)
		if err == nil {
			minFileAge = settings.AutoUploadMinAge()
		}
	}

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
	seenPaths := make(map[string]struct{})
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

		relativePath, err := filepath.Rel(baseDirAbs, path)
		if err != nil {
			return err
		}
		relativePath = filepath.Clean(relativePath)
		if relativePath == "." || strings.HasPrefix(relativePath, "..") {
			return fmt.Errorf("auto-upload candidate escapes base dir: %s", path)
		}
		relativePath = filepath.ToSlash(relativePath)
		seenPaths[relativePath] = struct{}{}

		if !s.isStableCandidate(relativePath, fileInfo, nowUTC, minFileAge) {
			return nil
		}

		candidates = append(candidates, autouploadapp.Candidate{
			Name:          entry.Name(),
			RelativePath:  relativePath,
			SizeBytes:     fileInfo.Size(),
			ModifiedAtUTC: fileInfo.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return autouploadapp.Candidate{}, false, fmt.Errorf("scan auto-upload source: %w", err)
	}

	s.pruneObservations(seenPaths)

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

	s.mu.Lock()
	delete(s.observations, candidate.RelativePath)
	s.mu.Unlock()

	s.cleanupEmptyParents(filepath.Dir(sourcePath))
	return nil
}

func (s *LocalSource) isStableCandidate(relativePath string, info fs.FileInfo, nowUTC time.Time, minFileAge time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.observations == nil {
		s.observations = make(map[string]fileObservation)
	}

	modifiedAtUTC := info.ModTime().UTC()
	current := fileObservation{
		SizeBytes:     info.Size(),
		ModifiedAtUTC: modifiedAtUTC,
		FirstSeenAt:   nowUTC,
	}

	previous, ok := s.observations[relativePath]
	if !ok {
		s.observations[relativePath] = current
		return false
	}

	if current.SizeBytes != previous.SizeBytes || !current.ModifiedAtUTC.Equal(previous.ModifiedAtUTC) {
		previous.SizeBytes = current.SizeBytes
		previous.ModifiedAtUTC = current.ModifiedAtUTC
		previous.LastGrowthAt = nowUTC
		s.observations[relativePath] = previous
		return false
	}

	s.observations[relativePath] = previous

	if !previous.LastGrowthAt.IsZero() {
		if minFileAge <= 0 {
			return true
		}
		return nowUTC.Sub(previous.LastGrowthAt) >= minFileAge
	}

	return nowUTC.Sub(previous.FirstSeenAt) >= s.stabilityProbeInterval
}

func (s *LocalSource) pruneObservations(seenPaths map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for path := range s.observations {
		if _, ok := seenPaths[path]; ok {
			continue
		}
		delete(s.observations, path)
	}
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
