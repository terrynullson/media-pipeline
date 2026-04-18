package transcription

import (
	"strings"
	"time"
)

type DurationClass string

const (
	DurationClassShort    DurationClass = "short"
	DurationClassMedium   DurationClass = "medium"
	DurationClassLong     DurationClass = "long"
	DurationClassVeryLong DurationClass = "very_long"
)

const (
	defaultRuntimeBaseTimeout = 5 * time.Minute
	maxAdaptiveTimeout        = 18 * time.Hour
	shortMediaLimit           = 10 * time.Minute
	mediumMediaLimit          = 30 * time.Minute
	longMediaLimit            = 90 * time.Minute
)

type RuntimePolicy struct {
	Settings         Settings
	MediaDuration    time.Duration
	DurationClass    DurationClass
	BaseTimeout      time.Duration
	EffectiveTimeout time.Duration
	Warnings         []string
	Blocked          bool
	BlockReason      string
}

func EvaluateRuntimePolicy(settings Settings, mediaDuration time.Duration, baseTimeout time.Duration) RuntimePolicy {
	normalized := NormalizeSettings(settings)
	if mediaDuration < 0 {
		mediaDuration = 0
	}
	if baseTimeout <= 0 {
		baseTimeout = defaultRuntimeBaseTimeout
	}

	durationClass := classifyDuration(mediaDuration)
	adaptiveTimeout := calculateAdaptiveTimeout(mediaDuration, normalized, durationClass)
	effectiveTimeout := baseTimeout
	if adaptiveTimeout > effectiveTimeout {
		effectiveTimeout = adaptiveTimeout
	}

	policy := RuntimePolicy{
		Settings:         normalized,
		MediaDuration:    mediaDuration,
		DurationClass:    durationClass,
		BaseTimeout:      baseTimeout,
		EffectiveTimeout: effectiveTimeout,
		Warnings:         buildRuntimeWarnings(normalized, mediaDuration, durationClass, baseTimeout, effectiveTimeout),
	}
	if effectiveTimeout > maxAdaptiveTimeout {
		policy.Blocked = true
		policy.BlockReason = "effective_timeout_exceeds_max"
	}

	return policy
}

func BuildRuntimeSettingsWarnings(settings Settings) []string {
	normalized := NormalizeSettings(settings)
	warnings := make([]string, 0, 3)

	switch {
	case normalized.Device == "cpu" && normalized.ModelName == "base":
		warnings = appendUniqueWarning(warnings, "Для длинных файлов модель base на CPU может работать заметно дольше обычного.")
	case normalized.Device == "cpu" && normalized.ModelName == "small":
		warnings = appendUniqueWarning(warnings, "Для длинных файлов модель small на CPU может работать очень долго.")
	}

	if normalized.Device == "cpu" && normalized.ModelName != "tiny" {
		warnings = appendUniqueWarning(warnings, "Если доступна CUDA, для такой модели лучше использовать её вместо CPU.")
	}
	if normalized.Device == "cpu" && normalized.ComputeType == "float32" {
		warnings = appendUniqueWarning(warnings, "Режим float32 на CPU обычно медленнее, чем int8.")
	}

	return warnings
}

func (p RuntimePolicy) HasAdaptiveTimeout() bool {
	return p.EffectiveTimeout > p.BaseTimeout
}

func classifyDuration(mediaDuration time.Duration) DurationClass {
	switch {
	case mediaDuration <= shortMediaLimit:
		return DurationClassShort
	case mediaDuration <= mediumMediaLimit:
		return DurationClassMedium
	case mediaDuration <= longMediaLimit:
		return DurationClassLong
	default:
		return DurationClassVeryLong
	}
}

func calculateAdaptiveTimeout(mediaDuration time.Duration, settings Settings, durationClass DurationClass) time.Duration {
	if mediaDuration <= 0 {
		return defaultRuntimeBaseTimeout
	}

	multiplierPercent := runtimeMultiplierPercent(settings)
	adaptive := mediaDuration * time.Duration(multiplierPercent) / 100
	adaptive += startupBuffer(durationClass)
	return adaptive
}

func runtimeMultiplierPercent(settings Settings) int {
	normalized := NormalizeSettings(settings)

	multiplier := 300
	switch normalized.Device {
	case "cuda":
		switch normalized.ModelName {
		case "tiny":
			multiplier = 200
		case "base":
			multiplier = 300
		case "small":
			multiplier = 400
		}
	case "cpu":
		switch normalized.ModelName {
		case "tiny":
			multiplier = 300
		case "base":
			multiplier = 600
		case "small":
			multiplier = 900
		}
	}

	switch {
	case normalized.Device == "cpu" && normalized.ComputeType == "float32":
		multiplier = multiplier * 120 / 100
	case normalized.Device == "cuda" && normalized.ComputeType == "int8_float16":
		multiplier = multiplier * 110 / 100
	}

	return multiplier
}

func startupBuffer(durationClass DurationClass) time.Duration {
	switch durationClass {
	case DurationClassShort:
		return 2 * time.Minute
	case DurationClassMedium:
		return 5 * time.Minute
	case DurationClassLong:
		return 15 * time.Minute
	default:
		return 25 * time.Minute
	}
}

func buildRuntimeWarnings(
	settings Settings,
	mediaDuration time.Duration,
	durationClass DurationClass,
	baseTimeout time.Duration,
	effectiveTimeout time.Duration,
) []string {
	warnings := BuildRuntimeSettingsWarnings(settings)

	if mediaDuration >= mediumMediaLimit && settings.Device == "cpu" {
		warnings = appendUniqueWarning(warnings, "Для этого файла система увеличит лимит распознавания.")
	}
	if effectiveTimeout > baseTimeout && effectiveTimeout >= 6*time.Hour {
		warnings = appendUniqueWarning(warnings, "Даже без ошибки такая задача может обрабатываться много часов.")
	}
	if durationClass == DurationClassVeryLong && settings.Device == "cpu" {
		warnings = appendUniqueWarning(warnings, "Для очень длинных файлов на CPU распознавание стоит запускать только если вы готовы ждать очень долго.")
	}

	return warnings
}

func appendUniqueWarning(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
