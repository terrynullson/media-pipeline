package appsettings

import (
	"fmt"
	"time"
)

type Settings struct {
	ID                  int64
	AutoUploadMinAgeSec int64
	PreviewTimeoutSec   int64
	MaxUploadSizeMB     int64
	CreatedAtUTC        time.Time
	UpdatedAtUTC        time.Time
}

func NormalizeSettings(value Settings) Settings {
	if value.AutoUploadMinAgeSec < 0 {
		value.AutoUploadMinAgeSec = 0
	}
	if value.PreviewTimeoutSec <= 0 {
		value.PreviewTimeoutSec = 600
	}
	if value.MaxUploadSizeMB <= 0 {
		value.MaxUploadSizeMB = 1024
	}
	return value
}

func ValidateSettings(value Settings) error {
	if value.AutoUploadMinAgeSec < 0 {
		return fmt.Errorf("auto upload min age must be zero or positive")
	}
	if value.PreviewTimeoutSec <= 0 {
		return fmt.Errorf("preview timeout must be greater than zero")
	}
	if value.MaxUploadSizeMB <= 0 {
		return fmt.Errorf("max upload size must be greater than zero")
	}
	return nil
}

func (s Settings) AutoUploadMinAge() time.Duration {
	return time.Duration(s.AutoUploadMinAgeSec) * time.Second
}

func (s Settings) PreviewTimeout() time.Duration {
	return time.Duration(s.PreviewTimeoutSec) * time.Second
}

func (s Settings) MaxUploadSizeBytes() int64 {
	return s.MaxUploadSizeMB * 1024 * 1024
}
