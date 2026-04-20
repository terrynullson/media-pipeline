package handlers

import (
	"fmt"
	"strings"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

const workerLogHintRU = "Подробности смотрите в логах worker."

type PipelineStepView struct {
	Label           string `json:"label"`
	StatusLabel     string `json:"statusLabel"`
	Tone            string `json:"tone"`
	IsCurrent       bool   `json:"isCurrent"`
	IsFailed        bool   `json:"isFailed"`
	TimingText      string `json:"timingText"`
	StartedAtLabel  string `json:"startedAtLabel"`
	FinishedAtLabel string `json:"finishedAtLabel"`
	DurationLabel   string `json:"durationLabel"`
	EtaLabel        string `json:"etaLabel"`
	ProgressLabel   string `json:"progressLabel"`
	ProgressPercent int    `json:"progressPercent"`
	ProgressVisible bool   `json:"progressVisible"`
}

type MediaPipelineView struct {
	StatusLabel       string             `json:"statusLabel"`
	StatusTone        string             `json:"statusTone"`
	StageLabel        string             `json:"stageLabel"`
	StageValue        int                `json:"stageValue"`
	StageTotal        int                `json:"stageTotal"`
	IsActive          bool               `json:"isActive"`
	CurrentStage      string             `json:"currentStage"`
	FailedStage       string             `json:"failedStage"`
	ErrorSummary      string             `json:"errorSummary"`
	ErrorLocation     string             `json:"errorLocation"`
	Steps             []PipelineStepView `json:"steps"`
	AutomaticJobFail  *job.Job           `json:"automaticJobFail,omitempty"`
	CurrentTimingText string             `json:"currentTimingText"`
	CurrentEtaLabel   string             `json:"currentEtaLabel"`
}

type pipelineStepState struct {
	label           string
	statusLabel     string
	tone            string
	kind            string
	job             *job.Job
	timingText      string
	startedAtLabel  string
	finishedAtLabel string
	durationLabel   string
	etaLabel        string
	progressLabel   string
	progressPercent int
	progressVisible bool
}

func buildMediaPipelineView(mediaItem media.Media, jobs []job.Job) MediaPipelineView {
	return buildMediaPipelineViewWithHistorical(mediaItem, jobs, nil)
}

func buildMediaPipelineViewWithHistorical(mediaItem media.Media, jobs []job.Job, historicalEstimates map[job.Type]time.Duration) MediaPipelineView {
	nowUTC := time.Now().UTC()
	jobsByType := latestJobByType(jobs)

	uploadStep := describeUploadStep(mediaItem, jobsByType[job.TypeUpload], nowUTC)
	previewStep := describePreviewStep(mediaItem, jobsByType[job.TypePreparePreviewVideo], uploadStep.kind == "done", historicalEstimates[job.TypePreparePreviewVideo], nowUTC)
	extractStep := describeExtractAudioStep(mediaItem, jobsByType[job.TypeExtractAudio], previewStep.kind == "done", historicalEstimates[job.TypeExtractAudio], nowUTC)
	transcribeStep := describeTranscribeStep(mediaItem, jobsByType[job.TypeTranscribe], extractStep.kind == "done", nowUTC)
	analyzeStep := describeQueuedStep("Анализ триггеров", jobsByType[job.TypeAnalyzeTriggers], transcribeStep.kind == "done", nowUTC)
	screenshotStep := describeScreenshotStep(mediaItem, jobsByType[job.TypeExtractScreenshots], analyzeStep.kind == "done", nowUTC)

	steps := []pipelineStepState{
		uploadStep,
		previewStep,
		extractStep,
		transcribeStep,
		analyzeStep,
		screenshotStep,
	}

	steps = normalizePipelineAfterFailure(steps)

	failedIndex := firstStepIndexByKind(steps, "failed")
	runningIndex := firstStepIndexByKind(steps, "running")
	pendingIndex := firstPendingIndex(steps)
	lastDoneIndex := lastCompletedIndex(steps)

	view := MediaPipelineView{
		StageTotal: len(steps),
		Steps:      make([]PipelineStepView, 0, len(steps)),
	}

	switch {
	case failedIndex >= 0:
		failedStep := steps[failedIndex]
		view.StatusLabel = "Ошибка"
		view.StatusTone = "error"
		view.StageLabel = "Сбой на этапе: " + strings.ToLower(failedStep.label)
		view.StageValue = failedIndex + 1
		view.FailedStage = failedStep.label
		view.CurrentStage = failedStep.label
		view.CurrentTimingText = failedStep.timingText
		view.ErrorSummary = userFacingJobError(failedStep.job)
		view.ErrorLocation = workerLogHintRU
		view.AutomaticJobFail = failedStep.job
	case runningIndex >= 0:
		currentStep := steps[runningIndex]
		view.StatusLabel = "В работе"
		view.StatusTone = "running"
		view.StageLabel = "Сейчас: " + currentStep.label
		view.StageValue = runningIndex + 1
		view.IsActive = true
		view.CurrentStage = currentStep.label
		view.CurrentTimingText = currentStep.timingText
		view.CurrentEtaLabel = currentStep.etaLabel
	case lastDoneIndex == len(steps)-1:
		view.StatusLabel = "Готово"
		view.StatusTone = "success"
		view.StageLabel = "Основные этапы завершены"
		view.StageValue = len(steps)
		view.CurrentStage = "Завершено"
		view.CurrentTimingText = steps[len(steps)-1].timingText
	case mediaItem.Status == media.StatusQueued || mediaItem.Status == media.StatusUploaded || lastDoneIndex == 0:
		view.StatusLabel = "В очереди"
		view.StatusTone = "queued"
		view.StageValue = 1
		view.StageLabel = "Ждёт запуска основной обработки"
		if pendingIndex >= 0 {
			view.CurrentStage = steps[pendingIndex].label
			view.CurrentTimingText = steps[pendingIndex].timingText
			view.CurrentEtaLabel = steps[pendingIndex].etaLabel
		} else {
			view.CurrentStage = previewStep.label
			view.CurrentTimingText = "Файл загружен и ждёт своей очереди"
		}
	default:
		view.StatusLabel = "Ожидает следующий шаг"
		view.StatusTone = "ready"
		view.StageValue = max(1, lastDoneIndex+1)
		if pendingIndex >= 0 {
			view.StageLabel = "Дальше: " + strings.ToLower(steps[pendingIndex].label)
			view.CurrentStage = steps[pendingIndex].label
			view.CurrentTimingText = steps[pendingIndex].timingText
			view.CurrentEtaLabel = steps[pendingIndex].etaLabel
		} else {
			view.StageLabel = "Ожидает продолжения обработки"
		}
	}

	for index, step := range steps {
		view.Steps = append(view.Steps, PipelineStepView{
			Label:           step.label,
			StatusLabel:     step.statusLabel,
			Tone:            step.tone,
			IsCurrent:       failedIndex < 0 && runningIndex == index,
			IsFailed:        failedIndex == index,
			TimingText:      step.timingText,
			StartedAtLabel:  step.startedAtLabel,
			FinishedAtLabel: step.finishedAtLabel,
			DurationLabel:   step.durationLabel,
			EtaLabel:        step.etaLabel,
			ProgressLabel:   step.progressLabel,
			ProgressPercent: step.progressPercent,
			ProgressVisible: step.progressVisible,
		})
	}

	return view
}

func normalizePipelineAfterFailure(steps []pipelineStepState) []pipelineStepState {
	failedIndex := firstStepIndexByKind(steps, "failed")
	if failedIndex < 0 {
		return steps
	}

	for index := failedIndex + 1; index < len(steps); index++ {
		step := &steps[index]
		if step.kind == "done" || step.kind == "running" {
			continue
		}
		step.statusLabel = "Не начато"
		step.tone = "neutral"
		step.kind = "blocked"
		step.timingText = "Не запускалось"
		step.startedAtLabel = ""
		step.finishedAtLabel = ""
		step.durationLabel = ""
		step.etaLabel = ""
		step.progressLabel = ""
		step.progressPercent = 0
		step.progressVisible = false
	}

	return steps
}

func describeUploadStep(mediaItem media.Media, currentJob *job.Job, nowUTC time.Time) pipelineStepState {
	if currentJob != nil {
		return describeJobBackedStep("Загрузка файла", currentJob, 0, nowUTC)
	}

	return pipelineStepState{
		label:           "Загрузка файла",
		statusLabel:     "Готово",
		tone:            "success",
		kind:            "done",
		timingText:      "Файл сохранён",
		finishedAtLabel: FormatClockUTC(mediaItem.CreatedAtUTC),
	}
}

func describePreviewStep(mediaItem media.Media, currentJob *job.Job, unlocked bool, historicalEstimate time.Duration, nowUTC time.Time) pipelineStepState {
	label := "Подготовка превью"
	if mediaItem.IsAudioOnly() {
		if currentJob != nil {
			step := describeJobBackedStep(label, currentJob, 0, nowUTC)
			if currentJob.Status == job.StatusDone {
				step.statusLabel = "Не требуется"
				step.tone = "neutral"
				step.timingText = "Для аудио не требуется"
			}
			return step
		}
		return pipelineStepState{label: label, statusLabel: "Не требуется", tone: "neutral", kind: "done", timingText: "Для аудио не требуется"}
	}
	if currentJob != nil {
		return describeJobBackedStep(label, currentJob, historicalEstimate, nowUTC)
	}
	if strings.TrimSpace(mediaItem.PreviewVideoPath) != "" {
		return pipelineStepState{label: label, statusLabel: "Готово", tone: "success", kind: "done", timingText: "Готово"}
	}
	if unlocked {
		return pendingStepWithEstimate(label, historicalEstimate)
	}
	return pipelineStepState{label: label, statusLabel: "Не начато", tone: "neutral", kind: "blocked", timingText: "Не запускалось"}
}

func describeExtractAudioStep(mediaItem media.Media, currentJob *job.Job, unlocked bool, historicalEstimate time.Duration, nowUTC time.Time) pipelineStepState {
	label := "Извлечение аудио"
	if !unlocked {
		return pipelineStepState{label: label, statusLabel: "Не начато", tone: "neutral", kind: "blocked", timingText: "Не запускалось"}
	}
	if currentJob != nil {
		return describeJobBackedStep(label, currentJob, historicalEstimate, nowUTC)
	}

	switch mediaItem.Status {
	case media.StatusProcessing:
		step := pipelineStepState{label: label, statusLabel: "В работе", tone: "running", kind: "running", timingText: "Идёт подготовка аудио"}
		step.etaLabel = formatHistoricalRemainingETA(nil, historicalEstimate, nowUTC)
		return step
	case media.StatusAudioExtracted, media.StatusTranscribing, media.StatusTranscribed:
		return pipelineStepState{label: label, statusLabel: "Готово", tone: "success", kind: "done", timingText: "Готово"}
	default:
		return pendingStepWithEstimate(label, historicalEstimate)
	}
}

func describeTranscribeStep(mediaItem media.Media, currentJob *job.Job, unlocked bool, nowUTC time.Time) pipelineStepState {
	label := "Распознавание текста"
	if currentJob != nil {
		return describeJobBackedStep(label, currentJob, 0, nowUTC)
	}
	if !unlocked {
		return pipelineStepState{label: label, statusLabel: "Не начато", tone: "neutral", kind: "blocked", timingText: "Не запускалось"}
	}

	switch mediaItem.Status {
	case media.StatusTranscribing:
		return pipelineStepState{label: label, statusLabel: "В работе", tone: "running", kind: "running", timingText: "Идёт распознавание"}
	case media.StatusTranscribed:
		return pipelineStepState{label: label, statusLabel: "Готово", tone: "success", kind: "done", timingText: "Готово"}
	case media.StatusAudioExtracted:
		return pipelineStepState{label: label, statusLabel: "Ждёт", tone: "ready", kind: "pending", timingText: "Ждёт запуска"}
	default:
		return pipelineStepState{label: label, statusLabel: "Ждёт", tone: "ready", kind: "pending", timingText: "Ждёт запуска"}
	}
}

func describeQueuedStep(label string, currentJob *job.Job, unlocked bool, nowUTC time.Time) pipelineStepState {
	if currentJob != nil {
		return describeJobBackedStep(label, currentJob, 0, nowUTC)
	}
	if unlocked {
		return pipelineStepState{label: label, statusLabel: "Ждёт", tone: "ready", kind: "pending", timingText: "Ждёт запуска"}
	}
	return pipelineStepState{label: label, statusLabel: "Не начато", tone: "neutral", kind: "blocked", timingText: "Не запускалось"}
}

func describeScreenshotStep(mediaItem media.Media, currentJob *job.Job, unlocked bool, nowUTC time.Time) pipelineStepState {
	label := "Снимки по триггерам"
	if mediaItem.IsAudioOnly() && unlocked && currentJob == nil {
		return pipelineStepState{label: label, statusLabel: "Не требуется", tone: "neutral", kind: "done", timingText: "Для аудио не требуется"}
	}
	if currentJob != nil {
		step := describeJobBackedStep(label, currentJob, 0, nowUTC)
		if currentJob.Status == job.StatusDone && mediaItem.IsAudioOnly() {
			step.statusLabel = "Не требуется"
			step.tone = "neutral"
			step.timingText = "Для аудио не требуется"
		}
		return step
	}
	if unlocked {
		if mediaItem.IsAudioOnly() {
			return pipelineStepState{label: label, statusLabel: "Не требуется", tone: "neutral", kind: "done", timingText: "Для аудио не требуется"}
		}
		return pipelineStepState{label: label, statusLabel: "Ждёт", tone: "ready", kind: "pending", timingText: "Ждёт запуска"}
	}
	return pipelineStepState{label: label, statusLabel: "Не начато", tone: "neutral", kind: "blocked", timingText: "Не запускалось"}
}

func describeJobBackedStep(label string, currentJob *job.Job, historicalEstimate time.Duration, nowUTC time.Time) pipelineStepState {
	step := pipelineStepState{
		label:           label,
		job:             currentJob,
		startedAtLabel:  formatOptionalClock(currentJob.StartedAtUTC),
		finishedAtLabel: formatOptionalClock(currentJob.FinishedAtUTC),
		durationLabel:   formatJobDuration(currentJob, nowUTC),
	}

	switch currentJob.Status {
	case job.StatusDone:
		step.statusLabel = "Готово"
		step.tone = "success"
		step.kind = "done"
		step.timingText = buildTimingText(step.startedAtLabel, step.finishedAtLabel, "Готово", step.durationLabel)
	case job.StatusRunning:
		step.statusLabel = "В работе"
		step.tone = "running"
		step.kind = "running"
		if currentJob.ProgressPercent != nil {
			step.progressVisible = true
			step.progressPercent = clampPercent(*currentJob.ProgressPercent)
			if currentJob.ProgressIsEstimated {
				progressLabel := strings.TrimSpace(currentJob.ProgressLabel)
				if progressLabel == "" {
					progressLabel = "Оценка по сегментам"
				}
				step.progressLabel = fmt.Sprintf("%s: %d%%", progressLabel, step.progressPercent)
			} else {
				step.progressLabel = fmt.Sprintf("%d%%", step.progressPercent)
				step.etaLabel = formatJobETA(currentJob, nowUTC)
			}
		} else if historicalEstimate > 0 {
			step.etaLabel = formatHistoricalRemainingETA(currentJob, historicalEstimate, nowUTC)
		}
		step.timingText = buildTimingText(step.startedAtLabel, "", "В работе", step.durationLabel)
	case job.StatusFailed:
		step.statusLabel = "Ошибка"
		step.tone = "error"
		step.kind = "failed"
		status := "Ошибка"
		if step.durationLabel != "" {
			status = "Ошибка через " + step.durationLabel
		}
		step.timingText = buildTimingText(step.startedAtLabel, step.finishedAtLabel, status, "")
	case job.StatusPending:
		step.statusLabel = "Ждёт"
		step.tone = "ready"
		step.kind = "pending"
		step.timingText = "Ждёт запуска"
		if historicalEstimate > 0 {
			step.etaLabel = "Обычно ~" + FormatDurationRU(historicalEstimate)
		}
	default:
		step.statusLabel = "Не начато"
		step.tone = "neutral"
		step.kind = "blocked"
		step.timingText = "Не запускалось"
	}

	return step
}

func pendingStepWithEstimate(label string, historicalEstimate time.Duration) pipelineStepState {
	step := pipelineStepState{
		label:       label,
		statusLabel: "Ждёт",
		tone:        "ready",
		kind:        "pending",
		timingText:  "Ждёт запуска",
	}
	if historicalEstimate > 0 {
		step.etaLabel = "Обычно ~" + FormatDurationRU(historicalEstimate)
	}
	return step
}

func buildTimingText(startedAt string, finishedAt string, status string, duration string) string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(startedAt) != "" {
		parts = append(parts, "Начало "+startedAt)
	}
	if strings.TrimSpace(finishedAt) != "" {
		parts = append(parts, "Завершено "+finishedAt)
	}
	if strings.TrimSpace(status) != "" {
		if duration != "" && strings.HasPrefix(status, "Готово") {
			parts = append(parts, status+" за "+duration)
		} else {
			parts = append(parts, status)
		}
	} else if strings.TrimSpace(duration) != "" {
		parts = append(parts, duration)
	}

	return strings.Join(parts, " · ")
}

