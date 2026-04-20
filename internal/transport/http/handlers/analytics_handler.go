package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"media-pipeline/internal/domain/appsettings"
	"media-pipeline/internal/observability"
)

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSON(r *http.Request, target any) error {
	return json.NewDecoder(r.Body).Decode(target)
}

// AnalyticsSettingsService is a narrow view of the runtime settings service —
// we only need read access to pull the user-configured stop-words list, and
// write access for the dedicated stop-words endpoint.
type AnalyticsSettingsService interface {
	GetCurrent(ctx context.Context) (appsettings.Settings, error)
	SaveCurrent(ctx context.Context, settings appsettings.Settings) (appsettings.Settings, error)
}

type AnalyticsHandler struct {
	db          *sql.DB
	settingsSvc AnalyticsSettingsService
	logger      *slog.Logger
}

func NewAnalyticsHandler(db *sql.DB, settingsSvc AnalyticsSettingsService, logger *slog.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{db: db, settingsSvc: settingsSvc, logger: logger}
}

// ─── GET /api/analytics ───────────────────────────────────────────────────────

type analyticsMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Help  string `json:"help"`
}

type topWord struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

type sourceBreakdown struct {
	Source          string  `json:"source"`
	MediaCount      int     `json:"mediaCount"`
	TotalDurationS  float64 `json:"totalDurationSec"`
	TranscriptCount int     `json:"transcriptCount"`
}

type dayActivity struct {
	Date       string `json:"date"`
	MediaCount int    `json:"mediaCount"`
}

func (h *AnalyticsHandler) APIAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := observability.LoggerFromContext(ctx, h.logger)

	settings, err := h.settingsSvc.GetCurrent(ctx)
	if err != nil {
		logger.Error("analytics: load settings", slog.Any("error", err))
	}
	stop := parseStopWords(settings.StopWords)

	overview, err := h.loadOverview(ctx)
	if err != nil {
		logger.Error("analytics: overview", slog.Any("error", err))
		http.Error(w, "не удалось загрузить аналитику", http.StatusInternalServerError)
		return
	}

	words, err := h.loadTopWords(ctx, stop, 30)
	if err != nil {
		logger.Error("analytics: top words", slog.Any("error", err))
		http.Error(w, "не удалось загрузить топ слов", http.StatusInternalServerError)
		return
	}

	sources, err := h.loadSources(ctx)
	if err != nil {
		logger.Error("analytics: sources", slog.Any("error", err))
		http.Error(w, "не удалось загрузить источники", http.StatusInternalServerError)
		return
	}

	activity, err := h.loadActivity(ctx, 30)
	if err != nil {
		logger.Error("analytics: activity", slog.Any("error", err))
		http.Error(w, "не удалось загрузить активность", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"overview":  overview,
		"topWords":  words,
		"sources":   sources,
		"activity":  activity,
		"stopWords": stop,
	})
}

