package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppPort              string
	DBPath               string
	UploadDir            string
	AudioDir             string
	PreviewDir           string
	ScreenshotsDir       string
	FFmpegBinary         string
	PythonBinary         string
	TranscribeScript     string
	TranscribeLanguage   string
	MaxUploadSizeMB      int64
	WorkerPollIntervalMS int64
	FFmpegTimeoutSec     int64
	PreviewTimeoutSec    int64
	ScreenshotTimeoutSec int64
	TranscribeTimeoutSec int64
}

func Load() Config {
	cfg := Config{
		AppPort:              getEnv("APP_PORT", "8080"),
		DBPath:               getEnv("DB_PATH", "./data/app.db"),
		UploadDir:            getEnv("UPLOAD_DIR", "./data/uploads"),
		AudioDir:             getEnv("AUDIO_DIR", "./data/audio"),
		PreviewDir:           getEnv("PREVIEW_DIR", "./data/previews"),
		ScreenshotsDir:       getEnv("SCREENSHOTS_DIR", "./data/screenshots"),
		FFmpegBinary:         getEnv("FFMPEG_BINARY", "ffmpeg"),
		PythonBinary:         getEnv("PYTHON_BINARY", "python"),
		TranscribeScript:     getEnv("TRANSCRIBE_SCRIPT", "./scripts/transcribe.py"),
		TranscribeLanguage:   getEnv("TRANSCRIBE_LANGUAGE", ""),
		MaxUploadSizeMB:      getEnvInt64("MAX_UPLOAD_SIZE_MB", 500),
		WorkerPollIntervalMS: getEnvInt64("WORKER_POLL_INTERVAL_MS", 2000),
		FFmpegTimeoutSec:     getEnvInt64("FFMPEG_TIMEOUT_SEC", 120),
		PreviewTimeoutSec:    getEnvInt64("PREVIEW_TIMEOUT_SEC", 600),
		ScreenshotTimeoutSec: getEnvInt64("SCREENSHOT_TIMEOUT_SEC", 60),
		TranscribeTimeoutSec: getEnvInt64("TRANSCRIBE_TIMEOUT_SEC", 300),
	}
	if cfg.MaxUploadSizeMB <= 0 {
		cfg.MaxUploadSizeMB = 500
	}
	if cfg.WorkerPollIntervalMS <= 0 {
		cfg.WorkerPollIntervalMS = 2000
	}
	if cfg.FFmpegTimeoutSec <= 0 {
		cfg.FFmpegTimeoutSec = 120
	}
	if cfg.PreviewTimeoutSec <= 0 {
		cfg.PreviewTimeoutSec = 600
	}
	if cfg.ScreenshotTimeoutSec <= 0 {
		cfg.ScreenshotTimeoutSec = 60
	}
	if cfg.TranscribeTimeoutSec <= 0 {
		cfg.TranscribeTimeoutSec = 300
	}
	return cfg
}

func (c Config) MaxUploadSizeBytes() int64 {
	return c.MaxUploadSizeMB * 1024 * 1024
}

func (c Config) WorkerPollInterval() time.Duration {
	return time.Duration(c.WorkerPollIntervalMS) * time.Millisecond
}

func (c Config) FFmpegTimeout() time.Duration {
	return time.Duration(c.FFmpegTimeoutSec) * time.Second
}

func (c Config) ScreenshotTimeout() time.Duration {
	return time.Duration(c.ScreenshotTimeoutSec) * time.Second
}

func (c Config) PreviewTimeout() time.Duration {
	return time.Duration(c.PreviewTimeoutSec) * time.Second
}

func (c Config) TranscribeTimeout() time.Duration {
	return time.Duration(c.TranscribeTimeoutSec) * time.Second
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
