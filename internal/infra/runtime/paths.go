package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolvePath tries a few stable locations so the app can start
// both from the repository root and from a built binary directory.
func ResolvePath(rel string) (string, error) {
	candidates := []string{
		rel,
		filepath.Join(projectRootFromSource(), rel),
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, rel),
			filepath.Join(exeDir, "..", rel),
		)
	}

	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if _, err := os.Stat(cleaned); err == nil {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf("resolve path %q: not found in known runtime locations", rel)
}

// SafeJoinBasePath joins baseDir and relativePath, ensuring the result
// does not escape baseDir. Returns an absolute path on success.
func SafeJoinBasePath(baseDir string, relativePath string) (string, error) {
	cleanRelativePath := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelativePath == "." || cleanRelativePath == string(filepath.Separator) {
		return "", fmt.Errorf("invalid relative path %q", relativePath)
	}
	fullPath := filepath.Join(baseDir, cleanRelativePath)

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	fullAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve full path: %w", err)
	}
	if fullAbs != baseAbs && !strings.HasPrefix(fullAbs, baseAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes base dir", relativePath)
	}

	return fullAbs, nil
}

func projectRootFromSource() string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}
