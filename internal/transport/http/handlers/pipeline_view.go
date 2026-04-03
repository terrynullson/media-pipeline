package handlers

import (
	"fmt"
	"strings"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

const (
	pipelineStageTotal = 5
	workerLogHintRU    = "Подробности смотрите в логах worker."
)

type PipelineStepView struct {
	Label           string
	StatusLabel     string
	Tone            string
	IsCurrent       bool
	IsFailed        bool
	TimingText      string
	StartedAtLabel  string
	FinishedAtLabel string
	DurationLabel   string
	ProgressLabel   string
	ProgressPercent int
	ProgressVisible bool
}

type MediaPipelineView struct {
	StatusLabel       string
	StatusTone        string
	StageLabel        string
	StageValue        int
	StageTotal        int
	IsActive          bool
	CurrentStage      string
	FailedStage       string
	ErrorSummary      string
	ErrorLocation     string
	Steps             []PipelineStepView
	AutomaticJobFail  *job.Job
	CurrentTimingText string
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
	progressLabel   string
	progressPercent int
	progressVisible bool
}

func buildMediaPipelineView(mediaItem media.Media, jobs []job.Job) MediaPipelineView {
	nowUTC := time.Now().UTC()
	jobsByType := latestJobsByType(jobs)

	uploadStep := describeUploadStep(mediaItem, jobsByType[job.TypeUpload], nowUTC)
	extractStep := describeExtractAudioStep(mediaItem, jobsByType[job.TypeExtractAudio], nowUTC)
	transcribeStep := describeTranscribeStep(mediaItem, jobsByType[job.TypeTranscribe], nowUTC)
	analyzeStep := describeQueuedStep(
		"Анализ триггеров",
		jobsByType[job.TypeAnalyzeTriggers],
		transcribeStep.kind == "done",
		nowUTC,
	)
	screenshotStep := describeScreenshotStep(
		mediaItem,
		jobsByType[job.TypeExtractScreenshots],
		analyzeStep.kind == "done",
		nowUTC,
	)

	steps := []pipelineStepState{
		uploadStep,
		extractStep,
		transcribeStep,
		analyzeStep,
		screenshotStep,
	}

	failedIndex := firstStepIndexByKind(steps, "failed")
	runningIndex := firstStepIndexByKind(steps, "running")
	pendingIndex := firstPendingIndex(steps)
	lastCompletedIndex := lastCompletedIndex(steps)

	view := MediaPipelineView{
		StageTotal: pipelineStageTotal,
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
	case lastCompletedIndex == len(steps)-1:
		view.StatusLabel = "Готово"
		view.StatusTone = "success"
		view.StageLabel = "Основные этапы завершены"
		view.StageValue = len(steps)
		view.CurrentStage = "Завершено"
		view.CurrentTimingText = steps[len(steps)-1].timingText
	case lastCompletedIndex == 0:
		view.StatusLabel = "Загружен"
		view.StatusTone = "uploaded"
		view.StageValue = 1
		view.StageLabel = "Дальше: извлечение аудио"
		view.CurrentStage = "Извлечение аудио"
		view.CurrentTimingText = steps[0].timingText
	default:
		view.StatusLabel = "Ожидает следующий шаг"
		view.StatusTone = "ready"
		view.StageValue = max(1, lastCompletedIndex+1)
		if pendingIndex >= 0 {
			view.StageLabel = "Дальше: " + strings.ToLower(steps[pendingIndex].label)
			view.CurrentStage = steps[pendingIndex].label
			view.CurrentTimingText = steps[pendingIndex].timingText
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
			ProgressLabel:   step.progressLabel,
			ProgressPercent: step.progressPercent,
			ProgressVisible: step.progressVisible,
		})
	}

	return view
}

func describeUploadStep(mediaItem media.Media, currentJob *job.Job, nowUTC time.Time) pipelineStepState {
	if currentJob != nil {
		return describeJobBackedStep("Загрузка файла", currentJob, nowUTC)
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

func describeExtractAudioStep(mediaItem media.Media, currentJob *job.Job, nowUTC time.Time) pipelineStepState {
	if currentJob != nil {
		return describeJobBackedStep("Извлечение аудио", currentJob, nowUTC)
	}

	switch mediaItem.Status {
	case media.StatusProcessing:
		return pipelineStepState{label: "Извлечение аудио", statusLabel: "В работе", tone: "running", kind: "running", timingText: "Идёт подготовка аудио"}
	case media.StatusAudioExtracted, media.StatusTranscribing, media.StatusTranscribed:
		return pipelineStepState{label: "Извлечение аудио", statusLabel: "Готово", tone: "success", kind: "done", timingText: "Готово"}
	default:
		return pipelineStepState{label: "Извлечение аудио", statusLabel: "Не начато", tone: "neutral", kind: "blocked", timingText: "Не запускалось"}
	}
}

func describeTranscribeStep(mediaItem media.Media, currentJob *job.Job, nowUTC time.Time) pipelineStepState {
	if currentJob != nil {
		return describeJobBackedStep("Распознавание текста", currentJob, nowUTC)
	}

	switch mediaItem.Status {
	case media.StatusTranscribing:
		return pipelineStepState{label: "Распознавание текста", statusLabel: "В работе", tone: "running", kind: "running", timingText: "Идёт распознавание"}
	case media.StatusTranscribed:
		return pipelineStepState{label: "Распознавание текста", statusLabel: "Готово", tone: "success", kind: "done", timingText: "Готово"}
	case media.StatusAudioExtracted:
		return pipelineStepState{label: "Распознавание текста", statusLabel: "Ждёт", tone: "ready", kind: "pending", timingText: "Ждёт запуска"}
	default:
		return pipelineStepState{label: "Распознавание текста", statusLabel: "Не начато", tone: "neutral", kind: "blocked", timingText: "Не запускалось"}
	}
}

func describeQueuedStep(label string, currentJob *job.Job, unlocked bool, nowUTC time.Time) pipelineStepState {
	if currentJob != nil {
		return describeJobBackedStep(label, currentJob, nowUTC)
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
		step := describeJobBackedStep(label, currentJob, nowUTC)
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

func describeJobBackedStep(label string, currentJob *job.Job, nowUTC time.Time) pipelineStepState {
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
				label := strings.TrimSpace(currentJob.ProgressLabel)
				if label == "" {
					label = "Оценка по сегментам"
				}
				step.progressLabel = fmt.Sprintf("%s: %d%%", label, step.progressPercent)
			} else {
				step.progressLabel = fmt.Sprintf("%d%%", step.progressPercent)
			}
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
	default:
		step.statusLabel = "Не начато"
		step.tone = "neutral"
		step.kind = "blocked"
		step.timingText = "Не запускалось"
	}

	return step
}

func buildTimingText(startedAt string, finishedAt string, status string, duration string) string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(startedAt) != "" {
		parts = append(parts, "Старт "+startedAt)
	}
	if strings.TrimSpace(finishedAt) != "" {
		parts = append(parts, "Финиш "+finishedAt)
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

func latestJobsByType(items []job.Job) map[job.Type]*job.Job {
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
