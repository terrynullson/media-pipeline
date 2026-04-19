package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	domaintrigger "media-pipeline/internal/domain/trigger"
)

type TriggerEventRepository struct {
	db *sql.DB
}

func NewTriggerEventRepository(db *sql.DB) *TriggerEventRepository {
	return &TriggerEventRepository{db: db}
}

func (r *TriggerEventRepository) ReplaceForMedia(ctx context.Context, mediaID int64, transcriptID *int64, events []domaintrigger.Event) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin trigger event tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if transcriptID != nil {
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM trigger_events
			 WHERE media_id = $1 AND transcript_id = $2`,
			mediaID,
			*transcriptID,
		); err != nil {
			return fmt.Errorf("delete trigger events by transcript: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM trigger_events WHERE media_id = $1`, mediaID); err != nil {
			return fmt.Errorf("delete trigger events by media: %w", err)
		}
	}

	for index, event := range events {
		var transcriptValue any
		if event.TranscriptID != nil {
			transcriptValue = *event.TranscriptID
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO trigger_events (
				media_id, transcript_id, rule_id, category, matched_text, segment_index,
				start_sec, end_sec, segment_text, context_text, created_at
			 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			event.MediaID,
			transcriptValue,
			event.RuleID,
			event.Category,
			event.MatchedText,
			event.SegmentIndex,
			event.StartSec,
			event.EndSec,
			event.SegmentText,
			event.ContextText,
			event.CreatedAtUTC.UTC(),
		); err != nil {
			return fmt.Errorf("insert trigger event %d: %w", index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit trigger event tx: %w", err)
	}

	return nil
}

func (r *TriggerEventRepository) ListByMediaID(ctx context.Context, mediaID int64) ([]domaintrigger.Event, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT e.id, e.media_id, e.transcript_id, e.rule_id, r.name, e.category, e.matched_text,
		        e.segment_index, e.start_sec, e.end_sec, e.segment_text, e.context_text, e.created_at
		 FROM trigger_events e
		 JOIN trigger_rules r ON r.id = e.rule_id
		 WHERE e.media_id = $1
		 ORDER BY e.start_sec ASC, e.segment_index ASC, e.id ASC`,
		mediaID,
	)
	if err != nil {
		return nil, fmt.Errorf("query trigger events by media id %d: %w", mediaID, err)
	}
	defer rows.Close()

	items := make([]domaintrigger.Event, 0)
	for rows.Next() {
		item, err := scanTriggerEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trigger events: %w", err)
	}

	return items, nil
}

func (r *TriggerEventRepository) CountByMediaIDs(ctx context.Context, mediaIDs []int64) (map[int64]int, error) {
	counts := make(map[int64]int, len(mediaIDs))
	if len(mediaIDs) == 0 {
		return counts, nil
	}

	placeholders := make([]string, 0, len(mediaIDs))
	args := make([]any, 0, len(mediaIDs))
	for i, mediaID := range mediaIDs {
		placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
		args = append(args, mediaID)
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT media_id, COUNT(*)
		 FROM trigger_events
		 WHERE media_id IN (`+strings.Join(placeholders, ", ")+`)
		 GROUP BY media_id`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("count trigger events by media ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mediaID int64
		var count int
		if err := rows.Scan(&mediaID, &count); err != nil {
			return nil, fmt.Errorf("scan trigger event count: %w", err)
		}
		counts[mediaID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trigger event counts: %w", err)
	}

	return counts, nil
}

func scanTriggerEvent(scanner rowScanner) (domaintrigger.Event, error) {
	var item domaintrigger.Event
	var transcriptID sql.NullInt64
	var createdAt time.Time

	if err := scanner.Scan(
		&item.ID,
		&item.MediaID,
		&transcriptID,
		&item.RuleID,
		&item.RuleName,
		&item.Category,
		&item.MatchedText,
		&item.SegmentIndex,
		&item.StartSec,
		&item.EndSec,
		&item.SegmentText,
		&item.ContextText,
		&createdAt,
	); err != nil {
		return domaintrigger.Event{}, fmt.Errorf("scan trigger event: %w", err)
	}

	if transcriptID.Valid {
		item.TranscriptID = &transcriptID.Int64
	}
	item.CreatedAtUTC = createdAt.UTC()

	return item, nil
}
