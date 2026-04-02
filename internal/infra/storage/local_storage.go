package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"media-pipeline/internal/domain/ports"
)

type LocalStorage struct {
	baseDir string
}

func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{baseDir: baseDir}
}

func (s *LocalStorage) Save(ctx context.Context, originalName string, src io.Reader) (ports.StoredFile, error) {
	if err := ctx.Err(); err != nil {
		return ports.StoredFile{}, err
	}

	dateDir := time.Now().UTC().Format("2006-01-02")
	targetDir := filepath.Join(s.baseDir, dateDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return ports.StoredFile{}, fmt.Errorf("create upload directory: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = ".bin"
	}

	storedName := fmt.Sprintf("%s_%d%s", time.Now().UTC().Format("20060102T150405.000000000"), time.Now().UTC().UnixNano(), ext)
	fullPath := filepath.Join(targetDir, storedName)

	dst, err := os.Create(fullPath)
	if err != nil {
		return ports.StoredFile{}, fmt.Errorf("create destination file: %w", err)
	}
	cleanupPath := fullPath
	defer func() {
		_ = dst.Close()
	}()

	written, err := io.Copy(dst, &contextReader{ctx: ctx, r: src})
	if err != nil {
		_ = dst.Close()
		_ = os.Remove(cleanupPath)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ports.StoredFile{}, err
		}
		return ports.StoredFile{}, fmt.Errorf("copy upload content: %w", err)
	}
	if written == 0 {
		_ = dst.Close()
		_ = os.Remove(cleanupPath)
		return ports.StoredFile{}, fmt.Errorf("uploaded file is empty")
	}
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		_ = os.Remove(cleanupPath)
		return ports.StoredFile{}, fmt.Errorf("sync destination file: %w", err)
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(cleanupPath)
		return ports.StoredFile{}, fmt.Errorf("close destination file: %w", err)
	}
	cleanupPath = ""

	relativePath := filepath.ToSlash(filepath.Join(dateDir, storedName))
	return ports.StoredFile{
		StoredName:   storedName,
		RelativePath: relativePath,
		SizeBytes:    written,
	}, nil
}

func (s *LocalStorage) Delete(_ context.Context, relativePath string) error {
	if strings.TrimSpace(relativePath) == "" {
		return fmt.Errorf("delete storage file: empty relative path")
	}

	cleanRelativePath := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelativePath == "." || cleanRelativePath == string(filepath.Separator) {
		return fmt.Errorf("delete storage file: invalid relative path %q", relativePath)
	}

	fullPath := filepath.Join(s.baseDir, cleanRelativePath)
	baseDirAbs, err := filepath.Abs(s.baseDir)
	if err != nil {
		return fmt.Errorf("resolve storage base dir: %w", err)
	}
	fullPathAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("resolve storage file path: %w", err)
	}
	if fullPathAbs != baseDirAbs && !strings.HasPrefix(fullPathAbs, baseDirAbs+string(filepath.Separator)) {
		return fmt.Errorf("delete storage file: path %q escapes base dir", relativePath)
	}

	if err := os.Remove(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete storage file %q: %w", relativePath, err)
	}

	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *contextReader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}