func (h *AnalyticsHandler) loadOverview(ctx context.Context) ([]analyticsMetric, error) {
	var mediaCount, transcriptCount, segmentCount int
	var totalWords, totalDuration sql.NullFloat64

	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media`).Scan(&mediaCount); err != nil {
		return nil, err
	}
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transcripts`).Scan(&transcriptCount); err != nil {
		return nil, err
	}
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transcript_segments`).Scan(&segmentCount); err != nil {
		return nil, err
	}
	if err := h.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(end_sec - start_sec), 0) FROM transcript_segments`,
	).Scan(&totalDuration); err != nil {
		return nil, err
	}
	if err := h.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(array_length(regexp_split_to_array(trim(text), '\s+'), 1)), 0) FROM transcript_segments WHERE trim(text) <> ''`,
	).Scan(&totalWords); err != nil {
		return nil, err
	}

	return []analyticsMetric{
		{Label: "Медиа", Value: strconv.Itoa(mediaCount), Help: "Файлов всего"},
		{Label: "Транскрипций", Value: strconv.Itoa(transcriptCount), Help: "Расшифрованных медиа"},
		{Label: "Сегментов", Value: strconv.Itoa(segmentCount), Help: "Фраз в транскрипциях"},
		{Label: "Слов", Value: strconv.FormatInt(int64(totalWords.Float64), 10), Help: "Всего слов в транскрипциях"},
		{Label: "Часов речи", Value: fmt.Sprintf("%.1f", totalDuration.Float64/3600.0), Help: "Суммарная длительность сегментов"},
	}, nil
}

func (h *AnalyticsHandler) loadTopWords(ctx context.Context, stopWords []string, limit int) ([]topWord, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT text FROM transcript_segments WHERE trim(text) <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stopSet := make(map[string]struct{}, len(stopWords))
	for _, s := range stopWords {
		stopSet[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
	}

	counts := make(map[string]int, 1024)
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		for _, word := range tokenize(text) {
			if len(word) < 2 {
				continue
			}
			if _, skip := stopSet[word]; skip {
				continue
			}
			counts[word]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	list := make([]topWord, 0, len(counts))
	for w, c := range counts {
		list = append(list, topWord{Word: w, Count: c})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Word < list[j].Word
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (h *AnalyticsHandler) loadSources(ctx context.Context) ([]sourceBreakdown, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT
			COALESCE(NULLIF(m.source_name, ''), '—') AS source,
			COUNT(DISTINCT m.id) AS media_count,
			COUNT(DISTINCT t.id) AS transcript_count,
			COALESCE(SUM(EXTRACT(EPOCH FROM (m.recording_ended_at - m.recording_started_at))), 0) AS duration
		FROM media m
		LEFT JOIN transcripts t ON t.media_id = m.id
		GROUP BY source
		ORDER BY media_count DESC, source ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]sourceBreakdown, 0)
	for rows.Next() {
		var item sourceBreakdown
		var dur sql.NullFloat64
		if err := rows.Scan(&item.Source, &item.MediaCount, &item.TranscriptCount, &dur); err != nil {
			return nil, err
		}
		item.TotalDurationS = dur.Float64
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *AnalyticsHandler) loadActivity(ctx context.Context, days int) ([]dayActivity, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT to_char(COALESCE(recording_started_at, created_at)::date, 'YYYY-MM-DD') AS day,
		       COUNT(*) AS count
		FROM media
		WHERE COALESCE(recording_started_at, created_at) >= NOW() - ($1 || ' days')::interval
		GROUP BY day
		ORDER BY day ASC`, strconv.Itoa(days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dayActivity, 0)
	for rows.Next() {
		var item dayActivity
		if err := rows.Scan(&item.Date, &item.MediaCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ─── GET /api/timeline ────────────────────────────────────────────────────────

type timelineItem struct {
	MediaID       int64  `json:"mediaId"`
	MediaName     string `json:"mediaName"`
	Source        string `json:"source"`
	SegmentStart  string `json:"segmentStart"`
	SegmentEnd    string `json:"segmentEnd"`
	StartSec      float64 `json:"startSec"`
	EndSec        float64 `json:"endSec"`
	Text          string `json:"text"`
	CorrectedText string `json:"correctedText"`
}

func (h *AnalyticsHandler) APITimeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := observability.LoggerFromContext(ctx, h.logger)

	items, err := h.queryTimeline(ctx, r.URL.Query(), 500)
	if err != nil {
		logger.Error("timeline query", slog.Any("error", err))
		http.Error(w, "не удалось загрузить таймлайн", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ─── GET /api/timeline/export ─────────────────────────────────────────────────

func (h *AnalyticsHandler) APITimelineExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := observability.LoggerFromContext(ctx, h.logger)

	items, err := h.queryTimeline(ctx, r.URL.Query(), 100000)
	if err != nil {
		logger.Error("timeline export query", slog.Any("error", err))
		http.Error(w, "не удалось экспортировать", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="timeline_%s.csv"`, time.Now().UTC().Format("20060102_150405")))

	// UTF-8 BOM so Excel detects encoding correctly.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"date", "time_from", "time_to", "source", "media", "text", "corrected_text"})

	for _, it := range items {
		date, timeFrom := splitDateTime(it.SegmentStart)
		_, timeTo := splitDateTime(it.SegmentEnd)
		_ = cw.Write([]string{date, timeFrom, timeTo, it.Source, it.MediaName, it.Text, it.CorrectedText})
	}
}

func (h *AnalyticsHandler) queryTimeline(ctx context.Context, q map[string][]string, limit int) ([]timelineItem, error) {
	from := firstParam(q, "from")
	to := firstParam(q, "to")
	source := strings.TrimSpace(firstParam(q, "source"))

	args := []any{}
	conds := []string{"s.segment_started_at IS NOT NULL"}

	if from != "" {
		if ts, ok := parseFilterTime(from, false); ok {
			args = append(args, ts)
			conds = append(conds, fmt.Sprintf("s.segment_started_at >= $%d", len(args)))
		}
	}
	if to != "" {
		if ts, ok := parseFilterTime(to, true); ok {
			args = append(args, ts)
			conds = append(conds, fmt.Sprintf("s.segment_started_at <= $%d", len(args)))
		}
	}
	if source != "" {
		args = append(args, source)
		conds = append(conds, fmt.Sprintf("m.source_name = $%d", len(args)))
	}

	args = append(args, limit)
	limitArg := len(args)

	query := `
		SELECT
			m.id,
			m.original_name,
			COALESCE(NULLIF(m.source_name, ''), '—') AS source,
			to_char(s.segment_started_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS seg_start,
			to_char(s.segment_ended_at   AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS seg_end,
			s.start_sec,
			s.end_sec,
			s.text
		FROM transcript_segments s
		JOIN transcripts t ON t.id = s.transcript_id
		JOIN media m       ON m.id = t.media_id
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY s.segment_started_at DESC
		LIMIT $` + strconv.Itoa(limitArg)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]timelineItem, 0)
	for rows.Next() {
		var it timelineItem
		var segEnd sql.NullString
		if err := rows.Scan(&it.MediaID, &it.MediaName, &it.Source, &it.SegmentStart, &segEnd, &it.StartSec, &it.EndSec, &it.Text); err != nil {
			return nil, err
		}
		if segEnd.Valid {
			it.SegmentEnd = segEnd.String
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ─── GET / PUT /api/settings/stop-words ───────────────────────────────────────

type stopWordsPayload struct {
	StopWords string `json:"stopWords"`
}

func (h *AnalyticsHandler) APIStopWords(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsSvc.GetCurrent(r.Context())
	if err != nil {
		http.Error(w, "не удалось загрузить stop-words", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stopWords": settings.StopWords})
}

func (h *AnalyticsHandler) APIUpdateStopWords(w http.ResponseWriter, r *http.Request) {
	var payload stopWordsPayload
	if err := decodeJSON(r, &payload); err != nil {
		http.Error(w, "некорректный JSON", http.StatusBadRequest)
		return
	}

	current, err := h.settingsSvc.GetCurrent(r.Context())
	if err != nil {
		http.Error(w, "не удалось загрузить settings", http.StatusInternalServerError)
		return
	}
	current.StopWords = payload.StopWords

	saved, err := h.settingsSvc.SaveCurrent(r.Context(), current)
	if err != nil {
		http.Error(w, "не удалось сохранить stop-words", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "stopWords": saved.StopWords})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return words
}

func parseStopWords(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return defaultStopWords()
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// defaultStopWords returns a ru+en baseline used when the user hasn't
// configured their own list yet.
func defaultStopWords() []string {
	return []string{
		// ru
		"и", "в", "не", "на", "я", "что", "он", "с", "а", "как", "это", "по", "но", "из",
		"так", "у", "то", "же", "за", "вот", "от", "да", "ты", "бы", "мы", "все", "они",
		"есть", "или", "о", "ну", "для", "ещё", "уже", "было", "если", "когда", "быть",
		"был", "была", "были", "может", "меня", "тебя", "его", "её", "их", "нас", "вас",
		"мне", "тебе", "ему", "ей", "нам", "вам", "им", "там", "тут", "где", "кто", "чем",
		"чтобы", "потому", "также", "только", "такой", "такая", "такие", "этот", "эта",
		"эти", "этого", "этой", "этих", "тот", "та", "те", "того", "той", "тех",
		// en
		"the", "a", "an", "and", "or", "but", "of", "to", "in", "on", "at", "for", "with",
		"is", "are", "was", "were", "be", "been", "being", "i", "you", "he", "she", "it",
		"we", "they", "this", "that", "these", "those", "as", "by", "from", "so", "if",
		"not", "no", "do", "does", "did", "will", "would", "can", "could", "have", "has",
		"had", "my", "your", "his", "her", "our", "their",
	}
}

func parseFilterTime(raw string, endOfDay bool) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			if layout == "2006-01-02" && endOfDay {
				ts = ts.Add(24*time.Hour - time.Nanosecond)
			}
			return ts.UTC(), true
		}
	}
	return time.Time{}, false
}

func firstParam(q map[string][]string, key string) string {
	if v, ok := q[key]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

func splitDateTime(iso string) (string, string) {
	if len(iso) < 19 {
		return iso, ""
	}
	return iso[:10], iso[11:19]
}
