package command

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
)

func TestUploadMediaUseCase_UploadCreatesPreviewJobForVideo(t *testing.T) {
	t.Parallel()

	mediaRepo := &stubUploadMediaRepository{createdID: 42}
	jobRepo := &stubUploadJobRepository{}
	storage := stubUploadStorage{
		stored: ports.StoredFile{
			StoredName:   "demo.mp4",
			RelativePath: "2026-04-04/demo.mp4",
			SizeBytes:    128,
		},
	}
	uc := NewUploadMediaUseCase(mediaRepo, jobRepo, storage, 1024*1024, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := uc.Upload(context.Background(), UploadMediaInput{
		OriginalName: "demo.mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    int64(len(testMP4HeaderBytes())),
		Content:      bytes.NewReader(testMP4HeaderBytes()),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if len(jobRepo.created) != 3 {
		t.Fatalf("created jobs = %d, want 3", len(jobRepo.created))
	}
	if jobRepo.created[0].Type != job.TypeUpload {
		t.Fatalf("job[0] type = %q, want upload", jobRepo.created[0].Type)
	}
	if jobRepo.created[1].Type != job.TypeExtractAudio {
		t.Fatalf("job[1] type = %q, want extract_audio", jobRepo.created[1].Type)
	}
	if jobRepo.created[2].Type != job.TypePreparePreviewVideo {
		t.Fatalf("job[2] type = %q, want prepare_preview_video", jobRepo.created[2].Type)
	}
}

func TestUploadMediaUseCase_UploadDoesNotCreatePreviewJobForAudioOnly(t *testing.T) {
	t.Parallel()

	mediaRepo := &stubUploadMediaRepository{createdID: 43}
	jobRepo := &stubUploadJobRepository{}
	storage := stubUploadStorage{
		stored: ports.StoredFile{
			StoredName:   "demo.wav",
			RelativePath: "2026-04-04/demo.wav",
			SizeBytes:    128,
		},
	}
	uc := NewUploadMediaUseCase(mediaRepo, jobRepo, storage, 1024*1024, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := uc.Upload(context.Background(), UploadMediaInput{
		OriginalName: "demo.wav",
		MIMEType:     "audio/wav",
		SizeBytes:    int64(len(testWAVHeaderBytes())),
		Content:      bytes.NewReader(testWAVHeaderBytes()),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if len(jobRepo.created) != 2 {
		t.Fatalf("created jobs = %d, want 2", len(jobRepo.created))
	}
	if jobRepo.created[0].Type != job.TypeUpload {
		t.Fatalf("job[0] type = %q, want upload", jobRepo.created[0].Type)
	}
	if jobRepo.created[1].Type != job.TypeExtractAudio {
		t.Fatalf("job[1] type = %q, want extract_audio", jobRepo.created[1].Type)
	}
}

type stubUploadMediaRepository struct {
	createdID int64
	created   []media.Media
}

func (s *stubUploadMediaRepository) Create(_ context.Context, item media.Media) (int64, error) {
	s.created = append(s.created, item)
	return s.createdID, nil
}

func (s *stubUploadMediaRepository) Delete(context.Context, int64) error {
	return nil
}

func (s *stubUploadMediaRepository) ListRecent(context.Context, int) ([]media.Media, error) {
	return nil, nil
}

type stubUploadJobRepository struct {
	created []job.Job
}

func (s *stubUploadJobRepository) Create(_ context.Context, item job.Job) (int64, error) {
	s.created = append(s.created, item)
	return int64(len(s.created)), nil
}

type stubUploadStorage struct {
	stored ports.StoredFile
}

func (s stubUploadStorage) Save(context.Context, string, io.Reader) (ports.StoredFile, error) {
	return s.stored, nil
}

func (s stubUploadStorage) Delete(context.Context, string) error {
	return nil
}

func testMP4HeaderBytes() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0x00, 0x00, 0x02, 0x00,
		'i', 's', 'o', 'm',
		'i', 's', 'o', '2',
	}
}

func testWAVHeaderBytes() []byte {
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
