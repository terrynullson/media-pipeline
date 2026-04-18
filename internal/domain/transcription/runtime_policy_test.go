package transcription

import (
	"strings"
	"testing"
	"time"
)

func TestEvaluateRuntimePolicy_LongSmallCPUUsesAdaptiveTimeout(t *testing.T) {
	t.Parallel()

	policy := EvaluateRuntimePolicy(Settings{
		Backend:     BackendFasterWhisper,
		ModelName:   "small",
		Device:      "cpu",
		ComputeType: "int8",
		BeamSize:    5,
		VADEnabled:  true,
	}, 60*time.Minute, 5*time.Minute)

	if policy.DurationClass != DurationClassLong {
		t.Fatalf("DurationClass = %q, want %q", policy.DurationClass, DurationClassLong)
	}
	if policy.EffectiveTimeout != 9*time.Hour+15*time.Minute {
		t.Fatalf("EffectiveTimeout = %s, want 9h15m", policy.EffectiveTimeout)
	}
	if policy.Blocked {
		t.Fatal("Blocked = true, want false")
	}
	if !policy.HasAdaptiveTimeout() {
		t.Fatal("HasAdaptiveTimeout() = false, want true")
	}
	if len(policy.Warnings) == 0 {
		t.Fatal("Warnings = empty, want contextual warning")
	}
}

func TestEvaluateRuntimePolicy_RealisticBaseCPUNotBlocked(t *testing.T) {
	t.Parallel()

	policy := EvaluateRuntimePolicy(Settings{
		Backend:     BackendFasterWhisper,
		ModelName:   "base",
		Device:      "cpu",
		ComputeType: "int8",
		BeamSize:    5,
		VADEnabled:  true,
	}, 45*time.Minute, 5*time.Minute)

	if policy.Blocked {
		t.Fatalf("Blocked = true, want false, reason = %q", policy.BlockReason)
	}
	if policy.EffectiveTimeout <= 5*time.Minute {
		t.Fatalf("EffectiveTimeout = %s, want adaptive timeout above base", policy.EffectiveTimeout)
	}
}

func TestEvaluateRuntimePolicy_BlocksUnrealisticSmallCPUScenario(t *testing.T) {
	t.Parallel()

	policy := EvaluateRuntimePolicy(Settings{
		Backend:     BackendFasterWhisper,
		ModelName:   "small",
		Device:      "cpu",
		ComputeType: "int8",
		BeamSize:    5,
		VADEnabled:  true,
	}, 2*time.Hour, 5*time.Minute)

	if !policy.Blocked {
		t.Fatal("Blocked = false, want true")
	}
	if policy.BlockReason != "effective_timeout_exceeds_max" {
		t.Fatalf("BlockReason = %q, want %q", policy.BlockReason, "effective_timeout_exceeds_max")
	}
}

func TestBuildRuntimeSettingsWarnings_CPUHeavyModel(t *testing.T) {
	t.Parallel()

	warnings := BuildRuntimeSettingsWarnings(Settings{
		Backend:     BackendFasterWhisper,
		ModelName:   "small",
		Device:      "cpu",
		ComputeType: "float32",
		BeamSize:    5,
		VADEnabled:  true,
	})

	joined := strings.Join(warnings, " ")
	for _, want := range []string{
		"модель small на CPU",
		"лучше использовать её вместо CPU",
		"float32 на CPU обычно медленнее",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings %q do not contain %q", joined, want)
		}
	}
}
