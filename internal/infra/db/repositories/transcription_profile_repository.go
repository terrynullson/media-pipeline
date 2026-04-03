package repositories

import (
	"context"
	"database/sql"
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

func (r *TranscriptionProfileRepository) GetDefault(ctx context.Context) (transcription.Profile, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, backend, model_name, device, compute_type, language, beam_size, vad_enabled, is_default, created_at, updated_at
		 FROM transcription_profiles
		 WHERE is_default = 1
		 ORDER BY id ASC
		 LIMIT 1`,
	)

	profile, ok, err := scanTranscriptionProfile(row)
	if err != nil {
		if err == sql.ErrNoRows {
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
	defer tx.Rollback()

	updatedAt := profile.UpdatedAtUTC.Format(time.RFC3339)
	if profile.IsDefault {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE transcription_profiles
			 SET is_default = 0, updated_at = ?
			 WHERE is_default = 1 AND id <> ?`,
			updatedAt,
			profile.ID,
		); err != nil {
			return transcription.Profile{}, fmt.Errorf("clear previous default transcription profile: %w", err)
		}
	}

	if profile.ID == 0 {
		result, err := tx.ExecContext(
			ctx,
			`INSERT INTO transcription_profiles (
				backend, model_name, device, compute_type, language, beam_size, vad_enabled, is_default, created_at, updated_at
			 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			profile.Backend,
			profile.ModelName,
			profile.Device,
			profile.ComputeType,
			profile.Language,
			profile.BeamSize,
			boolToInt(profile.VADEnabled),
			boolToInt(profile.IsDefault),
			profile.CreatedAtUTC.Format(time.RFC3339),
			updatedAt,
		)
		if err != nil {
			return transcription.Profile{}, fmt.Errorf("insert transcription profile: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return transcription.Profile{}, fmt.Errorf("transcription profile last insert id: %w", err)
		}
		profile.ID = id
	} else {
		result, err := tx.ExecContext(
			ctx,
			`UPDATE transcription_profiles
			 SET backend = ?, model_name = ?, device = ?, compute_type = ?, language = ?, beam_size = ?,
			     vad_enabled = ?, is_default = ?, updated_at = ?
			 WHERE id = ?`,
			profile.Backend,
			profile.ModelName,
			profile.Device,
			profile.ComputeType,
			profile.Language,
			profile.BeamSize,
			boolToInt(profile.VADEnabled),
			boolToInt(profile.IsDefault),
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

func scanTranscriptionProfile(scanner interface {
	Scan(dest ...any) error
}) (transcription.Profile, bool, error) {
	var profile transcription.Profile
	var createdAt string
	var updatedAt string
	var vadEnabled int
	var isDefault int
	if err := scanner.Scan(
		&profile.ID,
		&profile.Backend,
		&profile.ModelName,
		&profile.Device,
		&profile.ComputeType,
		&profile.Language,
		&profile.BeamSize,
		&vadEnabled,
		&isDefault,
		&createdAt,
		&updatedAt,
	); err != nil {
		return transcription.Profile{}, false, err
	}

	var err error
	profile.CreatedAtUTC, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return transcription.Profile{}, false, fmt.Errorf("parse transcription profile created_at: %w", err)
	}
	profile.UpdatedAtUTC, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return transcription.Profile{}, false, fmt.Errorf("parse transcription profile updated_at: %w", err)
	}
	profile.VADEnabled = vadEnabled == 1
	profile.IsDefault = isDefault == 1

	return profile, true, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
