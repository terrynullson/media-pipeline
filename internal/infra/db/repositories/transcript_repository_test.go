package repositories

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
)

func TestTranscriptRepository_SavePersistsTranscriptAndSegments(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	transcriptRepo := NewTranscriptRepository(sqlDB)

	nowUTC := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName:       "sample.wav",
		StoredName:         "sample.wav",
		Extension:          ".wav",
		MIMEType:           "audio/wav",
		SizeBytes:          2048,
		StoragePath:        "2026-04-03/sample.wav",
		ExtractedAudioPath: "2026-04-03/media_1_sample.wav",
		Status:             media.StatusAudioExtracted,
		CreatedAtUTC:       nowUTC,
		UpdatedAtUTC:       nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	confidence := 0.91
	err = transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "ru",
		FullText:     "privet mir",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1.25, Text: "privet", Confidence: &confidence},
			{StartSec: 1.25, EndSec: 2.5, Text: "mir"},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var fullText string
	var language string
	if err := sqlDB.QueryRowContext(ctx, "SELECT full_text, language FROM transcripts WHERE media_id = ?", mediaID).
		Scan(&fullText, &language); err != nil {
		t.Fatalf("QueryRow(transcripts) error = %v", err)
	}
	if fullText != "privet mir" {
		t.Fatalf("full_text = %q, want %q", fullText, "privet mir")
	}
	if language != "ru" {
		t.Fatalf("language = %q, want %q", language, "ru")
	}

	rows, err := sqlDB.QueryContext(
		ctx,
		`SELECT segment_index, start_sec, end_sec, text, confidence
		 FROM transcript_segments
		 WHERE transcript_id = (SELECT id FROM transcripts WHERE media_id = ?)
		 ORDER BY segment_index ASC`,
		mediaID,
	)
	if err != nil {
		t.Fatalf("Query(transcript_segments) error = %v", err)
	}
	defer rows.Close()

	type dbSegment struct {
		index      int
		startSec   float64
		endSec     float64
		text       string
		confidence *float64
	}

	var segments []dbSegment
	for rows.Next() {
		var item dbSegment
		if err := rows.Scan(&item.index, &item.startSec, &item.endSec, &item.text, &item.confidence); err != nil {
			t.Fatalf("Scan(segment) error = %v", err)
		}
		segments = append(segments, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Rows(segment) error = %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("segments count = %d, want 2", len(segments))
	}
	if segments[0].text != "privet" || segments[1].text != "mir" {
		t.Fatalf("segment texts = %#v, want privet/mir", segments)
	}
}
