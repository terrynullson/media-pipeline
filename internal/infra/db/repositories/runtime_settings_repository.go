package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"media-pipeline/internal/domain/appsettings"
)

type RuntimeSettingsRepository struct {
	db *sql.DB
}

func NewRuntimeSettingsRepository(db *sql.DB) *RuntimeSettingsRepository {
	return &RuntimeSettingsRepository{db: db}
}

func (r *RuntimeSettingsRepository) Get(ctx context.Context) (appsettings.Settings, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, auto_upload_min_age_sec, preview_timeout_sec, max_upload_size_mb, stop_words, created_at, updated_at
		 FROM runtime_settings
		 WHERE id = 1
		 LIMIT 1`,
	)

	var item appsettings.Settings
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&item.ID,
		&item.AutoUploadMinAgeSec,
		&item.PreviewTimeoutSec,
		&item.MaxUploadSizeMB,
		&item.StopWords,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appsettings.Settings{}, false, nil
		}
		return appsettings.Settings{}, false, fmt.Errorf("get runtime settings: %w", err)
	}

	item.CreatedAtUTC = createdAt.UTC()
	item.UpdatedAtUTC = updatedAt.UTC()

	return item, true, nil
}

func (r *RuntimeSettingsRepository) Save(ctx context.Context, settings appsettings.Settings) (appsettings.Settings, error) {
	if settings.ID == 0 {
		settings.ID = 1
	}

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO runtime_settings (
			id, auto_upload_min_age_sec, preview_timeout_sec, max_upload_size_mb, stop_words, created_at, updated_at
		 ) VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (id) DO UPDATE SET
			auto_upload_min_age_sec = EXCLUDED.auto_upload_min_age_sec,
			preview_timeout_sec     = EXCLUDED.preview_timeout_sec,
			max_upload_size_mb      = EXCLUDED.max_upload_size_mb,
			stop_words              = EXCLUDED.stop_words,
			updated_at              = EXCLUDED.updated_at`,
		settings.ID,
		settings.AutoUploadMinAgeSec,
		settings.PreviewTimeoutSec,
		settings.MaxUploadSizeMB,
		settings.StopWords,
		settings.CreatedAtUTC.UTC(),
		settings.UpdatedAtUTC.UTC(),
	)
	if err != nil {
		return appsettings.Settings{}, fmt.Errorf("save runtime settings: %w", err)
	}

	return settings, nil
}
