package command

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

var allowedExtensions = map[string]struct{}{
	".mp4":  {},
	".mov":  {},
	".mkv":  {},
	".avi":  {},
	".webm": {},
	".mp3":  {},
	".wav":  {},
	".m4a":  {},
	".aac":  {},
	".flac": {},
}

func validateUploadInput(in UploadMediaInput, maxUploadBytes int64) (string, *bufio.Reader, string, error) {
	if in.Content == nil {
		return "", nil, "", fmt.Errorf("file content is required")
	}
	if strings.TrimSpace(in.OriginalName) == "" {
		return "", nil, "", fmt.Errorf("file name is required")
	}
	if in.SizeBytes == 0 {
		return "", nil, "", fmt.Errorf("empty file is not allowed")
	}
	if in.SizeBytes > 0 && in.SizeBytes > maxUploadBytes {
		return "", nil, "", fmt.Errorf("file exceeds max size of %d bytes", maxUploadBytes)
	}

	ext := strings.ToLower(filepath.Ext(in.OriginalName))
	if _, ok := allowedExtensions[ext]; !ok {
		return "", nil, "", fmt.Errorf("unsupported file format: %s", ext)
	}

	buffered := bufio.NewReader(in.Content)
	sniffed, err := sniffContentType(buffered)
	if err != nil {
		return "", nil, "", fmt.Errorf("inspect uploaded file: %w", err)
	}
	if err := validateDetectedContentType(ext, sniffed); err != nil {
		return "", nil, "", err
	}

	return ext, buffered, sniffed, nil
}

func sniffContentType(r *bufio.Reader) (string, error) {
	header, err := r.Peek(512)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		return "", fmt.Errorf("peek upload header: %w", err)
	}
	if len(header) == 0 {
		return "", fmt.Errorf("empty file is not allowed")
	}

	return http.DetectContentType(header), nil
}

func validateDetectedContentType(ext, contentType string) error {
	if contentType == "" {
		return fmt.Errorf("uploaded file type could not be detected")
	}

	if strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/") {
		return nil
	}

	// Some media containers are not recognized precisely by DetectContentType.
	if contentType == "application/octet-stream" && ext != "" {
		return nil
	}

	return fmt.Errorf("uploaded file content type is not supported: %s", contentType)
}
