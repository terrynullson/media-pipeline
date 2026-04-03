package handlers

import (
	"strings"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

const (
	pipelineStageTotal = 5
	workerLogHintRU    = "Подробности смотрите в логах worker."
)

type PipelineStepView struct {
	Label       string
	StatusLabel string
	Tone        string
	IsCurrent   bool
	IsFailed    bool
}

type MediaPipelineView struct {
	StatusLabel      string
	StatusTone       string
	StageLabel       string
	StageValue       int
	StageTotal       int
	IsActive         bool
	CurrentStage     string
	FailedStage      string
	ErrorSummary     string
	ErrorLocation    string
	Steps            []PipelineStepView
	AutomaticJobFail *job.Job
}

type pipelineStepState struct {
	label       string
	statusLabel string
	tone        string
	kind        string
	job         *job.Job
}

func buildMediaPipelineView(mediaItem media.Media, jobs []job.Job) MediaPipelineView {
	jobsByType := latestJobsByType(jobs)

	uploadStep := pipelineStepState{
		label:       "Загрузка файла",
		statusLabel: "Готово",
		tone:        "success",
		kind:        "done",
	}
	extractStep := describeExtractAudioStep(mediaItem, jobsByType[job.TypeExtractAudio])
	transcribeStep := describeTranscribeStep(mediaItem, jobsByType[job.TypeTranscribe])
	analyzeStep := describeQueuedStep(
		"Анализ триггеров",
		jobsByType[job.TypeAnalyzeTriggers],
		transcribeStep.kind == "done",
	)
	screenshotStep := describeScreenshotStep(
		mediaItem,
		jobsByType[job.TypeExtractScreenshots],
		analyzeStep.kind == "done",
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
		view.StageLabel = "Ошибка на шаге: " + strings.ToLower(failedStep.label)
		view.StageValue = failedIndex + 1
		view.FailedStage = failedStep.label
		view.CurrentStage = failedStep.label
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
	case lastCompletedIndex == len(steps)-1:
		view.StatusLabel = "Готово"
		view.StatusTone = "success"
		view.StageLabel = "Все обязательные шаги завершены"
		view.StageValue = len(steps)
		view.CurrentStage = "Завершено"
	case lastCompletedIndex == 0:
		view.StatusLabel = "Загружен"
		view.StatusTone = "uploaded"
		view.StageLabel = "Дальше: извлечение аудио"
		view.StageValue = 1
		view.CurrentStage = "Извлечение аудио"
	default:
		view.StatusLabel = "Ожидает следующий шаг"
		view.StatusTone = "ready"
		view.StageValue = lastCompletedIndex + 1
		if pendingIndex >= 0 {
			view.StageLabel = "Дальше: " + steps[pendingIndex].label
			view.CurrentStage = steps[pendingIndex].label
		} else {
			view.StageLabel = "Ожидает продолжения обработки"
		}
	}

	for index, step := range steps {
		view.Steps = append(view.Steps, PipelineStepView{
			Label:       step.label,
			StatusLabel: step.statusLabel,
			Tone:        step.tone,
			IsCurrent:   failedIndex < 0 && runningIndex == index,
			IsFailed:    failedIndex == index,
		})
	}

	return view
}

func describeExtractAudioStep(mediaItem media.Media, currentJob *job.Job) pipelineStepState {
	if currentJob != nil {
		switch currentJob.Status {
		case job.StatusFailed:
			return failedStep("Извлечение аудио", currentJob)
		case job.StatusRunning:
			return runningStep("Извлечение аудио")
		case job.StatusDone:
			return doneStep("Извлечение аудио")
		case job.StatusPending:
			return pendingStep("Извлечение аудио")
		}
	}

	switch mediaItem.Status {
	case media.StatusProcessing:
		return runningStep("Извлечение аудио")
	case media.StatusAudioExtracted, media.StatusTranscribing, media.StatusTranscribed:
		return doneStep("Извлечение аудио")
	default:
		return pendingStep("Извлечение аудио")
	}
}

func describeTranscribeStep(mediaItem media.Media, currentJob *job.Job) pipelineStepState {
	if currentJob != nil {
		switch currentJob.Status {
		case job.StatusFailed:
			return failedStep("Распознавание текста", currentJob)
		case job.StatusRunning:
			return runningStep("Распознавание текста")
		case job.StatusDone:
			return doneStep("Распознавание текста")
		case job.StatusPending:
			return pendingStep("Распознавание текста")
		}
	}

	switch mediaItem.Status {
	case media.StatusTranscribing:
		return runningStep("Распознавание текста")
	case media.StatusTranscribed:
		return doneStep("Распознавание текста")
	case media.StatusAudioExtracted:
		return pendingStep("Распознавание текста")
	default:
		return blockedStep("Распознавание текста")
	}
}

func describeQueuedStep(label string, currentJob *job.Job, unlocked bool) pipelineStepState {
	if currentJob != nil {
		switch currentJob.Status {
		case job.StatusFailed:
			return failedStep(label, currentJob)
		case job.StatusRunning:
			return runningStep(label)
		case job.StatusDone:
			return doneStep(label)
		case job.StatusPending:
			return pendingStep(label)
		}
	}

	if unlocked {
		return pendingStep(label)
	}

	return blockedStep(label)
}

func describeScreenshotStep(mediaItem media.Media, currentJob *job.Job, unlocked bool) pipelineStepState {
	label := "Снимки по триггерам"
	if mediaItem.IsAudioOnly() && unlocked && currentJob == nil {
		return skippedStep(label)
	}
	if currentJob != nil {
		switch currentJob.Status {
		case job.StatusFailed:
			return failedStep(label, currentJob)
		case job.StatusRunning:
			return runningStep(label)
		case job.StatusDone:
			if mediaItem.IsAudioOnly() {
				return skippedStep(label)
			}
			return doneStep(label)
		case job.StatusPending:
			return pendingStep(label)
		}
	}

	if unlocked {
		if mediaItem.IsAudioOnly() {
			return skippedStep(label)
		}
		return pendingStep(label)
	}

	return blockedStep(label)
}

func doneStep(label string) pipelineStepState {
	return pipelineStepState{
		label:       label,
		statusLabel: "Готово",
		tone:        "success",
		kind:        "done",
	}
}

func runningStep(label string) pipelineStepState {
	return pipelineStepState{
		label:       label,
		statusLabel: "В работе",
		tone:        "running",
		kind:        "running",
	}
}

func pendingStep(label string) pipelineStepState {
	return pipelineStepState{
		label:       label,
		statusLabel: "Ждёт",
		tone:        "ready",
		kind:        "pending",
	}
}

func blockedStep(label string) pipelineStepState {
	return pipelineStepState{
		label:       label,
		statusLabel: "Не начато",
		tone:        "neutral",
		kind:        "blocked",
	}
}

func skippedStep(label string) pipelineStepState {
	return pipelineStepState{
		label:       label,
		statusLabel: "Не требуется",
		tone:        "neutral",
		kind:        "done",
	}
}

func failedStep(label string, currentJob *job.Job) pipelineStepState {
	return pipelineStepState{
		label:       label,
		statusLabel: "Ошибка",
		tone:        "error",
		kind:        "failed",
		job:         currentJob,
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
