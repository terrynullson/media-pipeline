package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domaintrigger "media-pipeline/internal/domain/trigger"
)

type TriggerScreenshotRepository struct {
	db *sql.DB
}

func NewTriggerScreenshotRepository(db *sql.DB) *TriggerScreenshotRepository {
	return &TriggerScreenshotRepository{db: db}
}

func (r *TriggerScreenshotRepository) ReplaceForMedia(
	ctx context.Context,
	mediaID int64,
	items []domaintrigger.Screenshot,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin trigger screenshot tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM trigger_event_screenshots WHERE media_id = ?", mediaID); err != nil {
		return fmt.Errorf("delete trigger screenshots by media: %w", err)
	}

	for index, item := range items {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO trigger_event_screenshots (
				media_id, trigger_event_id, timestamp_sec, image_path, width, height, created_at
			 ) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			item.MediaID,
			item.TriggerEventID,
			item.TimestampSec,
			item.ImagePath,
			item.Width,
			item.Height,
			item.CreatedAtUTC.Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("insert trigger screenshot %d: %w", index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit trigger screenshot tx: %w", err)
	}

	return nil
}

func (r *TriggerScreenshotRepository) ListByMediaID(ctx context.Context, mediaID int64) ([]domaintrigger.Screenshot, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, media_id, trigger_event_id, timestamp_sec, image_path, width, height, created_at
		 FROM trigger_event_screenshots
		 WHERE media_id = ?
		 ORDER BY trigger_event_id ASC, id ASC`,
		mediaID,
	)
	if err != nil {
		return nil, fmt.Errorf("query trigger screenshots by media id %d: %w", mediaID, err)
	}
	defer rows.Close()

	items := make([]domaintrigger.Screenshot, 0)
	for rows.Next() {
		item, err := scanTriggerScreenshot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trigger screenshots: %w", err)
	}

	return items, nil
}

func (r *TriggerScreenshotRepository) ListPathsByMediaID(ctx context.Context, mediaID int64) ([]string, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT image_path
		 FROM trigger_event_screenshots
		 WHERE media_id = ?
		 ORDER BY id ASC`,
		mediaID,
	)
	if err != nil {
		return nil, fmt.Errorf("query trigger screenshot paths by media id %d: %w", mediaID, err)
	}
	defer rows.Close()

	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scan trigger screenshot path: %w", err)
		}
		paths = append(paths, path)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trigger screenshot paths: %w", err)
	}

	return paths, nil
}

func scanTriggerScreenshot(scanner interface {
	Scan(dest ...any) error
}) (domaintrigger.Screenshot, error) {
	var item domaintrigger.Screenshot
	var createdAt string

	if err := scanner.Scan(
		&item.ID,
		&item.MediaID,
		&item.TriggerEventID,
		&item.TimestampSec,
		&item.ImagePath,
		&item.Width,
		&item.Height,
		&createdAt,
	); err != nil {
		return domaintrigger.Screenshot{}, fmt.Errorf("scan trigger screenshot: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return domaintrigger.Screenshot{}, fmt.Errorf("parse trigger screenshot created_at: %w", err)
	}
	item.CreatedAtUTC = parsedCreatedAt

	return item, nil
}
