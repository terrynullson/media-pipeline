package config

import "testing"

func TestLoad_DefaultMaxUploadSizeMB(t *testing.T) {
	t.Setenv("MAX_UPLOAD_SIZE_MB", "")

	cfg := Load()

	if cfg.MaxUploadSizeMB != 1024 {
		t.Fatalf("MaxUploadSizeMB = %d, want 1024", cfg.MaxUploadSizeMB)
	}
	if cfg.MaxUploadSizeBytes() != 1024*1024*1024 {
		t.Fatalf("MaxUploadSizeBytes = %d, want %d", cfg.MaxUploadSizeBytes(), 1024*1024*1024)
	}
}

func TestLoad_InvalidOrNonPositiveMaxUploadSizeMBFallsBackToDefault(t *testing.T) {
	testCases := []string{"invalid", "0", "-10"}

	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			t.Setenv("MAX_UPLOAD_SIZE_MB", value)

			cfg := Load()

			if cfg.MaxUploadSizeMB != 1024 {
				t.Fatalf("MaxUploadSizeMB = %d, want 1024", cfg.MaxUploadSizeMB)
			}
		})
	}
}

func TestLoad_AutoUploadDefaults(t *testing.T) {
	t.Setenv("AUTO_UPLOAD_DIR", "")
	t.Setenv("AUTO_UPLOAD_ARCHIVE_DIR", "")
	t.Setenv("AUTO_UPLOAD_MIN_AGE_SEC", "")

	cfg := Load()

	if cfg.AutoUploadDir != "./data/auto_uploads" {
		t.Fatalf("AutoUploadDir = %q, want %q", cfg.AutoUploadDir, "./data/auto_uploads")
	}
	if cfg.AutoUploadArchiveDir != "./data/auto_uploads_imported" {
		t.Fatalf("AutoUploadArchiveDir = %q, want %q", cfg.AutoUploadArchiveDir, "./data/auto_uploads_imported")
	}
	if cfg.AutoUploadMinAgeSec != 60 {
		t.Fatalf("AutoUploadMinAgeSec = %d, want 60", cfg.AutoUploadMinAgeSec)
	}
}
