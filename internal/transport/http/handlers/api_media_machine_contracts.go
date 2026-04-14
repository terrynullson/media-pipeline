package handlers

// Machine API — polling endpoints for n8n / external automation.
//
// GET /api/media/{mediaID}/status  — lightweight status poll (done/failed/in-progress)
// GET /api/media/{mediaID}/result  — full result once done=true

import (
	"fmt"
	"strings"

	mediaapp "media-pipeline/internal/app/media"
	"media-pipeline/internal/domain/job"
)

// apiMediaStatusResponse is returned by the /status endpoint.
type apiMediaStatusResponse struct {
	Done       bool   `json:"done"`
	Failed     bool   `json:"failed"`
	Error      string `json:"error,omitempty"`
	StageIndex int    `json:"stageIndex"` // 1-based index of current stage
	StageTotal int    `json:"stageTotal"` // total stages for this media type
	Stage      string `json:"stage"`      // human-readable current stage name
}

// apiMediaResultResponse is returned by the /result endpoint.
type apiMediaResultResponse struct {
	MediaID       int64  `json:"mediaId"`
	Name          string `json:"name"`
	Transcript    string `json:"transcript"`
	Language      string `json:"language"`
	LanguageLabel string `json:"languageLabel"`
}

// buildTranscriptAutomationStatus builds the status response for the machine API.
//
// Pipeline stages (1-based):
//
//	Stage 1: prepare_preview  (skipped for audio-only → stageTotal=4)
//	Stage 2: extract_audio
//	Stage 3: transcribe
//	Stage 4: analyze_triggers
//	Stage 5: extract_screenshots (skipped for audio-only)
//	Stage 6: completed
func buildTranscriptAutomationStatus(result mediaapp.MediaStatusResult) apiMediaStatusResponse {
	audioOnly := result.Media.IsAudioOnly()

	// stageTotal: 6 for video, 4 for audio-only (no preview / no screenshots).
	stageTotal := 6
	if audioOnly {
		stageTotal = 4
	}

	// Check any failed job first — all 4 pipeline jobs must be checked.
	if failed := failedTranscriptAutomationJob(result); failed != nil {
		msg := failed.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("job %s failed", failed.Type)
		}
		return apiMediaStatusResponse{
			Done:       true,
			Failed:     true,
			Error:      msg,
			StageIndex: stageTotal,
			StageTotal: stageTotal,
			Stage:      string(failed.Type),
		}
	}

	if result.HasTranscript {
		return apiMediaStatusResponse{
			Done:       true,
			StageIndex: stageTotal,
			StageTotal: stageTotal,
			Stage:      "completed",
		}
	}

	// Determine current stage index and label from in-progress / pending jobs.
	index, stage := currentStageIndex(result, audioOnly)
	return apiMediaStatusResponse{
		Done:       false,
		StageIndex: index,
		StageTotal: stageTotal,
		Stage:      stage,
	}
}

// failedTranscriptAutomationJob returns the first failed job among all four
// pipeline stages, or nil if none are failed.
// All four stages must be checked so that failures in analyze_triggers or
// extract_screenshots are also detected by polling clients.
func failedTranscriptAutomationJob(result mediaapp.MediaStatusResult) *job.Job {
	for _, current := range []*job.Job{
		result.ExtractAudioJob,
		result.TranscribeJob,
		result.AnalyzeJob,
		result.ScreenshotJob,
	} {
		if current != nil && current.Status == job.StatusFailed {
			return current
		}
	}
	return nil
}

// currentStageIndex maps the in-progress pipeline state to a 1-based stage
// index and a human-readable stage name.
func currentStageIndex(result mediaapp.MediaStatusResult, audioOnly bool) (int, string) {
	// Stage progression (video): prepare_preview(1) → extract_audio(2) →
	//                             transcribe(3) → analyze_triggers(4) → extract_screenshots(5) → done(6)
	// Stage progression (audio):  extract_audio(1) → transcribe(2) → analyze_triggers(3) → done(4)

	if !audioOnly {
		if result.PreviewJob != nil && isActive(result.PreviewJob) {
			return 1, "prepare_preview"
		}
	}
	if result.ExtractAudioJob != nil && isActive(result.ExtractAudioJob) {
		if audioOnly {
			return 1, "extract_audio"
		}
		return 2, "extract_audio"
	}
	if result.TranscribeJob != nil && isActive(result.TranscribeJob) {
		if audioOnly {
			return 2, "transcribe"
		}
		return 3, "transcribe"
	}
	if result.AnalyzeJob != nil && isActive(result.AnalyzeJob) {
		if audioOnly {
			return 3, "analyze_triggers"
		}
		return 4, "analyze_triggers"
	}
	if !audioOnly && result.ScreenshotJob != nil && isActive(result.ScreenshotJob) {
		return 5, "extract_screenshots"
	}

	// Nothing active yet — still in queue.
	return 1, "queued"
}

func isActive(j *job.Job) bool {
	return j.Status == job.StatusPending || j.Status == job.StatusRunning
}

// languageLabel returns a human-readable name for a BCP-47 language code.
// Unknown codes are returned as-is.
func languageLabel(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "ru":
		return "Русский"
	case "en":
		return "English"
	case "de":
		return "Deutsch"
	case "fr":
		return "Français"
	case "es":
		return "Español"
	case "zh":
		return "中文"
	case "ja":
		return "日本語"
	case "it":
		return "Italiano"
	case "pt":
		return "Português"
	case "pl":
		return "Polski"
	case "uk":
		return "Українська"
	case "":
		return ""
	default:
		return code
	}
}