func formatOptionalClock(value *time.Time) string {
	if value == nil {
		return ""
	}

	return FormatClockUTC(*value)
}

func formatJobDuration(currentJob *job.Job, nowUTC time.Time) string {
	if currentJob == nil {
		return ""
	}
	if currentJob.DurationMS != nil {
		return FormatDurationRU(time.Duration(*currentJob.DurationMS) * time.Millisecond)
	}
	if currentJob.StartedAtUTC != nil && currentJob.Status == job.StatusRunning {
		return FormatDurationRU(nowUTC.Sub(currentJob.StartedAtUTC.UTC()))
	}

	return ""
}

func formatJobETA(currentJob *job.Job, nowUTC time.Time) string {
	if currentJob == nil || currentJob.Status != job.StatusRunning || currentJob.ProgressPercent == nil || currentJob.StartedAtUTC == nil || currentJob.ProgressIsEstimated {
		return ""
	}

	progress := *currentJob.ProgressPercent
	if progress <= 0 || progress >= 100 {
		return ""
	}

	elapsed := nowUTC.Sub(currentJob.StartedAtUTC.UTC())
	if elapsed <= 0 {
		return ""
	}

	totalEstimate := time.Duration(float64(elapsed) / (progress / 100))
	remaining := totalEstimate - elapsed
	if remaining <= 0 {
		return ""
	}

	return "Осталось ~" + FormatDurationRU(remaining)
}

