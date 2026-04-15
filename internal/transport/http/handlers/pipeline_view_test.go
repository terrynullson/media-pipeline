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
		media.Media{
			ID:       10,
			Status:   media.StatusFailed,
			MIMEType: "audio/mpeg",
		},
		[]job.Job{
			{ID: 100, MediaID: 10, Type: job.TypeUpload, Status: job.StatusDone},
			{ID: 1, MediaID: 10, Type: job.TypeExtractAudio, Status: job.StatusDone},
			{
				ID:            2,
				MediaID:       10,
				Type:          job.TypeTranscribe,
				Status:        job.StatusFailed,
				ErrorMessage:  "Не удалось распознать текст: модель small вернула ошибку Подробности смотрите в логах worker.",
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
	if view.ErrorSummary != "Не удалось распознать текст: модель small вернула ошибку" {
		t.Fatalf("ErrorSummary = %q, want cleaned message", view.ErrorSummary)
	}
	if view.StageValue != 3 {
		t.Fatalf("StageValue = %d, want 3", view.StageValue)
	}
	if got := view.Steps[2].TimingText; !strings.Contains(got, "Ошибка через") {
		t.Fatalf("TimingText = %q, want failed duration label", got)
	}
	if got := view.Steps[2].TimingText; !strings.Contains(got, "Старт ") || !strings.Contains(got, "Финиш ") {
		t.Fatalf("TimingText = %q, want start and finish labels", got)
	}
}

func TestBuildMediaPipelineView_AudioOnlySkipsScreenshots(t *testing.T) {
	t.Parallel()

	view := buildMediaPipelineView(
		media.Media{
			ID:        11,
			Status:    media.StatusTranscribed,
			MIMEType:  "audio/wav",
			Extension: ".wav",
		},
		[]job.Job{
			{ID: 100, MediaID: 11, Type: job.TypeUpload, Status: job.StatusDone},
			{ID: 1, MediaID: 11, Type: job.TypeExtractAudio, Status: job.StatusDone},
			{ID: 2, MediaID: 11, Type: job.TypeTranscribe, Status: job.StatusDone},
			{ID: 3, MediaID: 11, Type: job.TypeAnalyzeTriggers, Status: job.StatusDone},
		},
	)

	if len(view.Steps) != 5 {
		t.Fatalf("len(Steps) = %d, want 5", len(view.Steps))
	}
	lastStep := view.Steps[len(view.Steps)-1]
	if lastStep.Label != "Снимки по триггерам" {
		t.Fatalf("last step label = %q, want screenshots step", lastStep.Label)
	}
	if lastStep.StatusLabel != "Не требуется" {
		t.Fatalf("last step status = %q, want Не требуется", lastStep.StatusLabel)
	}
}

func TestBuildMediaPipelineView_UsesEstimatedProgressForRunningTranscription(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-2 * time.Minute)
	progress := 61.8

	view := buildMediaPipelineView(
		media.Media{
			ID:       12,
			Status:   media.StatusTranscribing,
			MIMEType: "audio/mpeg",
		},
		[]job.Job{
			{ID: 100, MediaID: 12, Type: job.TypeUpload, Status: job.StatusDone},
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

	step := view.Steps[2]
	if !step.ProgressVisible {
		t.Fatal("ProgressVisible = false, want true")
	}
	if step.ProgressPercent != 62 {
		t.Fatalf("ProgressPercent = %d, want 62", step.ProgressPercent)
	}
	if step.ProgressLabel != "Оценка по сегментам: 62%" {
		t.Fatalf("ProgressLabel = %q, want estimated label", step.ProgressLabel)
	}
	if got := step.TimingText; !strings.Contains(got, "Старт ") || !strings.Contains(got, "В работе") {
		t.Fatalf("TimingText = %q, want running start label", got)
	}
}

func TestUserFacingJobError_RemovesLogHint(t *testing.T) {
	t.Parallel()

	message := userFacingJobError(&job.Job{
		ErrorMessage: "Не удалось распознать текст: модель small вернула ошибку. Подробности смотрите в логах worker.",
	})

	if message != "Не удалось распознать текст: модель small вернула ошибку." {
		t.Fatalf("message = %q, want cleaned user message", message)
	}
}
