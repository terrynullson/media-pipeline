package config

import "testing"

func TestLoad_DefaultMaxUploadSizeMB(t *testing.T) {
	t.Setenv("MAX_UPLOAD_SIZE_MB", "")

	cfg := Load()

	if cfg.MaxUploadSizeMB != 500 {
		t.Fatalf("MaxUploadSizeMB = %d, want 500", cfg.MaxUploadSizeMB)
	}
	if cfg.MaxUploadSizeBytes() != 500*1024*1024 {
		t.Fatalf("MaxUploadSizeBytes = %d, want %d", cfg.MaxUploadSizeBytes(), 500*1024*1024)
	}
}

func TestLoad_InvalidOrNonPositiveMaxUploadSizeMBFallsBackToDefault(t *testing.T) {
	testCases := []string{"invalid", "0", "-10"}

	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			t.Setenv("MAX_UPLOAD_SIZE_MB", value)

			cfg := Load()

			if cfg.MaxUploadSizeMB != 500 {
				t.Fatalf("MaxUploadSizeMB = %d, want 500", cfg.MaxUploadSizeMB)
			}
		})
	}
}
