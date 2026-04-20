package handlers

import (
	"strings"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

func TestBuildMediaPipelineView_FailedTranscribeShowsStageAndReason(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-10 * time.Second)
	finishedAt := time.Now().UTC().Add(-2 * time.Second)
	durationMS := int64(8000)

	view := buildMediaPipelineView(
		media.Media{ID: 10, Status: media.StatusFailed},
		[]job.Job{
			{ID: 100, MediaID: 10, Type: job.TypeUpload, Status: job.StatusDone},
			{ID: 101, MediaID: 10, Type: job.TypePreparePreviewVideo, Status: job.StatusDone},
			{ID: 1, MediaID: 10, Type: job.TypeExtractAudio, Status: job.StatusDone},
			{
				ID:            2,
				MediaID:       10,
				Type:          job.TypeTranscribe,
				Status:        job.StatusFailed,
				ErrorMessage:  "Не удалось распознать текст: модель small вернула ошибку. Подробности смотрите в логах worker.",
				StartedAtUTC:  &startedAt,
				FinishedAtUTC: &finishedAt,
				DurationMS:    &durationMS,
			},
		},
	)

	if view.StatusLabel != "Ошибка" {
		t.Fatalf("StatusLabel = %q, want Ошибка", view.StatusLabel)
	}
	if view.FailedStage != "Распознавание текста" {
		t.Fatalf("FailedStage = %q, want Распознавание текста", view.FailedStage)
	}
	if view.ErrorSummary != "Не удалось распознать текст: модель small вернула ошибку." {
		t.Fatalf("ErrorSummary = %q, want cleaned message", view.ErrorSummary)
	}
	if view.StageValue != 4 {
		t.Fatalf("StageValue = %d, want 4", view.StageValue)
	}
	if got := view.Steps[3].TimingText; !strings.Contains(got, "Ошибка через") {
		t.Fatalf("TimingText = %q, want failed duration label", got)
	}
	if got := view.Steps[3].TimingText; !strings.Contains(got, "Начало ") || !strings.Contains(got, "Завершено ") {
		t.Fatalf("TimingText = %q, want start and finish labels", got)
	}
}

func TestBuildMediaPipelineView_AudioOnlyMarksPreviewAndScreenshotsAsNotRequired(t *testing.T) {
	t.Parallel()

	view := buildMediaPipelineView(
		media.Media{ID: 11, Status: media.StatusTranscribed, MIMEType: "audio/wav", Extension: ".wav"},
		[]job.Job{
			{ID: 100, MediaID: 11, Type: job.TypeUpload, Status: job.StatusDone},
			{ID: 1, MediaID: 11, Type: job.TypeExtractAudio, Status: job.StatusDone},
			{ID: 2, MediaID: 11, Type: job.TypeTranscribe, Status: job.StatusDone},
			{ID: 3, MediaID: 11, Type: job.TypeAnalyzeTriggers, Status: job.StatusDone},
		},
	)

	if len(view.Steps) != 6 {
		t.Fatalf("len(Steps) = %d, want 6", len(view.Steps))
	}
	if view.Steps[1].Label != "Подготовка превью" {
		t.Fatalf("preview step label = %q, want Подготовка превью", view.Steps[1].Label)
	}
	if view.Steps[1].StatusLabel != "Не требуется" {
		t.Fatalf("preview step status = %q, want Не требуется", view.Steps[1].StatusLabel)
	}
	lastStep := view.Steps[len(view.Steps)-1]
	if lastStep.Label != "Снимки по триггерам" {
		t.Fatalf("last step label = %q, want screenshots step", lastStep.Label)
	}
	if lastStep.StatusLabel != "Не требуется" {
		t.Fatalf("last step status = %q, want Не требуется", lastStep.StatusLabel)
	}
}

func TestBuildMediaPipelineView_QueuedVideoStartsWithPreviewStep(t *testing.T) {
	t.Parallel()

	view := buildMediaPipelineView(
		media.Media{ID: 13, Status: media.StatusUploaded, MIMEType: "video/mp4", Extension: ".mp4"},
		[]job.Job{{ID: 100, MediaID: 13, Type: job.TypeUpload, Status: job.StatusDone}},
	)

	if view.CurrentStage != "Подготовка превью" {
		t.Fatalf("CurrentStage = %q, want Подготовка превью", view.CurrentStage)
	}
	if view.Steps[1].StatusLabel != "Ждёт" {
		t.Fatalf("preview step status = %q, want Ждёт", view.Steps[1].StatusLabel)
	}
	if view.Steps[2].StatusLabel != "Не начато" {
		t.Fatalf("extract step status = %q, want Не начато", view.Steps[2].StatusLabel)
	}
}

func TestBuildMediaPipelineView_UsesEstimatedProgressForRunningTranscription(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-2 * time.Minute)
	progress := 61.8

	view := buildMediaPipelineView(
		media.Media{ID: 12, Status: media.StatusTranscribing},
		[]job.Job{
			{ID: 100, MediaID: 12, Type: job.TypeUpload, Status: job.StatusDone},
			{ID: 101, MediaID: 12, Type: job.TypePreparePreviewVideo, Status: job.StatusDone},
			{ID: 1, MediaID: 12, Type: job.TypeExtractAudio, Status: job.StatusDone},
			{
				ID:                  2,
				MediaID:             12,
				Type:                job.TypeTranscribe,
				Status:              job.StatusRunning,
				StartedAtUTC:        &startedAt,
				ProgressPercent:     &progress,
				ProgressIsEstimated: true,
			},
		},
	)

	step := view.Steps[3]
	if !step.ProgressVisible {
		t.Fatal("ProgressVisible = false, want true")
	}
	if step.ProgressPercent != 62 {
		t.Fatalf("ProgressPercent = %d, want 62", step.ProgressPercent)
	}
	if step.ProgressLabel != "Оценка по сегментам: 62%" {
		t.Fatalf("ProgressLabel = %q, want estimated label", step.ProgressLabel)
	}
	if step.EtaLabel != "" {
		t.Fatalf("EtaLabel = %q, want empty for estimated progress", step.EtaLabel)
	}
}

