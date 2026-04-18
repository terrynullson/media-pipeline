package repositories

import (
	"context"
	"database/sql"
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
		`SELECT id, auto_upload_min_age_sec, preview_timeout_sec, max_upload_size_mb, created_at, updated_at
		 FROM runtime_settings
		 WHERE id = 1
		 LIMIT 1`,
	)

	var item appsettings.Settings
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&item.ID,
		&item.AutoUploadMinAgeSec,
		&item.PreviewTimeoutSec,
		&item.MaxUploadSizeMB,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return appsettings.Settings{}, false, nil
		}
		return appsettings.Settings{}, false, fmt.Errorf("get runtime settings: %w", err)
	}

	var err error
	item.CreatedAtUTC, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return appsettings.Settings{}, false, fmt.Errorf("parse runtime settings created_at: %w", err)
	}
	item.UpdatedAtUTC, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return appsettings.Settings{}, false, fmt.Errorf("parse runtime settings updated_at: %w", err)
	}

	return item, true, nil
}

func (r *RuntimeSettingsRepository) Save(ctx context.Context, settings appsettings.Settings) (appsettings.Settings, error) {
	if settings.ID == 0 {
		settings.ID = 1
	}

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO runtime_settings (
			id, auto_upload_min_age_sec, preview_timeout_sec, max_upload_size_mb, created_at, updated_at
		 ) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		 	auto_upload_min_age_sec = excluded.auto_upload_min_age_sec,
		 	preview_timeout_sec = excluded.preview_timeout_sec,
		 	max_upload_size_mb = excluded.max_upload_size_mb,
		 	updated_at = excluded.updated_at`,
		settings.ID,
		settings.AutoUploadMinAgeSec,
		settings.PreviewTimeoutSec,
		settings.MaxUploadSizeMB,
		settings.CreatedAtUTC.Format(time.RFC3339),
		settings.UpdatedAtUTC.Format(time.RFC3339),
	)
	if err != nil {
		return appsettings.Settings{}, fmt.Errorf("save runtime settings: %w", err)
	}

	return settings, nil
}
