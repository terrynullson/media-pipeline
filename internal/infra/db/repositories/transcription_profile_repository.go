package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"media-pipeline/internal/domain/transcription"
)

type TranscriptionProfileRepository struct {
	db *sql.DB
}

func NewTranscriptionProfileRepository(db *sql.DB) *TranscriptionProfileRepository {
	return &TranscriptionProfileRepository{db: db}
}

const transcriptionProfileColumns = `
	id, backend, model_name, device, compute_type, language, beam_size,
	vad_enabled, ui_theme, is_default, created_at, updated_at
`

func (r *TranscriptionProfileRepository) GetDefault(ctx context.Context) (transcription.Profile, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT `+transcriptionProfileColumns+`
		 FROM transcription_profiles
		 WHERE is_default = TRUE
		 ORDER BY id ASC
		 LIMIT 1`,
	)

	profile, ok, err := scanTranscriptionProfile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return transcription.Profile{}, false, nil
		}
		return transcription.Profile{}, false, fmt.Errorf("get default transcription profile: %w", err)
	}

	return profile, ok, nil
}

func (r *TranscriptionProfileRepository) Save(ctx context.Context, profile transcription.Profile) (transcription.Profile, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return transcription.Profile{}, fmt.Errorf("begin transcription profile tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	updatedAt := profile.UpdatedAtUTC.UTC()
	if profile.IsDefault {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE transcription_profiles
			 SET is_default = FALSE, updated_at = $1
			 WHERE is_default = TRUE AND id <> $2`,
			updatedAt,
			profile.ID,
		); err != nil {
			return transcription.Profile{}, fmt.Errorf("clear previous default transcription profile: %w", err)
		}
	}

	if profile.ID == 0 {
		var id int64
		err := tx.QueryRowContext(
			ctx,
			`INSERT INTO transcription_profiles (
				backend, model_name, device, compute_type, language, beam_size,
				vad_enabled, ui_theme, is_default, created_at, updated_at
			 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			 RETURNING id`,
			profile.Backend,
			profile.ModelName,
			profile.Device,
			profile.ComputeType,
			profile.Language,
			profile.BeamSize,
			profile.VADEnabled,
			profile.UITheme,
			profile.IsDefault,
			profile.CreatedAtUTC.UTC(),
			updatedAt,
		).Scan(&id)
		if err != nil {
			return transcription.Profile{}, fmt.Errorf("insert transcription profile: %w", err)
		}
		profile.ID = id
	} else {
		result, err := tx.ExecContext(
			ctx,
			`UPDATE transcription_profiles
			 SET backend = $1, model_name = $2, device = $3, compute_type = $4, language = $5, beam_size = $6,
			     vad_enabled = $7, ui_theme = $8, is_default = $9, updated_at = $10
			 WHERE id = $11`,
			profile.Backend,
			profile.ModelName,
			profile.Device,
			profile.ComputeType,
			profile.Language,
			profile.BeamSize,
			profile.VADEnabled,
			profile.UITheme,
			profile.IsDefault,
			updatedAt,
			profile.ID,
		)
		if err != nil {
			return transcription.Profile{}, fmt.Errorf("update transcription profile: %w", err)
		}
		if err := ensureRowsAffected(result, profile.ID, "update transcription profile"); err != nil {
			return transcription.Profile{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return transcription.Profile{}, fmt.Errorf("commit transcription profile tx: %w", err)
	}

	return profile, nil
}

func scanTranscriptionProfile(scanner rowScanner) (transcription.Profile, bool, error) {
	var profile transcription.Profile
	var createdAt, updatedAt time.Time
	if err := scanner.Scan(
		&profile.ID,
		&profile.Backend,
		&profile.ModelName,
		&profile.Device,
		&profile.ComputeType,
		&profile.Language,
		&profile.BeamSize,
		&profile.VADEnabled,
		&profile.UITheme,
		&profile.IsDefault,
		&createdAt,
		&updatedAt,
	); err != nil {
		return transcription.Profile{}, false, err
	}

	profile.CreatedAtUTC = createdAt.UTC()
	profile.UpdatedAtUTC = updatedAt.UTC()

	return profile, true, nil
}
