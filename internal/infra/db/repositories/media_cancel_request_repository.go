package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type MediaCancelRequestRepository struct {
	db *sql.DB
}

func NewMediaCancelRequestRepository(db *sql.DB) *MediaCancelRequestRepository {
	return &MediaCancelRequestRepository{db: db}
}

func (r *MediaCancelRequestRepository) Request(ctx context.Context, mediaID int64, requestedAtUTC time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO media_cancel_requests (media_id, requested_at)
		 VALUES ($1, $2)
		 ON CONFLICT (media_id) DO UPDATE SET requested_at = EXCLUDED.requested_at`,
		mediaID,
		requestedAtUTC.UTC(),
	)
	if err != nil {
		return fmt.Errorf("request media cancellation: %w", err)
	}
	return nil
}

func (r *MediaCancelRequestRepository) Exists(ctx context.Context, mediaID int64) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM media_cancel_requests WHERE media_id = $1 LIMIT 1`,
		mediaID,
	).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("check media cancellation request: %w", err)
}

func (r *MediaCancelRequestRepository) Delete(ctx context.Context, mediaID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM media_cancel_requests WHERE media_id = $1`, mediaID)
	if err != nil {
		return fmt.Errorf("delete media cancellation request: %w", err)
	}
	return nil
}