func TestBuildMediaPipelineView_UsesETAForRunningPreview(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-10 * time.Second)
	progress := 25.0

	view := buildMediaPipelineView(
		media.Media{ID: 15, Status: media.StatusProcessing, MIMEType: "video/mp4", Extension: ".mp4"},
		[]job.Job{
			{ID: 100, MediaID: 15, Type: job.TypeUpload, Status: job.StatusDone},
			{
				ID:              101,
				MediaID:         15,
				Type:            job.TypePreparePreviewVideo,
				Status:          job.StatusRunning,
				StartedAtUTC:    &startedAt,
				ProgressPercent: &progress,
			},
		},
	)

	step := view.Steps[1]
	if step.EtaLabel == "" {
		t.Fatal("EtaLabel = empty, want ETA for running preview")
	}
	if view.CurrentEtaLabel == "" {
		t.Fatal("CurrentEtaLabel = empty, want current ETA")
	}
}

func TestBuildMediaPipelineView_UsesHistoricalETAForRunningPreviewWithoutLiveProgress(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-20 * time.Second)

	view := buildMediaPipelineViewWithHistorical(
		media.Media{ID: 17, Status: media.StatusProcessing, MIMEType: "video/mp4", Extension: ".mp4", SizeBytes: 200 * 1024 * 1024},
		[]job.Job{
			{ID: 100, MediaID: 17, Type: job.TypeUpload, Status: job.StatusDone},
			{
				ID:           101,
				MediaID:      17,
				Type:         job.TypePreparePreviewVideo,
				Status:       job.StatusRunning,
				StartedAtUTC: &startedAt,
			},
		},
		map[job.Type]time.Duration{
			job.TypePreparePreviewVideo: 2 * time.Minute,
		},
	)

	step := view.Steps[1]
	if step.EtaLabel == "" {
		t.Fatal("EtaLabel = empty, want historical ETA")
	}
	if !strings.Contains(step.EtaLabel, "Осталось ~") {
		t.Fatalf("EtaLabel = %q, want remaining historical ETA", step.EtaLabel)
	}
}

func TestBuildMediaPipelineView_UsesHistoricalETAForPendingExtractAudio(t *testing.T) {
	t.Parallel()

	view := buildMediaPipelineViewWithHistorical(
		media.Media{ID: 18, Status: media.StatusUploaded, MIMEType: "video/mp4", Extension: ".mp4", SizeBytes: 300 * 1024 * 1024},
		[]job.Job{
			{ID: 100, MediaID: 18, Type: job.TypeUpload, Status: job.StatusDone},
			{ID: 101, MediaID: 18, Type: job.TypePreparePreviewVideo, Status: job.StatusDone},
			{ID: 102, MediaID: 18, Type: job.TypeExtractAudio, Status: job.StatusPending},
		},
		map[job.Type]time.Duration{
			job.TypeExtractAudio: 45 * time.Second,
		},
	)

	step := view.Steps[2]
	if step.EtaLabel != "Обычно ~45.0 сек" {
		t.Fatalf("EtaLabel = %q, want historical pending ETA", step.EtaLabel)
	}
}

func TestBuildMediaPipelineView_StopsMarkingDownstreamStepsAsFailedAfterRootFailure(t *testing.T) {
	t.Parallel()

	view := buildMediaPipelineView(
		media.Media{ID: 16, Status: media.StatusFailed, MIMEType: "video/mp4", Extension: ".mp4"},
		[]job.Job{
			{ID: 100, MediaID: 16, Type: job.TypeUpload, Status: job.StatusDone},
			{
				ID:           101,
				MediaID:      16,
				Type:         job.TypePreparePreviewVideo,
				Status:       job.StatusFailed,
				ErrorMessage: "preview failed",
			},
			{
				ID:           102,
				MediaID:      16,
				Type:         job.TypeTranscribe,
				Status:       job.StatusFailed,
				ErrorMessage: "stale downstream failure",
			},
		},
	)

	if view.FailedStage != "Подготовка превью" {
		t.Fatalf("FailedStage = %q, want preview step", view.FailedStage)
	}
	if got := view.Steps[3].StatusLabel; got != "Не начато" {
		t.Fatalf("transcribe status = %q, want Не начато after upstream failure", got)
	}
	if got := view.Steps[3].Tone; got != "neutral" {
		t.Fatalf("transcribe tone = %q, want neutral after upstream failure", got)
	}
	if got := view.Steps[3].TimingText; got != "Не запускалось" {
		t.Fatalf("transcribe timing = %q, want Не запускалось after upstream failure", got)
	}
}

func TestUserFacingJobError_RemovesLogHint(t *testing.T) {
	t.Parallel()

	message := userFacingJobError(&job.Job{ErrorMessage: "Не удалось распознать текст: модель small вернула ошибку. Подробности смотрите в логах worker."})
	if message != "Не удалось распознать текст: модель small вернула ошибку." {
		t.Fatalf("message = %q, want cleaned user message", message)
	}
}
