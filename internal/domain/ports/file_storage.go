package ports

import (
	"context"
	"io"
)

type StoredFile struct {
	StoredName   string
	RelativePath string
	SizeBytes    int64
}

type FileStorage interface {
	Save(ctx context.Context, originalName string, src io.Reader) (StoredFile, error)
	Delete(ctx context.Context, relativePath string) error
}
