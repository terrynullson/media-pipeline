package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domainsummary "media-pipeline/internal/domain/summary"
)

type SummaryRepository struct {
	db *sql.DB
}

func NewSummaryRepository(db *sql.DB) *SummaryRepository {
	return &SummaryRepository{db: db}
}

func (r *SummaryRepository) Save(ctx context.Context, item domainsummary.Summary) error {
	highlightsJSON, err := json.Marshal(item.Highlights)
	if err != nil {
		return fmt.Errorf("marshal summary highlights: %w", err)
	}

	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO summaries (
			media_id, summary_text, highlights_json, provider, created_at, updated_at
		 ) VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (media_id) DO UPDATE SET
			summary_text    = EXCLUDED.summary_text,
			highlights_json = EXCLUDED.highlights_json,
			provider        = EXCLUDED.provider,
			updated_at      = EXCLUDED.updated_at`,
		item.MediaID,
		item.SummaryText,
		string(highlightsJSON),
		item.Provider,
		item.CreatedAtUTC.UTC(),
		item.UpdatedAtUTC.UTC(),
	)
	if err != nil {
		return fmt.Errorf("save summary: %w", err)
	}

	return nil
}

func (r *SummaryRepository) GetByMediaID(ctx context.Context, mediaID int64) (domainsummary.Summary, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, media_id, summary_text, highlights_json, provider, created_at, updated_at
		 FROM summaries
		 WHERE media_id = $1`,
		mediaID,
	)

	var item domainsummary.Summary
	var highlightsJSON string
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&item.ID,
		&item.MediaID,
		&item.SummaryText,
		&highlightsJSON,
		&item.Provider,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainsummary.Summary{}, false, nil
		}
		return domainsummary.Summary{}, false, fmt.Errorf("scan summary by media id %d: %w", mediaID, err)
	}

	if err := json.Unmarshal([]byte(highlightsJSON), &item.Highlights); err != nil {
		return domainsummary.Summary{}, false, fmt.Errorf("unmarshal summary highlights: %w", err)
	}

	item.CreatedAtUTC = createdAt.UTC()
	item.UpdatedAtUTC = updatedAt.UTC()

	return item, true, nil
}
