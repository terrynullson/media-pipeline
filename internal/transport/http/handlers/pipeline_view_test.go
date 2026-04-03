package handlers

import (
	"testing"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

func TestBuildMediaPipelineView_FailedTranscribeShowsStageAndReason(t *testing.T) {
	t.Parallel()

	view := buildMediaPipelineView(
		media.Media{
			ID:     10,
			Status: media.StatusFailed,
		},
		[]job.Job{
			{ID: 1, MediaID: 10, Type: job.TypeExtractAudio, Status: job.StatusDone},
			{ID: 2, MediaID: 10, Type: job.TypeTranscribe, Status: job.StatusFailed, ErrorMessage: "Не удалось распознать текст: модель small вернула ошибку Подробности смотрите в логах worker."},
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

func TestUserFacingJobError_RemovesLogHint(t *testing.T) {
	t.Parallel()

	message := userFacingJobError(&job.Job{
		ErrorMessage: "Не удалось распознать текст: модель small вернула ошибку. Подробности смотрите в логах worker.",
	})

	if message != "Не удалось распознать текст: модель small вернула ошибку." {
		t.Fatalf("message = %q, want cleaned user message", message)
	}
}
