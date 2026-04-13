package worker

import (
	"fmt"
	"log/slog"

	transcriptionapp "media-pipeline/internal/app/transcription"
	"media-pipeline/internal/domain/transcription"
)

func buildTranscriptionTimeoutFailure(settings transcription.Settings, policy transcription.RuntimePolicy) string {
	return fmt.Sprintf(
		"Не удалось распознать текст: модель %s на %s (%s) для %s превысила адаптивный лимит %s.",
		settings.ModelName,
		transcriptionDeviceLabel(settings.Device),
		settings.ComputeType,
		transcriptionapp.DurationClassPhraseRU(policy.DurationClass),
		transcriptionapp.FormatRuntimeDurationRU(policy.EffectiveTimeout),
	)
}

func buildTranscriptionBlockedFailure(policy transcription.RuntimePolicy) string {
	phrase := transcriptionapp.FormatBlockReason(policy)
	if phrase == "" {
		return "Не удалось распознать текст: задача заблокирована конфигурацией."
	}
	return "Не удалось распознать текст: " + phrase
}

func transcriptionPolicyLogAttrs(policy transcription.RuntimePolicy) []slog.Attr {
	return []slog.Attr{
		slog.Duration("media_duration", policy.MediaDuration),
		slog.String("duration_class", string(policy.DurationClass)),
		slog.Duration("base_timeout", policy.BaseTimeout),
		slog.Duration("effective_timeout", policy.EffectiveTimeout),
		slog.Bool("blocked", policy.Blocked),
		slog.String("block_reason", policy.BlockReason),
		slog.Any("policy_warnings", policy.Warnings),
	}
}

func transcriptionDeviceLabel(value string) string {
	switch value {
	case "cpu":
		return "CPU"
	case "cuda":
		return "CUDA"
	default:
		return value
	}
}
