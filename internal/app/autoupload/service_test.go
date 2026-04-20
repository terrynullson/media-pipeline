package autoupload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"media-pipeline/internal/app/command"
)

func TestServiceImportNextUploadsAndArchivesCandidate(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		candidate: Candidate{
			Name:         "Recorder_1_25.02.10_09.00.00.00.mp4",
			RelativePath: "2026-02-10/Recorder_1_25.02.10_09.00.00.00.mp4",
			SizeBytes:    int64(len(testImportMP4HeaderBytes())),
		},
		content: testImportMP4HeaderBytes(),
	}
	uploader := &stubUploader{}
	service := NewService(source, uploader, slog.New(slog.NewTextHandler(io.Discard, nil)))

	imported, err := service.ImportNext(context.Background())
	if err != nil {
		t.Fatalf("ImportNext() error = %v", err)
	}
	if !imported {
		t.Fatal("ImportNext() imported = false, want true")
	}
	if uploader.calls != 1 {
		t.Fatalf("uploader calls = %d, want 1", uploader.calls)
	}
	if uploader.lastInput.OriginalName != source.candidate.Name {
		t.Fatalf("OriginalName = %q, want %q", uploader.lastInput.OriginalName, source.candidate.Name)
	}
	if !source.markImportedCalled {
		t.Fatal("MarkImported() was not called")
	}
}

func TestServiceImportNextReturnsFalseWhenNoCandidate(t *testing.T) {
	t.Parallel()

	service := NewService(&stubSource{}, &stubUploader{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	imported, err := service.ImportNext(context.Background())
	if err != nil {
		t.Fatalf("ImportNext() error = %v", err)
	}
	if imported {
		t.Fatal("ImportNext() imported = true, want false")
	}
}

func TestServiceImportNextReturnsErrorWhenUploadFails(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		candidate: Candidate{
			Name:         "broken.mp4",
			RelativePath: "2026-02-10/broken.mp4",
			SizeBytes:    int64(len(testImportMP4HeaderBytes())),
		},
		content: testImportMP4HeaderBytes(),
	}
	service := NewService(source, &stubUploader{err: errors.New("boom")}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	imported, err := service.ImportNext(context.Background())
	if err == nil {
		t.Fatal("ImportNext() error = nil, want non-nil")
	}
	if imported {
		t.Fatal("ImportNext() imported = true, want false")
	}
	if source.markImportedCalled {
		t.Fatal("MarkImported() called on failed upload")
	}
}

func TestServiceImportNextSkipsKnownDuplicateAndArchivesIt(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		candidate: Candidate{
			Name:          "duplicate.mp4",
			RelativePath:  "2026-02-10/duplicate.mp4",
			SizeBytes:     int64(len(testImportMP4HeaderBytes())),
			ModifiedAtUTC: time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC),
		},
		content: testImportMP4HeaderBytes(),
	}
	tracker := &stubImportTracker{
		beginResult: BeginImportResult{Started: false, Status: ImportStatusImported},
	}
	uploader := &stubUploader{}
	service := NewService(source, uploader, slog.New(slog.NewTextHandler(io.Discard, nil))).WithImportTracker(tracker)

	imported, err := service.ImportNext(context.Background())
	if err != nil {
		t.Fatalf("ImportNext() error = %v", err)
	}
	if !imported {
		t.Fatal("ImportNext() imported = false, want true")
	}
	if uploader.calls != 0 {
		t.Fatalf("uploader calls = %d, want 0", uploader.calls)
	}
	if !source.markImportedCalled {
		t.Fatal("MarkImported() was not called for duplicate")
	}
}

func TestServiceImportNextReleasesTrackerWhenUploadFails(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		candidate: Candidate{
			Name:          "broken.mp4",
			RelativePath:  "2026-02-10/broken.mp4",
			SizeBytes:     int64(len(testImportMP4HeaderBytes())),
			ModifiedAtUTC: time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC),
		},
		content: testImportMP4HeaderBytes(),
	}
	tracker := &stubImportTracker{
		beginResult: BeginImportResult{Started: true, Status: ImportStatusImporting},
	}
	service := NewService(source, &stubUploader{err: errors.New("boom")}, slog.New(slog.NewTextHandler(io.Discard, nil))).WithImportTracker(tracker)

	if _, err := service.ImportNext(context.Background()); err == nil {
		t.Fatal("ImportNext() error = nil, want non-nil")
	}
	if tracker.deleteCalls != 1 {
		t.Fatalf("deleteCalls = %d, want 1", tracker.deleteCalls)
	}
}

type stubSource struct {
	candidate          Candidate
	content            []byte
	markImportedCalled bool
}

func (s *stubSource) FindNext(context.Context, time.Time) (Candidate, bool, error) {
	if s.candidate.Name == "" {
		return Candidate{}, false, nil
	}
	return s.candidate, true, nil
}

func (s *stubSource) Open(context.Context, Candidate) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.content)), nil
}

func (s *stubSource) MarkImported(context.Context, Candidate) error {
	s.markImportedCalled = true
	return nil
}

type stubUploader struct {
	calls     int
	lastInput command.UploadMediaInput
	err       error
}

type stubImportTracker struct {
	beginResult BeginImportResult
	beginCalls  int
	deleteCalls int
	importedIDs []int64
}

func (s *stubImportTracker) Begin(context.Context, ImportKey, time.Time) (BeginImportResult, error) {
	s.beginCalls++
	return s.beginResult, nil
}

func (s *stubImportTracker) MarkImported(_ context.Context, _ ImportKey, mediaID int64, _ time.Time) error {
	s.importedIDs = append(s.importedIDs, mediaID)
	return nil
}

func (s *stubImportTracker) Delete(context.Context, ImportKey) error {
	s.deleteCalls++
	return nil
}

func (s *stubUploader) Upload(_ context.Context, in command.UploadMediaInput) (command.UploadMediaResult, error) {
	s.calls++
	s.lastInput = in
	if s.err != nil {
		return command.UploadMediaResult{}, s.err
	}
	return command.UploadMediaResult{MediaID: 77}, nil
}

func testImportMP4HeaderBytes() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0x00, 0x00, 0x02, 0x00,
		'i', 's', 'o', 'm',
		'i', 's', 'o', '2',
	}
}