func formatHistoricalRemainingETA(currentJob *job.Job, historicalEstimate time.Duration, nowUTC time.Time) string {
	if historicalEstimate <= 0 {
		return ""
	}
	if currentJob == nil || currentJob.StartedAtUTC == nil {
		return "Обычно ~" + FormatDurationRU(historicalEstimate)
	}

	elapsed := nowUTC.Sub(currentJob.StartedAtUTC.UTC())
	if elapsed <= 0 {
		return "Обычно ~" + FormatDurationRU(historicalEstimate)
	}

	remaining := historicalEstimate - elapsed
	if remaining <= 0 {
		return "Дольше обычного"
	}

	return "Осталось ~" + FormatDurationRU(remaining)
}

func clampPercent(value float64) int {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return int(value + 0.5)
	}
}

func latestJobByType(items []job.Job) map[job.Type]*job.Job {
	result := make(map[job.Type]*job.Job, len(items))
	for _, item := range items {
		if _, ok := result[item.Type]; ok {
			continue
		}

		current := item
		result[item.Type] = &current
	}

	return result
}

func firstStepIndexByKind(steps []pipelineStepState, kind string) int {
	for index, step := range steps {
		if step.kind == kind {
			return index
		}
	}

	return -1
}

func firstPendingIndex(steps []pipelineStepState) int {
	for index, step := range steps {
		if step.kind == "pending" {
			return index
		}
	}

	return -1
}

func lastCompletedIndex(steps []pipelineStepState) int {
	lastIndex := 0
	for index, step := range steps {
		if step.kind != "done" {
			break
		}
		lastIndex = index
	}

	return lastIndex
}

func userFacingJobError(currentJob *job.Job) string {
	if currentJob == nil {
		return ""
	}

	message := strings.TrimSpace(currentJob.ErrorMessage)
	if message == "" {
		return "Причину ошибки не удалось определить."
	}

	message = strings.ReplaceAll(message, "\r\n", "\n")
	message = strings.ReplaceAll(message, "\r", "\n")
	message = strings.Join(strings.Fields(message), " ")
	message = strings.TrimSuffix(message, " "+workerLogHintRU)
	message = strings.TrimSuffix(message, workerLogHintRU)
	message = strings.TrimSpace(message)
	if message == "" {
		return "Причину ошибки не удалось определить."
	}

	return message
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
