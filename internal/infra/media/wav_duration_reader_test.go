package media

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWAVDurationReader_ReadDuration(t *testing.T) {
	t.Parallel()

	audioPath := filepath.Join(t.TempDir(), "sample.wav")
	if err := os.WriteFile(audioPath, wavWithSilence(3*time.Second), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	reader := NewWAVDurationReader()
	duration, err := reader.ReadDuration(audioPath)
	if err != nil {
		t.Fatalf("ReadDuration() error = %v", err)
	}
	if duration != 3*time.Second {
		t.Fatalf("duration = %s, want 3s", duration)
	}
}

func wavWithSilence(duration time.Duration) []byte {
	dataSize := uint32(duration.Seconds() * 32000)
	fileSize := dataSize + 36

	return []byte{
		'R', 'I', 'F', 'F',
		byte(fileSize), byte(fileSize >> 8), byte(fileSize >> 16), byte(fileSize >> 24),
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		0x10, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00,
		0x80, 0x3e, 0x00, 0x00,
		0x00, 0x7d, 0x00, 0x00,
		0x02, 0x00, 0x10, 0x00,
		'd', 'a', 't', 'a',
		byte(dataSize), byte(dataSize >> 8), byte(dataSize >> 16), byte(dataSize >> 24),
	}
}
