package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func projectRootFromSource() string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}
