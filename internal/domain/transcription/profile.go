package transcription

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Backend string

const (
	BackendFasterWhisper Backend = "faster-whisper"
)

var languagePattern = regexp.MustCompile(`^[A-Za-z]{2,12}([_-][A-Za-z]{2,12})?$`)

type Settings struct {
	Backend     Backend
	ModelName   string
	Device      string
	ComputeType string
	Language    string
	BeamSize    int
	VADEnabled  bool
}

type Profile struct {
	ID           int64
	Backend      Backend
	ModelName    string
	Device       string
	ComputeType  string
	Language     string
	BeamSize     int
	VADEnabled   bool
	IsDefault    bool
	CreatedAtUTC time.Time
	UpdatedAtUTC time.Time
}

func DefaultProfile(language string) Profile {
	return Profile{
		Backend:     BackendFasterWhisper,
		ModelName:   "tiny",
		Device:      "cpu",
		ComputeType: "int8",
		Language:    normalizeLanguage(language),
		BeamSize:    5,
		VADEnabled:  true,
		IsDefault:   true,
	}
}

func (p Profile) Settings() Settings {
	return Settings{
		Backend:     p.Backend,
		ModelName:   p.ModelName,
		Device:      p.Device,
		ComputeType: p.ComputeType,
		Language:    p.Language,
		BeamSize:    p.BeamSize,
		VADEnabled:  p.VADEnabled,
	}
}

func SupportedBackends() []Backend {
	return []Backend{BackendFasterWhisper}
}

func SupportedModels() []string {
	return []string{"tiny", "base", "small"}
}

func SupportedDevices() []string {
	return []string{"cpu", "cuda"}
}

func SupportedComputeTypes(device string) []string {
	switch strings.TrimSpace(strings.ToLower(device)) {
	case "cpu":
		return []string{"int8", "float32"}
	case "cuda":
		return []string{"float16", "int8_float16"}
	default:
		return nil
	}
}

func NormalizeSettings(in Settings) Settings {
	in.Backend = Backend(strings.TrimSpace(strings.ToLower(string(in.Backend))))
	in.ModelName = strings.TrimSpace(strings.ToLower(in.ModelName))
	in.Device = strings.TrimSpace(strings.ToLower(in.Device))
	in.ComputeType = strings.TrimSpace(strings.ToLower(in.ComputeType))
	in.Language = normalizeLanguage(in.Language)
	return in
}

func NormalizeProfile(in Profile) Profile {
	settings := NormalizeSettings(in.Settings())
	in.Backend = settings.Backend
	in.ModelName = settings.ModelName
	in.Device = settings.Device
	in.ComputeType = settings.ComputeType
	in.Language = settings.Language
	in.BeamSize = settings.BeamSize
	in.VADEnabled = settings.VADEnabled
	return in
}

func ValidateSettings(in Settings) error {
	normalized := NormalizeSettings(in)

	if normalized.Backend != BackendFasterWhisper {
		return fmt.Errorf("неподдерживаемый backend %q", in.Backend)
	}
	if !containsString(SupportedModels(), normalized.ModelName) {
		return fmt.Errorf("неподдерживаемая model_name %q", in.ModelName)
	}
	if !containsString(SupportedDevices(), normalized.Device) {
		return fmt.Errorf("неподдерживаемое device %q", in.Device)
	}
	if !containsString(SupportedComputeTypes(normalized.Device), normalized.ComputeType) {
		return fmt.Errorf("неподдерживаемый compute_type %q для device %q", in.ComputeType, in.Device)
	}
	if normalized.BeamSize < 1 || normalized.BeamSize > 10 {
		return fmt.Errorf("beam_size должен быть в диапазоне от 1 до 10")
	}
	if normalized.Language != "" && !languagePattern.MatchString(normalized.Language) {
		return fmt.Errorf("language должен быть пустым или выглядеть как ru, en, en-US")
	}

	return nil
}

func ValidateProfile(in Profile) error {
	return ValidateSettings(in.Settings())
}

func normalizeLanguage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "auto") {
		return ""
	}
	return trimmed
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
