package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the loaded runtime configuration. It is intentionally a flat,
// dependency-free value object so domain/app code never has to import it.
type Config struct {
	AppPort              string
	DatabaseURL          string // canonical PostgreSQL DSN, see BuildDatabaseURL
	DBHost               string
	DBPort               string
	DBName               string
	DBUser               string
	DBPassword           string
	DBSSLMode            string
	DBMaxOpenConns       int
	DBMaxIdleConns       int
	DBConnMaxLifetimeSec int64
	UploadDir            string
	AutoUploadDir        string
	AutoUploadArchiveDir string
	AudioDir             string
	PreviewDir           string
	ScreenshotsDir       string
	FFmpegBinary         string
	PythonBinary         string
	TranscribeScript     string
	TranscribeLanguage   string
	MaxUploadSizeMB      int64
	WorkerPollIntervalMS int64
	AutoUploadMinAgeSec  int64
	FFmpegTimeoutSec     int64
	PreviewTimeoutSec    int64
	ScreenshotTimeoutSec int64
	TranscribeTimeoutSec int64
	OllamaURL            string
	OllamaModel          string
	SummaryProvider      string
	MediaAccessToken     string
	HTTPRequestTimeoutSec    int64
	UploadRateLimitPerMinute int64
}

func Load() Config {
	cfg := Config{
		AppPort:                  getEnv("APP_PORT", "8080"),
		DatabaseURL:              getEnv("DATABASE_URL", ""),
		DBHost:                   getEnv("DB_HOST", ""),
		DBPort:                   getEnv("DB_PORT", "5432"),
		DBName:                   getEnv("DB_NAME", ""),
		DBUser:                   getEnv("DB_USER", ""),
		DBPassword:               getEnv("DB_PASSWORD", ""),
		DBSSLMode:                getEnv("DB_SSLMODE", "disable"),
		DBMaxOpenConns:           int(getEnvInt64("DB_MAX_OPEN_CONNS", 25)),
		DBMaxIdleConns:           int(getEnvInt64("DB_MAX_IDLE_CONNS", 5)),
		DBConnMaxLifetimeSec:     getEnvInt64("DB_CONN_MAX_LIFETIME_SEC", 1800),
		UploadDir:                getEnv("UPLOAD_DIR", "./data/uploads"),
		AutoUploadDir:            getEnv("AUTO_UPLOAD_DIR", "./data/auto_uploads"),
		AutoUploadArchiveDir:     getEnv("AUTO_UPLOAD_ARCHIVE_DIR", "./data/auto_uploads_imported"),
		AudioDir:                 getEnv("AUDIO_DIR", "./data/audio"),
		PreviewDir:               getEnv("PREVIEW_DIR", "./data/previews"),
		ScreenshotsDir:           getEnv("SCREENSHOTS_DIR", "./data/screenshots"),
		FFmpegBinary:             getEnv("FFMPEG_BINARY", "ffmpeg"),
		PythonBinary:             getEnv("PYTHON_BINARY", "python"),
		TranscribeScript:         getEnv("TRANSCRIBE_SCRIPT", "./scripts/transcribe.py"),
		TranscribeLanguage:       getEnv("TRANSCRIBE_LANGUAGE", "ru"),
		MaxUploadSizeMB:          getEnvInt64("MAX_UPLOAD_SIZE_MB", 1024),
		WorkerPollIntervalMS:     getEnvInt64("WORKER_POLL_INTERVAL_MS", 2000),
		AutoUploadMinAgeSec:      getEnvInt64("AUTO_UPLOAD_MIN_AGE_SEC", 60),
		FFmpegTimeoutSec:         getEnvInt64("FFMPEG_TIMEOUT_SEC", 120),
		PreviewTimeoutSec:        getEnvInt64("PREVIEW_TIMEOUT_SEC", 600),
		ScreenshotTimeoutSec:     getEnvInt64("SCREENSHOT_TIMEOUT_SEC", 60),
		TranscribeTimeoutSec:     getEnvInt64("TRANSCRIBE_TIMEOUT_SEC", 300),
		OllamaURL:                getEnv("OLLAMA_URL", "http://127.0.0.1:11434"),
		OllamaModel:              getEnv("OLLAMA_MODEL", "phi3:mini"),
		SummaryProvider:          getEnv("SUMMARY_PROVIDER", "simple"),
		MediaAccessToken:         getEnv("MEDIA_ACCESS_TOKEN", ""),
		HTTPRequestTimeoutSec:    getEnvInt64("HTTP_REQUEST_TIMEOUT_SEC", 30),
		UploadRateLimitPerMinute: getEnvInt64("UPLOAD_RATE_LIMIT_PER_MINUTE", 0),
	}
	if cfg.MaxUploadSizeMB <= 0 {
		cfg.MaxUploadSizeMB = 1024
	}
	if cfg.WorkerPollIntervalMS <= 0 {
		cfg.WorkerPollIntervalMS = 2000
	}
	if cfg.AutoUploadMinAgeSec < 0 {
		cfg.AutoUploadMinAgeSec = 60
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
	if cfg.HTTPRequestTimeoutSec <= 0 {
		cfg.HTTPRequestTimeoutSec = 30
	}
	return cfg
}

// BuildDatabaseURL returns the resolved PostgreSQL DSN.
//
// Precedence:
//  1. DATABASE_URL — used verbatim if set.
//  2. discrete DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASSWORD/DB_SSLMODE — assembled
//     into a postgres:// URL.
//
// Returns an error when neither form is configured. Never logs or returns the
// password in plaintext via SafeDSN.
func (c Config) BuildDatabaseURL() (string, error) {
	if strings.TrimSpace(c.DatabaseURL) != "" {
		return c.DatabaseURL, nil
	}
	if c.DBHost == "" || c.DBName == "" || c.DBUser == "" {
		return "", fmt.Errorf("database is not configured: set DATABASE_URL or DB_HOST/DB_NAME/DB_USER")
	}
	host := c.DBHost
	if c.DBPort != "" {
		host = c.DBHost + ":" + c.DBPort
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   host,
		Path:   "/" + c.DBName,
	}
	q := u.Query()
	if c.DBSSLMode != "" {
		q.Set("sslmode", c.DBSSLMode)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// SafeDSN returns a redacted DSN suitable for logging — host, port, db, sslmode
// are preserved; the password is replaced with "***".
func (c Config) SafeDSN() string {
	raw, err := c.BuildDatabaseURL()
	if err != nil || raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Treat as opaque KV-style DSN: do a coarse password=*** replacement.
		return redactKVPassword(raw)
	}
	if u.User != nil {
		name := u.User.Username()
		u.User = url.UserPassword(name, "***")
	}
	return u.String()
}

func redactKVPassword(dsn string) string {
	parts := strings.Fields(dsn)
	for i, part := range parts {
		if strings.HasPrefix(strings.ToLower(part), "password=") {
			parts[i] = "password=***"
		}
	}
	return strings.Join(parts, " ")
}

func (c Config) MaxUploadSizeBytes() int64 {
	return c.MaxUploadSizeMB * 1024 * 1024
}

func (c Config) WorkerPollInterval() time.Duration {
	return time.Duration(c.WorkerPollIntervalMS) * time.Millisecond
}

func (c Config) AutoUploadMinAge() time.Duration {
	return time.Duration(c.AutoUploadMinAgeSec) * time.Second
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

func (c Config) HTTPRequestTimeout() time.Duration {
	return time.Duration(c.HTTPRequestTimeoutSec) * time.Second
}

func (c Config) DBConnMaxLifetime() time.Duration {
	return time.Duration(c.DBConnMaxLifetimeSec) * time.Second
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
