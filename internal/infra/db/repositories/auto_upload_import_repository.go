package repositories

import (
	"context"
	"crypto/sha1"
	"database/sql"
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

func (r *AutoUploadImportRepository) Begin(ctx context.Context, key autouploadapp.ImportKey, nowUTC time.Time) (autouploadapp.BeginImportResult, error) {
	fingerprint := autoUploadFingerprint(key)

	result, err := r.db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO auto_upload_imports (
			fingerprint, source_path, size_bytes, modified_at, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fingerprint,
		key.RelativePath,
		key.SizeBytes,
		key.ModifiedAtUTC.UTC().Format(time.RFC3339Nano),
		autouploadapp.ImportStatusImporting,
		nowUTC.UTC().Format(time.RFC3339),
		nowUTC.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return autouploadapp.BeginImportResult{}, fmt.Errorf("begin auto-upload import: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return autouploadapp.BeginImportResult{}, fmt.Errorf("begin auto-upload import rows affected: %w", err)
	}
	if rowsAffected > 0 {
		return autouploadapp.BeginImportResult{Started: true, Status: autouploadapp.ImportStatusImporting}, nil
	}

	var status string
	var mediaID sql.NullInt64
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT status, media_id FROM auto_upload_imports WHERE fingerprint = ?`,
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

func (r *AutoUploadImportRepository) MarkImported(ctx context.Context, key autouploadapp.ImportKey, mediaID int64, nowUTC time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE auto_upload_imports
		 SET status = ?, media_id = ?, updated_at = ?
		 WHERE fingerprint = ?`,
		autouploadapp.ImportStatusImported,
		mediaID,
		nowUTC.UTC().Format(time.RFC3339),
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
		`DELETE FROM auto_upload_imports WHERE fingerprint = ?`,
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
