package repositories

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"time"

	autouploadapp "media-pipeline/internal/app/autoupload"
)

type AutoUploadImportRepository struct {
	db *sql.DB
}

func NewAutoUploadImportRepository(db *sql.DB) *AutoUploadImportRepository {
	return &AutoUploadImportRepository{db: db}
}

// Begin claims an import slot for the given key. Returns Started=true when
// this call inserted the row, Started=false (along with the existing status)
// when another worker had already claimed it. We use INSERT ... ON CONFLICT
// DO NOTHING RETURNING fingerprint so the "did we insert?" check is a single
// round trip on PostgreSQL without introducing a surrogate id column.
func (r *AutoUploadImportRepository) Begin(
	ctx context.Context,
	key autouploadapp.ImportKey,
	nowUTC time.Time,
) (autouploadapp.BeginImportResult, error) {
	fingerprint := autoUploadFingerprint(key)
	now := nowUTC.UTC()

	var insertedFingerprint string
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO auto_upload_imports (
			fingerprint, source_path, size_bytes, modified_at, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (fingerprint) DO NOTHING
		RETURNING fingerprint`,
		fingerprint,
		key.RelativePath,
		key.SizeBytes,
		key.ModifiedAtUTC.UTC(),
		autouploadapp.ImportStatusImporting,
		now,
	).Scan(&insertedFingerprint)

	switch {
	case err == nil:
		// fresh insert — we own the import
		return autouploadapp.BeginImportResult{Started: true, Status: autouploadapp.ImportStatusImporting}, nil
	case errors.Is(err, sql.ErrNoRows):
		// existing row — fetch its current state
	default:
		return autouploadapp.BeginImportResult{}, fmt.Errorf("begin auto-upload import: %w", err)
	}

	var status string
	var mediaID sql.NullInt64
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT status, media_id FROM auto_upload_imports WHERE fingerprint = $1`,
		fingerprint,
	).Scan(&status, &mediaID); err != nil {
		return autouploadapp.BeginImportResult{}, fmt.Errorf("load existing auto-upload import: %w", err)
	}

	existing := autouploadapp.BeginImportResult{Started: false, Status: autouploadapp.ImportStatus(status)}
	if mediaID.Valid {
		value := mediaID.Int64
		existing.MediaID = &value
	}
	return existing, nil
}

func (r *AutoUploadImportRepository) MarkImported(
	ctx context.Context,
	key autouploadapp.ImportKey,
	mediaID int64,
	nowUTC time.Time,
) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE auto_upload_imports
		 SET status = $1, media_id = $2, updated_at = $3
		 WHERE fingerprint = $4`,
		autouploadapp.ImportStatusImported,
		mediaID,
		nowUTC.UTC(),
		autoUploadFingerprint(key),
	)
	if err != nil {
		return fmt.Errorf("mark auto-upload import imported: %w", err)
	}
	return nil
}

func (r *AutoUploadImportRepository) Delete(ctx context.Context, key autouploadapp.ImportKey) error {
	_, err := r.db.ExecContext(
		ctx,
		`DELETE FROM auto_upload_imports WHERE fingerprint = $1`,
		autoUploadFingerprint(key),
	)
	if err != nil {
		return fmt.Errorf("delete auto-upload import: %w", err)
	}
	return nil
}

func autoUploadFingerprint(key autouploadapp.ImportKey) string {
	payload := fmt.Sprintf("%s|%d|%s", key.RelativePath, key.SizeBytes, key.ModifiedAtUTC.UTC().Format(time.RFC3339Nano))
	return fmt.Sprintf("%x", sha1.Sum([]byte(payload)))
}
