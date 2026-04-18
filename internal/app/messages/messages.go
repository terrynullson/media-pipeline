// Package messages centralises user-facing Russian-language strings so that
// terminology changes require edits in a single file.
//
// Only error messages and labels that are surfaced directly to users belong
// here. Internal log messages, template strings, and timing labels ("Готово",
// "В работе") stay where they are used.
package messages

// Transcription failure reasons — returned by humanizeUserReason.
const (
	TranscriptionEmpty   = "модель вернула пустой результат"
	PythonDepsNotFound   = "не удалось запустить Python-зависимости распознавания"
	OutOfMemory          = "не хватило памяти для запуска модели"
	CUDAUnavailable      = "CUDA недоступна для этой модели"
	TranscribeExitError  = "процесс распознавания завершился с ошибкой"
	FFmpegExitError      = "ffmpeg завершился с ошибкой"
	UnknownFailureReason = "не удалось определить причину"
)

// Timeout messages — returned when context.DeadlineExceeded is detected.
const (
	TimeoutFFmpeg     = "Истекло время ожидания ffmpeg."
	TimeoutTranscribe = "Истекло время ожидания распознавания текста."
	TimeoutGeneric    = "Истекло время ожидания обработки."
	TimeoutPreview    = "Истекло время ожидания подготовки browser-safe preview видео."
)

// Stage error prefixes — prepended to a human-readable reason string.
const (
	PrefixExtractAudio = "Не удалось извлечь аудио: "
	PrefixTranscribe   = "Не удалось распознать текст: "
	PrefixAnalyze      = "Не удалось проанализировать триггеры: "
	PrefixScreenshots  = "Не удалось подготовить скриншоты: "
	PrefixSummary      = "Не удалось собрать саммари: "
	PrefixPreviewVideo = "Не удалось подготовить browser-safe preview видео: "
)
