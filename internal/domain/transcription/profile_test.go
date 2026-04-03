package transcription

import "testing"

func TestValidateSettings(t *testing.T) {
	t.Parallel()

	valid := Settings{
		Backend:     BackendFasterWhisper,
		ModelName:   "small",
		Device:      "cpu",
		ComputeType: "int8",
		Language:    "ru",
		BeamSize:    5,
		VADEnabled:  true,
	}

	if err := ValidateSettings(valid); err != nil {
		t.Fatalf("ValidateSettings() error = %v", err)
	}
}

func TestValidateSettingsRejectsUnsupportedCombination(t *testing.T) {
	t.Parallel()

	err := ValidateSettings(Settings{
		Backend:     BackendFasterWhisper,
		ModelName:   "tiny",
		Device:      "cuda",
		ComputeType: "int8",
		BeamSize:    5,
	})
	if err == nil {
		t.Fatal("ValidateSettings() error = nil, want unsupported combination")
	}
}

func TestNormalizeSettingsTreatsAutoLanguageAsEmpty(t *testing.T) {
	t.Parallel()

	normalized := NormalizeSettings(Settings{
		Backend:     BackendFasterWhisper,
		ModelName:   "Tiny",
		Device:      "CPU",
		ComputeType: "INT8",
		Language:    " auto ",
		BeamSize:    2,
	})

	if normalized.Language != "" {
		t.Fatalf("normalized language = %q, want empty", normalized.Language)
	}
	if normalized.ModelName != "tiny" {
		t.Fatalf("normalized model = %q, want tiny", normalized.ModelName)
	}
}
