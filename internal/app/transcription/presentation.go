package transcriptionapp

import (
	"fmt"
	"time"

	"media-pipeline/internal/domain/transcription"
)

// DurationClassLabelRU returns a short Russian label for a duration class,
// e.g. "короткий файл".
func DurationClassLabelRU(class transcription.DurationClass) string {
	switch class {
	case transcription.DurationClassShort:
		return "короткий файл"
	case transcription.DurationClassMedium:
		return "средний файл"
	case transcription.DurationClassLong:
		return "длинный файл"
	case transcription.DurationClassVeryLong:
		return "очень длинный файл"
	default:
		return "файл"
	}
}

// DurationClassPhraseRU returns a genitive Russian phrase for a duration class,
// e.g. "короткого файла".
func DurationClassPhraseRU(class transcription.DurationClass) string {
	switch class {
	case transcription.DurationClassShort:
		return "короткого файла"
	case transcription.DurationClassMedium:
		return "файла средней длины"
	case transcription.DurationClassLong:
		return "длинного файла"
	case transcription.DurationClassVeryLong:
		return "очень длинного файла"
	default:
		return "файла"
	}
}

// FormatRuntimeDurationRU formats a duration as a Russian human-readable string,
// e.g. "9 ч 15 мин", "30 мин", "1 ч".
func FormatRuntimeDurationRU(value time.Duration) string {
	if value <= 0 {
		return "0 мин"
	}

	rounded := value.Round(time.Minute)
	if rounded < time.Minute {
		rounded = time.Minute
	}

	hours := rounded / time.Hour
	minutes := (rounded % time.Hour) / time.Minute
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%d ч %d мин", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%d ч", hours)
	default:
		return fmt.Sprintf("%d мин", minutes)
	}
}

// FormatBlockReason returns a Russian description of why a RuntimePolicy is
// blocked. Returns an empty string if the policy is not blocked.
func FormatBlockReason(policy transcription.RuntimePolicy) string {
	if !policy.Blocked {
		return ""
	}
	return fmt.Sprintf(
		"Конфигурация %s на %s для %s обычно требует слишком много времени. Ожидаемый лимит около %s, поэтому задача не запускается автоматически.",
		runtimeModelLabel(policy.Settings.ModelName),
		runtimeDeviceLabel(policy.Settings.Device),
		DurationClassPhraseRU(policy.DurationClass),
		FormatRuntimeDurationRU(policy.EffectiveTimeout),
	)
}

func runtimeModelLabel(value string) string {
	if value == "" {
		return "неизвестная модель"
	}
	return "модель " + value
}

func runtimeDeviceLabel(value string) string {
	switch value {
	case "cpu":
		return "CPU"
	case "cuda":
		return "CUDA"
	default:
		if value == "" {
			return "неизвестном устройстве"
		}
		return value
	}
}
