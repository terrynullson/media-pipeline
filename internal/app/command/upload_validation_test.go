package command

import (
	"strings"
	"testing"
)

func TestValidateUploadInput_AllowsMediaContent(t *testing.T) {
	t.Parallel()

	input := UploadMediaInput{
		OriginalName: "clip.wav",
		SizeBytes:    int64(len(wavSampleBytes())),
		Content:      strings.NewReader(string(wavSampleBytes())),
	}

	ext, _, detectedType, err := validateUploadInput(input, 10*1024*1024)
	if err != nil {
		t.Fatalf("validateUploadInput() error = %v", err)
	}
	if ext != ".wav" {
		t.Fatalf("validateUploadInput() ext = %q, want %q", ext, ".wav")
	}
	if detectedType != "audio/wave" && detectedType != "audio/wav" {
		t.Fatalf("validateUploadInput() detectedType = %q, want wave mime", detectedType)
	}
}

func TestValidateUploadInput_RejectsUnsupportedContent(t *testing.T) {
	t.Parallel()

	input := UploadMediaInput{
		OriginalName: "fake.mp4",
		SizeBytes:    int64(len("plain text")),
		Content:      strings.NewReader("plain text"),
	}

	_, _, _, err := validateUploadInput(input, 10*1024*1024)
	if err == nil {
		t.Fatal("validateUploadInput() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "content type is not supported") {
		t.Fatalf("validateUploadInput() error = %v, want content type error", err)
	}
}

func TestValidateUploadInput_RejectsUnsupportedExtension(t *testing.T) {
	t.Parallel()

	input := UploadMediaInput{
		OriginalName: "notes.txt",
		SizeBytes:    int64(len(wavSampleBytes())),
		Content:      strings.NewReader(string(wavSampleBytes())),
	}

	_, _, _, err := validateUploadInput(input, 10*1024*1024)
	if err == nil {
		t.Fatal("validateUploadInput() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Fatalf("validateUploadInput() error = %v, want unsupported extension error", err)
	}
}

func wavSampleBytes() []byte {
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
