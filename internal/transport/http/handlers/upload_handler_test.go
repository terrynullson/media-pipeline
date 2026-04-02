package handlers_test

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"media-pipeline/internal/app/command"
	"media-pipeline/internal/infra/db"
	"media-pipeline/internal/infra/db/repositories"
	infraRuntime "media-pipeline/internal/infra/runtime"
	"media-pipeline/internal/infra/storage"
	httptransport "media-pipeline/internal/transport/http"
	"media-pipeline/internal/transport/http/handlers"
)

func TestUploadHandler_UploadHappyPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "app.db")
	uploadDir := filepath.Join(tempDir, "uploads")

	sqlDB, err := db.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer sqlDB.Close()

	migrationsPath, err := infraRuntime.ResolvePath("internal/infra/db/migrations")
	if err != nil {
		t.Fatalf("ResolvePath(migrations) error = %v", err)
	}
	if err := db.RunMigrations(sqlDB, migrationsPath); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	templatePath, err := infraRuntime.ResolvePath("internal/transport/http/views/templates/index.html")
	if err != nil {
		t.Fatalf("ResolvePath(template) error = %v", err)
	}
	staticPath, err := infraRuntime.ResolvePath("web/static")
	if err != nil {
		t.Fatalf("ResolvePath(static) error = %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	uploadUC := command.NewUploadMediaUseCase(
		repositories.NewMediaRepository(sqlDB),
		repositories.NewJobRepository(sqlDB),
		storage.NewLocalStorage(uploadDir),
		10*1024*1024,
		logger,
	)
	handler, err := handlers.NewUploadHandler(uploadUC, templatePath, 10*1024*1024, logger)
	if err != nil {
		t.Fatalf("NewUploadHandler() error = %v", err)
	}

	router := httptransport.NewRouter(logger, handler, staticPath)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "sample.wav")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(testWAVBytes()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() multipart error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want %d, body = %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/?status=uploaded" {
		t.Fatalf("upload redirect = %q, want %q", location, "/?status=uploaded")
	}

	files, err := os.ReadDir(uploadDir)
	if err != nil {
		t.Fatalf("ReadDir(uploadDir) error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("upload dir entries = %d, want 1 dated directory", len(files))
	}
}

func TestUploadHandler_InvalidUpload(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "app.db")
	uploadDir := filepath.Join(tempDir, "uploads")

	sqlDB, err := db.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer sqlDB.Close()

	migrationsPath, err := infraRuntime.ResolvePath("internal/infra/db/migrations")
	if err != nil {
		t.Fatalf("ResolvePath(migrations) error = %v", err)
	}
	if err := db.RunMigrations(sqlDB, migrationsPath); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	templatePath, err := infraRuntime.ResolvePath("internal/transport/http/views/templates/index.html")
	if err != nil {
		t.Fatalf("ResolvePath(template) error = %v", err)
	}
	staticPath, err := infraRuntime.ResolvePath("web/static")
	if err != nil {
		t.Fatalf("ResolvePath(static) error = %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	uploadUC := command.NewUploadMediaUseCase(
		repositories.NewMediaRepository(sqlDB),
		repositories.NewJobRepository(sqlDB),
		storage.NewLocalStorage(uploadDir),
		10*1024*1024,
		logger,
	)
	handler, err := handlers.NewUploadHandler(uploadUC, templatePath, 10*1024*1024, logger)
	if err != nil {
		t.Fatalf("NewUploadHandler() error = %v", err)
	}

	router := httptransport.NewRouter(logger, handler, staticPath)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "fake.mp4")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("not media")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() multipart error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Uploaded content does not look like audio or video.") {
		t.Fatalf("response body = %q, want content type validation message", rec.Body.String())
	}
}

func testWAVBytes() []byte {
	return []byte{
		'R', 'I', 'F', 'F',
		0x24, 0x08, 0x00, 0x00,
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		0x10, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00,
		0x44, 0xAC, 0x00, 0x00,
		0x88, 0x58, 0x01, 0x00,
		0x02, 0x00, 0x10, 0x00,
		'd', 'a', 't', 'a',
		0x00, 0x08, 0x00, 0x00,
	}
}
